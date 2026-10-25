package acceptance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/wailsui"
	"github.com/AlexGalhardo/kali-tools/internal/app"
)

type wailsDriver struct {
	t        *testing.T
	bindings *wailsui.Bindings
}

func newWails(t *testing.T, a *app.Application) Driver {
	b := wailsui.NewWithRuntime(a, wailsui.Runtime{Emit: func(context.Context, string, ...any) {}})
	b.Startup(t.Context())
	t.Cleanup(func() { b.Shutdown(t.Context()) })
	return wailsDriver{t: t, bindings: b}
}
func (wailsDriver) Name() string { return "wails" }
func (d wailsDriver) failure(err error) error {
	if err == nil {
		return nil
	}
	var mapped contract.Error
	if !errors.As(err, &mapped) {
		d.t.Fatalf("unmapped Wails error: %v", err)
	}
	return &mapped
}
func (d wailsDriver) Diagnose(_ context.Context, r contract.DiagnosticRequest) (contract.DiagnosticResult, error) {
	v, err := d.bindings.Diagnose(r)
	if err != nil {
		return contract.DiagnosticResult{}, d.failure(err)
	}
	return v, nil
}
func (d wailsDriver) Discover(_ context.Context, cidr string, ports []int, _ time.Duration) ([]contract.PortResult, error) {
	v, err := d.bindings.Discover(cidr, ports)
	if err != nil {
		return nil, d.failure(err)
	}
	return v.Results, nil
}
func (d wailsDriver) History(context.Context) ([]contract.Job, error) {
	v, err := d.bindings.History()
	if err != nil {
		return nil, d.failure(err)
	}
	return v, nil
}
func (d wailsDriver) ClearHistory(context.Context) error { return d.failure(d.bindings.ClearHistory()) }
