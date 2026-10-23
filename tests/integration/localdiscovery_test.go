//go:build integration

package integration

import (
	"context"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/localdiscovery"
	"github.com/AlexGalhardo/kali-tools/internal/localdiscovery/snmpfake"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func TestPrivateLabLocalDiscovery(t *testing.T) {
	host := os.Getenv("LAB_FIXTURE")
	if host == "" {
		t.Fatal("LAB_FIXTURE is required; use the private Docker lab")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	ip := fixtureIPv4(t, ctx, host)
	target, err := netip.ParseAddr(ip)
	if err != nil {
		t.Fatal(err)
	}
	ifaces, err := network.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	var eth network.InterfaceInfo
	found := false
	for _, iface := range ifaces {
		if iface.Name == "eth0" {
			eth, found = iface, true
		}
	}
	if !found {
		t.Fatalf("lab interface eth0 is missing, so multicast discovery cannot run: %+v", ifaces)
	}
	scope := netip.PrefixFrom(target, 24).Masked()
	overlaps := false
	for _, raw := range eth.Addresses {
		prefix, err := netip.ParsePrefix(raw)
		if err == nil && prefix.Overlaps(scope) {
			overlaps = true
		}
	}
	if !overlaps {
		t.Fatalf("fixture %s is outside a /24 that overlaps eth0 %v; multicast scope cannot be formed", ip, eth.Addresses)
	}
	discovered, err := localdiscovery.Discover(ctx, localdiscovery.Request{
		Scope:           localdiscovery.Scope{Interface: "eth0", CIDR: scope.String()},
		Protocols:       []string{"mdns", "ssdp"},
		DurationSeconds: 3,
		ServiceTypes:    []string{"_kali-lab._tcp"},
		SSDPTarget:      "upnp:rootdevice",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasResponder(discovered, "ssdp", "uuid:kali-tools-lab::upnp:rootdevice") || !hasResponder(discovered, "mdns", "fixture._kali-lab._tcp.local.") {
		t.Fatalf("multicast discovery failed in the lab: %+v", discovered)
	}
	secret, err := localdiscovery.NewSecret("lab-community")
	if err != nil {
		t.Fatal(err)
	}
	report, err := localdiscovery.QuerySNMP(ctx, localdiscovery.SNMPRequest{
		Scope: localdiscovery.Scope{Interface: "eth0", CIDR: scope.String()}, Target: ip, Port: 1161,
		Owned: true, Version: "v2c", Community: secret, TimeoutMS: 1000,
	})
	if err != nil || report.System["sysName"] != snmpfake.SysName {
		t.Fatalf("snmp: %v %+v", err, report.System)
	}
}

func hasResponder(result localdiscovery.Result, protocol, name string) bool {
	for _, item := range result.Responders {
		if item.Protocol == protocol && item.Name == name {
			return true
		}
	}
	return false
}
