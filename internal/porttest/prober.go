package porttest

import (
	"context"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func ProberContract(t *testing.T, newProber func(*testing.T) network.Prober) {
	t.Helper()
	t.Run("rejects_invalid_target", func(t *testing.T) {
		_, err := newProber(t).Run(deadline(t), "a;whoami", false)
		if err == nil {
			t.Fatal("invalid target accepted")
		}
		RequireHonestState(t, err, "invalid_input", "unsupported")
	})
	t.Run("loopback_or_honest_state", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(deadline(t), time.Second)
		defer cancel()
		v, err := newProber(t).Run(ctx, "127.0.0.1", false)
		RequireHonestState(t, err, "unsupported", "permission_required", "timeout")
		if err == nil && len(v) == 0 {
			t.Fatal("empty echo result")
		}
	})
}
