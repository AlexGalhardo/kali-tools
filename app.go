//go:build desktop || bindings

package main

import (
	"context"
	"github.com/AlexGalhardo/hacker-tools/internal/app"
	"github.com/AlexGalhardo/hacker-tools/internal/capture"
	"github.com/AlexGalhardo/hacker-tools/internal/domain"
	"github.com/AlexGalhardo/hacker-tools/internal/network"
	"github.com/AlexGalhardo/hacker-tools/internal/platform"
	"github.com/AlexGalhardo/hacker-tools/internal/storage"
	"github.com/AlexGalhardo/hacker-tools/internal/tools"
	"github.com/AlexGalhardo/hacker-tools/internal/wifi"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"sync"
	"time"
)

type App struct {
	ctx           context.Context
	cancel        context.CancelFunc
	service       *app.Service
	initErr       error
	captureMu     sync.Mutex
	captureCancel context.CancelFunc
	captureWG     sync.WaitGroup
}

func NewApp() *App { return &App{} }
func (a *App) startup(ctx context.Context) {
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
	if a.cancel != nil {
		a.cancel()
	}
	a.StopCapture()
	a.captureWG.Wait()
	if a.service != nil {
		a.service.Close()
	}
}
func (a *App) ready() error {
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
func (a *App) Tools() []tools.Tool                          { return tools.Inventory(a.ctx) }
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
	if a.service == nil {
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
	ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
	defer cancel()
	return network.Discover(ctx, cidr, ports, 500*time.Millisecond)
}
func (a *App) ImportCapture() (capture.ImportResult, error) {
	path, e := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Import capture", Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*.pcap;*.pcapng"}}})
	if e != nil {
		return capture.ImportResult{}, e
	}
	if path == "" {
		return capture.ImportResult{Packets: []capture.Packet{}}, nil
	}
	return capture.ReadFile(path)
}
func (a *App) StartCapture(r capture.LiveRequest) error {
	a.captureMu.Lock()
	defer a.captureMu.Unlock()
	if a.captureCancel != nil {
		return domain.Invalid("capture already running")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.captureCancel = cancel
	a.captureWG.Add(1)
	go func() {
		defer a.captureWG.Done()
		defer cancel()
		result, e := capture.Live(ctx, r)
		a.captureMu.Lock()
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
