package app

import (
	"github.com/AlexGalhardo/hacker-tools/internal/domain"
	"runtime"
)

func Capabilities() []domain.Capability {
	all := []domain.Capability{{ID: "interfaces", State: "available", Detail: "Go interface inventory", Provider: "go/net"}, {ID: "diagnostics", State: "available", Detail: "DNS, bounded TCP, HTTP and TLS; redirects are not followed", Provider: "go/net"}, {ID: "discovery", State: "available", Detail: "IPv4 TCP connect, /24 or narrower, at most 32 ports", Provider: "go/net"}, {ID: "capture_import", State: "available", Detail: "PCAP and PCAPNG, 64 MiB and 10000 decoded packets", Provider: "gopacket"}}
	for _, id := range []string{"routes_counters", "icmp", "wifi", "capture_live"} {
		c := domain.Capability{ID: id, State: "unsupported", Provider: runtime.GOOS, Detail: "Provider is not implemented"}
		if runtime.GOOS == "windows" {
			c.State = "available"
			c.Detail = "Native Windows API; operational permission checks occur on request"
			if id == "capture_live" {
				c.State = "permission_required"
				c.Detail = "Administrator permission; raw IPv4 only, no Ethernet frames or drop counts"
			}
		} else if runtime.GOOS == "linux" && id == "routes_counters" {
			c.State = "available"
			c.Detail = "procfs counters and IPv4 default route"
		}
		all = append(all, c)
	}
	return all
}
