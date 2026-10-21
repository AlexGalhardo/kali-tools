package probe

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/network"
)

// ICMP adapts the native provider to network.Prober.
type ICMP struct{}

func (ICMP) Provider() string { return Provider }
func (ICMP) Run(ctx context.Context, target string, trace bool) ([]string, error) {
	if err := network.ValidateHost(target); err != nil {
		return nil, err
	}
	return Run(ctx, target, trace)
}
