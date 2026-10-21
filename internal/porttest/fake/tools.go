package fake

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

type Tools struct {
	Tools  []app.Tool
	Result app.Nmap
	Err    error
}

func (f Tools) Inventory(ctx context.Context) []app.Tool {
	if ctx.Err() != nil {
		return nil
	}
	return slices.Clone(f.Tools)
}
func (f Tools) RunNmap(ctx context.Context, cidr string, ports []int) (app.Nmap, error) {
	if err := ctx.Err(); err != nil {
		return app.Nmap{}, fmt.Errorf("fake tools: %w", err)
	}
	if err := network.ValidateDiscovery(cidr, ports, time.Second); err != nil {
		return app.Nmap{}, err
	}
	for _, tool := range f.Tools {
		if tool.Name == "nmap" && tool.State != "available" {
			return app.Nmap{}, &domain.Error{Code: tool.State, Message: "synthetic tool unavailable"}
		}
	}
	v := f.Result
	v.Hosts = slices.Clone(v.Hosts)
	for i := range v.Hosts {
		v.Hosts[i].Addresses = slices.Clone(v.Hosts[i].Addresses)
		v.Hosts[i].Ports = slices.Clone(v.Hosts[i].Ports)
	}
	return v, f.Err
}
