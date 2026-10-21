package porttest

import (
	"slices"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
)

func WiFiProviderContract(t *testing.T, newProvider func(*testing.T) app.WiFiProvider) {
	t.Helper()
	t.Run("current_or_honest_state", func(t *testing.T) {
		v, err := newProvider(t).Current(deadline(t))
		RequireHonestState(t, err, "unsupported", "permission_required", "no_connection", "location_consent_required")
		if err == nil && v.State == "" {
			t.Fatal("missing Wi-Fi state")
		}
	})
	t.Run("reveal_states", func(t *testing.T) {
		v, err := newProvider(t).Reveal(deadline(t))
		RequireHonestState(t, err, "unsupported", "permission_required", "no_connection", "location_consent_required")
		if err != nil {
			return
		}
		if !slices.Contains([]string{"available", "permission_required", "no_saved_key", "open_network", "enterprise"}, v.State) {
			t.Fatalf("unexpected reveal state %s", v.State)
		}
		if v.State != "available" && v.Value != "" {
			t.Fatal("key material present outside available reveal state")
		}
	})
}
