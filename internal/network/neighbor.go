package network

import (
	"cmp"
	"net/netip"
	"slices"
	"strings"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const MaxNeighbors = 4096

type Neighbor struct {
	Address        string `json:"address"`
	MAC            string `json:"mac"`
	Family         string `json:"family"`
	InterfaceIndex uint32 `json:"interface_index"`
	InterfaceName  string `json:"interface_name"`
	State          string `json:"state"`
	Router         bool   `json:"router"`
}

type NeighborTable struct {
	Neighbors []Neighbor `json:"neighbors"`
	Limited   bool       `json:"limited"`
	Provider  string     `json:"provider"`
}

const (
	NeighborReachable   = "reachable"
	NeighborStale       = "stale"
	NeighborDelay       = "delay"
	NeighborProbe       = "probe"
	NeighborIncomplete  = "incomplete"
	NeighborUnreachable = "unreachable"
	NeighborPermanent   = "permanent"
	NeighborUnknown     = "unknown"
)

func ValidateNeighborFamily(family string) error {
	if family != "all" && family != "ipv4" && family != "ipv6" {
		return domain.Invalid("family must be all, ipv4 or ipv6")
	}
	return nil
}

func NormalizeNeighbors(rows []Neighbor, family string) NeighborTable {
	type candidate struct {
		row     Neighbor
		address netip.Addr
	}
	type identity struct {
		index   uint32
		address netip.Addr
	}
	seen := make(map[identity]struct{})
	usable := make([]candidate, 0, min(len(rows), MaxNeighbors))
	for _, row := range rows {
		address, err := netip.ParseAddr(row.Address)
		if err != nil || address.IsUnspecified() || address.IsLoopback() || address.IsMulticast() ||
			address == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
			continue
		}
		if strings.EqualFold(row.MAC, "ff:ff:ff:ff:ff:ff") || (family != "all" && row.Family != family) {
			continue
		}
		key := identity{row.InterfaceIndex, address}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		row.Address = address.String()
		usable = append(usable, candidate{row, address})
	}
	slices.SortFunc(usable, func(a, b candidate) int {
		if order := cmp.Compare(a.row.InterfaceIndex, b.row.InterfaceIndex); order != 0 {
			return order
		}
		if order := cmp.Compare(a.row.Family, b.row.Family); order != 0 {
			return order
		}
		return a.address.Compare(b.address)
	})
	result := NeighborTable{Neighbors: make([]Neighbor, min(len(usable), MaxNeighbors)), Limited: len(usable) > MaxNeighbors}
	for i := range result.Neighbors {
		result.Neighbors[i] = usable[i].row
	}
	return result
}
