//go:build desktop || bindings

package main

import (
	"context"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/AlexGalhardo/kali-tools/internal/app"
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
	app     *app.Application
	boot    func(context.Context) (*app.Application, func() error, error)
	cleanup func() error
	initErr error
}

func (a *App) startup(ctx context.Context) {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.closed {
		return
	}
	a.ctx, a.cancel = context.WithCancel(ctx)
	application, cleanup, err := a.boot(a.ctx)
	if err != nil {
		a.initErr = err
		return
	}
	a.app = application
	a.cleanup = cleanup
	events, _ := a.app.Subscribe(a.ctx)
	go func() {
		for event := range events {
			name, payload := wailsEvent(event)
			wruntime.EventsEmit(a.ctx, name, payload)
		}
	}()
	a.app.StartMetrics(a.ctx)
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
	if a.app == nil {
		return domain.Invalid("application is starting")
	}
	return nil
}
func (a *App) Version() string { return domain.Version }
func (a *App) Capabilities() []domain.Capability {
	if a.ready() != nil {
		return []domain.Capability{}
	}
	return a.app.Capabilities(a.ctx)
}
func (a *App) Interfaces() ([]network.InterfaceInfo, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.app.Interfaces(a.ctx)
}
func (a *App) Inventory() (app.HostSnapshot, error) {
	if err := a.ready(); err != nil {
		return app.HostSnapshot{}, err
	}
	return a.app.Inventory(a.ctx)
}
func (a *App) Tools() ([]app.Tool, error) {
	if e := a.ready(); e != nil {
		return nil, e
	}
	return a.app.Tools(a.ctx)
}
func (a *App) Diagnose(r network.Request) (network.Result, error) {
	if e := a.ready(); e != nil {
		return network.Result{}, e
	}
	return a.app.Diagnose(a.ctx, r)
}
func (a *App) StartDiagnostic(r network.Request) (job.Status, error) {
	if e := a.ready(); e != nil {
		return job.Status{}, e
	}
	return a.app.StartDiagnostic(a.ctx, r)
}
func (a *App) CancelJob(id string) bool {
	if a.ready() != nil {
		return false
	}
	return a.app.CancelJob(a.ctx, job.ID(id))
}
func (a *App) History() ([]job.Status, error) {
	if e := a.ready(); e != nil {
		return nil, e
	}
	return a.app.History(a.ctx)
}
func (a *App) ClearHistory() error {
	if e := a.ready(); e != nil {
		return e
	}
	return a.app.ClearHistory(a.ctx)
}
func (a *App) Settings() (app.Settings, error) {
	if e := a.ready(); e != nil {
		return app.Settings{}, e
	}
	return a.app.Settings(a.ctx)
}
func (a *App) SaveSettings(v app.Settings) error {
	if e := a.ready(); e != nil {
		return e
	}
	return a.app.SaveSettings(a.ctx, v)
}
func (a *App) CurrentWiFi() (wifi.Network, error) {
	if err := a.ready(); err != nil {
		return wifi.Network{}, err
	}
	return a.app.CurrentWiFi(a.ctx)
}
func (a *App) RevealWiFi() (wifi.Secret, error) {
	if err := a.ready(); err != nil {
		return wifi.Secret{}, err
	}
	return a.app.RevealWiFi(a.ctx)
}
func (a *App) Discover(cidr string, ports []int) ([]network.PortResult, error) {
	if e := a.ready(); e != nil {
		return nil, e
	}
	return a.app.Discover(a.ctx, cidr, ports, app.DefaultDiscoveryTimeout)
}
func (a *App) ImportCapture() (capture.ImportResult, error) {
	if err := a.ready(); err != nil {
		return capture.ImportResult{}, err
	}
	if err := a.app.CanImportCapture(a.ctx); err != nil {
		return capture.ImportResult{}, err
	}
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Import capture", Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*.pcap;*.pcapng"}}})
	if err != nil {
		return capture.ImportResult{}, err
	}
	if path == "" {
		return capture.ImportResult{Packets: []capture.Packet{}}, nil
	}
	return a.app.ImportCapture(a.ctx, path)
}

func (a *App) StartCapture(request capture.LiveRequest) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.app.StartCapture(a.ctx, request)
}

func (a *App) StopCapture() {
	if a.ready() == nil {
		a.app.StopCapture(a.ctx)
	}
}

func (a *App) SaveCapture(format string) (bool, error) {
	if err := a.ready(); err != nil {
		return false, err
	}
	if err := a.app.CanExportCapture(a.ctx, format); err != nil {
		return false, err
	}
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: "Save capture", DefaultFilename: "capture." + format, Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*." + format}}})
	if err != nil || path == "" {
		return false, err
	}
	if err := a.app.ExportCapture(a.ctx, path, format); err != nil {
		return false, err
	}
	return true, nil
}
func (a *App) InspectServices(cidr string, ports []int) (app.Nmap, error) {
	if e := a.ready(); e != nil {
		return app.Nmap{}, e
	}
	return a.app.InspectServices(a.ctx, cidr, ports)
}
