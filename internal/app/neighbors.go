package app

import (
	"context"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const NeighborTimeout = 5 * time.Second

func (a *Application) Neighbors(ctx context.Context, family string) (network.NeighborTable, error) {
	empty := network.NeighborTable{Neighbors: []network.Neighbor{}}
	if err := network.ValidateNeighborFamily(family); err != nil {
		return empty, err
	}
	ctx, cancel := context.WithTimeout(ctx, NeighborTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if a.deps.Host == nil {
		return empty, domain.Unsupported("host network provider is not configured")
	}
	value, err := a.deps.Host.Neighbors(ctx)
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	if err != nil {
		return empty, err
	}
	result := network.NormalizeNeighbors(value.Neighbors, family)
	result.Provider = value.Provider
	result.Limited = result.Limited || value.Limited
	return result, nil
}
