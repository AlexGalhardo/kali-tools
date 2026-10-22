package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const persistTimeout = 5 * time.Second

func jobSummary(kind string, state job.State, detail string) string {
	label := "Diagnostic"
	if strings.HasPrefix(kind, "capture ") {
		label = "Capture " + strings.TrimPrefix(kind, "capture ")
	}
	switch kind {
	case "tls_audit":
		label = "TLS audit"
	case "discovery":
		label = "Discovery"
	case "nmap":
		label = "Service inspection"
	}
	summary := label + " " + string(state)
	if detail != "" && state.Terminal() {
		summary += ": " + detail
	}
	return summary
}

func (a *Application) StartDiagnostic(parent context.Context, request network.Request) (job.Status, error) {
	if err := network.Validate(request); err != nil {
		return job.Status{}, err
	}
	st, err := a.jobs.Start(request.Kind, func(ctx context.Context) (string, any, error) {
		result, err := a.diagnostics.Diagnose(ctx, request)
		return "", &result, err
	})
	return st, err
}

func (a *Application) CancelJob(ctx context.Context, id job.ID) bool { return a.jobs.Cancel(id) }

// Discover persists counts only; addresses and port results remain in memory.
func (a *Application) Discover(parent context.Context, cidr string, ports []int, timeout time.Duration) (results []network.PortResult, err error) {
	if err = network.ValidateDiscovery(cidr, ports, timeout); err != nil {
		return nil, err
	}
	err = a.runOperation(parent, "discovery", func(ctx context.Context) (string, error) {
		var err error
		results, err = network.Discover(ctx, cidr, ports, timeout)
		open := 0
		for _, result := range results {
			if result.State == "open" {
				open++
			}
		}
		return fmt.Sprintf("%d endpoints checked, %d open", len(results), open), err
	})
	return results, err
}

func (a *Application) InspectServices(parent context.Context, cidr string, ports []int) (result Nmap, err error) {
	if err = network.ValidateDiscovery(cidr, ports, time.Second); err != nil {
		return result, err
	}
	a.mu.Lock()
	closed := a.closed
	a.mu.Unlock()
	if closed {
		return result, job.ErrClosed
	}
	if a.deps.Tools == nil {
		return result, domain.Unsupported("tool runner is not configured")
	}
	err = a.runOperation(parent, "nmap", func(ctx context.Context) (string, error) {
		var err error
		result, err = a.deps.Tools.RunNmap(ctx, cidr, ports)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Nmap %s: %d hosts", result.Version, len(result.Hosts)), nil
	})
	return result, err
}

func (a *Application) runOperation(parent context.Context, kind string, operation func(context.Context) (string, error)) error {
	st, err := a.jobs.Start(kind, func(ctx context.Context) (string, any, error) {
		detail, err := operation(ctx)
		return detail, nil, err
	})
	if err != nil {
		return err
	}
	stop := context.AfterFunc(parent, func() { a.jobs.Cancel(st.ID) })
	defer stop()
	_, err = a.jobs.Wait(context.Background(), st.ID)
	return err
}
