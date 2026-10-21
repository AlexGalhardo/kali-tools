package job

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

type entry struct {
	status Status
	err    error
	cancel context.CancelFunc
	done   chan struct{}
}

type subscriber struct {
	mu     sync.Mutex
	ch     chan Event
	closed bool
	drops  atomic.Uint64
}

type Manager struct {
	// Set hooks before the first Start. Hooks run outside the manager lock.
	OnChange      func(Status) error
	Summarize     func(kind string, state State, detail string) string
	mu            sync.Mutex
	pubMu         sync.Mutex
	clock         func() time.Time
	limits        Limits
	seq           uint64
	active        map[ID]*entry
	finished      map[ID]*entry
	finishedOrder []ID
	subs          map[*subscriber]func()
	wg            sync.WaitGroup
	closed        bool
	closeDone     chan struct{}
}

func NewManager(clock func() time.Time, limits Limits) *Manager {
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	if limits.MaxConcurrent <= 0 {
		limits.MaxConcurrent = DefaultMaxConcurrent
	}
	if limits.SubscriberBuffer <= 0 {
		limits.SubscriberBuffer = DefaultSubscriberBuffer
	}
	limits.MaxConcurrent = min(limits.MaxConcurrent, MaxConcurrentLimit)
	limits.SubscriberBuffer = min(limits.SubscriberBuffer, MaxSubscriberBuffer)
	return &Manager{
		clock: clock, limits: limits, active: make(map[ID]*entry), finished: make(map[ID]*entry),
		subs: make(map[*subscriber]func()), closeDone: make(chan struct{}),
	}
}

func (m *Manager) Start(kind string, run func(context.Context) (string, any, error)) (Status, error) {
	if kind == "" || run == nil {
		return Status{}, domain.Invalid("job kind and run function are required")
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Status{}, ErrClosed
	}
	if len(m.active) >= m.limits.MaxConcurrent {
		m.mu.Unlock()
		return Status{}, ErrBusy
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		m.mu.Unlock()
		return Status{}, err
	}
	st := Status{ID: ID(hex.EncodeToString(raw[:])), Kind: kind, State: Queued, Created: m.clock(), Summary: m.summary(kind, Queued, "")}
	ctx, cancel := context.WithCancel(context.Background())
	e := &entry{status: st, cancel: cancel, done: make(chan struct{})}
	m.active[st.ID] = e
	m.wg.Add(1)
	m.mu.Unlock()
	if err := m.persist(st); err != nil {
		m.mu.Lock()
		delete(m.active, st.ID)
		e.err = err
		close(e.done)
		m.mu.Unlock()
		cancel()
		m.wg.Done()
		return Status{}, err
	}
	m.deliver(TopicJob, &st, nil, false)
	go m.run(ctx, e, run)
	return st, nil
}

func (m *Manager) run(ctx context.Context, e *entry, run func(context.Context) (string, any, error)) {
	defer m.wg.Done()
	defer e.cancel()
	m.mu.Lock()
	st := e.status
	st.State = Running
	st.Summary = m.summary(st.Kind, Running, "")
	e.status = st
	m.mu.Unlock()
	var detail string
	var result any
	runErr := m.persist(st)
	if runErr == nil {
		m.deliver(TopicJob, &st, nil, false)
		detail, result, runErr = run(ctx)
	}
	st.State = stateFor(ctx, runErr)
	finished := m.clock()
	st.Finished = &finished
	st.Summary = m.summary(st.Kind, st.State, detail)
	persistErr := m.persist(st)
	if persistErr != nil {
		st.State = Failed
		st.Summary = "History write failed"
	}
	m.mu.Lock()
	e.status = st
	e.err = errors.Join(runErr, persistErr)
	delete(m.active, st.ID)
	m.finished[st.ID] = e
	m.finishedOrder = append(m.finishedOrder, st.ID)
	if len(m.finishedOrder) > MaxFinished {
		delete(m.finished, m.finishedOrder[0])
		m.finishedOrder = m.finishedOrder[1:]
	}
	m.mu.Unlock()
	m.deliver(TopicJob, &st, result, true)
	close(e.done)
}

func stateFor(ctx context.Context, err error) State {
	switch {
	case errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled):
		return Cancelled
	case err != nil:
		return Failed
	default:
		return Completed
	}
}

func (m *Manager) summary(kind string, state State, detail string) string {
	if m.Summarize != nil {
		return m.Summarize(kind, state, detail)
	}
	return kind + " " + string(state)
}

func (m *Manager) persist(st Status) error {
	if m.OnChange != nil {
		return m.OnChange(copyStatus(st))
	}
	return nil
}

func (m *Manager) Cancel(id ID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.active[id]
	if ok {
		e.cancel()
	}
	return ok
}

func (m *Manager) lookup(id ID) (*entry, bool) {
	e, ok := m.active[id]
	if !ok {
		e, ok = m.finished[id]
	}
	return e, ok
}

func (m *Manager) Snapshot(id ID) (Status, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.lookup(id)
	if !ok {
		return Status{}, false
	}
	return copyStatus(e.status), true
}

func (m *Manager) Wait(ctx context.Context, id ID) (Status, error) {
	m.mu.Lock()
	e, ok := m.lookup(id)
	m.mu.Unlock()
	if !ok {
		return Status{}, ErrNotFound
	}
	select {
	case <-ctx.Done():
		return Status{}, ctx.Err()
	case <-e.done:
		m.mu.Lock()
		defer m.mu.Unlock()
		return copyStatus(e.status), e.err
	}
}

func (m *Manager) Subscribe(ctx context.Context) (<-chan Event, func() uint64) {
	sub := &subscriber{ch: make(chan Event, m.limits.SubscriberBuffer)}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		close(sub.ch)
		return sub.ch, func() uint64 { return 0 }
	}
	unsubscribe := func() {
		sub.mu.Lock()
		defer sub.mu.Unlock()
		if sub.closed {
			return
		}
		sub.closed = true
		close(sub.ch)
		m.mu.Lock()
		delete(m.subs, sub)
		m.mu.Unlock()
	}
	stop := context.AfterFunc(ctx, unsubscribe)
	m.subs[sub] = func() { stop(); unsubscribe() }
	m.mu.Unlock()
	return sub.ch, sub.drops.Load
}

func (m *Manager) Publish(topic string, data any) {
	if topic != TopicMetrics && topic != TopicCapture {
		return
	}
	m.pubMu.Lock()
	defer m.pubMu.Unlock()
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if !closed {
		m.deliverLocked(topic, nil, data, topic == TopicCapture)
	}
}

func (m *Manager) deliver(topic string, st *Status, data any, terminal bool) {
	m.pubMu.Lock()
	defer m.pubMu.Unlock()
	m.deliverLocked(topic, st, data, terminal)
}

func (m *Manager) deliverLocked(topic string, st *Status, data any, terminal bool) {
	m.seq++
	ev := Event{SchemaVersion: domain.SchemaVersion, Seq: m.seq, Topic: topic, Data: data, At: m.clock()}
	m.mu.Lock()
	subs := make([]*subscriber, 0, len(m.subs))
	for sub := range m.subs {
		subs = append(subs, sub)
	}
	m.mu.Unlock()
	var timer *time.Timer
	var deadline <-chan time.Time
	if terminal {
		timer = time.NewTimer(TerminalWait)
		defer timer.Stop()
		deadline = timer.C
	}
	expired := false
	for _, sub := range subs {
		if st != nil {
			status := copyStatus(*st)
			ev.Job = &status
		}
		sub.mu.Lock()
		if !sub.closed {
			select {
			case sub.ch <- ev:
			default:
				if !terminal || expired {
					sub.drops.Add(1)
				} else {
					select {
					case sub.ch <- ev:
					case <-deadline:
						sub.drops.Add(1)
						expired = true
					}
				}
			}
		}
		sub.mu.Unlock()
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		done := m.closeDone
		m.mu.Unlock()
		<-done
		return
	}
	m.closed = true
	for _, e := range m.active {
		e.cancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
	m.mu.Lock()
	unsubscribers := make([]func(), 0, len(m.subs))
	for _, unsubscribe := range m.subs {
		unsubscribers = append(unsubscribers, unsubscribe)
	}
	m.mu.Unlock()
	for _, unsubscribe := range unsubscribers {
		unsubscribe()
	}
	close(m.closeDone)
}

func copyStatus(st Status) Status {
	if st.Finished != nil {
		finished := *st.Finished
		st.Finished = &finished
	}
	return st
}
