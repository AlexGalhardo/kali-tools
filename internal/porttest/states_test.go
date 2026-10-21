package porttest

import (
	"errors"
	"fmt"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestRequireHonestState(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, true}, {"allowed", domain.Unsupported("x"), true}, {"wrapped allowed", fmt.Errorf("wrapped: %w", domain.Unsupported("x")), true}, {"other", domain.Invalid("x"), false}, {"plain", errors.New("x"), false}, {"wrapped plain", fmt.Errorf("wrapped: %w", errors.New("x")), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := honestState(test.err, "unsupported") == nil; got != test.want {
				t.Fatalf("accepted=%v, want=%v", got, test.want)
			}
		})
	}
}
