package pcap

import (
	"context"
	"fmt"
	"net/netip"
	"runtime"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const NpcapDownloadURL = domain.NpcapDownloadURL
const maxVersionInfoBytes = 64 << 10

const (
	NpcapSnapLen     = 65535
	NpcapReadTimeout = 250 * time.Millisecond
	NpcapBufferBytes = 4 << 20
	MaxPcapDevices   = 256
)

type device struct {
	Name      string
	Addresses []netip.Addr
}

func selectDevice(devices []device, address netip.Addr) (string, error) {
	return selectDeviceMessage(devices, address, "address does not belong to an Npcap device")
}

func selectDeviceMessage(devices []device, address netip.Addr, missing string) (string, error) {
	if len(devices) > MaxPcapDevices {
		return "", &domain.Error{Code: domain.CodeFailed, Message: "device list exceeds limit"}
	}
	for _, d := range devices {
		for _, candidate := range d.Addresses {
			if candidate.WithZone("") == address.WithZone("") {
				return d.Name, nil
			}
		}
	}
	return "", &domain.Error{Code: domain.CodeInvalidInput, Message: missing}
}

type NpcapStatus struct {
	Installed       bool   `json:"installed"`
	LegacyWinPcap   bool   `json:"legacy_winpcap"`
	Version         string `json:"version"`
	AdminOnly       bool   `json:"admin_only"`
	LoopbackSupport bool   `json:"loopback_support"`
}

type npcapProbe struct {
	ServiceKey, WpcapDLL, PacketDLL, LegacyWpcap bool
	AdminOnly, LoopbackSupport                   uint32
	Version                                      string
}

func statusFromProbe(p npcapProbe) NpcapStatus {
	installed := p.ServiceKey && p.WpcapDLL && p.PacketDLL
	return NpcapStatus{Installed: installed, LegacyWinPcap: !installed && p.LegacyWpcap,
		Version: p.Version, AdminOnly: p.AdminOnly == 1, LoopbackSupport: p.LoopbackSupport == 1}
}

func capability(s NpcapStatus, built, elevated bool) domain.Capability {
	c := domain.Capability{ID: "capture_npcap", Provider: "npcap"}
	switch {
	case !s.Installed && s.LegacyWinPcap:
		c.State, c.Detail = domain.CapabilityUnsupported, "WinPcap is not supported; install Npcap yourself from "+NpcapDownloadURL
	case !s.Installed:
		c.State, c.Detail = domain.CapabilityDependencyMissing, "Npcap is not installed. kali-tools never bundles or installs it; install it yourself from "+NpcapDownloadURL
	case !built:
		c.State, c.Detail = domain.CapabilityUnsupported, fmt.Sprintf("Npcap %s detected; this build does not include the Npcap backend (build tag pcap)", s.Version)
	case s.AdminOnly && !elevated:
		c.State, c.Detail = domain.CapabilityPermissionRequired, "Npcap is installed in Administrators-only mode; start kali-tools as Administrator"
	default:
		c.State, c.Detail = domain.CapabilityAvailable, fmt.Sprintf("Npcap %s; Ethernet frames with drop counters", s.Version)
	}
	return c
}

func validVersionInfoSize(size uint32) bool { return size > 0 && size <= maxVersionInfoBytes }

type Npcap struct {
	detect   func() (NpcapStatus, error)
	elevated func() bool
}

func NewNpcap() *Npcap { return &Npcap{detect: DetectNpcap, elevated: isElevated} }

func (n *Npcap) Status(ctx context.Context) domain.Capability {
	if ctx.Err() != nil {
		return domain.Capability{ID: "capture_npcap", Provider: "npcap", State: domain.CapabilityFailed, Detail: "Npcap detection cancelled"}
	}
	if runtime.GOOS != "windows" {
		return domain.Capability{ID: "capture_npcap", Provider: "npcap", State: domain.CapabilityUnsupported, Detail: "Npcap capture exists only on Windows in this build"}
	}
	status, err := n.detect()
	if err != nil {
		return domain.Capability{ID: "capture_npcap", Provider: "npcap", State: domain.CapabilityFailed, Detail: "Npcap detection failed"}
	}
	return capability(status, backendBuilt, n.elevated())
}
