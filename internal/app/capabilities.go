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
	all := []domain.Capability{{ID: "interfaces", State: domain.CapabilityAvailable, Detail: "Go interface inventory", Provider: "go/net"}, {ID: "diagnostics", State: domain.CapabilityAvailable, Detail: "DNS, bounded TCP, HTTP (phase timings, security headers, redirect target; explicit hop trace at most 5) and TLS; redirects are never followed automatically", Provider: "go/net"}, {ID: "discovery", State: domain.CapabilityAvailable, Detail: "IPv4 TCP connect, /24 or narrower, at most 32 ports", Provider: "go/net"}, {ID: "capture_import", State: domain.CapabilityAvailable, Detail: "PCAP and PCAPNG, 64 MiB and 10000 decoded packets", Provider: "gopacket"}}
	for _, id := range []string{"routes_counters", "icmp", "wifi"} {
		c := domain.Capability{ID: id, State: domain.CapabilityUnsupported, Provider: runtime.GOOS, Detail: "Provider is not implemented"}
		if runtime.GOOS == "windows" {
			c.State = domain.CapabilityAvailable
			c.Detail = "Native Windows API; operational permission checks occur on request"
		} else if runtime.GOOS == "linux" && id == "routes_counters" {
			c.State = domain.CapabilityAvailable
			c.Detail = "netlink routes and counters; procfs fallback"
		}
		all = append(all, c)
	}
	c := domain.Capability{ID: "neighbors", State: domain.CapabilityUnsupported, Provider: runtime.GOOS, Detail: "Provider is not implemented"}
	if runtime.GOOS == "windows" {
		c.State, c.Detail = domain.CapabilityAvailable, "OS ARP/ND cache via GetIpNetTable2; no packets sent"
	}
	if runtime.GOOS == "linux" {
		c.State, c.Detail = domain.CapabilityAvailable, "netlink neighbor table (procfs IPv4 fallback); no packets sent"
	}
	all = append(all, c)
	all = append(all, domain.Capability{ID: "tls_audit", State: domain.CapabilityAvailable, Provider: "go/crypto+zlint", Detail: "Chain, protocol matrix (TLS 1.0–1.3) and zlint; no exploit checks"})
	all = append(all, domain.Capability{ID: "dns_inspect", State: domain.CapabilityAvailable, Provider: "miekg/dns", Detail: "Resolver comparison, SOA/NS, DNSSEC chain, DoH/DoT; read-only"})
	all = append(all, domain.Capability{ID: "local_discovery", State: domain.CapabilityAvailable, Provider: "mdns+ssdp", Detail: "mDNS and SSDP on one interface, IPv4 /24 or narrower; read-only"})
	all = append(all, domain.Capability{ID: "snmp", State: domain.CapabilityAvailable, Provider: "gosnmp", Detail: "SNMP v2c/v3 GET/GETBULK of system and interface MIBs on owned devices"})
	return all
}
