//go:build linux

package platform

import (
	"context"
	"net"
	"os"
	"syscall"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func (Host) Neighbors(ctx context.Context) (network.NeighborTable, error) {
	empty := network.NeighborTable{Neighbors: []network.Neighbor{}}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return empty, &domain.Error{Code: domain.CodeFailed, Message: "neighbor interface names could not be read"}
	}
	names := make(map[uint32]string, len(interfaces))
	indices := make(map[string]uint32, len(interfaces))
	for _, iface := range interfaces {
		if iface.Index > 0 {
			names[uint32(iface.Index)] = iface.Name
			indices[iface.Name] = uint32(iface.Index)
		}
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	raw, err := syscall.NetlinkRIB(syscall.RTM_GETNEIGH, syscall.AF_UNSPEC)
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	var rows []network.Neighbor
	provider := "Linux netlink RTM_GETNEIGH"
	if err == nil {
		rows, err = parseNetlinkNeighbors(raw, names)
	} else {
		provider = "Linux /proc/net/arp (IPv4 only)"
		file, openErr := os.Open("/proc/net/arp")
		if openErr != nil {
			return empty, &domain.Error{Code: domain.CodeFailed, Message: "neighbor cache could not be read"}
		}
		defer file.Close()
		rows, err = parseProcNetARP(file, indices)
	}
	if err != nil {
		return empty, err
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	result := network.NormalizeNeighbors(rows, "all")
	result.Provider = provider
	return result, nil
}
