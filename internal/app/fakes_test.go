package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

type fakeJobStore = fake.Store
type fakeSettings = fake.Store

func newFakeSettings(t *testing.T, value app.Settings) *fake.Store {
	t.Helper()
	s := fake.NewStore()
	if err := s.SaveSettings(t.Context(), value); err != nil {
		t.Fatal(err)
	}
	return s
}

type fakeHost = fake.Host
type fakeTools = fake.Tools
type fakeWiFi = fake.WiFi
type fakeCapture = fake.LiveCapture
type fakeProber = fake.Prober

func fakeDeps() app.Deps {
	return app.Deps{Jobs: &fakeJobStore{}, Settings: &fakeSettings{}, WiFi: fakeWiFi{Network: wifi.Network{State: "disconnected"}, Secret: wifi.Secret{State: "no_saved_key"}}}
}

func TestDiagnosticJobWithFakes(t *testing.T) {
	deps := fakeDeps()
	deps.Diagnostics = network.Diagnostics{Prober: fakeProber{Values: []string{"synthetic response"}}}
	s := app.New(deps)
	defer func() { _ = s.Close() }()
	st, err := s.StartDiagnostic(t.Context(), network.Request{Kind: "ping", Target: "synthetic-target.invalid", TimeoutMS: 100})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if _, err := s.WaitForTest(ctx, st.ID); err != nil {
		t.Fatal(err)
	}
	rows, err := s.History(t.Context())
	if err != nil || len(rows) != 1 || rows[0].State != job.Completed || rows[0].Summary != "Diagnostic completed" {
		t.Fatalf("history = %+v, err = %v", rows, err)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "synthetic-target.invalid") {
		t.Fatal("target persisted")
	}
	settings := app.Settings{Language: "pt-BR", RetentionDays: 7}
	if err := s.SaveSettings(t.Context(), settings); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Settings(t.Context()); err != nil || got != settings {
		t.Fatalf("settings = %+v, err = %v", got, err)
	}
	if err := s.ClearHistory(t.Context()); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.History(t.Context()); err != nil || len(rows) != 0 {
		t.Fatalf("history = %+v, err = %v", rows, err)
	}
}

func TestInspectServicesUsesToolRunner(t *testing.T) {
	deps := fakeDeps()
	deps.Tools = fakeTools{Result: app.Nmap{Version: "7.95", Hosts: []app.NmapHost{{}, {}}}}
	s := app.New(deps)
	defer func() { _ = s.Close() }()
	result, err := s.InspectServices(t.Context(), "127.0.0.1/32", []int{80})
	if err != nil || len(result.Hosts) != 2 {
		t.Fatalf("inspection = %+v, err = %v", result, err)
	}
	rows, err := s.History(t.Context())
	if err != nil || len(rows) != 1 || rows[0].Summary != "Service inspection completed: Nmap 7.95: 2 hosts" {
		t.Fatalf("history = %+v, err = %v", rows, err)
	}
	if strings.Contains(rows[0].Summary, "127.0.0.1") {
		t.Fatal("scope persisted")
	}
}

func TestMissingProvidersAreUnsupported(t *testing.T) {
	s := app.New(fakeDeps())
	defer func() { _ = s.Close() }()
	_, err := s.InspectServices(t.Context(), "127.0.0.1/32", []int{80})
	var typed *domain.Error
	if !errors.As(err, &typed) || typed.Code != "unsupported" {
		t.Fatal(err)
	}
	request := capture.LiveRequest{Address: captureAddress(t), DurationSeconds: 1, PacketLimit: 1}
	err = s.StartCapture(t.Context(), request)
	if !errors.As(err, &typed) || typed.Code != "unsupported" {
		t.Fatal(err)
	}
}

func TestNewRequiresStores(t *testing.T) {
	for _, test := range []struct {
		name string
		deps app.Deps
	}{
		{"jobs", app.Deps{Settings: &fakeSettings{}}},
		{"settings", app.Deps{Jobs: &fakeJobStore{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("missing required store did not panic")
				}
			}()
			app.New(test.deps)
		})
	}
}
