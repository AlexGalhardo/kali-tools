package porttest

import (
	"context"
	"net/netip"
	"reflect"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func HostNetworkContract(t *testing.T, newHost func(*testing.T) app.HostNetwork) {
	t.Helper()
	t.Run("neighbors_are_bounded_and_normalized", func(t *testing.T) {
		host := newHost(t)
		value, err := host.Neighbors(deadline(t))
		RequireHonestState(t, err, "unsupported", "permission_required")
		if err != nil {
			return
		}
		if value.Neighbors == nil || len(value.Neighbors) > network.MaxNeighbors {
			t.Fatal("invalid neighbor count")
		}
		normalized := network.NormalizeNeighbors(value.Neighbors, "all")
		if !reflect.DeepEqual(value.Neighbors, normalized.Neighbors) {
			t.Fatal("neighbor rows are not normalized")
		}
		for _, row := range value.Neighbors {
			address, err := netip.ParseAddr(row.Address)
			if err != nil || address.IsMulticast() {
				t.Fatal("invalid neighbor address")
			}
		}
	})
	t.Run("inventory_or_honest_state", func(t *testing.T) {
		host := newHost(t)
		v, err := host.Inventory(deadline(t))
		RequireHonestState(t, err, "unsupported", "permission_required")
		if err != nil {
			return
		}
		for _, c := range v.Counters {
			if c.ID == "" {
				t.Fatal("empty counter ID")
			}
		}
		for _, a := range v.Adapters {
			if a.Name == "" {
				t.Fatal("empty adapter name")
			}
		}
	})
	t.Run("cancelled_context_returns", func(t *testing.T) {
		host := newHost(t)
		ctx, cancel := context.WithCancel(deadline(t))
		cancel()
		_, _ = host.Inventory(ctx)
		if _, err := host.Neighbors(ctx); err == nil {
			t.Fatal("cancelled neighbor query succeeded")
		}
	})
}
