package cli_test

import (
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

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
		Tools:       fake.Tools{Tools: []app.Tool{{Name: "nmap", State: "dependency_missing"}, {Name: "tshark", State: "dependency_missing"}}},
		Diagnostics: network.Diagnostics{Prober: fake.Prober{Values: []string{"echo reply"}}},
	})
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	return a
}
