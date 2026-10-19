package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/AlexGalhardo/hacker-tools/internal/domain"
	"github.com/AlexGalhardo/hacker-tools/internal/network"
	"github.com/AlexGalhardo/hacker-tools/internal/storage"
)

type Event struct {
	SchemaVersion int             `json:"schema_version"`
	Job           storage.Job     `json:"job"`
	Result        *network.Result `json:"result,omitempty"`
}
type Clock interface{ Now() time.Time }
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

type Service struct {
	Store  *storage.Store
	Clock  Clock
	Events chan Event
	mu     sync.Mutex
	jobs   map[string]context.CancelFunc
	wg     sync.WaitGroup
	closed bool
}

func New(s *storage.Store) *Service {
	return &Service{Store: s, Clock: SystemClock{}, Events: make(chan Event, 128), jobs: map[string]context.CancelFunc{}}
}
func (s *Service) Interfaces() ([]network.InterfaceInfo, error) { return network.Interfaces() }
func (s *Service) emit(j storage.Job) {
	select {
	case s.Events <- Event{SchemaVersion: domain.SchemaVersion, Job: j}:
	default:
	}
}
func (s *Service) Start(parent context.Context, r network.Request) (storage.Job, error) {
	if e := network.Validate(r); e != nil {
		return storage.Job{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return storage.Job{}, domain.Invalid("service is shutting down")
	}
	if len(s.jobs) >= 8 {
		return storage.Job{}, domain.Invalid("maximum 8 concurrent diagnostics")
	}
	var id [16]byte
	if _, e := rand.Read(id[:]); e != nil {
		return storage.Job{}, e
	}
	j := storage.Job{ID: hex.EncodeToString(id[:]), Kind: r.Kind, State: "queued", Created: s.Clock.Now(), Summary: "Diagnostic queued"}
	if e := s.Store.PutJob(parent, j); e != nil {
		return j, e
	}
	ctx, cancel := context.WithCancel(parent)
	s.jobs[j.ID] = cancel
	s.emit(j)
	s.wg.Add(1)
	initial := j
	go func() {
		var result *network.Result
		defer s.wg.Done()
		defer cancel()
		defer func() { s.mu.Lock(); delete(s.jobs, j.ID); s.mu.Unlock() }()
		j.State = "running"
		if e := s.Store.PutJob(ctx, j); e != nil {
			j.State = "failed"
			if errors.Is(ctx.Err(), context.Canceled) {
				j.State = "cancelled"
			}
		} else {
			s.emit(j)
			value, diagnosticErr := network.Diagnose(ctx, r)
			e = diagnosticErr
			result = &value
			j.State = "completed"
			if e != nil {
				j.State = "failed"
			}
			if errors.Is(e, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				j.State = "cancelled"
			}
		}
		end := s.Clock.Now()
		j.Finished = &end
		j.Summary = "Diagnostic " + j.State // Never store targets, URLs, errors or secrets.
		persist, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if e := s.Store.PutJob(persist, j); e != nil {
			j.State = "failed"
			j.Summary = "History write failed"
		}
		select {
		case s.Events <- Event{SchemaVersion: domain.SchemaVersion, Job: j, Result: result}:
		default:
		}
	}()
	return initial, nil
}
func (s *Service) Cancel(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.jobs[id]
	if ok {
		c()
	}
	return ok
}
func (s *Service) Close() error {
	s.mu.Lock()
	s.closed = true
	for _, cancel := range s.jobs {
		cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
	return s.Store.Close()
}
