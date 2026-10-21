package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

type cliDriver struct {
	t   *testing.T
	app *app.Application
}

func newCLI(t *testing.T, a *app.Application) Driver { return cliDriver{t: t, app: a} }
func (cliDriver) Name() string                       { return "cli" }
func cliResult[T any](d cliDriver, ctx context.Context, args []string) (T, error) {
	d.t.Helper()
	var out, stderr bytes.Buffer
	code := cli.Run(ctx, d.app, append([]string{"--json"}, args...), &out, &stderr)
	var response struct {
		SchemaVersion int             `json:"schema_version"`
		Version       string          `json:"version"`
		Data          json.RawMessage `json:"data"`
		Error         *contract.Error `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		d.t.Fatalf("decode CLI: %v; stderr=%s", err, stderr.String())
	}
	if response.SchemaVersion != 1 || response.Version != domain.Version {
		d.t.Fatal("invalid CLI envelope versions")
	}
	wantCode := 0
	if response.Error != nil {
		wantCode = cli.ExitCode(response.Error.Code)
	}
	if code != wantCode {
		d.t.Fatalf("exit=%d, want=%d", code, wantCode)
	}
	var result T
	if response.Error != nil {
		return result, response.Error
	}
	if err := json.Unmarshal(response.Data, &result); err != nil {
		d.t.Fatal(err)
	}
	return result, nil
}
func (d cliDriver) Diagnose(ctx context.Context, r contract.DiagnosticRequest) (contract.DiagnosticResult, error) {
	args := []string{"diagnose", "--kind", r.Kind, "--target", r.Target, "--port", strconv.Itoa(r.Port), "--timeout", strconv.Itoa(r.TimeoutMS)}
	if r.RecordType != "" {
		args = append(args, "--record", r.RecordType)
	}
	return cliResult[contract.DiagnosticResult](d, ctx, args)
}
func (d cliDriver) Discover(ctx context.Context, cidr string, ports []int, timeout time.Duration) ([]contract.PortResult, error) {
	numbers := make([]string, len(ports))
	for i, p := range ports {
		numbers[i] = strconv.Itoa(p)
	}
	return cliResult[[]contract.PortResult](d, ctx, []string{"discover", "--target", cidr, "--ports", strings.Join(numbers, ","), "--timeout", strconv.FormatInt(timeout.Milliseconds(), 10)})
}
func (d cliDriver) History(ctx context.Context) ([]contract.Job, error) {
	return cliResult[[]contract.Job](d, ctx, []string{"history", "list"})
}
func (d cliDriver) ClearHistory(ctx context.Context) error {
	_, err := cliResult[struct{ Cleared bool }](d, ctx, []string{"history", "clear"})
	return err
}
