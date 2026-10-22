//go:build windows

package platform

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

var procGetIpNetTable2 = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetIpNetTable2")

type mibIpNetRow2 struct {
	Address               windows.RawSockaddrInet
	InterfaceIndex        uint32
	InterfaceLuid         uint64
	PhysicalAddress       [32]byte
	PhysicalAddressLength uint32
	State                 uint32
	Flags                 uint8
	_                     [3]byte
	ReachabilityTime      uint32
}

type mibIpNetTable2 struct {
	NumEntries uint32
	_          uint32
	Table      [1]mibIpNetRow2
}

const (
	errorNotFound   = 1168
	afUnspec        = 0
	providerWindows = "Windows GetIpNetTable2"
)

func (Host) Neighbors(ctx context.Context) (network.NeighborTable, error) {
	empty := network.NeighborTable{Neighbors: []network.Neighbor{}, Provider: providerWindows}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if err := procGetIpNetTable2.Find(); err != nil {
		return empty, domain.Unsupported("neighbor cache API is unavailable")
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return empty, &domain.Error{Code: domain.CodeFailed, Message: "neighbor interface names could not be read"}
	}
	names := make(map[uint32]string, len(interfaces))
	for _, iface := range interfaces {
		if iface.Index > 0 {
			names[uint32(iface.Index)] = iface.Name
		}
	}
	var table *mibIpNetTable2
	result, _, _ := procGetIpNetTable2.Call(afUnspec, uintptr(unsafe.Pointer(&table)))
	if result == errorNotFound {
		return empty, nil
	}
	if result != 0 {
		return empty, &domain.Error{Code: domain.CodeFailed, Message: fmt.Sprintf("GetIpNetTable2 returned %d", result)}
	}
	if table == nil {
		return empty, &domain.Error{Code: domain.CodeFailed, Message: "neighbor cache API returned no table"}
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if table.NumEntries > maxRawNeighborRows {
		return empty, &domain.Error{Code: domain.CodeFailed, Message: "neighbor table exceeds limit"}
	}
	rows := make([]network.Neighbor, 0, int(table.NumEntries))
	for _, raw := range unsafe.Slice(&table.Table[0], int(table.NumEntries)) {
		if row, ok := neighborFromRow(&raw, names); ok {
			rows = append(rows, row)
		}
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	normalized := network.NormalizeNeighbors(rows, "all")
	normalized.Provider = providerWindows
	return normalized, nil
}

func neighborFromRow(row *mibIpNetRow2, names map[uint32]string) (network.Neighbor, bool) {
	if row.PhysicalAddressLength > maxNeighborMACBytes {
		return network.Neighbor{}, false
	}
	var address netip.Addr
	family := "ipv4"
	switch row.Address.Family {
	case windows.AF_INET:
		address = netip.AddrFrom4((*windows.RawSockaddrInet4)(unsafe.Pointer(&row.Address)).Addr)
	case windows.AF_INET6:
		address = netip.AddrFrom16((*windows.RawSockaddrInet6)(unsafe.Pointer(&row.Address)).Addr)
		family = "ipv6"
		if address.IsLinkLocalUnicast() && names[row.InterfaceIndex] != "" {
			address = address.WithZone(names[row.InterfaceIndex])
		}
	default:
		return network.Neighbor{}, false
	}
	state := network.NeighborUnknown
	states := [...]string{network.NeighborUnreachable, network.NeighborIncomplete, network.NeighborProbe,
		network.NeighborDelay, network.NeighborStale, network.NeighborReachable, network.NeighborPermanent}
	if row.State < uint32(len(states)) {
		state = states[row.State]
	}
	return network.Neighbor{
		Address: address.String(), MAC: neighborMAC(row.PhysicalAddress[:row.PhysicalAddressLength]), Family: family,
		InterfaceIndex: row.InterfaceIndex, InterfaceName: names[row.InterfaceIndex], State: state, Router: row.Flags&1 != 0,
	}, true
}
