package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/tools"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const persistTimeout = 5 * time.Second

type Service struct {
	Store          *storage.Store
	Diagnostics    network.Diagnostics
	Jobs           *job.Manager
	Counters       func(context.Context) ([]network.Counter, error)
	Live           func(context.Context, capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error)
	mu             sync.Mutex
	closed         bool
	closeErr       error
	done           chan struct{}
	closeDone      chan struct{}
	bg             sync.WaitGroup
	metricsStarted bool
	capMu          sync.Mutex
	capCancel      context.CancelFunc
	capArtifact    *capture.Artifact
}

func New(store *storage.Store, diagnostics network.Diagnostics, clock func() time.Time) *Service {
	s := &Service{
		Store: store, Diagnostics: diagnostics, Jobs: job.NewManager(clock, job.Limits{}),
		Live: capture.LiveSession, done: make(chan struct{}), closeDone: make(chan struct{}),
	}
	s.Jobs.Summarize = jobSummary
	s.Jobs.OnChange = func(st job.Status) error {
		ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
		defer cancel()
		return s.Store.PutJob(ctx, storedJob(st))
	}
	return s
}

func storedJob(st job.Status) storage.Job {
	return storage.Job{ID: string(st.ID), Kind: st.Kind, State: string(st.State), Created: st.Created, Finished: st.Finished, Summary: st.Summary}
}

func jobSummary(kind string, state job.State, detail string) string {
	label := "Diagnostic"
	switch kind {
	case "discovery":
		label = "Discovery"
	case "nmap":
		label = "Service inspection"
	}
	summary := label + " " + string(state)
	if detail != "" && state.Terminal() {
		summary += ": " + detail
	}
	return summary
}

func (s *Service) Interfaces() ([]network.InterfaceInfo, error) { return network.Interfaces() }

func (s *Service) Start(parent context.Context, request network.Request) (storage.Job, error) {
	if err := network.Validate(request); err != nil {
		return storage.Job{}, err
	}
	st, err := s.Jobs.Start(request.Kind, func(ctx context.Context) (string, any, error) {
		result, err := s.Diagnostics.Diagnose(ctx, request)
		return "", &result, err
	})
	return storedJob(st), err
}

func (s *Service) Cancel(id string) bool { return s.Jobs.Cancel(job.ID(id)) }

// Close cancels producers and jobs before closing storage. Concurrent callers
// wait for the first close so they receive the same result.
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		done := s.closeDone
		s.mu.Unlock()
		<-done
		return s.closeErr
	}
	s.closed = true
	s.mu.Unlock()
	s.StopCapture()
	close(s.done)
	s.bg.Wait()
	s.capMu.Lock()
	s.capArtifact = nil
	s.capMu.Unlock()
	s.Jobs.Close()
	s.closeErr = s.Store.Close()
	close(s.closeDone)
	return s.closeErr
}

// Discover persists counts only; addresses and port results remain in memory.
func (s *Service) Discover(parent context.Context, cidr string, ports []int, timeout time.Duration) (results []network.PortResult, err error) {
	if err = network.ValidateDiscovery(cidr, ports, timeout); err != nil {
		return nil, err
	}
	err = s.runOperation(parent, "discovery", func(ctx context.Context) (string, error) {
		var err error
		results, err = network.Discover(ctx, cidr, ports, timeout)
		open := 0
		for _, result := range results {
			if result.State == "open" {
				open++
			}
		}
		return fmt.Sprintf("%d endpoints checked, %d open", len(results), open), err
	})
	return results, err
}

func (s *Service) InspectServices(parent context.Context, cidr string, ports []int) (result tools.Nmap, err error) {
	if err = network.ValidateDiscovery(cidr, ports, time.Second); err != nil {
		return result, err
	}
	err = s.runOperation(parent, "nmap", func(ctx context.Context) (string, error) {
		var err error
		result, err = tools.RunNmap(ctx, cidr, ports)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Nmap %s: %d hosts", result.Version, len(result.Hosts)), nil
	})
	return result, err
}

func (s *Service) runOperation(parent context.Context, kind string, operation func(context.Context) (string, error)) error {
	st, err := s.Jobs.Start(kind, func(ctx context.Context) (string, any, error) {
		detail, err := operation(ctx)
		return detail, nil, err
	})
	if err != nil {
		return err
	}
	stop := context.AfterFunc(parent, func() { s.Jobs.Cancel(st.ID) })
	defer stop()
	_, err = s.Jobs.Wait(context.Background(), st.ID)
	return err
}
