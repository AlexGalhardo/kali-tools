package app

import (
	"context"
	"maps"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

const DefaultDiscoveryTimeout = 500 * time.Millisecond

type Application struct {
	deps           Deps
	diagnostics    network.Diagnostics
	jobs           *job.Manager
	mu             sync.Mutex
	closed         bool
	done           chan struct{}
	closeDone      chan struct{}
	bg             sync.WaitGroup
	metricsStarted bool
	capMu          sync.Mutex
	capCancel      context.CancelFunc
	capArtifact    *capture.Artifact
	wifiObserved   wifiObservation
}

func New(deps Deps) *Application {
	if deps.Jobs == nil {
		panic("app: job store is required")
	}
	if deps.Settings == nil {
		panic("app: settings store is required")
	}
	deps.CaptureBackends = maps.Clone(deps.CaptureBackends)
	a := &Application{deps: deps, diagnostics: deps.Diagnostics, jobs: job.NewManager(deps.Clock, job.Limits{}), done: make(chan struct{}), closeDone: make(chan struct{})}
	a.jobs.Summarize = jobSummary
	a.jobs.OnChange = func(st job.Status) error {
		ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
		defer cancel()
		return deps.Jobs.PutJob(ctx, st)
	}
	return a
}

func (a *Application) Capabilities(ctx context.Context) []domain.Capability {
	state, detail := a.wifiObserved.snapshot()
	return append(WithObserved(capabilities(), "wifi", state, detail), a.CaptureBackendStatus(ctx)...)
}
func (a *Application) Interfaces(ctx context.Context) ([]network.InterfaceInfo, error) {
	return network.Interfaces()
}
func (a *Application) Inventory(ctx context.Context) (HostSnapshot, error) {
	if a.deps.Host == nil {
		return HostSnapshot{}, domain.Unsupported("host network provider is not configured")
	}
	return a.deps.Host.Inventory(ctx)
}
func (a *Application) Tools(ctx context.Context) ([]Tool, error) {
	if a.deps.Tools == nil {
		return nil, domain.Unsupported("tool runner is not configured")
	}
	return a.deps.Tools.Inventory(ctx), nil
}
func (a *Application) Diagnose(ctx context.Context, request network.Request) (network.Result, error) {
	return a.diagnostics.Diagnose(ctx, request)
}
func (a *Application) JobSnapshot(ctx context.Context, id job.ID) (job.Status, bool) {
	return a.jobs.Snapshot(id)
}
func (a *Application) Subscribe(ctx context.Context) (<-chan job.Event, func() uint64) {
	return a.jobs.Subscribe(ctx)
}
func (a *Application) History(ctx context.Context) ([]job.Status, error) {
	return a.deps.Jobs.History(ctx)
}
func (a *Application) ClearHistory(ctx context.Context) error { return a.deps.Jobs.Clear(ctx) }
func (a *Application) Settings(ctx context.Context) (Settings, error) {
	return a.deps.Settings.Settings(ctx)
}
func (a *Application) SaveSettings(ctx context.Context, value Settings) error {
	return a.deps.Settings.SaveSettings(ctx, value)
}
func (a *Application) CurrentWiFi(ctx context.Context) (wifi.Network, error) {
	if a.deps.WiFi == nil {
		return wifi.Network{}, domain.Unsupported("Wi-Fi provider is not configured")
	}
	value, err := a.deps.WiFi.Current(ctx)
	a.wifiObserved.record(err)
	if err != nil {
		return wifi.Network{}, err
	}
	return value, err
}
func (a *Application) RevealWiFi(ctx context.Context) (wifi.Secret, error) {
	if a.deps.WiFi == nil {
		return wifi.Secret{}, domain.Unsupported("Wi-Fi provider is not configured")
	}
	value, err := a.deps.WiFi.Reveal(ctx)
	a.wifiObserved.record(err)
	if err != nil {
		return wifi.Secret{}, err
	}
	return value, err
}

// Close stops producers and jobs. The composition root owns adapter cleanup.
func (a *Application) Close() error {
	a.mu.Lock()
	if a.closed {
		done := a.closeDone
		a.mu.Unlock()
		<-done
		return nil
	}
	a.closed = true
	a.mu.Unlock()
	a.StopCapture(context.Background())
	close(a.done)
	a.bg.Wait()
	a.capMu.Lock()
	a.capArtifact = nil
	a.capMu.Unlock()
	a.jobs.Close()
	close(a.closeDone)
	return nil
}
