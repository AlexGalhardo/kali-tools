package app

import (
	"context"
	"fmt"

	"github.com/AlexGalhardo/kali-tools/internal/localdiscovery"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func (a *Application) DiscoverLocal(ctx context.Context, r localdiscovery.Request) (result localdiscovery.Result, err error) {
	ifaces, err := network.Interfaces()
	if err != nil {
		return result, err
	}
	if err = localdiscovery.Validate(r, ifaces); err != nil {
		return result, err
	}
	err = a.runOperation(ctx, "local_discovery", func(ctx context.Context) (string, error) {
		var opErr error
		result, opErr = localdiscovery.Discover(ctx, r)
		return fmt.Sprintf("%d responders, %d out of scope", len(result.Responders), result.OutOfScope), opErr
	})
	return result, err
}

func (a *Application) QuerySNMP(ctx context.Context, r localdiscovery.SNMPRequest) (result localdiscovery.SNMPResult, err error) {
	ifaces, err := network.Interfaces()
	if err != nil {
		return result, err
	}
	if err = localdiscovery.ValidateSNMP(r, ifaces); err != nil {
		return result, err
	}
	err = a.runOperation(ctx, "snmp", func(ctx context.Context) (string, error) {
		var opErr error
		result, opErr = localdiscovery.QuerySNMP(ctx, r)
		return fmt.Sprintf("%d interfaces", len(result.Interfaces)), opErr
	})
	return result, err
}
