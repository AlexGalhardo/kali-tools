package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

type fakeJobStore struct {
	mu       sync.Mutex
	statuses map[job.ID]job.Status
}

func (s *fakeJobStore) PutJob(ctx context.Context, st job.Status) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.statuses == nil {
		s.statuses = make(map[job.ID]job.Status)
	}
	s.statuses[st.ID] = cloneStatus(st)
	return nil
}

func (s *fakeJobStore) History(ctx context.Context) ([]job.Status, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]job.Status, 0, len(s.statuses))
	for _, st := range s.statuses {
		rows = append(rows, cloneStatus(st))
	}
	slices.SortFunc(rows, func(a, b job.Status) int { return b.Created.Compare(a.Created) })
	return rows[:min(len(rows), 200)], nil
}

func (s *fakeJobStore) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, st := range s.statuses {
		if st.State.Terminal() {
			delete(s.statuses, id)
		}
	}
	return nil
}

func (s *fakeJobStore) Retain(ctx context.Context, before time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, st := range s.statuses {
		if st.State.Terminal() && st.Created.Before(before) {
			delete(s.statuses, id)
		}
	}
	return nil
}

func cloneStatus(st job.Status) job.Status {
	if st.Finished != nil {
		finished := *st.Finished
		st.Finished = &finished
	}
	return st
}

type fakeSettings struct {
	mu    sync.Mutex
	value app.Settings
}

func (s *fakeSettings) Settings(ctx context.Context) (app.Settings, error) {
	if err := ctx.Err(); err != nil {
		return app.Settings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.value.Language == "" {
		return app.Settings{Language: "en", RetentionDays: 30}, nil
	}
	return s.value, nil
}
func (s *fakeSettings) SaveSettings(ctx context.Context, value app.Settings) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = value
	return nil
}

type fakeHost struct {
	inventory func(context.Context) (app.HostSnapshot, error)
}

func (h fakeHost) Inventory(ctx context.Context) (app.HostSnapshot, error) { return h.inventory(ctx) }

type fakeTools struct {
	result app.Nmap
	err    error
}

func (f fakeTools) Inventory(context.Context) []app.Tool                     { return []app.Tool{} }
func (f fakeTools) RunNmap(context.Context, string, []int) (app.Nmap, error) { return f.result, f.err }

type fakeWiFi struct{}

func (fakeWiFi) Current(context.Context) (wifi.Network, error) {
	return wifi.Network{State: "disconnected"}, nil
}
func (fakeWiFi) Reveal(context.Context) (wifi.Secret, error) {
	return wifi.Secret{State: "no_saved_key"}, nil
}

type fakeCapture struct {
	run func(context.Context, capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error)
}

func (f fakeCapture) Capture(ctx context.Context, request capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
	return f.run(ctx, request)
}

type fakeProber struct{}

func (fakeProber) Provider() string { return "fake" }
func (fakeProber) Run(context.Context, string, bool) ([]string, error) {
	return []string{"synthetic response"}, nil
}

func fakeDeps() app.Deps {
	return app.Deps{Jobs: &fakeJobStore{}, Settings: &fakeSettings{}, WiFi: fakeWiFi{}}
}

func TestDiagnosticJobWithFakes(t *testing.T) {
	deps := fakeDeps()
	deps.Diagnostics = network.Diagnostics{Prober: fakeProber{}}
	s := app.New(deps)
	defer func() { _ = s.Close() }()
	st, err := s.Start(t.Context(), network.Request{Kind: "ping", Target: "synthetic-target.invalid", TimeoutMS: 100})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if _, err := s.Jobs.Wait(ctx, st.ID); err != nil {
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
	deps.Tools = fakeTools{result: app.Nmap{Version: "7.95", Hosts: []app.NmapHost{{}, {}}}}
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
	err = s.StartCapture(request)
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
