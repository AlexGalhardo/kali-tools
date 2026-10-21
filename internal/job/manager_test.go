package job_test

import (
	"context"
	"errors"
	"regexp"
	"runtime"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/job"
)

func wait(t *testing.T, m *job.Manager, id job.ID) (job.Status, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	return m.Wait(ctx, id)
}

func start(t *testing.T, m *job.Manager, run func(context.Context) (string, any, error)) job.Status {
	t.Helper()
	status, err := m.Start("tcp", run)
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func complete(context.Context) (string, any, error) { return "", "result", nil }
func blocked(ctx context.Context) (string, any, error) {
	<-ctx.Done()
	return "", nil, ctx.Err()
}

func TestLifecycleEventsAndPersistence(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	m := job.NewManager(func() time.Time { return now }, job.Limits{})
	defer m.Close()
	var states []job.State
	m.OnChange = func(st job.Status) error { states = append(states, st.State); return nil }
	events, _ := m.Subscribe(t.Context())
	initial := start(t, m, complete)
	final, err := wait(t, m, initial.ID)
	if err != nil || final.State != job.Completed || final.Finished == nil || !final.Created.Equal(now) {
		t.Fatalf("terminal = %+v, err = %v", final, err)
	}
	want := []job.State{job.Queued, job.Running, job.Completed}
	if !slices.Equal(states, want) {
		t.Fatalf("persisted = %v, want %v", states, want)
	}
	for i, state := range want {
		ev := <-events
		if ev.Seq != uint64(i+1) || ev.Topic != job.TopicJob || ev.Job == nil || ev.Job.State != state || ev.SchemaVersion != 1 || !ev.At.Equal(now) {
			t.Fatalf("event = %+v, want state %s, seq %d", ev, state, i+1)
		}
		if state == job.Completed && ev.Data != "result" {
			t.Fatalf("result = %v", ev.Data)
		}
	}
	got, ok := m.Snapshot(initial.ID)
	if !ok || got.Finished == nil || got.State != job.Completed {
		t.Fatalf("snapshot = %+v, found = %v", got, ok)
	}
	if m.Cancel(initial.ID) {
		t.Fatal("cancelled an already finished job")
	}
}

func TestFailedAndCancelledStates(t *testing.T) {
	m := job.NewManager(nil, job.Limits{})
	defer m.Close()
	failure := errors.New("synthetic failure")
	failed := start(t, m, func(context.Context) (string, any, error) { return "", nil, failure })
	if st, err := wait(t, m, failed.ID); st.State != job.Failed || !errors.Is(err, failure) {
		t.Fatalf("status = %+v, err = %v", st, err)
	}
	cancelled := start(t, m, blocked)
	if !m.Cancel(cancelled.ID) || m.Cancel("unknown") {
		t.Fatal("incorrect cancellation result")
	}
	if st, err := wait(t, m, cancelled.ID); st.State != job.Cancelled || !errors.Is(err, context.Canceled) {
		t.Fatalf("status = %+v, err = %v", st, err)
	}
	if _, err := m.Wait(t.Context(), "unknown"); !errors.Is(err, job.ErrNotFound) {
		t.Fatal(err)
	}
	if _, ok := m.Snapshot("unknown"); ok {
		t.Fatal("found unknown job")
	}
}

func TestConcurrencyLimitBoundary(t *testing.T) {
	m := job.NewManager(nil, job.Limits{})
	defer m.Close()
	var first job.ID
	for range job.DefaultMaxConcurrent {
		first = start(t, m, blocked).ID
	}
	if _, err := m.Start("tcp", blocked); !errors.Is(err, job.ErrBusy) {
		t.Fatalf("ninth job: %v", err)
	}
	m.Cancel(first)
	_, _ = wait(t, m, first)
	start(t, m, blocked)
}

func TestSlowSubscriberDropsWithoutBlocking(t *testing.T) {
	m := job.NewManager(nil, job.Limits{SubscriberBuffer: 1})
	defer m.Close()
	_, drops := m.Subscribe(t.Context())
	fast, _ := m.Subscribe(t.Context())
	for i := range 10 {
		m.Publish(job.TopicMetrics, i)
		ev := <-fast
		if ev.Seq != uint64(i+1) || ev.Data != i {
			t.Fatalf("event = %+v", ev)
		}
	}
	if drops() != 9 {
		t.Fatalf("drops = %d, want 9", drops())
	}
}

func TestTerminalEventWaitsForSubscriber(t *testing.T) {
	m := job.NewManager(nil, job.Limits{SubscriberBuffer: 1})
	defer m.Close()
	events, _ := m.Subscribe(t.Context())
	m.Publish(job.TopicMetrics, nil)
	st := start(t, m, complete)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	for {
		select {
		case ev := <-events:
			if ev.Job != nil && ev.Job.ID == st.ID && ev.Job.State.Terminal() {
				return
			}
		case <-ctx.Done():
			t.Fatal("terminal event lost")
		}
	}
}

func TestTerminalWaitIsBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := job.NewManager(nil, job.Limits{SubscriberBuffer: 1})
		defer m.Close()
		for range 2 {
			m.Subscribe(t.Context())
		}
		m.Publish(job.TopicMetrics, nil)
		before := time.Now()
		st := start(t, m, complete)
		if _, err := wait(t, m, st.ID); err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(before); elapsed != job.TerminalWait || elapsed >= 3*time.Second {
			t.Fatalf("terminal wait = %s, want %s", elapsed, job.TerminalWait)
		}
		before = time.Now()
		m.Publish(job.TopicCapture, nil)
		if elapsed := time.Since(before); elapsed != job.TerminalWait {
			t.Fatalf("capture wait = %s", elapsed)
		}
	})
}

func TestOnChangeFailures(t *testing.T) {
	for _, failureState := range []job.State{job.Queued, job.Running, job.Completed} {
		t.Run(string(failureState), func(t *testing.T) {
			m := job.NewManager(nil, job.Limits{})
			defer m.Close()
			failure := errors.New("synthetic persistence failure")
			m.OnChange = func(st job.Status) error {
				if st.State == failureState {
					return failure
				}
				return nil
			}
			called := false
			st, err := m.Start("tcp", func(context.Context) (string, any, error) { called = true; return "", nil, nil })
			if failureState == job.Queued {
				if !errors.Is(err, failure) || st.ID != "" {
					t.Fatalf("start = %+v, err = %v", st, err)
				}
				m.OnChange = nil
				for range job.DefaultMaxConcurrent {
					start(t, m, blocked)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			terminal, err := wait(t, m, st.ID)
			if terminal.State != job.Failed || !errors.Is(err, failure) {
				t.Fatalf("terminal = %+v, err = %v", terminal, err)
			}
			if failureState == job.Running && called {
				t.Fatal("run called after failed running write")
			}
			if failureState == job.Completed && terminal.Summary != "History write failed" {
				t.Fatal(terminal.Summary)
			}
		})
	}
}

func TestCloseCancelsJobsAndClosesSubscribers(t *testing.T) {
	baseline := runtime.NumGoroutine()
	m := job.NewManager(nil, job.Limits{})
	events, _ := m.Subscribe(context.Background())
	st := start(t, m, blocked)
	m.Close()
	m.Close()
	for range events {
	}
	if terminal, err := wait(t, m, st.ID); terminal.State != job.Cancelled || !errors.Is(err, context.Canceled) {
		t.Fatalf("terminal = %+v, err = %v", terminal, err)
	}
	if _, err := m.Start("tcp", complete); !errors.Is(err, job.ErrClosed) {
		t.Fatal(err)
	}
	closed, drops := m.Subscribe(t.Context())
	if _, ok := <-closed; ok || drops() != 0 {
		t.Fatal("closed manager accepted subscription")
	}
	m.Publish(job.TopicCapture, nil)
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > baseline && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if count := runtime.NumGoroutine(); count > baseline {
		t.Fatalf("goroutines = %d, baseline = %d", count, baseline)
	}
}

func TestJobIDsAndSnapshotRetention(t *testing.T) {
	m := job.NewManager(nil, job.Limits{})
	defer m.Close()
	ids := make(map[job.ID]bool)
	pattern := regexp.MustCompile(`^[0-9a-f]{32}$`)
	var first job.ID
	for i := range job.MaxFinished + 1 {
		st := start(t, m, complete)
		if i == 0 {
			first = st.ID
		}
		if !pattern.MatchString(string(st.ID)) || ids[st.ID] {
			t.Fatalf("invalid/duplicate ID %q", st.ID)
		}
		ids[st.ID] = true
		if _, err := wait(t, m, st.ID); err != nil {
			t.Fatal(err)
		}
		if snap, ok := m.Snapshot(st.ID); !ok || snap.ID != st.ID {
			t.Fatal("missing snapshot")
		}
		if i == job.MaxFinished-1 {
			if _, ok := m.Snapshot(first); !ok {
				t.Fatal("early eviction")
			}
		}
	}
	if _, ok := m.Snapshot(first); ok {
		t.Fatal("finished history exceeds bound")
	}
}

func TestPublishIgnoresJobAndUnknownTopics(t *testing.T) {
	m := job.NewManager(nil, job.Limits{})
	defer m.Close()
	events, _ := m.Subscribe(t.Context())
	m.Publish(job.TopicJob, nil)
	m.Publish("unknown", nil)
	select {
	case ev := <-events:
		t.Fatalf("unexpected event %+v", ev)
	default:
	}
}

func TestLimitsDefaultsAndClamp(t *testing.T) {
	for _, test := range []struct {
		name     string
		in, want job.Limits
	}{
		{"default", job.Limits{}, job.Limits{MaxConcurrent: 8, SubscriberBuffer: 256}},
		{"at limit", job.Limits{MaxConcurrent: 64, SubscriberBuffer: 4096}, job.Limits{MaxConcurrent: 64, SubscriberBuffer: 4096}},
		{"limit plus one", job.Limits{MaxConcurrent: 65, SubscriberBuffer: 4097}, job.Limits{MaxConcurrent: 64, SubscriberBuffer: 4096}},
		{"large", job.Limits{MaxConcurrent: 1000, SubscriberBuffer: 1 << 20}, job.Limits{MaxConcurrent: 64, SubscriberBuffer: 4096}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := job.NewManager(nil, test.in)
			defer m.Close()
			if got := m.LimitsForTest(); got != test.want {
				t.Fatalf("limits = %+v, want %+v", got, test.want)
			}
			events, _ := m.Subscribe(t.Context())
			if got := cap(events); got != test.want.SubscriberBuffer {
				t.Fatalf("subscriber capacity = %d, want %d", got, test.want.SubscriberBuffer)
			}
		})
	}
}

func TestWaitCancellationAndInvalidStart(t *testing.T) {
	m := job.NewManager(nil, job.Limits{})
	defer m.Close()
	if _, err := m.Start("", complete); err == nil {
		t.Fatal("empty kind accepted")
	}
	if _, err := m.Start("tcp", nil); err == nil {
		t.Fatal("nil run accepted")
	}
	st := start(t, m, blocked)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := m.Wait(ctx, st.ID); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if snap, ok := m.Snapshot(st.ID); !ok || snap.State.Terminal() {
		t.Fatal("wait cancelled job")
	}
	if !m.Cancel(st.ID) {
		t.Fatal("job not active")
	}
}

func TestConcurrentPublishAndUnsubscribe(t *testing.T) {
	m := job.NewManager(nil, job.Limits{})
	defer m.Close()
	var wg sync.WaitGroup
	for range 32 {
		ctx, cancel := context.WithCancel(t.Context())
		events, _ := m.Subscribe(ctx)
		wg.Go(func() {
			for range events {
			}
		})
		wg.Go(func() { m.Publish(job.TopicMetrics, nil); cancel() })
	}
	wg.Wait()
}

func TestSnapshotDoesNotAliasFinishedTime(t *testing.T) {
	m := job.NewManager(nil, job.Limits{})
	defer m.Close()
	st := start(t, m, complete)
	finished, err := wait(t, m, st.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := *finished.Finished
	*finished.Finished = time.Time{}
	snapshot, ok := m.Snapshot(st.ID)
	if !ok || !snapshot.Finished.Equal(want) {
		t.Fatal("Wait exposed manager state")
	}
	*snapshot.Finished = time.Time{}
	again, _ := m.Snapshot(st.ID)
	if !again.Finished.Equal(want) {
		t.Fatal("Snapshot exposed manager state")
	}
}
