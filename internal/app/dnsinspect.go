package app

import (
	"context"
	"fmt"

	"github.com/AlexGalhardo/kali-tools/internal/dnsinspect"
)

func (a *Application) InspectDNS(ctx context.Context, r dnsinspect.Request) (report dnsinspect.Report, err error) {
	if err := dnsinspect.Validate(r); err != nil {
		return report, err
	}
	err = a.runOperation(ctx, "dns_inspect", func(ctx context.Context) (string, error) {
		var inspectErr error
		report, inspectErr = dnsinspect.Inspect(ctx, r, a.systemDNS(ctx))
		return fmt.Sprintf("%d queries, checks=%d", report.Queries, len(r.Checks)), inspectErr
	})
	return report, err
}

func (a *Application) systemDNS(ctx context.Context) []string {
	if a.deps.Host == nil {
		return nil
	}
	snap, err := a.deps.Host.Inventory(ctx)
	if err != nil {
		return nil
	}
	out := []string{}
	seen := map[string]struct{}{}
	for _, adapter := range snap.Adapters {
		for _, server := range adapter.DNS {
			if _, ok := seen[server]; ok {
				continue
			}
			seen[server] = struct{}{}
			out = append(out, server)
		}
	}
	return out
}
