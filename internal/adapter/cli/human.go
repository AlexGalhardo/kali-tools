package cli

import (
	"fmt"
	"io"
	"maps"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
)

func writeResponse(call invocation, out, stderr io.Writer, data any, err error) int {
	if call.g.JSON {
		return writeEnvelope(out, data, err)
	}
	if err != nil {
		call.values = maps.Clone(call.values)
		delete(call.values, "output")
		switch data.(type) {
		case contract.DiagnosticResult, contract.Capture, contract.LiveCapture:
		default:
			data = nil
		}
	}
	if data != nil {
		if renderHuman(out, call, data) != nil {
			return ExitFailed
		}
	}
	if err != nil {
		mapped := contract.FromError(err)
		if _, e := fmt.Fprintf(stderr, "%s: %s: %s\n", label(call.g.Lang, "error"), label(call.g.Lang, mapped.Code), mapped.Message); e != nil {
			return ExitFailed
		}
		if mapped.SettingsURI != "" {
			if _, e := fmt.Fprintf(stderr, "%s: %s\n", label(call.g.Lang, "Open"), mapped.SettingsURI); e != nil {
				return ExitFailed
			}
		}
		return ExitCode(mapped.Code)
	}
	return ExitOK
}

func renderHuman(out io.Writer, call invocation, data any) error {
	w := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	lang := call.g.Lang
	l := func(key string) string { return label(lang, key) }
	state := func(value string) string {
		text := l(value)
		file, ok := out.(*os.File)
		if !ok || call.g.NoColor {
			return text
		}
		info, err := file.Stat()
		if err != nil || info.Mode()&os.ModeCharDevice == 0 {
			return text
		}
		color := "33"
		switch value {
		case "available", "completed", "open", "connected":
			color = "32"
		case "failed", "closed":
			color = "31"
		}
		return "\x1b[" + color + "m" + text + "\x1b[0m"
	}
	var writeErr error
	line := func(format string, args ...any) {
		if writeErr == nil {
			_, writeErr = fmt.Fprintf(w, format+"\n", args...)
		}
	}
	header := func(value string) { line("%s", headers(lang, value)) }
	field := func(key string, value any) { line("%s: %v", l(key), value) }
	tools := func(values []contract.Tool) {
		header("NAME STATE VERSION PATH")
		for _, v := range values {
			line("%s\t%s\t%s\t%s", v.Name, state(v.State), v.Version, v.Path)
		}
	}
	capabilities := func(values []contract.Capability) {
		header("CAPABILITY STATE PROVIDER DETAIL")
		for _, v := range values {
			line("%s\t%s\t%s\t%s", v.ID, state(v.State), v.Provider, v.Detail)
		}
	}
	packets := func(values []contract.Packet, format string, limited bool) {
		suffix := ""
		if limited {
			suffix = ", " + l("limited")
		}
		line("%d %s, %s %s%s", len(values), l("packets"), l("format"), format, suffix)
		header("NO TIME LENGTH SOURCE DESTINATION PROTOCOL INFO")
		for _, v := range values {
			line("%d\t%s\t%d\t%s\t%s\t%s\t%s", v.Number, v.Timestamp.UTC().Format(time.RFC3339), v.Length, v.Source, v.Destination, v.Protocol, v.Info)
		}
	}
	switch v := data.(type) {
	case contract.LocalDiscoveryResult:
		line("%d %s", len(v.Responders), l("responders"))
		line("%s %d", l("out of scope"), v.OutOfScope)
		line("%s %d", l("malformed"), v.Malformed)
		header("PROTOCOL ADDRESS NAME TYPE")
		for _, item := range v.Responders {
			line("%s\t%s\t%s\t%s", item.Protocol, item.Address, item.Name, item.ServiceType)
		}
	case contract.SNMPResult:
		field("SNMP", v.Target)
		for _, key := range []string{"sysDescr", "sysObjectID", "sysUpTime", "sysContact", "sysName", "sysLocation"} {
			line("%s\t%s", key, v.System[key])
		}
		header("INDEX DESCR STATUS MAC")
		for _, row := range v.Interfaces {
			line("%d\t%s\t%s\t%s", row.Index, row.Descr, row.OperStatus, row.MAC)
		}
	case contract.DNSInspectReport:
		field("DNS inspection", v.Name+" "+v.Type)
		line("%d %s", v.Queries, l("queries"))
		if v.Compare != nil {
			line("%s %t", l("agree"), v.Compare.Agree)
			for _, result := range v.Compare.Results {
				line("%s\t%s\t%s", result.Resolver, result.Status, result.Transport)
			}
		}
		if v.SOANS != nil {
			line("%s %s", l("verdict"), v.SOANS.Verdict)
		}
		if v.DNSSEC != nil {
			line("%s %s", l("dnssec"), v.DNSSEC.Verdict)
		}
		for _, item := range v.Encrypted {
			line("%s\t%s\t%s", item.Protocol, item.Status, item.Endpoint)
		}
	case contract.TLSAuditReport:
		field("TLS audit", v.Host)
		line("%s / %s / %s: %t (%s)", v.NegotiatedVersion, v.CipherSuite, l("verified"), v.Verified, v.Verifier)
		for _, p := range v.Protocols {
			line("%s\t%s\t%s", p.Version, state(p.Status), p.Cipher)
		}
		for _, c := range v.Chain {
			line("%d\t%s\t%s\t%s", c.Index, c.Subject, c.Issuer, c.NotAfter.Format(time.RFC3339))
		}
		for _, f := range v.Findings {
			line("%s\t%d\t%s\t%s", l(f.Severity), f.CertIndex, f.ID, f.Detail)
		}
	case map[string]string:
		if call.command.path == "version" {
			line("kali-tools %s (%s %s/%s)", v["version"], v["go"], v["os"], v["arch"])
		} else {
			line("%s %s (%s)", l("saved"), v["path"], v["format"])
		}
	case map[string]any:
		if call.command.path == "doctor" {
			line("%s: %s/%s", l("System"), v["os"], v["arch"])
			if text, _ := v["baseline"].(string); text != "" {
				line("%s\n", l(text))
			} else {
				line("")
			}
			caps, _ := v["capabilities"].([]contract.Capability)
			capabilities(caps)
			line("")
			values, _ := v["tools"].([]contract.Tool)
			tools(values)
		} else {
			line("%s: %v", l("cancellation requested"), v["id"])
		}
	case map[string]bool:
		line("%s", l("history cleared (queued and running jobs are kept)"))
	case []contract.Capability:
		capabilities(v)
	case []contract.Tool:
		tools(v)
	case []contract.Interface:
		header("NAME INDEX UP MTU MAC ADDRESSES")
		for _, item := range v {
			line("%s\t%d\t%t\t%d\t%s\t%s", item.Name, item.Index, item.Up, item.MTU, item.MAC, strings.Join(item.Addresses, ", "))
		}
	case contract.Inventory:
		header("ADAPTER GATEWAYS DNS")
		for _, item := range v.Adapters {
			line("%s\t%s\t%s", item.Name, strings.Join(item.Gateways, ", "), strings.Join(item.DNS, ", "))
		}
		line("")
		header("COUNTER RECEIVED SENT")
		for _, item := range v.Counters {
			line("%s\t%d\t%d", item.ID, item.Received, item.Sent)
		}
	case contract.NeighborTable:
		line("%d %s (%s)", len(v.Neighbors), l("neighbors"), v.Provider)
		if v.Limited {
			line("%s", l("limited"))
		}
		header("INTERFACE FAMILY ADDRESS MAC STATE ROUTER")
		for _, row := range v.Neighbors {
			line("%s\t%s\t%s\t%s\t%s\t%t", row.InterfaceName, row.Family, row.Address, row.MAC, state(row.State), row.Router)
		}
	case contract.DiagnosticResult:
		line("%s %s: %s (%g ms, %s)", v.Kind, v.Target, state(v.Status), v.ElapsedMS, v.Provider)
		for _, value := range v.Values {
			line("  %s", value)
		}
	case []contract.PortResult:
		header("HOST PORT STATE")
		for _, item := range v {
			line("%s\t%d\t%s", item.Host, item.Port, state(item.State))
		}
	case contract.ServiceInspection:
		line("%s %s", v.Scanner, v.Version)
		header("HOST PORT STATE SERVICE METHOD")
		for _, host := range v.Hosts {
			addresses := []string{}
			for _, addr := range host.Addresses {
				addresses = append(addresses, addr.Address)
			}
			for _, port := range host.Ports {
				line("%s\t%d\t%s\t%s\t%s", strings.Join(addresses, ", "), port.Port, state(port.State.State), port.Service.Name, port.Service.Method)
			}
		}
	case contract.WiFi:
		field("SSID", v.SSID)
		field("BSSID", v.BSSID)
		field("profile", v.Profile)
		field("signal", fmt.Sprintf("%d%%", v.Signal))
		field("authentication", v.Authentication)
		if v.Channel != nil {
			field("channel", *v.Channel)
		}
		field("state", state(v.State))
	case contract.WiFiSecret:
		field("state", state(v.State))
		if v.Value != "" {
			field("key", v.Value)
		}
	case contract.Settings:
		field("language", v.Language)
		field("retention_days", v.RetentionDays)
	case []contract.Job:
		header("ID CREATED STATE KIND SUMMARY")
		for _, item := range v {
			line("%s\t%s\t%s\t%s\t%s", item.ID, item.Created.UTC().Format(time.RFC3339), state(item.State), item.Kind, item.Summary)
		}
	case contract.Job:
		field("id", v.ID)
		field("kind", v.Kind)
		field("state", state(v.State))
		field("created", v.Created.UTC().Format(time.RFC3339))
		if v.Finished != nil {
			field("finished", v.Finished.UTC().Format(time.RFC3339))
		}
		field("summary", v.Summary)
	case contract.Capture:
		packets(v.Packets, v.Format, v.Limited)
	case contract.LiveCapture:
		packets(v.Packets, "live", v.Limited)
	case contract.ConnectionTable:
		header("PROTOCOL LOCAL REMOTE STATE PID PROCESS")
		for _, row := range v.Connections {
			line("%s\t%s\t%s\t%s\t%d\t%s", row.Protocol, row.Local, row.Remote, row.State, row.PID, row.Process)
		}
		if v.Limited {
			line("%s", l("limited"))
		}
	case contract.ThroughputSample:
		field("endpoint", v.Endpoint)
		field("duration_ms", v.DurationMS)
		field("byte_count", v.ByteCount)
		field("source", v.Source)
	default:
		return fmt.Errorf("unsupported CLI presentation type %T", data)
	}
	if call.values["output"] != "" && call.command.path != "capture export" {
		line("%s %s (%s)", l("saved"), call.values["output"], call.values["format"])
	}
	if writeErr != nil {
		return writeErr
	}
	return w.Flush()
}
