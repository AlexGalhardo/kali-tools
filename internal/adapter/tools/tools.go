package tools

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
)

type Tool = app.Tool

func inspect(ctx context.Context, name string) Tool {
	item := Tool{Name: name, State: "dependency_missing"}
	path, err := resolve(name)
	if err != nil {
		return item
	}
	item.Path = path
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	raw, err := runCommand(probe, path, []string{"--version"}, 8192)
	if err != nil {
		item.State = "failed"
		return item
	}
	item.Version = strings.TrimSpace(strings.SplitN(string(raw), "\n", 2)[0])
	item.State = "available"
	if !supportedVersion(name, item.Version) {
		item.State = "unsupported"
	}
	return item
}

// resolve prefers the standard install location over PATH so a user-writable
// PATH entry cannot shadow the binary (CWE-427).
func resolve(name string) (string, error) {
	if path := knownToolPath(name); path != "" {
		return path, nil
	}
	return exec.LookPath(name)
}

func Inventory(ctx context.Context) []Tool {
	return []Tool{inspect(ctx, "tshark")}
}

// Runner adapts optional executable inventory and native service inspection to the application port.
type Runner struct{}

func (Runner) Inventory(ctx context.Context) []app.Tool { return Inventory(ctx) }
func (Runner) InspectServices(ctx context.Context, cidr string, ports []int) (app.ServiceInspection, error) {
	return InspectServices(ctx, cidr, ports)
}
