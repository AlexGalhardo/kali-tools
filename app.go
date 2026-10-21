//go:build desktop || bindings

package main

import (
	"context"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/platform"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/tools"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

type App struct {
	initMu  sync.RWMutex
	closed  bool
	ctx     context.Context
	cancel  context.CancelFunc
	service *app.Service
	config  bootstrap.Config
	cleanup func() error
	initErr error
}

func NewApp() *App { return &App{} }
func (a *App) startup(ctx context.Context) {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.closed {
		return
	}
	a.ctx, a.cancel = context.WithCancel(ctx)
	application, cleanup, err := bootstrap.New(a.ctx, a.config)
	if err != nil {
		a.initErr = err
		return
	}
	a.service = application.Service
	a.cleanup = cleanup
	events, _ := a.service.Jobs.Subscribe(a.ctx)
	go func() {
		for event := range events {
			name, payload := wailsEvent(event)
			wruntime.EventsEmit(a.ctx, name, payload)
		}
	}()
	a.service.StartMetrics()
}

type jobEventPayload struct {
	SchemaVersion int         `json:"schema_version"`
	Seq           uint64      `json:"seq"`
	Job           *job.Status `json:"job"`
	Result        any         `json:"result,omitempty"`
}

func wailsEvent(event job.Event) (string, any) {
	if event.Topic == job.TopicJob {
		return job.TopicJob, jobEventPayload{SchemaVersion: event.SchemaVersion, Seq: event.Seq, Job: event.Job, Result: event.Data}
	}
	return event.Topic, event.Data
}
func (a *App) shutdown(context.Context) {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.closed {
		return
	}
	a.closed = true
	if a.cancel != nil {
		a.cancel()
	}
	if a.cleanup != nil {
		_ = a.cleanup()
	}
}
func (a *App) ready() error {
	a.initMu.RLock()
	defer a.initMu.RUnlock()
	return a.readyLocked()
}
func (a *App) readyLocked() error {
	if a.closed {
		return domain.Invalid("application is shutting down")
	}
	if a.initErr != nil {
		return a.initErr
	}
	if a.service == nil {
		return domain.Invalid("application is starting")
	}
	return nil
}
func (a *App) Version() string                              { return domain.Version }
func (a *App) Capabilities() []domain.Capability            { return app.Capabilities() }
func (a *App) Interfaces() ([]network.InterfaceInfo, error) { return network.Interfaces() }
func (a *App) Inventory() (platform.Snapshot, error)        { return platform.Inventory() }
func (a *App) Tools() ([]tools.Tool, error) {
	if e := a.ready(); e != nil {
		return nil, e
	}
	return tools.Inventory(a.ctx), nil
}
func (a *App) Diagnose(r network.Request) (network.Result, error) {
	if e := a.ready(); e != nil {
		return network.Result{}, e
	}
	return a.service.Diagnostics.Diagnose(a.ctx, r)
}
func (a *App) StartDiagnostic(r network.Request) (job.Status, error) {
	if e := a.ready(); e != nil {
		return job.Status{}, e
	}
	return a.service.Start(a.ctx, r)
}
func (a *App) CancelJob(id string) bool {
	if a.ready() != nil {
		return false
	}
	return a.service.Cancel(id)
}
func (a *App) History() ([]job.Status, error) {
	if e := a.ready(); e != nil {
		return nil, e
	}
	return a.service.History(a.ctx)
}
func (a *App) ClearHistory() error {
	if e := a.ready(); e != nil {
		return e
	}
	return a.service.ClearHistory(a.ctx)
}
func (a *App) Settings() (app.Settings, error) {
	if e := a.ready(); e != nil {
		return app.Settings{}, e
	}
	return a.service.Settings(a.ctx)
}
func (a *App) SaveSettings(v app.Settings) error {
	if e := a.ready(); e != nil {
		return e
	}
	return a.service.SaveSettings(a.ctx, v)
}
func (a *App) CurrentWiFi() (wifi.Network, error) { return wifi.Current() }
func (a *App) RevealWiFi() (wifi.Secret, error)   { return wifi.Reveal() }
func (a *App) Discover(cidr string, ports []int) ([]network.PortResult, error) {
	if e := a.ready(); e != nil {
		return nil, e
	}
	return a.service.Discover(a.ctx, cidr, ports, 500*time.Millisecond)
}
func (a *App) ImportCapture() (capture.ImportResult, error) {
	if err := a.ready(); err != nil {
		return capture.ImportResult{}, err
	}
	if err := a.service.CanImportCapture(); err != nil {
		return capture.ImportResult{}, err
	}
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Import capture", Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*.pcap;*.pcapng"}}})
	if err != nil {
		return capture.ImportResult{}, err
	}
	if path == "" {
		return capture.ImportResult{Packets: []capture.Packet{}}, nil
	}
	return a.service.ImportCapture(path)
}

func (a *App) StartCapture(request capture.LiveRequest) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.service.StartCapture(request)
}

func (a *App) StopCapture() {
	if a.ready() == nil {
		a.service.StopCapture()
	}
}

func (a *App) SaveCapture(format string) (bool, error) {
	if err := a.ready(); err != nil {
		return false, err
	}
	if err := a.service.CanExportCapture(format); err != nil {
		return false, err
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: "Save capture", DefaultFilename: "capture." + format, Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*." + format}}})
	if err != nil || path == "" {
		return false, err
	}
	if err := a.service.ExportCapture(a.ctx, path, format); err != nil {
		return false, err
	}
	return true, nil
}
func (a *App) InspectServices(cidr string, ports []int) (tools.Nmap, error) {
	if e := a.ready(); e != nil {
		return tools.Nmap{}, e
	}
	return a.service.InspectServices(a.ctx, cidr, ports)
}
