package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const persistTimeout = 5 * time.Second

type Service struct {
	Diagnostics    network.Diagnostics
	Jobs           *job.Manager
	deps           Deps
	mu             sync.Mutex
	closed         bool
	done           chan struct{}
	closeDone      chan struct{}
	bg             sync.WaitGroup
	metricsStarted bool
	capMu          sync.Mutex
	capCancel      context.CancelFunc
	capArtifact    *capture.Artifact
}

func New(deps Deps) *Service {
	if deps.Jobs == nil {
		panic("app: job store is required")
	}
	if deps.Settings == nil {
		panic("app: settings store is required")
	}
	s := &Service{
		deps: deps, Diagnostics: deps.Diagnostics, Jobs: job.NewManager(deps.Clock, job.Limits{}),
		done: make(chan struct{}), closeDone: make(chan struct{}),
	}
	s.Jobs.Summarize = jobSummary
	s.Jobs.OnChange = func(st job.Status) error {
		ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
		defer cancel()
		return deps.Jobs.PutJob(ctx, st)
	}
	return s
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

func (s *Service) Start(parent context.Context, request network.Request) (job.Status, error) {
	if err := network.Validate(request); err != nil {
		return job.Status{}, err
	}
	st, err := s.Jobs.Start(request.Kind, func(ctx context.Context) (string, any, error) {
		result, err := s.Diagnostics.Diagnose(ctx, request)
		return "", &result, err
	})
	return st, err
}

func (s *Service) Cancel(id string) bool { return s.Jobs.Cancel(job.ID(id)) }

// Close stops producers and jobs. The composition root owns adapter cleanup.
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		done := s.closeDone
		s.mu.Unlock()
		<-done
		return nil
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
	close(s.closeDone)
	return nil
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

func (s *Service) InspectServices(parent context.Context, cidr string, ports []int) (result Nmap, err error) {
	if err = network.ValidateDiscovery(cidr, ports, time.Second); err != nil {
		return result, err
	}
	if s.deps.Tools == nil {
		return result, domain.Unsupported("tool runner is not configured")
	}
	err = s.runOperation(parent, "nmap", func(ctx context.Context) (string, error) {
		var err error
		result, err = s.deps.Tools.RunNmap(ctx, cidr, ports)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Nmap %s: %d hosts", result.Version, len(result.Hosts)), nil
	})
	return result, err
}

func (s *Service) History(ctx context.Context) ([]job.Status, error) { return s.deps.Jobs.History(ctx) }
func (s *Service) ClearHistory(ctx context.Context) error            { return s.deps.Jobs.Clear(ctx) }
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	return s.deps.Settings.Settings(ctx)
}
func (s *Service) SaveSettings(ctx context.Context, value Settings) error {
	return s.deps.Settings.SaveSettings(ctx, value)
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
