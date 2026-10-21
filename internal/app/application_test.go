package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

type recordingJobs struct {
	*fakeJobStore
	see func(context.Context)
}

func (f recordingJobs) History(ctx context.Context) ([]job.Status, error) {
	f.see(ctx)
	return f.fakeJobStore.History(ctx)
}
func (f recordingJobs) Clear(ctx context.Context) error { f.see(ctx); return f.fakeJobStore.Clear(ctx) }

type recordingSettings struct {
	*fakeSettings
	see func(context.Context)
}

func (f recordingSettings) Settings(ctx context.Context) (app.Settings, error) {
	f.see(ctx)
	return f.fakeSettings.Settings(ctx)
}
func (f recordingSettings) SaveSettings(ctx context.Context, v app.Settings) error {
	f.see(ctx)
	return f.fakeSettings.SaveSettings(ctx, v)
}

type recordingTools struct {
	fakeTools
	see func(context.Context)
}

func (f recordingTools) Inventory(ctx context.Context) []app.Tool {
	f.see(ctx)
	return []app.Tool{{Name: "synthetic-tool"}}
}

type recordingWiFi struct{ see func(context.Context) }

func (f recordingWiFi) Current(ctx context.Context) (wifi.Network, error) {
	f.see(ctx)
	return wifi.Network{SSID: "synthetic-network"}, nil
}
func (f recordingWiFi) Reveal(ctx context.Context) (wifi.Secret, error) {
	f.see(ctx)
	return wifi.Secret{State: "available", Value: "synthetic-psk"}, nil
}

func TestApplicationDelegatesToPorts(t *testing.T) {
	preferences := app.Settings{Language: "pt-BR", RetentionDays: 7}
	host := app.HostSnapshot{Adapters: []app.HostAdapter{{Name: "synthetic-host"}}}
	for _, test := range []struct {
		name string
		call func(*app.Application, context.Context) (any, error)
		want any
	}{
		{"inventory", func(a *app.Application, ctx context.Context) (any, error) { return a.Inventory(ctx) }, host},
		{"tools", func(a *app.Application, ctx context.Context) (any, error) { return a.Tools(ctx) }, []app.Tool{{Name: "synthetic-tool"}}},
		{"wifi", func(a *app.Application, ctx context.Context) (any, error) { return a.CurrentWiFi(ctx) }, wifi.Network{SSID: "synthetic-network"}},
		{"reveal", func(a *app.Application, ctx context.Context) (any, error) { return a.RevealWiFi(ctx) }, wifi.Secret{State: "available", Value: "synthetic-psk"}},
		{"settings", func(a *app.Application, ctx context.Context) (any, error) { return a.Settings(ctx) }, preferences},
		{"save settings", func(a *app.Application, ctx context.Context) (any, error) {
			return nil, a.SaveSettings(ctx, preferences)
		}, nil},
		{"history", func(a *app.Application, ctx context.Context) (any, error) { return a.History(ctx) }, []job.Status{}},
		{"clear history", func(a *app.Application, ctx context.Context) (any, error) { return nil, a.ClearHistory(ctx) }, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			type key struct{}
			ctx := context.WithValue(t.Context(), key{}, "caller")
			calls := 0
			see := func(got context.Context) {
				calls++
				if got.Value(key{}) != "caller" {
					t.Error("caller context not propagated")
				}
			}
			deps := app.Deps{
				Jobs:     recordingJobs{fakeJobStore: &fakeJobStore{}, see: see},
				Settings: recordingSettings{fakeSettings: &fakeSettings{value: preferences}, see: see},
				Host:     fakeHost{inventory: func(ctx context.Context) (app.HostSnapshot, error) { see(ctx); return host, nil }},
				Tools:    recordingTools{see: see}, WiFi: recordingWiFi{see: see},
			}
			a := app.New(deps)
			defer func() { _ = a.Close() }()
			got, err := test.call(a, ctx)
			if err != nil || !reflect.DeepEqual(got, test.want) || calls != 1 {
				t.Fatalf("got=%+v, want=%+v, err=%v, calls=%d", got, test.want, err, calls)
			}
		})
	}
}

func TestRevealNeverReachesJobStoreOrEvents(t *testing.T) {
	deps := fakeDeps()
	deps.WiFi = recordingWiFi{see: func(context.Context) {}}
	a := app.New(deps)
	defer func() { _ = a.Close() }()
	events, _ := a.Subscribe(t.Context())
	secret, err := a.RevealWiFi(t.Context())
	if err != nil || secret.Value != "synthetic-psk" {
		t.Fatalf("secret state=%s, err=%v", secret.State, err)
	}
	rows, err := a.History(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "synthetic-psk") {
		t.Fatal("secret persisted")
	}
	select {
	case event := <-events:
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "synthetic-psk") {
			t.Fatal("secret published")
		}
	default:
	}
}

func TestJobSnapshotAndCancel(t *testing.T) {
	deps := fakeDeps()
	deps.Diagnostics = network.Diagnostics{Prober: blockingProber{}}
	a := app.New(deps)
	defer func() { _ = a.Close() }()
	events, _ := a.Subscribe(t.Context())
	st, err := a.StartDiagnostic(t.Context(), network.Request{Kind: "ping", Target: "synthetic.invalid", TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if snap, ok := a.JobSnapshot(t.Context(), st.ID); !ok || snap.ID != st.ID {
		t.Fatal("active snapshot missing")
	}
	if !a.CancelJob(t.Context(), st.ID) || a.CancelJob(t.Context(), "missing") {
		t.Fatal("invalid cancel result")
	}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case event := <-events:
			if event.Job != nil && event.Job.ID == st.ID && event.Job.State.Terminal() {
				if event.Job.State != job.Cancelled {
					t.Fatal(event.Job.State)
				}
				if snap, ok := a.JobSnapshot(t.Context(), st.ID); !ok || snap.ID != st.ID || snap.State != job.Cancelled {
					t.Fatal("terminal snapshot missing")
				}
				return
			}
		case <-deadline.C:
			t.Fatal("job did not cancel")
		}
	}
}

type blockingProber struct{}

func (blockingProber) Provider() string { return "synthetic" }
func (blockingProber) Run(ctx context.Context, _ string, _ bool) ([]string, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestFacadeMissingProviders(t *testing.T) {
	a := app.New(fakeDeps())
	defer func() { _ = a.Close() }()
	for _, call := range []func() error{
		func() error { _, err := a.Inventory(t.Context()); return err },
		func() error { _, err := a.Tools(t.Context()); return err },
	} {
		var typed *domain.Error
		if err := call(); !errors.As(err, &typed) || typed.Code != "unsupported" {
			t.Fatal(err)
		}
	}
	deps := fakeDeps()
	deps.WiFi = nil
	a = app.New(deps)
	defer func() { _ = a.Close() }()
	for _, call := range []func() error{
		func() error { _, err := a.CurrentWiFi(t.Context()); return err },
		func() error { _, err := a.RevealWiFi(t.Context()); return err },
	} {
		var typed *domain.Error
		if err := call(); !errors.As(err, &typed) || typed.Code != "unsupported" {
			t.Fatal(err)
		}
	}
}

func TestApplicationAfterClose(t *testing.T) {
	a := app.New(fakeDeps())
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := a.StartDiagnostic(t.Context(), network.Request{Kind: "tcp", Target: "127.0.0.1", Port: 80, TimeoutMS: 100})
	if !errors.Is(err, job.ErrClosed) {
		t.Fatal(err)
	}
	_, err = a.Discover(t.Context(), "127.0.0.1/32", []int{80}, 100*time.Millisecond)
	if !errors.Is(err, job.ErrClosed) {
		t.Fatal(err)
	}
	_, err = a.InspectServices(t.Context(), "127.0.0.1/32", []int{80})
	if !errors.Is(err, job.ErrClosed) {
		t.Fatal(err)
	}
	expectMessage(t, a.CanImportCapture(t.Context()), "application is shutting down")
	_, err = a.ImportCapture(t.Context(), "unused")
	expectMessage(t, err, "application is shutting down")
	events, drops := a.Subscribe(t.Context())
	if _, ok := <-events; ok || drops() != 0 {
		t.Fatal("closed application accepted subscription")
	}
	t.Run("capture", func(t *testing.T) {
		expectMessage(t, a.StartCapture(t.Context(), capture.LiveRequest{Address: captureAddress(t), DurationSeconds: 1, PacketLimit: 1}), "application is shutting down")
	})
}

func TestSynchronousDiagnoseDoesNotPersist(t *testing.T) {
	deps := fakeDeps()
	deps.Diagnostics = network.Diagnostics{Prober: fakeProber{}}
	a := app.New(deps)
	defer func() { _ = a.Close() }()
	result, err := a.Diagnose(t.Context(), network.Request{Kind: "ping", Target: "synthetic.invalid", TimeoutMS: 100})
	if err != nil || result.Provider != "fake" {
		t.Fatalf("result=%+v, err=%v", result, err)
	}
	rows, err := a.History(t.Context())
	if err != nil || len(rows) != 0 {
		t.Fatalf("synchronous diagnose wrote history: %+v, err=%v", rows, err)
	}
	if len(a.Capabilities(t.Context())) == 0 {
		t.Fatal("missing capability registry")
	}
	if _, err := a.Interfaces(t.Context()); err != nil {
		t.Fatal(err)
	}
}
