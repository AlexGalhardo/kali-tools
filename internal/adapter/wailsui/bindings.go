package wailsui

import (
	"context"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

const (
	desktopDiscoveryTimeout = 500 * time.Millisecond
	shutdownWait            = 3 * time.Second
)

type Runtime struct {
	Emit     func(context.Context, string, ...any)
	OpenFile func(context.Context, wruntime.OpenDialogOptions) (string, error)
	SaveFile func(context.Context, wruntime.SaveDialogOptions) (string, error)
}
type Bindings struct {
	app    *app.Application
	emit   func(context.Context, string, ...any)
	open   func(context.Context, wruntime.OpenDialogOptions) (string, error)
	save   func(context.Context, wruntime.SaveDialogOptions) (string, error)
	mu     sync.RWMutex
	ctx    context.Context
	closed bool
	stop   context.CancelFunc
	done   chan struct{}
}

func New(a *app.Application) *Bindings {
	return NewWithRuntime(a, Runtime{Emit: wruntime.EventsEmit, OpenFile: wruntime.OpenFileDialog, SaveFile: wruntime.SaveFileDialog})
}
func NewWithRuntime(a *app.Application, rt Runtime) *Bindings {
	return &Bindings{app: a, emit: rt.Emit, open: rt.OpenFile, save: rt.SaveFile}
}
func (b *Bindings) Startup(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.ctx != nil {
		return
	}
	b.ctx = ctx
	fctx, stop := context.WithCancel(ctx)
	b.stop = stop
	events, _ := b.app.Subscribe(fctx)
	b.done = make(chan struct{})
	go b.forward(ctx, fctx, events)
	b.app.StartMetrics(ctx)
}
func (b *Bindings) Shutdown(ctx context.Context) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	if b.stop != nil {
		b.stop()
	}
	done := b.done
	b.mu.Unlock()
	if done == nil {
		return
	}
	timer := time.NewTimer(shutdownWait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}
func (b *Bindings) ready() (context.Context, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, domain.Invalid("application is shutting down")
	}
	if b.ctx == nil {
		return nil, domain.Invalid("application is starting")
	}
	return b.ctx, nil
}
func (b *Bindings) Version() string { return domain.Version }
func (b *Bindings) Capabilities() []domain.Capability {
	ctx, err := b.ready()
	if err != nil {
		return nil
	}
	return b.app.Capabilities(ctx)
}
func (b *Bindings) Interfaces() ([]network.InterfaceInfo, error) {
	ctx, err := b.ready()
	if err != nil {
		return nil, err
	}
	return b.app.Interfaces(ctx)
}
func (b *Bindings) Inventory() (app.HostSnapshot, error) {
	ctx, err := b.ready()
	if err != nil {
		return app.HostSnapshot{}, err
	}
	return b.app.Inventory(ctx)
}
func (b *Bindings) Tools() ([]app.Tool, error) {
	ctx, e := b.ready()
	if e != nil {
		return nil, e
	}
	return b.app.Tools(ctx)
}
func (b *Bindings) Diagnose(r network.Request) (network.Result, error) {
	ctx, e := b.ready()
	if e != nil {
		return network.Result{}, e
	}
	return b.app.Diagnose(ctx, r)
}
func (b *Bindings) StartDiagnostic(r network.Request) (job.Status, error) {
	ctx, e := b.ready()
	if e != nil {
		return job.Status{}, e
	}
	return b.app.StartDiagnostic(ctx, r)
}
func (b *Bindings) CancelJob(id string) bool {
	ctx, err := b.ready()
	if err != nil {
		return false
	}
	return b.app.CancelJob(ctx, job.ID(id))
}
func (b *Bindings) History() ([]job.Status, error) {
	ctx, e := b.ready()
	if e != nil {
		return nil, e
	}
	return b.app.History(ctx)
}
func (b *Bindings) ClearHistory() error {
	ctx, e := b.ready()
	if e != nil {
		return e
	}
	return b.app.ClearHistory(ctx)
}
func (b *Bindings) Settings() (app.Settings, error) {
	ctx, e := b.ready()
	if e != nil {
		return app.Settings{}, e
	}
	return b.app.Settings(ctx)
}
func (b *Bindings) SaveSettings(v app.Settings) error {
	ctx, e := b.ready()
	if e != nil {
		return e
	}
	return b.app.SaveSettings(ctx, v)
}
func (b *Bindings) CurrentWiFi() (wifi.Network, error) {
	ctx, err := b.ready()
	if err != nil {
		return wifi.Network{}, err
	}
	return b.app.CurrentWiFi(ctx)
}
func (b *Bindings) RevealWiFi() (wifi.Secret, error) {
	ctx, err := b.ready()
	if err != nil {
		return wifi.Secret{}, err
	}
	return b.app.RevealWiFi(ctx)
}
func (b *Bindings) Discover(cidr string, ports []int) ([]network.PortResult, error) {
	ctx, e := b.ready()
	if e != nil {
		return nil, e
	}
	return b.app.Discover(ctx, cidr, ports, desktopDiscoveryTimeout)
}
func (b *Bindings) ImportCapture() (capture.ImportResult, error) {
	ctx, err := b.ready()
	if err != nil {
		return capture.ImportResult{}, err
	}
	if err := b.app.CanImportCapture(ctx); err != nil {
		return capture.ImportResult{}, err
	}
	path, err := b.open(ctx, wruntime.OpenDialogOptions{Title: "Import capture", Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*.pcap;*.pcapng"}}})
	if err != nil {
		return capture.ImportResult{}, err
	}
	if path == "" {
		return capture.ImportResult{Packets: []capture.Packet{}}, nil
	}
	return b.app.ImportCapture(ctx, path)
}

func (b *Bindings) StartCapture(request capture.LiveRequest) error {
	ctx, err := b.ready()
	if err != nil {
		return err
	}
	return b.app.StartCapture(ctx, request)
}

func (b *Bindings) StopCapture() {
	if ctx, err := b.ready(); err == nil {
		b.app.StopCapture(ctx)
	}
}

func (b *Bindings) SaveCapture(format string) (bool, error) {
	ctx, err := b.ready()
	if err != nil {
		return false, err
	}
	if format != "pcap" && format != "pcapng" {
		return false, domain.Invalid("capture format must be pcap or pcapng")
	}
	path, err := b.save(ctx, wruntime.SaveDialogOptions{Title: "Save capture", DefaultFilename: "capture." + format, Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*." + format}}})
	if err != nil || path == "" {
		return false, err
	}
	if err := b.app.ExportCapture(ctx, path, format); err != nil {
		return false, err
	}
	return true, nil
}
func (b *Bindings) InspectServices(cidr string, ports []int) (app.Nmap, error) {
	ctx, e := b.ready()
	if e != nil {
		return app.Nmap{}, e
	}
	return b.app.InspectServices(ctx, cidr, ports)
}
