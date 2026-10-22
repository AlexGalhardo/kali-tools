//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func TestMibIpNetRow2Layout(t *testing.T) {
	if size := unsafe.Sizeof(mibIpNetRow2{}); size != 88 {
		t.Fatal(size)
	}
	row := mibIpNetRow2{}
	for _, test := range []struct {
		name      string
		got, want uintptr
	}{
		{"index", unsafe.Offsetof(row.InterfaceIndex), 28},
		{"luid", unsafe.Offsetof(row.InterfaceLuid), 32},
		{"mac", unsafe.Offsetof(row.PhysicalAddress), 40},
		{"mac length", unsafe.Offsetof(row.PhysicalAddressLength), 72},
		{"state", unsafe.Offsetof(row.State), 76},
		{"flags", unsafe.Offsetof(row.Flags), 80},
		{"reachability", unsafe.Offsetof(row.ReachabilityTime), 84},
		{"table", unsafe.Offsetof(mibIpNetTable2{}.Table), 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatal(test.got, test.want)
			}
		})
	}
}

func TestNeighborFromRow(t *testing.T) {
	row := mibIpNetRow2{InterfaceIndex: 7, PhysicalAddressLength: 6, PhysicalAddress: [32]byte{2, 0, 0, 0, 0, 1}, State: 5}
	*(*windows.RawSockaddrInet4)(unsafe.Pointer(&row.Address)) = windows.RawSockaddrInet4{Family: windows.AF_INET, Addr: [4]byte{192, 0, 2, 1}}
	got, ok := neighborFromRow(&row, map[uint32]string{7: "fixture"})
	if !ok || got.Address != "192.0.2.1" || got.Family != "ipv4" || got.MAC != "02:00:00:00:00:01" || got.State != "reachable" || got.InterfaceName != "fixture" {
		t.Fatal(got, ok)
	}
	row.Address = windows.RawSockaddrInet{}
	*(*windows.RawSockaddrInet6)(unsafe.Pointer(&row.Address)) = windows.RawSockaddrInet6{Family: windows.AF_INET6, Addr: [16]byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}}
	row.State, row.Flags = 4, 1
	got, ok = neighborFromRow(&row, map[uint32]string{7: "fixture"})
	if !ok || got.Address != "fe80::1%fixture" || got.Family != "ipv6" || got.State != "stale" || !got.Router {
		t.Fatal(got, ok)
	}
	for _, length := range []uint32{0, 32, 33} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			row.PhysicalAddressLength = length
			got, ok := neighborFromRow(&row, nil)
			if ok != (length <= 32) || (length == 0 && got.MAC != "") {
				t.Fatal(got, ok)
			}
		})
	}
	row.PhysicalAddressLength = 6
	clear(row.PhysicalAddress[:])
	if got, ok := neighborFromRow(&row, nil); !ok || got.MAC != "" {
		t.Fatal(got, ok)
	}
	for state, want := range []string{"unreachable", "incomplete", "probe", "delay", "stale", "reachable", "permanent", "unknown"} {
		row.State = uint32(state)
		got, ok := neighborFromRow(&row, nil)
		if !ok || got.State != want {
			t.Fatal(got, ok)
		}
	}
	row.Address.Family = 99
	if _, ok := neighborFromRow(&row, nil); ok {
		t.Fatal("unknown address family accepted")
	}
}

func TestNativeNeighbors(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (Host{}).Neighbors(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	table, err := (Host{}).Neighbors(t.Context())
	if err != nil || table.Neighbors == nil || len(table.Neighbors) > network.MaxNeighbors {
		t.Fatal(len(table.Neighbors), err)
	}
	for _, row := range table.Neighbors {
		if _, err := netip.ParseAddr(row.Address); err != nil {
			t.Fatal("invalid address in native neighbor table")
		}
	}
	t.Logf("read-only host smoke: %d normalized rows; hardware comparison not performed", len(table.Neighbors))
}
