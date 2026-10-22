//go:build !windows && !linux

package platform

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func (Host) Neighbors(ctx context.Context) (network.NeighborTable, error) {
	result := network.NeighborTable{Neighbors: []network.Neighbor{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, domain.Unsupported("neighbor cache provider is not implemented on this platform")
}
