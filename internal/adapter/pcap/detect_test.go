package pcap

import (
	"context"
	"errors"
	"net/netip"
	"runtime"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
)

func TestNpcapStatusMapping(t *testing.T) {
	installed := npcapProbe{ServiceKey: true, WpcapDLL: true, PacketDLL: true, Version: "1.79.0.0"}
	for _, test := range []struct {
		name            string
		probe           npcapProbe
		built, elevated bool
		state           string
	}{
		{"missing", npcapProbe{}, false, false, "dependency_missing"},
		{"legacy only", npcapProbe{LegacyWpcap: true}, true, true, "unsupported"},
		{"partial install", npcapProbe{ServiceKey: true, WpcapDLL: true}, true, true, "dependency_missing"},
		{"not built", installed, false, true, "unsupported"},
		{"built", installed, true, false, "available"},
		{"administrator required", npcapProbe{ServiceKey: true, WpcapDLL: true, PacketDLL: true, AdminOnly: 1}, true, false, "permission_required"},
		{"administrator allowed", npcapProbe{ServiceKey: true, WpcapDLL: true, PacketDLL: true, AdminOnly: 1, LoopbackSupport: 1}, true, true, "available"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status := statusFromProbe(test.probe)
			got := capability(status, test.built, test.elevated)
			if got.ID != "capture_npcap" || got.Provider != "npcap" || got.State != test.state {
				t.Fatal(got)
			}
			if got.State == "dependency_missing" && !strings.Contains(got.Detail, "https://npcap.com/#download") {
				t.Fatal("missing install link")
			}
			if status.AdminOnly != (test.probe.AdminOnly == 1) || status.LoopbackSupport != (test.probe.LoopbackSupport == 1) {
				t.Fatal(status)
			}
		})
	}
}

func TestVersionInfoSizeBound(t *testing.T) {
	for _, size := range []uint32{0, 1, 65536, 65537} {
		if got := validVersionInfoSize(size); got != (size > 0 && size <= 65536) {
			t.Fatal(size, got)
		}
	}
}

func TestNpcapDefaultStatusAndStub(t *testing.T) {
	n := NewNpcap()
	status := n.Status(t.Context())
	if status.ID != "capture_npcap" {
		t.Fatal(status)
	}
	if runtime.GOOS != "windows" && status.State != "unsupported" {
		t.Fatal(status)
	}
	n.detect = func() (NpcapStatus, error) { return NpcapStatus{}, errors.New("synthetic private DLL path") }
	if got := n.Status(t.Context()); runtime.GOOS == "windows" && (got.State != "failed" || strings.Contains(got.Detail, "synthetic")) {
		t.Fatal(got)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if n.Status(ctx).State != "failed" {
		t.Fatal("cancelled status probe ignored")
	}
	if _, _, err := n.Capture(t.Context(), capture.LiveRequest{}); err == nil {
		t.Fatal("stub captured")
	}
	if _, _, err := n.Capture(ctx, capture.LiveRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestNpcapDeviceLimitAndSelection(t *testing.T) {
	for _, count := range []int{256, 257} {
		devices := make([]device, count)
		devices[0] = device{Name: "fixture", Addresses: []netip.Addr{netip.MustParseAddr("fe80::1%test")}}
		got, err := selectDevice(devices, netip.MustParseAddr("fe80::1%other"))
		if count == 256 && (err != nil || got != "fixture") {
			t.Fatal(got, err)
		}
		if count == 257 && err == nil {
			t.Fatal("oversized device list accepted")
		}
	}
	if _, err := selectDevice(nil, netip.MustParseAddr("127.0.0.1")); err == nil {
		t.Fatal("missing device accepted")
	}
}
