package wailsui_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/wailsui"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func newApplication(t *testing.T) *app.Application {
	t.Helper()
	a, cleanup, err := bootstrap.New(t.Context(), bootstrap.Config{DataPath: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	return a
}
func quietRuntime() wailsui.Runtime {
	return wailsui.Runtime{
		Emit:     func(context.Context, string, ...any) {},
		OpenFile: func(context.Context, wruntime.OpenDialogOptions) (string, error) { return "", nil },
		SaveFile: func(context.Context, wruntime.SaveDialogOptions) (string, error) { return "", nil },
	}
}
func expectInvalid(t *testing.T, err error, message string) {
	t.Helper()
	var typed *domain.Error
	if !errors.As(err, &typed) || typed.Code != "invalid_input" || typed.Message != message {
		t.Fatalf("error=%v, want invalid_input %q", err, message)
	}
}

func TestBindingsRejectBeforeStartupAndAfterShutdown(t *testing.T) {
	b := wailsui.NewWithRuntime(newApplication(t), quietRuntime())
	for _, message := range []string{"application is starting", "application is shutting down"} {
		if b.Version() != domain.Version {
			t.Fatal("wrong version")
		}
		_, err := b.Interfaces()
		expectInvalid(t, err, message)
		_, err = b.History()
		expectInvalid(t, err, message)
		_, err = b.SaveCapture("pcap")
		expectInvalid(t, err, message)
		for _, call := range []func() error{
			func() error { _, err := b.Inventory(); return err },
			func() error { _, err := b.Tools(); return err },
			func() error { _, err := b.Diagnose(network.Request{}); return err },
			func() error { _, err := b.StartDiagnostic(network.Request{}); return err },
			func() error { return b.ClearHistory() },
			func() error { _, err := b.Settings(); return err },
			func() error { return b.SaveSettings(app.Settings{}) },
			func() error { _, err := b.CurrentWiFi(); return err },
			func() error { _, err := b.RevealWiFi(); return err },
			func() error { _, err := b.Discover("", nil); return err },
			func() error { _, err := b.InspectServices("", nil); return err },
			func() error { _, err := b.ImportCapture(); return err },
			func() error { return b.StartCapture(capture.LiveRequest{}) },
		} {
			expectInvalid(t, call(), message)
		}
		b.StopCapture()
		if b.CancelJob("missing") {
			t.Fatal("cancelled missing job")
		}
		if b.Capabilities() != nil {
			t.Fatal("capabilities before readiness")
		}
		b.Shutdown(t.Context())
	}
	b.Startup(t.Context()) // Shutdown is final, even before the first Startup.
	_, err := b.Interfaces()
	expectInvalid(t, err, "application is shutting down")
}

func TestForwarderEmitsLegacyJobPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer server.Close()
	a := newApplication(t)
	type emission struct {
		name string
		data any
	}
	seen := make(chan emission, 16)
	rt := quietRuntime()
	rt.Emit = func(ctx context.Context, name string, data ...any) {
		if len(data) != 1 {
			t.Error("invalid event arity")
			return
		}
		seen <- emission{name, data[0]}
	}
	b := wailsui.NewWithRuntime(a, rt)
	b.Startup(t.Context())
	defer b.Shutdown(t.Context())
	b.Startup(t.Context())
	st, err := b.StartDiagnostic(network.Request{Kind: "http", Target: server.URL, TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	var seq uint64
	for {
		select {
		case event := <-seen:
			if event.name != "job" {
				continue
			}
			raw, err := json.Marshal(event.data)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			var status job.Status
			if err := json.Unmarshal(fields["job"], &status); err != nil {
				t.Fatal(err)
			}
			var next uint64
			if err := json.Unmarshal(fields["seq"], &next); err != nil {
				t.Fatal(err)
			}
			if next <= seq {
				t.Fatal("unordered sequence")
			}
			seq = next
			want := 3
			if status.State.Terminal() {
				want = 4
			}
			if len(fields) != want || fields["schema_version"] == nil || fields["seq"] == nil || fields["job"] == nil {
				t.Fatalf("payload=%s", raw)
			}
			if status.State.Terminal() {
				if status.ID != st.ID || status.State != job.Completed || fields["result"] == nil {
					t.Fatalf("terminal=%s", raw)
				}
				return
			}
		case <-time.After(3 * time.Second):
			t.Fatal("missing terminal job event")
		}
	}
}

func TestShutdownStopsForwarder(t *testing.T) {
	a := newApplication(t)
	var count atomic.Int32
	rt := quietRuntime()
	rt.Emit = func(context.Context, string, ...any) { count.Add(1) }
	b := wailsui.NewWithRuntime(a, rt)
	b.Startup(t.Context())
	before := time.Now()
	b.Shutdown(t.Context())
	b.Shutdown(t.Context())
	if time.Since(before) >= 3*time.Second {
		t.Fatal("shutdown exceeded bound")
	}
	want := count.Load()
	events, _ := a.Subscribe(t.Context())
	listener := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer listener.Close()
	if _, err := a.StartDiagnostic(t.Context(), network.Request{Kind: "http", Target: listener.URL, TimeoutMS: 1000}); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case event := <-events:
			if event.Job != nil && event.Job.State.Terminal() {
				goto finished
			}
		case <-time.After(3 * time.Second):
			t.Fatal("operation did not finish")
		}
	}
finished:
	<-time.After(200 * time.Millisecond)
	if count.Load() != want {
		t.Fatal("emitted after shutdown")
	}
}

func TestSaveCaptureValidatesFormatBeforeDialog(t *testing.T) {
	var calls int
	rt := quietRuntime()
	rt.SaveFile = func(context.Context, wruntime.SaveDialogOptions) (string, error) { calls++; return "", nil }
	b := wailsui.NewWithRuntime(newApplication(t), rt)
	b.Startup(t.Context())
	defer b.Shutdown(t.Context())
	_, err := b.SaveCapture("invalid")
	expectInvalid(t, err, "capture format must be pcap or pcapng")
	if calls != 0 {
		t.Fatal("invalid format opened dialog")
	}
}

func TestSaveCaptureCancelledDialog(t *testing.T) {
	b := wailsui.NewWithRuntime(newApplication(t), quietRuntime())
	b.Startup(t.Context())
	defer b.Shutdown(t.Context())
	if saved, err := b.SaveCapture("pcap"); saved || err != nil {
		t.Fatalf("saved=%v, err=%v", saved, err)
	}
}

func TestImportCaptureEmptySelection(t *testing.T) {
	b := wailsui.NewWithRuntime(newApplication(t), quietRuntime())
	b.Startup(t.Context())
	defer b.Shutdown(t.Context())
	result, err := b.ImportCapture()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"packets":[],"limited":false,"format":""}` {
		t.Fatalf("JSON=%s", raw)
	}
}

func TestImportCaptureReadsSelectedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synthetic.pcapng")
	artifact := capture.NewArtifact(layers.LinkTypeRaw)
	packet := []byte{0x45, 0, 0, 20, 0, 0, 0, 0, 64, 1, 0, 0, 192, 0, 2, 1, 192, 0, 2, 2}
	if err := artifact.Add(packet, gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: len(packet), Length: len(packet)}); err != nil {
		t.Fatal(err)
	}
	if err := artifact.Save(t.Context(), path, "pcapng"); err != nil {
		t.Fatal(err)
	}
	rt := quietRuntime()
	rt.OpenFile = func(context.Context, wruntime.OpenDialogOptions) (string, error) { return path, nil }
	destination := filepath.Join(t.TempDir(), "output.pcap")
	rt.SaveFile = func(context.Context, wruntime.SaveDialogOptions) (string, error) { return destination, nil }
	b := wailsui.NewWithRuntime(newApplication(t), rt)
	b.Startup(t.Context())
	defer b.Shutdown(t.Context())
	if result, err := b.ImportCapture(); err != nil || len(result.Packets) != 1 {
		t.Fatalf("result=%+v, err=%v", result, err)
	}
	if saved, err := b.SaveCapture("pcap"); !saved || err != nil {
		t.Fatalf("saved=%v, err=%v", saved, err)
	}
	if result, err := capture.ReadFile(destination); err != nil || len(result.Packets) != 1 {
		t.Fatalf("saved result=%+v, err=%v", result, err)
	}
}

func TestBindingsDelegateToApplication(t *testing.T) {
	a := newApplication(t)
	if wailsui.New(a).Version() != domain.Version {
		t.Fatal("wrong default binding version")
	}
	b := wailsui.NewWithRuntime(a, quietRuntime())
	b.Startup(t.Context())
	defer b.Shutdown(t.Context())
	if len(b.Capabilities()) == 0 {
		t.Fatal("missing capabilities")
	}
	if _, err := b.Interfaces(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Inventory(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Tools(); err != nil {
		t.Fatal(err)
	}
	settings := app.Settings{Language: "pt-BR", RetentionDays: 7}
	if err := b.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if got, err := b.Settings(); err != nil || got != settings {
		t.Fatalf("settings=%+v, err=%v", got, err)
	}
	if _, err := b.History(); err != nil {
		t.Fatal(err)
	}
	if err := b.ClearHistory(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Diagnose(network.Request{}); err == nil {
		t.Fatal("invalid diagnostic accepted")
	}
	if _, err := b.Discover("0.0.0.0/0", []int{80}); err == nil {
		t.Fatal("invalid discovery accepted")
	}
	if _, err := b.InspectServices("0.0.0.0/0", []int{80}); err == nil {
		t.Fatal("invalid inspection accepted")
	}
	if err := b.StartCapture(capture.LiveRequest{}); err == nil {
		t.Fatal("invalid capture accepted")
	}
	b.StopCapture()
	if b.CancelJob("missing") {
		t.Fatal("cancelled unknown job")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cancelled := wailsui.NewWithRuntime(a, quietRuntime())
	cancelled.Startup(ctx)
	defer cancelled.Shutdown(t.Context())
	if _, err := cancelled.CurrentWiFi(); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := cancelled.RevealWiFi(); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestDialogErrorsAreReturned(t *testing.T) {
	failure := errors.New("synthetic dialog failure")
	rt := quietRuntime()
	rt.OpenFile = func(context.Context, wruntime.OpenDialogOptions) (string, error) { return "", failure }
	rt.SaveFile = func(context.Context, wruntime.SaveDialogOptions) (string, error) { return "", failure }
	b := wailsui.NewWithRuntime(newApplication(t), rt)
	b.Startup(t.Context())
	defer b.Shutdown(t.Context())
	if _, err := b.ImportCapture(); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if _, err := b.SaveCapture("pcap"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}

func TestEventPayloadTypesAndKeys(t *testing.T) {
	for _, event := range []job.Event{
		{Topic: "unknown"}, {Topic: job.TopicJob}, {Topic: job.TopicJob, Job: &job.Status{}, Data: "bad"},
		{Topic: job.TopicMetrics, Data: "bad"}, {Topic: job.TopicCapture, Data: "bad"},
	} {
		if _, _, ok := wailsui.EventPayloadForTest(event); ok {
			t.Fatalf("accepted invalid event %+v", event)
		}
	}
	metrics := network.Metrics{Timestamp: time.Unix(100, 0), Interfaces: []network.Rate{}}
	if name, data, ok := wailsui.EventPayloadForTest(job.Event{Topic: job.TopicMetrics, Data: metrics}); !ok || name != "metrics" || !reflect.DeepEqual(data, metrics) {
		t.Fatal("metrics changed")
	}
	name, data, ok := wailsui.EventPayloadForTest(job.Event{Topic: job.TopicCapture, Data: app.CaptureEvent{Result: capture.LiveResult{Packets: []capture.Packet{}}, Error: "synthetic failure", Err: errors.New("private error")}})
	if !ok || name != "capture" {
		t.Fatal("capture rejected")
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields["result"] == nil || fields["error"] == nil {
		t.Fatalf("capture=%s", raw)
	}
}

func TestBindingMethodSetMatchesBackend(t *testing.T) {
	typeOf := reflect.TypeOf(&wailsui.Bindings{})
	var names []string
	for i := range typeOf.NumMethod() {
		name := typeOf.Method(i).Name
		if name != "Startup" && name != "Shutdown" {
			names = append(names, name)
		}
	}
	want := []string{"CancelJob", "Capabilities", "ClearHistory", "CurrentWiFi", "Diagnose", "Discover", "History", "ImportCapture", "InspectServices", "Interfaces", "Inventory", "RevealWiFi", "SaveCapture", "SaveSettings", "Settings", "StartCapture", "StartDiagnostic", "StopCapture", "Tools", "Version"}
	if !slices.Equal(names, want) {
		t.Fatalf("methods=%v, want=%v", names, want)
	}
}
