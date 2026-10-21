package fake_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/porttest"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

func TestFakeStoreContracts(t *testing.T) {
	t.Run("jobs", func(t *testing.T) {
		porttest.JobStoreContract(t, func(*testing.T) app.JobStore { return fake.NewStore() })
	})
	t.Run("settings", func(t *testing.T) {
		porttest.SettingsStoreContract(t, func(*testing.T) app.SettingsStore { return fake.NewStore() })
	})
}

func TestFakeCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	store := fake.NewStore()
	for _, test := range []struct {
		name string
		run  func() error
	}{
		{"put", func() error { return store.PutJob(ctx, job.Status{}) }},
		{"history", func() error { _, err := store.History(ctx); return err }},
		{"clear", func() error { return store.Clear(ctx) }},
		{"retain", func() error { return store.Retain(ctx, time.Now()) }},
		{"settings", func() error { _, err := store.Settings(ctx); return err }},
		{"save settings", func() error { return store.SaveSettings(ctx, app.Settings{}) }},
		{"host", func() error { _, err := (fake.Host{}).Inventory(ctx); return err }},
		{"tools", func() error { _, err := (fake.Tools{}).RunNmap(ctx, "", nil); return err }},
		{"wifi", func() error { _, err := (fake.WiFi{}).Current(ctx); return err }},
		{"reveal", func() error { _, err := (fake.WiFi{}).Reveal(ctx); return err }},
		{"prober", func() error { _, err := (fake.Prober{}).Run(ctx, "", false); return err }},
		{"capture", func() error { _, _, err := (fake.LiveCapture{}).Capture(ctx, capture.LiveRequest{}); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, context.Canceled) || errors.Unwrap(err) == nil {
				t.Fatalf("cancellation not wrapped: %v", err)
			}
		})
	}
	if got := (fake.Tools{}).Inventory(ctx); got != nil {
		t.Fatal(got)
	}
}

func TestFakeReturnedValuesAreCopies(t *testing.T) {
	t.Run("host", func(t *testing.T) {
		f := fake.Host{Snapshot: app.HostSnapshot{Adapters: []app.HostAdapter{{Name: "one", Gateways: []string{"192.0.2.1"}, DNS: []string{"192.0.2.53"}}}, Counters: []network.Counter{{ID: "one"}}}}
		v, err := f.Inventory(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		v.Adapters[0].Gateways[0] = "changed"
		v.Adapters[0].DNS[0] = "changed"
		v.Counters[0].ID = "changed"
		again, _ := f.Inventory(t.Context())
		if again.Adapters[0].Gateways[0] != "192.0.2.1" || again.Adapters[0].DNS[0] != "192.0.2.53" || again.Counters[0].ID != "one" {
			t.Fatal("host data aliases")
		}
	})
	t.Run("tools", func(t *testing.T) {
		f := fake.Tools{Tools: []app.Tool{{Name: "nmap", State: "available"}}, Result: app.Nmap{Hosts: []app.NmapHost{{Addresses: []app.NmapAddress{{Address: "192.0.2.1"}}, Ports: []app.NmapPort{{Port: 80}}}}}}
		v, err := f.RunNmap(t.Context(), "192.0.2.0/24", []int{80})
		if err != nil {
			t.Fatal(err)
		}
		v.Hosts[0].Addresses[0].Address = "changed"
		v.Hosts[0].Ports[0].Port = 9
		inventory := f.Inventory(t.Context())
		inventory[0].Name = "changed"
		again, _ := f.RunNmap(t.Context(), "192.0.2.0/24", []int{80})
		if again.Hosts[0].Addresses[0].Address != "192.0.2.1" || again.Hosts[0].Ports[0].Port != 80 || f.Tools[0].Name != "nmap" {
			t.Fatal("tool data aliases")
		}
	})
	t.Run("wifi", func(t *testing.T) {
		channel := uint32(6)
		f := fake.WiFi{Network: wifi.Network{Channel: &channel}}
		v, _ := f.Current(t.Context())
		*v.Channel = 11
		if channel != 6 {
			t.Fatal("channel aliases")
		}
	})
	t.Run("prober", func(t *testing.T) {
		f := fake.Prober{Values: []string{"response"}}
		v, _ := f.Run(t.Context(), "127.0.0.1", false)
		v[0] = "changed"
		if f.Values[0] != "response" {
			t.Fatal("probe values alias")
		}
		if f.Provider() != "fake" {
			t.Fatal("missing provider")
		}
	})
	t.Run("store", func(t *testing.T) {
		s := fake.NewStore()
		finished := time.Now().UTC()
		want := finished
		if err := s.PutJob(t.Context(), job.Status{ID: "one", State: job.Completed, Created: finished, Finished: &finished}); err != nil {
			t.Fatal(err)
		}
		finished = finished.Add(time.Hour)
		rows, err := s.History(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if !rows[0].Finished.Equal(want) {
			t.Fatal("input timestamp aliases")
		}
		*rows[0].Finished = finished
		again, _ := s.History(t.Context())
		if !again[0].Finished.Equal(want) {
			t.Fatal("output timestamp aliases")
		}
	})
}

func TestFakePortContracts(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "data"
		var err error
		if failed {
			name = "honest_state"
			err = domain.Unsupported("synthetic provider")
		}
		t.Run(name, func(t *testing.T) {
			t.Run("host", func(t *testing.T) {
				porttest.HostNetworkContract(t, func(*testing.T) app.HostNetwork {
					return fake.Host{Snapshot: app.HostSnapshot{Adapters: []app.HostAdapter{{Name: "synthetic"}}, Counters: []network.Counter{{ID: "one"}}}, Err: err}
				})
			})
			t.Run("tools", func(t *testing.T) {
				state := "available"
				version := "synthetic"
				if failed {
					state = "unsupported"
					version = ""
				}
				porttest.ToolRunnerContract(t, func(*testing.T) app.ToolRunner {
					return fake.Tools{Tools: []app.Tool{{Name: "nmap", State: state, Version: version}, {Name: "tshark", State: state, Version: version}}, Err: err}
				})
			})
			t.Run("wifi", func(t *testing.T) {
				porttest.WiFiProviderContract(t, func(*testing.T) app.WiFiProvider {
					return fake.WiFi{Network: wifi.Network{State: "connected"}, Secret: wifi.Secret{State: "no_saved_key"}, Err: err}
				})
			})
			t.Run("prober", func(t *testing.T) {
				porttest.ProberContract(t, func(*testing.T) network.Prober { return fake.Prober{Values: []string{"synthetic response"}, Err: err} })
			})
			t.Run("capture", func(t *testing.T) {
				porttest.LiveProviderContract(t, func(*testing.T) capture.LiveProvider {
					return fake.LiveCapture{Result: capture.LiveResult{Packets: []capture.Packet{}}, Err: err}
				})
			})
		})
	}
}
