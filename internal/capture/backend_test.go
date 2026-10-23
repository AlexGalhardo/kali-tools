package capture

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestNativeBackendStatusAndIsolation(t *testing.T) {
	status := (Native{}).Status(t.Context())
	if status.ID != "capture_live" || !domain.IsCapabilityState(status.State) {
		t.Fatal(status)
	}
	for _, run := range []func() error{
		func() error {
			_, _, err := (Native{}).Capture(t.Context(), LiveRequest{Backend: "pktmon", DurationSeconds: 1, PacketLimit: 1})
			return err
		},
		func() error {
			_, err := Live(t.Context(), LiveRequest{Backend: "pktmon", DurationSeconds: 1, PacketLimit: 1})
			return err
		},
	} {
		var failure *domain.Error
		want := "unsupported"
		if runtime.GOOS == "windows" {
			want = "invalid_input"
		}
		if err := run(); !errors.As(err, &failure) || failure.Code != want {
			t.Fatalf("error=%v", err)
		}
	}
}

func TestLiveRequestBackendValidation(t *testing.T) {
	for _, test := range []struct {
		name, backend, address string
		valid                  bool
	}{
		{"pktmon all NICs", "pktmon", "", true},
		{"pktmon rejects address", "pktmon", "127.0.0.1", false},
		{"npcap IPv4 loopback", "npcap", "127.0.0.1", true},
		{"npcap IPv6 loopback", "npcap", "::1", true},
		{"npcap nonlocal", "npcap", "192.0.2.1", false},
		{"npcap invalid", "npcap", "not-an-address", false},
		{"libpcap IPv4 loopback", "libpcap", "127.0.0.1", true},
		{"libpcap IPv6 loopback", "libpcap", "::1", true},
		{"libpcap nonlocal", "libpcap", "192.0.2.1", false},
		{"raw loopback", "raw_socket", "127.0.0.1", false},
		{"default raw loopback", "", "127.0.0.1", false},
		{"unknown backend", "unknown", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateLiveRequest(LiveRequest{Backend: test.backend, Address: test.address, DurationSeconds: 1, PacketLimit: 1})
			if (err == nil) != test.valid {
				t.Fatalf("error=%v want valid=%v", err, test.valid)
			}
		})
	}
	for _, duration := range []int{0, 1, 60, 61} {
		for _, packets := range []int{0, 1, 10000, 10001} {
			t.Run(fmt.Sprintf("duration %d packets %d", duration, packets), func(t *testing.T) {
				err := ValidateLiveRequest(LiveRequest{Backend: "pktmon", DurationSeconds: duration, PacketLimit: packets})
				valid := duration >= 1 && duration <= 60 && packets >= 1 && packets <= 10000
				if (err == nil) != valid {
					t.Fatalf("error=%v want valid=%v", err, valid)
				}
			})
		}
	}
}
