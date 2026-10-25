package wailsui

import (
	"context"
	"os"
	"runtime"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

const (
	desktopDiscoveryTimeout = 500 * time.Millisecond
	shutdownWait            = 3 * time.Second
)

type Runtime struct {
	Emit     func(context.Context, string, ...any)
	OpenFile func(context.Context, wruntime.OpenDialogOptions) (string, error)
	SaveFile func(context.Context, wruntime.SaveDialogOptions) (string, error)
	OpenURL  func(context.Context, string)
}
type Bindings struct {
	app     *app.Application
	emit    func(context.Context, string, ...any)
	open    func(context.Context, wruntime.OpenDialogOptions) (string, error)
	save    func(context.Context, wruntime.SaveDialogOptions) (string, error)
	openURL func(context.Context, string)
	mu      sync.RWMutex
	ctx     context.Context
	closed  bool
	stop    context.CancelFunc
	done    chan struct{}
}

func New(a *app.Application) *Bindings {
	return NewWithRuntime(a, Runtime{Emit: wruntime.EventsEmit, OpenFile: wruntime.OpenFileDialog, SaveFile: wruntime.SaveFileDialog, OpenURL: wruntime.BrowserOpenURL})
}
func NewWithRuntime(a *app.Application, rt Runtime) *Bindings {
	return &Bindings{app: a, emit: rt.Emit, open: rt.OpenFile, save: rt.SaveFile, openURL: rt.OpenURL}
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
func fail(err error) error {
	if err == nil {
		return nil
	}
	return contract.FromError(err)
}
func (b *Bindings) Version() string { return domain.Version }

func (b *Bindings) AuditTLS(r contract.TLSAuditRequest) (contract.TLSAuditReport, error) {
	ctx, err := b.ready()
	if err != nil {
		return contract.TLSAuditReport{}, fail(err)
	}
	value, err := b.app.AuditTLS(ctx, r.Request())
	return contract.FromTLSAudit(value), fail(err)
}

func (b *Bindings) InspectDNS(r contract.DNSInspectRequest) (contract.DNSInspectReport, error) {
	ctx, err := b.ready()
	if err != nil {
		return contract.DNSInspectReport{}, fail(err)
	}
	value, err := b.app.InspectDNS(ctx, r.Request())
	return contract.FromDNSInspect(value), fail(err)
}

func (b *Bindings) DiscoverLocal(r contract.LocalDiscoveryRequest) (contract.LocalDiscoveryResult, error) {
	ctx, err := b.ready()
	if err != nil {
		return contract.LocalDiscoveryResult{}, fail(err)
	}
	value, err := b.app.DiscoverLocal(ctx, r.Request())
	return contract.FromLocalDiscovery(value), fail(err)
}

func (b *Bindings) QuerySNMP(r contract.SNMPRequest) (contract.SNMPResult, error) {
	ctx, err := b.ready()
	if err != nil {
		return contract.SNMPResult{}, fail(err)
	}
	query, err := r.Request()
	if err != nil {
		return contract.SNMPResult{}, fail(err)
	}
	value, err := b.app.QuerySNMP(ctx, query)
	return contract.FromSNMP(value), fail(err)
}

func (b *Bindings) Connections() (contract.ConnectionTable, error) {
	ctx, err := b.ready()
	if err != nil {
		return contract.ConnectionTable{}, fail(err)
	}
	value, err := b.app.Connections(ctx)
	return contract.FromConnectionTable(value), fail(err)
}

func (b *Bindings) Throughput(durationMS int) (contract.ThroughputSample, error) {
	ctx, err := b.ready()
	if err != nil {
		return contract.ThroughputSample{}, fail(err)
	}
	if durationMS < 1 || durationMS > 10_000 {
		return contract.ThroughputSample{}, fail(domain.Invalid("throughput duration must be 1ms..10s"))
	}
	value, err := b.app.Throughput(ctx, time.Duration(durationMS)*time.Millisecond, nil)
	return contract.FromThroughput(value), fail(err)
}

func (b *Bindings) Neighbors(family string) (contract.NeighborTable, error) {
	ctx, err := b.ready()
	if err != nil {
		return contract.NeighborTable{}, fail(err)
	}
	value, err := b.app.Neighbors(ctx, family)
	return contract.FromNeighborTable(value), fail(err)
}
func (b *Bindings) Capabilities() []contract.Capability {
	ctx, e := b.ready()
	if e != nil {
		return nil
	}
	return contract.FromCapabilities(b.app.Capabilities(ctx))
}
func (b *Bindings) Interfaces() ([]contract.Interface, error) {
	ctx, e := b.ready()
	if e != nil {
		return nil, fail(e)
	}
	v, e := b.app.Interfaces(ctx)
	return contract.FromInterfaces(v), fail(e)
}
func (b *Bindings) Inventory() (contract.Inventory, error) {
	ctx, e := b.ready()
	if e != nil {
		return contract.Inventory{}, fail(e)
	}
	v, e := b.app.Inventory(ctx)
	return contract.FromInventory(v), fail(e)
}
func (b *Bindings) Tools() ([]contract.Tool, error) {
	ctx, e := b.ready()
	if e != nil {
		return nil, fail(e)
	}
	v, e := b.app.Tools(ctx)
	return contract.FromTools(v), fail(e)
}
func (b *Bindings) Diagnose(r contract.DiagnosticRequest) (contract.DiagnosticResult, error) {
	ctx, e := b.ready()
	if e != nil {
		return contract.DiagnosticResult{}, fail(e)
	}
	v, e := b.app.Diagnose(ctx, r.Request())
	return contract.FromDiagnosticResult(v), fail(e)
}
func (b *Bindings) StartDiagnostic(r contract.DiagnosticRequest) (contract.Job, error) {
	ctx, e := b.ready()
	if e != nil {
		return contract.Job{}, fail(e)
	}
	v, e := b.app.StartDiagnostic(ctx, r.Request())
	return contract.FromStatus(v), fail(e)
}
func (b *Bindings) History() ([]contract.Job, error) {
	ctx, e := b.ready()
	if e != nil {
		return nil, fail(e)
	}
	v, e := b.app.History(ctx)
	return contract.FromStatuses(v), fail(e)
}
func (b *Bindings) Settings() (contract.Settings, error) {
	ctx, e := b.ready()
	if e != nil {
		return contract.Settings{}, fail(e)
	}
	v, e := b.app.Settings(ctx)
	return contract.FromSettings(v), fail(e)
}
func (b *Bindings) CurrentWiFi() (contract.WiFi, error) {
	ctx, e := b.ready()
	if e != nil {
		return contract.WiFi{}, fail(e)
	}
	v, e := b.app.CurrentWiFi(ctx)
	return contract.FromWiFi(v), fail(e)
}
func (b *Bindings) RevealWiFi() (contract.WiFiSecret, error) {
	ctx, e := b.ready()
	if e != nil {
		return contract.WiFiSecret{}, fail(e)
	}
	v, e := b.app.RevealWiFi(ctx)
	return contract.FromWiFiSecret(v), fail(e)
}

func (b *Bindings) OpenLocationSettings() error {
	ctx, err := b.ready()
	if err != nil {
		return fail(err)
	}
	if runtime.GOOS != "windows" {
		return fail(domain.Unsupported("location settings exist only on Windows"))
	}
	if b.openURL == nil {
		return fail(domain.Unsupported("location settings opener is unavailable"))
	}
	b.openURL(ctx, domain.LocationSettingsURI)
	return nil
}

func (b *Bindings) CaptureBackends() ([]contract.Capability, error) {
	ctx, err := b.ready()
	if err != nil {
		return nil, fail(err)
	}
	return contract.FromCapabilities(b.app.CaptureBackendStatus(ctx)), nil
}

func (b *Bindings) OpenNpcapDownload() error {
	ctx, err := b.ready()
	if err != nil {
		return fail(err)
	}
	if b.openURL == nil {
		return fail(domain.Unsupported("browser opener is unavailable"))
	}
	b.openURL(ctx, domain.NpcapDownloadURL)
	return nil
}
func (b *Bindings) Discover(cidr string, ports []int) (contract.Discovery, error) {
	ctx, e := b.ready()
	if e != nil {
		return contract.Discovery{}, fail(e)
	}
	v, truncated, e := b.app.Discover(ctx, cidr, ports, desktopDiscoveryTimeout)
	return contract.FromDiscovery(v, truncated), fail(e)
}
func (b *Bindings) InspectServices(cidr string, ports []int) (contract.ServiceInspection, error) {
	ctx, e := b.ready()
	if e != nil {
		return contract.ServiceInspection{}, fail(e)
	}
	v, e := b.app.InspectServices(ctx, cidr, ports)
	return contract.FromServiceInspection(v), fail(e)
}
func (b *Bindings) ClearHistory() error {
	ctx, e := b.ready()
	if e != nil {
		return fail(e)
	}
	return fail(b.app.ClearHistory(ctx))
}
func (b *Bindings) SaveSettings(v contract.Settings) error {
	ctx, e := b.ready()
	if e != nil {
		return fail(e)
	}
	return fail(b.app.SaveSettings(ctx, v.AppSettings()))
}
func (b *Bindings) StartCapture(request contract.LiveCaptureRequest) error {
	ctx, e := b.ready()
	if e != nil {
		return fail(e)
	}
	return fail(b.app.StartCapture(ctx, request.LiveRequest()))
}
func (b *Bindings) CancelJob(id string) bool {
	ctx, e := b.ready()
	if e != nil {
		return false
	}
	return b.app.CancelJob(ctx, job.ID(id))
}
func (b *Bindings) StopCapture() {
	if ctx, e := b.ready(); e == nil {
		b.app.StopCapture(ctx)
	}
}
func (b *Bindings) ImportCapture() (contract.Capture, error) {
	ctx, err := b.ready()
	if err != nil {
		return contract.Capture{}, fail(err)
	}
	if err := b.app.CanImportCapture(ctx); err != nil {
		return contract.Capture{}, fail(err)
	}
	path, err := b.open(ctx, wruntime.OpenDialogOptions{Title: "Import capture", Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*.pcap;*.pcapng"}}})
	if err != nil {
		return contract.Capture{}, fail(err)
	}
	if path == "" {
		return contract.Capture{Packets: []contract.Packet{}}, nil
	}
	v, err := b.app.ImportCapture(ctx, path)
	return contract.FromImportResult(v), fail(err)
}

func (b *Bindings) SaveCapture(format string) (bool, error) {
	ctx, err := b.ready()
	if err != nil {
		return false, fail(err)
	}
	if format != "pcap" && format != "pcapng" {
		return false, fail(domain.Invalid("capture format must be pcap or pcapng"))
	}
	path, err := b.save(ctx, wruntime.SaveDialogOptions{Title: "Save capture", DefaultFilename: "capture." + format, Filters: []wruntime.FileFilter{{DisplayName: "Packet capture", Pattern: "*." + format}}})
	if err != nil || path == "" {
		return false, fail(err)
	}
	if err := b.app.ExportCapture(ctx, path, format); err != nil {
		return false, fail(err)
	}
	return true, nil
}

func (b *Bindings) SaveDiagnosticBundle() (bool, error) {
	ctx, err := b.ready()
	if err != nil {
		return false, fail(err)
	}
	path, err := b.save(ctx, wruntime.SaveDialogOptions{Title: "Save diagnostic bundle", DefaultFilename: "kali-tools-diagnostic.zip", Filters: []wruntime.FileFilter{{DisplayName: "Zip archive", Pattern: "*.zip"}}})
	if err != nil || path == "" {
		return false, fail(err)
	}
	body, err := b.app.DiagnosticBundle(ctx)
	if err != nil {
		return false, fail(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return false, fail(domain.Invalid("diagnostic bundle path is not writable"))
	}
	return true, nil
}
