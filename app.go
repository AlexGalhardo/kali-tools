//go:build desktop || bindings

package main

import (
	"context"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/platform"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/tools"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

type App struct {
	initMu          sync.RWMutex
	closed          bool
	ctx             context.Context
	cancel          context.CancelFunc
	service         *app.Service
	initErr         error
	captureMu       sync.Mutex
	captureCancel   context.CancelFunc
	captureWG       sync.WaitGroup
	captureArtifact *capture.Artifact
}

func NewApp() *App { return &App{} }
func (a *App) startup(ctx context.Context) {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.closed {
		return
	}
	a.ctx, a.cancel = context.WithCancel(ctx)
	path, e := storage.DefaultPath()
	if e != nil {
		a.initErr = e
		return
	}
	store, e := storage.Open(path)
	if e != nil {
		a.initErr = e
		return
	}
	a.service = app.New(store)
	settings, e := store.Settings(a.ctx)
	if e == nil {
		a.initErr = store.Retain(a.ctx, time.Now().AddDate(0, 0, -settings.RetentionDays))
	}
	go func() {
		for {
			select {
			case <-a.ctx.Done():
				return
			case event := <-a.service.Events:
				wruntime.EventsEmit(a.ctx, "job", event)
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		var tracker network.Tracker
		for {
			select {
			case <-a.ctx.Done():
				return
			case now := <-ticker.C:
				sample, e := platform.Inventory()
				if e == nil {
					wruntime.EventsEmit(a.ctx, "metrics", tracker.Update(sample.Counters, now))
				}
			}
		}
	}()
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
	a.StopCapture()
	a.captureWG.Wait()
	a.captureMu.Lock()
	a.captureArtifact = nil
	a.captureMu.Unlock()
	if a.service != nil {
		_ = a.service.Close()
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
	return network.Diagnose(a.ctx, r)
}
func (a *App) StartDiagnostic(r network.Request) (storage.Job, error) {
	if e := a.ready(); e != nil {
		return storage.Job{}, e
	}
	return a.service.Start(a.ctx, r)
}
func (a *App) CancelJob(id string) bool {
	if a.ready() != nil {
		return false
	}
	return a.service.Cancel(id)
}
func (a *App) History() ([]storage.Job, error) {
	if e := a.ready(); e != nil {
		return nil, e
	}
	return a.service.Store.History(a.ctx)
}
func (a *App) ClearHistory() error {
	if e := a.ready(); e != nil {
		return e
	}
	return a.service.Store.Clear(a.ctx)
}
func (a *App) Settings() (storage.Settings, error) {
	if e := a.ready(); e != nil {
		return storage.Settings{}, e
	}
	return a.service.Store.Settings(a.ctx)
}
func (a *App) SaveSettings(v storage.Settings) error {
	if e := a.ready(); e != nil {
		return e
	}
	return a.service.Store.SaveSettings(a.ctx, v)
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
	if e := a.ready(); e != nil {
		return capture.ImportResult{}, e
	}
	a.captureMu.Lock()
	running := a.captureCancel != nil
	a.captureMu.Unlock()
	if running {
		return capture.ImportResult{}, domain.Invalid("stop capture before importing a file")
	}
	path, e := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Import capture", Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*.pcap;*.pcapng"}}})
	if e != nil {
		return capture.ImportResult{}, e
	}
	if path == "" {
		return capture.ImportResult{Packets: []capture.Packet{}}, nil
	}
	result, artifact, e := capture.ReadArtifactFile(path)
	if e == nil {
		a.captureMu.Lock()
		defer a.captureMu.Unlock()
		if a.captureCancel != nil {
			return capture.ImportResult{}, domain.Invalid("capture started while importing")
		}
		a.captureArtifact = artifact
	}
	return result, e
}
func (a *App) StartCapture(r capture.LiveRequest) error {
	if e := capture.ValidateLiveRequest(r); e != nil {
		return e
	}
	a.initMu.RLock()
	defer a.initMu.RUnlock()
	if e := a.readyLocked(); e != nil {
		return e
	}
	a.captureMu.Lock()
	defer a.captureMu.Unlock()
	if a.captureCancel != nil {
		return domain.Invalid("capture already running")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.captureCancel = cancel
	a.captureArtifact = nil
	a.captureWG.Add(1)
	go func() {
		defer a.captureWG.Done()
		defer cancel()
		result, artifact, e := capture.LiveSession(ctx, r)
		a.captureMu.Lock()
		a.captureArtifact = artifact
		a.captureCancel = nil
		a.captureMu.Unlock()
		message := ""
		if e != nil {
			message = e.Error()
		}
		wruntime.EventsEmit(a.ctx, "capture", map[string]any{"result": result, "error": message})
	}()
	return nil
}
func (a *App) StopCapture() {
	a.captureMu.Lock()
	defer a.captureMu.Unlock()
	if a.captureCancel != nil {
		a.captureCancel()
	}
}

func (a *App) SaveCapture(format string) (bool, error) {
	if e := a.ready(); e != nil {
		return false, e
	}
	if format != "pcap" && format != "pcapng" {
		return false, domain.Invalid("capture format must be pcap or pcapng")
	}
	a.captureMu.Lock()
	artifact := a.captureArtifact
	running := a.captureCancel != nil
	a.captureMu.Unlock()
	if running || artifact == nil {
		return false, domain.Invalid("import or finish a capture before saving")
	}
	path, e := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: "Save capture", DefaultFilename: "capture." + format, Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*." + format}}})
	if e != nil || path == "" {
		return false, e
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	if e = artifact.Save(ctx, path, format); e != nil {
		return false, e
	}
	return true, nil
}

func (a *App) InspectServices(cidr string, ports []int) (tools.Nmap, error) {
	if e := a.ready(); e != nil {
		return tools.Nmap{}, e
	}
	return a.service.InspectServices(a.ctx, cidr, ports)
}
