package acceptance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
)

var errUnsupported = errors.New("driver does not support this operation yet")

type Driver interface {
	Name() string
	Diagnose(context.Context, contract.DiagnosticRequest) (contract.DiagnosticResult, error)
	Discover(context.Context, string, []int, time.Duration) ([]contract.PortResult, error)
	History(context.Context) ([]contract.Job, error)
	ClearHistory(context.Context) error
}
type driverFactory func(*testing.T, *app.Application) Driver
type scenarioDriver func(*testing.T) Driver

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)
	return ctx
}
func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	var mapped *contract.Error
	if !errors.As(err, &mapped) || mapped.Code != code {
		t.Fatalf("error=%v, want %s", err, code)
	}
}
