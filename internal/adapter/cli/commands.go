package cli

import (
	"context"
	"fmt"
	"io"

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
	{path: "wifi reveal", synopsis: "Print the saved key only with --confirm", needsApp: true, flags: []string{"confirm"}},
	{path: "jobs list", synopsis: "List active jobs; --all includes terminal history", needsApp: true, flags: []string{"all"}},
	{path: "jobs show", synopsis: "Show a job by its 32-character ID", needsApp: true},
	{path: "jobs cancel", synopsis: "Cancel a job started in this process", needsApp: true},
	{path: "history list", synopsis: "List redacted diagnostic history", needsApp: true},
	{path: "history clear", synopsis: "Clear terminal history; keep active jobs", needsApp: true},
	{path: "settings get", synopsis: "Show language and retention settings", needsApp: true},
	{path: "settings set", synopsis: "Update language or retention settings", needsApp: true, flags: []string{"language", "retention-days"}},
	{path: "capture live", synopsis: "Capture bounded local IPv4 traffic", needsApp: true, flags: []string{"target", "duration", "limit", "output", "format"}},
	{path: "capture import", synopsis: "Import a PCAP or PCAPNG file", needsApp: true, flags: []string{"file", "output", "format"}},
	{path: "capture export", synopsis: "Convert a packet file to an explicit destination", needsApp: true, flags: []string{"file", "output", "format"}},
	{path: "serve", synopsis: "Serve the shared UI (implemented by W01/W04)"},
	{path: "capture", synopsis: "Capture bounded local IPv4 traffic", needsApp: true, flags: []string{"target", "duration", "limit", "output", "format"}},
	{path: "import", synopsis: "Import a PCAP or PCAPNG file", needsApp: true, flags: []string{"file", "output", "format"}},
}

func runAdditional(ctx context.Context, a *app.Application, call invocation, stderr io.Writer) (any, error, bool) {
	switch call.command.path {
	case "serve":
		return nil, domain.Invalid("serve is handled by the kali-tools binary"), true
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
	case "history clear":
		err := a.ClearHistory(ctx)
		return map[string]bool{"cleared": err == nil}, err, true
	case "jobs cancel":
		if !a.CancelJob(ctx, job.ID(call.arg)) {
			return nil, &domain.Error{Code: domain.CodeNotFound, Message: "job is not running in this process; cancel it from the interface that started it"}, true
		}
		return map[string]any{"id": call.arg, "cancelled": true}, nil, true
	case "settings set":
		if !call.set["language"] && !call.set["retention-days"] {
			return nil, domain.Invalid("nothing to set"), true
		}
		value, err := a.Settings(ctx)
		if err != nil {
			return nil, err, true
		}
		if call.set["language"] {
			value.Language = call.values["language"]
		}
		if call.set["retention-days"] {
			_, err = fmt.Sscan(call.values["retention-days"], &value.RetentionDays)
			if err != nil {
				return nil, domain.Invalid("invalid value for --retention-days"), true
			}
		}
		if err := a.SaveSettings(ctx, value); err != nil {
			return nil, err, true
		}
		return contract.FromSettings(value), nil, true
	case "wifi reveal":
		if call.values["confirm"] != "true" {
			return nil, domain.Invalid("wifi reveal prints the saved network key; re-run with --confirm"), true
		}
		if _, err := fmt.Fprintln(stderr, label(call.g.Lang, "warning: the saved Wi-Fi key will be printed to standard output. Terminal scrollback, shell logs and SSH session recordings may retain it.")); err != nil {
			return nil, err, true
		}
		value, err := a.RevealWiFi(ctx)
		return contract.FromWiFiSecret(value), err, true
	case "capture export":
		file, output, format := call.values["file"], call.values["output"], call.values["format"]
		if file == "" || output == "" || (format != "pcap" && format != "pcapng") {
			return nil, domain.Invalid("capture export requires --file, --output and --format pcap or pcapng"), true
		}
		if _, err := a.ImportCapture(ctx, file); err != nil {
			return nil, err, true
		}
		if err := a.ExportCapture(ctx, output, format); err != nil {
			return nil, err, true
		}
		return map[string]string{"path": output, "format": format}, nil, true
	default:
		return nil, nil, false
	}
}

func deprecation(call invocation) (string, string) {
	switch call.command.path {
	case "wifi":
		return "wifi", "wifi show"
	case "capture":
		return "capture", "capture live"
	case "import":
		return "import", "capture import"
	case "diagnose":
		if call.set["kind"] {
			return "diagnose --kind", "diagnose <kind>"
		}
	case "discover", "inspect":
		if call.set["format"] && call.values["format"] == "json" {
			return call.command.path + " --format json", "--json"
		}
	}
	return "", ""
}
