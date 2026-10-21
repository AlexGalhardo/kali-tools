package cli

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

type command struct {
	path     string
	synopsis string
	needsApp bool
	flags    []string
}

var commands = []command{
	{path: "version", synopsis: "Show version and build platform"},
	{path: "doctor", synopsis: "Show capabilities and optional dependencies", needsApp: true},
	{path: "capabilities", synopsis: "List capabilities and availability states", needsApp: true},
	{path: "tools", synopsis: "List separately installed optional tools", needsApp: true},
	{path: "neighbors", synopsis: "Neighbor inventory (not implemented)"},
	{path: "interfaces", synopsis: "List local network interfaces", needsApp: true},
	{path: "inventory", synopsis: "Show routes, DNS and traffic counters", needsApp: true},
	{path: "diagnose", synopsis: "diagnose [dns|tcp|http|tls|ping|trace] [flags]", needsApp: true, flags: []string{"target", "kind", "port", "record"}},
	{path: "discover", synopsis: "Discover a bounded IPv4 scope", needsApp: true, flags: []string{"target", "ports", "format"}},
	{path: "inspect", synopsis: "Inspect services with separately installed Nmap", needsApp: true, flags: []string{"target", "ports", "format"}},
	{path: "wifi", synopsis: "Show current Wi-Fi metadata", needsApp: true},
	{path: "wifi show", synopsis: "Show current Wi-Fi metadata", needsApp: true},
	{path: "jobs list", synopsis: "List active jobs; --all includes terminal history", needsApp: true, flags: []string{"all"}},
	{path: "jobs show", synopsis: "Show a job by its 32-character ID", needsApp: true},
	{path: "history list", synopsis: "List redacted diagnostic history", needsApp: true},
	{path: "settings get", synopsis: "Show language and retention settings", needsApp: true},
	{path: "capture", synopsis: "Capture bounded local IPv4 traffic", needsApp: true, flags: []string{"target", "duration", "limit", "output", "format"}},
	{path: "import", synopsis: "Import a PCAP or PCAPNG file", needsApp: true, flags: []string{"file", "output", "format"}},
}

func runReadOnly(ctx context.Context, a *app.Application, call invocation) (any, error, bool) {
	switch call.command.path {
	case "capabilities":
		return contract.FromCapabilities(a.Capabilities(ctx)), nil, true
	case "tools":
		value, err := a.Tools(ctx)
		return contract.FromTools(value), err, true
	case "neighbors":
		return nil, domain.Unsupported("neighbor inventory is not implemented yet"), true
	case "wifi show":
		value, err := a.CurrentWiFi(ctx)
		return contract.FromWiFi(value), err, true
	case "history list", "jobs list":
		value, err := a.History(ctx)
		if call.command.path == "jobs list" && call.values["all"] != "true" {
			active := []job.Status{}
			for _, status := range value {
				if status.State == job.Queued || status.State == job.Running {
					active = append(active, status)
				}
			}
			value = active
		}
		return contract.FromStatuses(value), err, true
	case "jobs show":
		id := job.ID(call.arg)
		if value, ok := a.JobSnapshot(ctx, id); ok {
			return contract.FromStatus(value), nil, true
		}
		history, err := a.History(ctx)
		if err != nil {
			return nil, err, true
		}
		for _, value := range history {
			if value.ID == id {
				return contract.FromStatus(value), nil, true
			}
		}
		return nil, &domain.Error{Code: domain.CodeNotFound, Message: "job not found"}, true
	case "settings get":
		value, err := a.Settings(ctx)
		return contract.FromSettings(value), err, true
	default:
		return nil, nil, false
	}
}
