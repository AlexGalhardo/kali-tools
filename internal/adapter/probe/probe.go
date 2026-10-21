package probe

import "context"

// ICMP adapts the native provider to network.Prober.
type ICMP struct{}

func (ICMP) Provider() string { return Provider }
func (ICMP) Run(ctx context.Context, target string, trace bool) ([]string, error) {
	return Run(ctx, target, trace)
}
