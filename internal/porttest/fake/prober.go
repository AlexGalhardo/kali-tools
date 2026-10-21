package fake

import (
	"context"
	"fmt"
	"slices"

	"github.com/AlexGalhardo/kali-tools/internal/network"
)

type Prober struct {
	Values []string
	Err    error
}

func (Prober) Provider() string { return "fake" }
func (f Prober) Run(ctx context.Context, target string, _ bool) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fake prober: %w", err)
	}
	if err := network.ValidateHost(target); err != nil {
		return nil, err
	}
	return slices.Clone(f.Values), f.Err
}
