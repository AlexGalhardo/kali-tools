package fake

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

type Tools struct {
	Tools  []app.Tool
	Result app.ServiceInspection
	Err    error
}

func (f Tools) Inventory(ctx context.Context) []app.Tool {
	if ctx.Err() != nil {
		return nil
	}
	return slices.Clone(f.Tools)
}
func (f Tools) InspectServices(ctx context.Context, cidr string, ports []int) (app.ServiceInspection, error) {
	if err := ctx.Err(); err != nil {
		return app.ServiceInspection{}, fmt.Errorf("fake tools: %w", err)
	}
	if err := network.ValidateDiscovery(cidr, ports, time.Second); err != nil {
		return app.ServiceInspection{}, err
	}
	v := f.Result
	v.Hosts = slices.Clone(v.Hosts)
	for i := range v.Hosts {
		v.Hosts[i].Addresses = slices.Clone(v.Hosts[i].Addresses)
		v.Hosts[i].Ports = slices.Clone(v.Hosts[i].Ports)
	}
	return v, f.Err
}
