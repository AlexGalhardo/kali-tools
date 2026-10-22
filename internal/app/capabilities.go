package app

import (
	"runtime"
	"slices"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

// WithObserved overlays a known observation without mutating the input slice.
func WithObserved(all []domain.Capability, id, state, detail string) []domain.Capability {
	if state == "" {
		return all
	}
	for i, c := range all {
		if c.ID == id {
			result := slices.Clone(all)
			result[i].State, result[i].Detail = state, detail
			return result
		}
	}
	return all
}

func capabilities() []domain.Capability {
	all := []domain.Capability{{ID: "interfaces", State: domain.CapabilityAvailable, Detail: "Go interface inventory", Provider: "go/net"}, {ID: "diagnostics", State: domain.CapabilityAvailable, Detail: "DNS, bounded TCP, HTTP and TLS; redirects are not followed", Provider: "go/net"}, {ID: "discovery", State: domain.CapabilityAvailable, Detail: "IPv4 TCP connect, /24 or narrower, at most 32 ports", Provider: "go/net"}, {ID: "capture_import", State: domain.CapabilityAvailable, Detail: "PCAP and PCAPNG, 64 MiB and 10000 decoded packets", Provider: "gopacket"}}
	for _, id := range []string{"routes_counters", "icmp", "wifi", "capture_live"} {
		c := domain.Capability{ID: id, State: domain.CapabilityUnsupported, Provider: runtime.GOOS, Detail: "Provider is not implemented"}
		if runtime.GOOS == "windows" {
			c.State = domain.CapabilityAvailable
			c.Detail = "Native Windows API; operational permission checks occur on request"
			if id == "capture_live" {
				c.State = domain.CapabilityPermissionRequired
				c.Detail = "Administrator permission; raw IPv4 only, no Ethernet frames or drop counts"
			}
		} else if runtime.GOOS == "linux" && id == "routes_counters" {
			c.State = domain.CapabilityAvailable
			c.Detail = "procfs counters and IPv4 default route"
		}
		all = append(all, c)
	}
	return all
}
