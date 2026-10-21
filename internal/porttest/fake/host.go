package fake

import (
	"context"
	"fmt"
	"slices"

	"github.com/AlexGalhardo/kali-tools/internal/app"
)

type Host struct {
	Snapshot      app.HostSnapshot
	Err           error
	InventoryFunc func(context.Context) (app.HostSnapshot, error)
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
