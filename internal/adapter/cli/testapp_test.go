package cli_test

import (
	"context"
	"io"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/porttest"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

type countingWiFi struct {
	fake.WiFi
	reveals atomic.Int32
}

func (p *countingWiFi) Reveal(ctx context.Context) (wifi.Secret, error) {
	p.reveals.Add(1)
	return p.WiFi.Reveal(ctx)
}

func revealApplication(t *testing.T) (*app.Application, *countingWiFi) {
	t.Helper()
	provider := &countingWiFi{WiFi: fake.WiFi{
		Network: wifi.Network{State: "connected"},
		Secret:  wifi.Secret{State: "available", Value: "synthetic-test-key"},
	}}
	store := fake.NewStore()
	a := app.New(app.Deps{Jobs: store, Settings: store, WiFi: provider})
	t.Cleanup(func() { _ = a.Close() })
	return a, provider
}

func TestCountingWiFiContract(t *testing.T) {
	porttest.WiFiProviderContract(t, func(t *testing.T) app.WiFiProvider {
		_, provider := revealApplication(t)
		return provider
	})
}

const fixtureJobID = "0123456789abcdef0123456789abcdef"

func newTestApp(t *testing.T) *app.Application {
	t.Helper()
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	store := fake.NewStore()
	if err := store.PutJob(t.Context(), job.Status{ID: job.ID(fixtureJobID), Kind: "tcp", State: job.Completed, Created: now, Finished: &now, Summary: "Diagnostic completed"}); err != nil {
		t.Fatal(err)
	}
	a := app.New(app.Deps{
		Jobs: store, Settings: store, Clock: func() time.Time { return now },
		Host: fake.Host{Snapshot: app.HostSnapshot{
			Adapters: []app.HostAdapter{{Index: 1, Name: "Fixture Ethernet", Gateways: []string{"192.0.2.1"}, DNS: []string{"192.0.2.53"}}},
			Counters: []network.Counter{{ID: "1", Received: 200, Sent: 100}},
		}},
		WiFi: fake.WiFi{
			Network: wifi.Network{SSID: "Fixture Network", Profile: "Fixture", BSSID: "02:00:00:00:00:02", Signal: 80, Authentication: 7, State: "connected"},
			Secret:  wifi.Secret{State: "available", Value: "synthetic-test-key"},
		},
		Tools:       fake.Tools{Tools: []app.Tool{{Name: "nmap", State: "available", Version: "fixture", Path: "/fixture/nmap"}, {Name: "tshark", State: "dependency_missing"}}, Result: app.Nmap{Scanner: "nmap", Version: "fixture", Finished: app.NmapFinished{Exit: "success"}, Hosts: []app.NmapHost{{Status: app.NmapState{State: "up"}, Addresses: []app.NmapAddress{{Address: "192.0.2.1", Type: "ipv4"}}, Ports: []app.NmapPort{{Port: 443, Protocol: "tcp", State: app.NmapState{State: "open"}, Service: app.NmapService{Name: "https", Method: "probed"}}}}}}},
		Diagnostics: network.Diagnostics{Prober: fake.Prober{Values: []string{"echo reply"}}},
	})
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	return a
}

func runJSON(ctx context.Context, a *app.Application, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (!slices.Contains(args, "discover") || !slices.Contains(args, "csv")) {
		args = append([]string{"--json"}, args...)
	}
	return cli.Run(ctx, a, args, stdout, stderr)
}

func assertGoroutinesReturn(t *testing.T, before int) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for runtime.NumGoroutine() > before {
		select {
		case <-poll.C:
		case <-deadline.C:
			t.Fatalf("goroutines before=%d after=%d", before, runtime.NumGoroutine())
		}
	}
}
