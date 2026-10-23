//go:build linux

package platform

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestNetlinkDefaultRoutesAndLinkCounters(t *testing.T) {
	oif := make([]byte, 4)
	binary.NativeEndian.PutUint32(oif, 2)
	route := rtBody(2, 0, neighborAttr(4, oif), neighborAttr(5, []byte{192, 0, 2, 1}))
	specific := rtBody(2, 24)
	raw := append(neighborMessage(24, route), neighborMessage(24, specific)...)
	gateways, err := parseNetlinkRoutes(raw)
	if err != nil || len(gateways[2]) != 1 || gateways[2][0] != "192.0.2.1" {
		t.Fatalf("routes %+v %v", gateways, err)
	}
	v6gw := []byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	v6 := rtBody(10, 0, neighborAttr(4, oif), neighborAttr(5, v6gw))
	gateways, err = parseNetlinkRoutes(neighborMessage(24, v6))
	if err != nil || gateways[2][0] != "2001:db8::1" {
		t.Fatalf("ipv6 %+v %v", gateways, err)
	}
	stats := make([]byte, 32)
	binary.NativeEndian.PutUint64(stats[16:], 100)
	binary.NativeEndian.PutUint64(stats[24:], 40)
	link := rtBody(0, 0)
	link = append(link, make([]byte, 4)...)
	link = append(link, neighborAttr(3, append([]byte("eth0"), 0))...)
	link = append(link, neighborAttr(23, stats)...)
	counters, err := parseNetlinkLinks(neighborMessage(16, link))
	if err != nil || len(counters) != 1 || counters[0].ID != "eth0" || counters[0].Received != 100 || counters[0].Sent != 40 {
		t.Fatalf("links %+v %v", counters, err)
	}
	if _, err = parseNetlinkRoutes([]byte{1, 2, 3}); err == nil {
		t.Fatal("truncated route dump accepted")
	}
	var invalid *domain.Error
	if _, err = parseNetlinkLinks(bytesRepeat(maxNetlinkBytes + 1)); err == nil || !errors.As(err, &invalid) || invalid.Code != domain.CodeInvalidInput {
		t.Fatal(err)
	}
}

func TestProcRouteAndCounterFallback(t *testing.T) {
	routes := parseProcRoutes("Iface Destination Gateway\neth0 00000000 010200C0 0003 0 0 0 00000000 0 0 0\neth0 0101A8C0 00000000 0001 0 0 0 00FFFFFF 0 0 0\n")
	if len(routes["eth0"]) != 1 || routes["eth0"][0] != "192.0.2.1" {
		t.Fatal(routes)
	}
	counters, err := parseProcCounters("Inter-|   Receive                                                |  Transmit\n face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets\neth0: 9 1 0 0 0 0 0 0 8 1 0 0 0 0 0 0\n")
	if err != nil || len(counters) != 1 || counters[0].ID != "eth0" || counters[0].Received != 9 || counters[0].Sent != 8 {
		t.Fatalf("%+v %v", counters, err)
	}
	if _, err = parseProcCounters("eth0: not-a-number 0 0 0 0 0 0 0 1 0 0 0 0 0 0 0\n"); err == nil {
		t.Fatal("bad counter accepted")
	}
}

func TestLinuxInventoryReturnsAdapters(t *testing.T) {
	snapshot, err := Inventory()
	if err != nil || len(snapshot.Adapters) == 0 || snapshot.Counters == nil {
		t.Fatalf("%+v %v", snapshot, err)
	}
	for _, adapter := range snapshot.Adapters {
		if strings.Contains(adapter.Name, "\n") {
			t.Fatal(adapter.Name)
		}
	}
}

func rtBody(family, dstLen byte, attrs ...[]byte) []byte {
	header := make([]byte, 12)
	header[0], header[1] = family, dstLen
	body := append([]byte{}, header...)
	for _, attr := range attrs {
		body = append(body, attr...)
	}
	return body
}

func bytesRepeat(n int) []byte {
	return make([]byte, n)
}
