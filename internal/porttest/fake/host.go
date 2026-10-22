package fake

import (
	"context"
	"fmt"
	"slices"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

type Host struct {
	Snapshot      app.HostSnapshot
	Err           error
	InventoryFunc func(context.Context) (app.HostSnapshot, error)
	NeighborTable network.NeighborTable
	NeighborsFunc func(context.Context) (network.NeighborTable, error)
}

func (h Host) Neighbors(ctx context.Context) (network.NeighborTable, error) {
	if err := ctx.Err(); err != nil {
		return network.NeighborTable{}, fmt.Errorf("fake host neighbors: %w", err)
	}
	v, err := h.NeighborTable, h.Err
	if h.NeighborsFunc != nil {
		v, err = h.NeighborsFunc(ctx)
	}
	result := network.NormalizeNeighbors(v.Neighbors, "all")
	result.Provider, result.Limited = v.Provider, result.Limited || v.Limited
	return result, err
}

func (h Host) Inventory(ctx context.Context) (app.HostSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return app.HostSnapshot{}, fmt.Errorf("fake host: %w", err)
	}
	v, err := h.Snapshot, h.Err
	if h.InventoryFunc != nil {
		v, err = h.InventoryFunc(ctx)
	}
	v.Adapters = slices.Clone(v.Adapters)
	for i := range v.Adapters {
		v.Adapters[i].Gateways = slices.Clone(v.Adapters[i].Gateways)
		v.Adapters[i].DNS = slices.Clone(v.Adapters[i].DNS)
	}
	v.Counters = slices.Clone(v.Counters)
	return v, err
}
