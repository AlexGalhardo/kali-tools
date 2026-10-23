package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/dnsinspect"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/tlsaudit"
)

type command struct {
	path     string
	synopsis string
	needsApp bool
	flags    []string
}

var commands = []command{
	{path: "tls-audit", synopsis: "Report TLS chain, protocols and certificate findings", needsApp: true, flags: []string{"target", "port", "server-name", "lint"}},
	{path: "version", synopsis: "Show version and build platform"},
	{path: "doctor", synopsis: "Show capabilities and optional dependencies", needsApp: true},
	{path: "capabilities", synopsis: "List capabilities and availability states", needsApp: true},
	{path: "tools", synopsis: "List separately installed optional tools", needsApp: true},
	{path: "neighbors", synopsis: "Read the passive OS ARP/ND cache", needsApp: true, flags: []string{"family", "format"}},
	{path: "interfaces", synopsis: "List local network interfaces", needsApp: true},
	{path: "inventory", synopsis: "Show routes, DNS and traffic counters", needsApp: true},
	{path: "diagnose", synopsis: "diagnose [dns|tcp|http|tls|ping|trace] [flags]", needsApp: true, flags: []string{"target", "kind", "port", "record", "redirects"}},
	{path: "discover", synopsis: "Discover a bounded IPv4 scope", needsApp: true, flags: []string{"target", "ports", "format"}},
	{path: "inspect", synopsis: "Inspect services on an explicit IPv4 scope", needsApp: true, flags: []string{"target", "ports", "format"}},
	{path: "dns-inspect", synopsis: "Compare resolvers and check SOA, NS, DNSSEC, DoH and DoT", needsApp: true, flags: []string{"target", "record", "resolvers", "doh", "dot", "checks", "trust-anchor"}},
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
	{path: "capture live", synopsis: "Capture bounded traffic with an optional backend", needsApp: true, flags: []string{"backend", "target", "duration", "limit", "output", "format"}},
	{path: "capture import", synopsis: "Import a PCAP or PCAPNG file", needsApp: true, flags: []string{"file", "output", "format"}},
	{path: "capture export", synopsis: "Convert a packet file to an explicit destination", needsApp: true, flags: []string{"file", "output", "format"}},
	{path: "serve", synopsis: "Serve the shared UI (implemented by W01/W04)"},
	{path: "capture", synopsis: "Capture bounded traffic with an optional backend", needsApp: true, flags: []string{"backend", "target", "duration", "limit", "output", "format"}},
	{path: "import", synopsis: "Import a PCAP or PCAPNG file", needsApp: true, flags: []string{"file", "output", "format"}},
}

func runAdditional(ctx context.Context, a *app.Application, call invocation, stderr io.Writer) (any, error, bool) {
	switch call.command.path {
	case "tls-audit":
		port, _ := strconv.Atoi(call.values["port"])
		timeout := call.g.Timeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}
		value, err := a.AuditTLS(ctx, tlsaudit.Request{Host: call.values["target"], Port: port, ServerName: call.values["server-name"], TimeoutMS: int(timeout.Milliseconds()), Lint: call.values["lint"] == "true"})
		if err != nil {
			return nil, err, true
		}
		return contract.FromTLSAudit(value), nil, true
	case "dns-inspect":
		timeout := call.g.Timeout
		if timeout == 0 {
			timeout = 2 * time.Second
		}
		resolvers, err := splitList(call.values["resolvers"])
		if err != nil {
			return nil, err, true
		}
		doh, err := splitList(call.values["doh"])
		if err != nil {
			return nil, err, true
		}
		dot, err := splitList(call.values["dot"])
		if err != nil {
			return nil, err, true
		}
		checks, err := splitList(call.values["checks"])
		if err != nil {
			return nil, err, true
		}
		var anchors []string
		if call.values["trust-anchor"] != "" {
			if len(call.values["trust-anchor"]) > 1024 {
				return nil, domain.Invalid("list is too long"), true
			}
			anchors = []string{call.values["trust-anchor"]}
		}
		value, err := a.InspectDNS(ctx, dnsinspect.Request{
			Name: call.values["target"], Type: call.values["record"], Resolvers: resolvers, DoH: doh, DoT: dot,
			Checks: checks, TrustAnchors: anchors, TimeoutMS: int(timeout.Milliseconds()),
		})
		if err != nil {
			return nil, err, true
		}
		return contract.FromDNSInspect(value), nil, true
	case "serve":
		return nil, domain.Invalid("serve is handled by the kali-tools binary"), true
	case "capabilities":
		return contract.FromCapabilities(a.Capabilities(ctx)), nil, true
	case "tools":
		value, err := a.Tools(ctx)
		return contract.FromTools(value), err, true
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

func splitList(value string) ([]string, error) {
	if len(value) > 1024 {
		return nil, domain.Invalid("list is too long")
	}
	if value == "" {
		return nil, nil
	}
	return strings.Split(value, ","), nil
}
