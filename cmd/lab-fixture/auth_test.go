package main

import (
	"net"
	"testing"
)

func TestFixtureIPv4IsNotLoopbackWhenALabAddressExists(t *testing.T) {
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	var addrs []net.Addr
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		found, err := iface.Addrs()
		if err != nil {
			continue
		}
		addrs = append(addrs, found...)
	}
	want := firstNonLoopbackIPv4(addrs)
	if want.IsLoopback() {
		t.Skip("this network has no lab IPv4")
	}
	if ip := fixtureIPv4(); !ip.Equal(want) {
		t.Fatalf("glue %v want %v", ip, want)
	}
}

func TestGlueUsesLabAddress(t *testing.T) {
	got := firstNonLoopbackIPv4([]net.Addr{
		&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)},
		&net.IPAddr{IP: net.ParseIP("fe80::1")},
		&net.IPNet{IP: net.IPv4(10, 88, 0, 2), Mask: net.CIDRMask(24, 32)},
	})
	if got.String() != "10.88.0.2" {
		t.Fatalf("glue %v", got)
	}
	if onlyLoopback := firstNonLoopbackIPv4([]net.Addr{
		&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)},
	}); !onlyLoopback.IsLoopback() {
		t.Fatalf("fallback %v", onlyLoopback)
	}
}
