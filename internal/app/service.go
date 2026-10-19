package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/storage"
	"github.com/AlexGalhardo/kali-tools/internal/tools"
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

// Discover persists counts only; addresses and port results remain in memory.
func (s *Service) Discover(parent context.Context, cidr string, ports []int, timeout time.Duration) (results []network.PortResult, err error) {
	if err = network.ValidateDiscovery(cidr, ports, timeout); err != nil {
		return nil, err
	}
	err = s.runOperation(parent, "discovery", func(ctx context.Context) (string, error) {
		var e error
		results, e = network.Discover(ctx, cidr, ports, timeout)
		open := 0
		for _, r := range results {
			if r.State == "open" {
				open++
			}
		}
		return fmt.Sprintf("%d endpoints checked, %d open", len(results), open), e
	})
	return results, err
}

func (s *Service) InspectServices(parent context.Context, cidr string, ports []int) (result tools.Nmap, err error) {
	if err = network.ValidateDiscovery(cidr, ports, time.Second); err != nil {
		return result, err
	}
	err = s.runOperation(parent, "nmap", func(ctx context.Context) (string, error) {
		var e error
		result, e = tools.RunNmap(ctx, cidr, ports)
		if e != nil {
			return "", e
		}
		return fmt.Sprintf("Nmap %s: %d hosts", result.Version, len(result.Hosts)), nil
	})
	return result, err
}

func (s *Service) runOperation(parent context.Context, kind string, operation func(context.Context) (string, error)) error {
	s.mu.Lock()
	if s.closed || len(s.jobs) >= 8 {
		s.mu.Unlock()
		return domain.Invalid("service unavailable or maximum 8 concurrent operations")
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		s.mu.Unlock()
		return err
	}
	j := storage.Job{ID: hex.EncodeToString(id[:]), Kind: kind, State: "running", Created: s.Clock.Now(), Summary: kind + " running"}
	if err := s.Store.PutJob(parent, j); err != nil {
		s.mu.Unlock()
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	s.jobs[j.ID] = cancel
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()
	defer cancel()
	defer func() { s.mu.Lock(); delete(s.jobs, j.ID); s.mu.Unlock() }()
	s.emit(j)
	summary, err := operation(ctx)
	j.State = "completed"
	if err != nil {
		j.State = "failed"
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		j.State = "cancelled"
	}
	label := "Discovery"
	if kind == "nmap" {
		label = "Service inspection"
	}
	j.Summary = label + " " + j.State
	if summary != "" {
		j.Summary += ": " + summary
	}
	finished := s.Clock.Now()
	j.Finished = &finished
	persist, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if writeErr := s.Store.PutJob(persist, j); writeErr != nil {
		j.State = "failed"
		j.Summary = "History write failed"
		s.emit(j)
		return errors.Join(err, writeErr)
	}
	s.emit(j)
	return err
}
