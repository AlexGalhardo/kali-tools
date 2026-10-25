package acceptance

import (
	"context"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
)

type directDriver struct{ app *app.Application }

func newDirect(_ *testing.T, a *app.Application) Driver { return directDriver{app: a} }
func (d directDriver) Name() string                     { return "direct" }
func mappedError(err error) error {
	if err == nil {
		return nil
	}
	v := contract.FromError(err)
	return &v
}
func (d directDriver) Diagnose(ctx context.Context, r contract.DiagnosticRequest) (contract.DiagnosticResult, error) {
	v, err := d.app.Diagnose(ctx, r.Request())
	if err != nil {
		return contract.DiagnosticResult{}, mappedError(err)
	}
	return contract.FromDiagnosticResult(v), nil
}
func (d directDriver) Discover(ctx context.Context, cidr string, ports []int, timeout time.Duration) ([]contract.PortResult, error) {
	v, _, err := d.app.Discover(ctx, cidr, ports, timeout)
	if err != nil {
		return nil, mappedError(err)
	}
	return contract.FromPortResults(v), nil
}
func (d directDriver) History(ctx context.Context) ([]contract.Job, error) {
	v, err := d.app.History(ctx)
	if err != nil {
		return nil, mappedError(err)
	}
	return contract.FromStatuses(v), nil
}
func (d directDriver) ClearHistory(ctx context.Context) error {
	return mappedError(d.app.ClearHistory(ctx))
}
