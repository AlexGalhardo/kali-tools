package app

import (
	"context"
	"fmt"
	"strconv"
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
	case "dns_inspect":
		label = "DNS inspection"
	case "local_discovery":
		label = "Local discovery"
	case "snmp":
		label = "SNMP query"
	case "discovery":
		label = "Discovery"
	case "inspect":
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

// Discover persists counts and the exact scope only; addresses and port results remain in memory.
// truncated reports that the scan deadline cut the run and results are partial.
func (a *Application) Discover(parent context.Context, cidr string, ports []int, timeout time.Duration) (results []network.PortResult, truncated bool, err error) {
	if err = network.ValidateDiscovery(cidr, ports, timeout); err != nil {
		return nil, false, err
	}
	if err = a.requireLegalNotice(parent); err != nil {
		return nil, false, err
	}
	err = a.runOperation(parent, "discovery", func(ctx context.Context) (string, error) {
		var err error
		results, truncated, err = network.Discover(ctx, cidr, ports, timeout)
		open := 0
		for _, result := range results {
			if result.State == "open" {
				open++
			}
		}
		return fmt.Sprintf("%s ports %s: %d endpoints checked, %d open%s", cidr, portList(ports), len(results), open, truncatedNote(truncated)), err
	})
	return results, truncated, err
}

func portList(ports []int) string {
	text := make([]string, len(ports))
	for i, p := range ports {
		text[i] = strconv.Itoa(p)
	}
	return strings.Join(text, ",")
}

func truncatedNote(truncated bool) string {
	if truncated {
		return " (truncated at deadline)"
	}
	return ""
}

func (a *Application) InspectServices(parent context.Context, cidr string, ports []int) (result ServiceInspection, err error) {
	if err = network.ValidateDiscovery(cidr, ports, time.Second); err != nil {
		return result, err
	}
	if err = a.requireLegalNotice(parent); err != nil {
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
	err = a.runOperation(parent, "inspect", func(ctx context.Context) (string, error) {
		var err error
		result, err = a.deps.Tools.InspectServices(ctx, cidr, ports)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s %s: %s ports %s: %d hosts%s", result.Scanner, result.Version, cidr, portList(ports), len(result.Hosts), truncatedNote(result.Truncated)), nil
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
