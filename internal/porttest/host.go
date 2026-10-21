package porttest

import (
	"context"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
)

func HostNetworkContract(t *testing.T, newHost func(*testing.T) app.HostNetwork) {
	t.Helper()
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
	})
}
