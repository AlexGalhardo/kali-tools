package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

type envelope struct {
	SchemaVersion int             `json:"schema_version"`
	Version       string          `json:"version"`
	Data          any             `json:"data,omitempty"`
	Error         *contract.Error `json:"error,omitempty"`
}

func Run(ctx context.Context, application *app.Application, args []string, out, errout io.Writer) int {
	if len(args) == 0 {
		_ = writeHelp(errout, invocation{})
		return exitInvalid
	}
	call, err := parseInvocation(args)
	if err != nil {
		return writeResponse(call, out, errout, nil, err)
	}
	if call.help {
		if err := writeHelp(out, call); err != nil {
			return ExitFailed
		}
		return ExitOK
	}
	if old, replacement := deprecation(call); old != "" {
		if _, err := fmt.Fprintf(errout, "warning: %q is deprecated; use %q\n", old, replacement); err != nil {
			return writeResponse(call, out, errout, nil, err)
		}
	}
	if call.g.Timeout != 0 && call.command.path != "diagnose" && call.command.path != "discover" && call.command.path != "version" && call.command.path != "tls-audit" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, call.g.Timeout)
		defer cancel()
	}
	finishProgress := startProgress(ctx, application, call, errout)
	defer func() { _ = finishProgress() }()
	if data, err, handled := runAdditional(ctx, application, call, errout); handled {
		if progressErr := finishProgress(); progressErr != nil {
			return ExitFailed
		}
		return writeResponse(call, out, errout, data, err)
	}
	return runLegacy(ctx, application, call, out, errout, finishProgress)
}

func runLegacy(ctx context.Context, application *app.Application, call invocation, out, errout io.Writer, finishProgress func() error) int {
	args := call.legacy
	var data any
	var err error
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(errout)
	target := flags.String("target", "", "host, URL or CIDR")
	recordType := flags.String("record", "", "DNS record type: A, AAAA, CNAME, MX, NS, TXT or PTR")
	kind := flags.String("kind", "tcp", "dns, tcp, http, tls, ping or traceroute")
	port := flags.Int("port", 443, "TCP port")
	timeout := flags.Int("timeout", 3000, "timeout milliseconds")
	file := flags.String("file", "", "capture path")
	output := flags.String("output", "", "explicit packet-file destination; requires --format pcap or pcapng")
	ports := flags.String("ports", "80,443", "comma separated ports")
	format := flags.String("format", "json", "json/csv discovery; pcap/pcapng with --output")
	duration := flags.Int("duration", 5, "capture duration seconds")
	limit := flags.Int("limit", 1000, "capture packet limit")
	backend := flags.String("backend", "raw_socket", "capture backend: raw_socket, pktmon or npcap")
	family := flags.String("family", "all", "neighbor family: all, ipv4 or ipv6")
	if e := flags.Parse(args[1:]); e != nil {
		return exitInvalid
	}
	if flags.NArg() != 0 {
		err = domain.Invalid("unexpected positional arguments")
	} else {
		switch args[0] {
		case "version":
			data = map[string]string{"version": domain.Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH}
		case "doctor":
			inventory, _ := application.Tools(ctx)
			data = map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH, "capabilities": contract.FromCapabilities(application.Capabilities(ctx)), "tools": contract.FromTools(inventory), "capture": "Raw IPv4 and Pktmon require administrator authorization. Optional Npcap is user-installed from https://npcap.com/#download and requires a pcap-tagged build; it is never bundled or installed. See capture backend capabilities.", "wifi": "Windows WLAN API; key reveal requires wifi reveal --confirm or the desktop reveal action"}
		case "interfaces":
			value, e := application.Interfaces(ctx)
			data, err = contract.FromInterfaces(value), e
		case "inventory":
			value, e := application.Inventory(ctx)
			data, err = contract.FromInventory(value), e
		case "neighbors":
			if *format != "json" && *format != "csv" {
				err = domain.Invalid("format must be json or csv")
				break
			}
			value, e := application.Neighbors(ctx, *family)
			data, err = contract.FromNeighborTable(value), e
			if err == nil && *format == "csv" {
				writer := csv.NewWriter(out)
				if e := writer.Write([]string{
					"schema_version", "interface_index", "interface_name", "family", "address", "mac", "state", "router",
				}); e != nil {
					return exitFailure
				}
				for _, row := range value.Neighbors {
					if e := writer.Write([]string{
						strconv.Itoa(domain.SchemaVersion), strconv.FormatUint(uint64(row.InterfaceIndex), 10),
						row.InterfaceName, row.Family, row.Address, row.MAC, row.State, strconv.FormatBool(row.Router),
					}); e != nil {
						return exitFailure
					}
				}
				writer.Flush()
				if writer.Error() != nil {
					return exitFailure
				}
				return exitOK
			}
		case "diagnose":
			value, e := runDiagnostic(ctx, application, network.Request{Kind: *kind, Target: *target, Port: *port, TimeoutMS: *timeout, RecordType: *recordType})
			data, err = contract.FromDiagnosticResult(value), e
		case "discover", "inspect":
			if *format != "json" && *format != "csv" {
				err = domain.Invalid("format must be json or csv")
				break
			}
			numbers, parseErr := parsePorts(*ports)
			err = parseErr
			if err == nil && args[0] == "inspect" {
				if *format != "json" {
					err = domain.Invalid("service inspection supports JSON only")
					break
				}
				value, e := application.InspectServices(ctx, *target, numbers)
				data, err = contract.FromServiceInspection(value), e
				break
			}
			if err == nil {
				var results []network.PortResult
				results, err = application.Discover(ctx, *target, numbers, time.Duration(*timeout)*time.Millisecond)
				data = contract.FromPortResults(results)
				if err == nil && *format == "csv" {
					writer := csv.NewWriter(out)
					if e := writer.Write([]string{"schema_version", "host", "port", "state"}); e != nil {
						return exitFailure
					}
					for _, r := range results {
						if e := writer.Write([]string{"1", r.Host, strconv.Itoa(r.Port), r.State}); e != nil {
							return exitFailure
						}
					}
					writer.Flush()
					if writer.Error() != nil {
						return exitFailure
					}
					return exitOK
				}
			}
		case "wifi":
			value, e := application.CurrentWiFi(ctx)
			data, err = contract.FromWiFi(value), e
		case "capture", "import":
			if *output != "" && *format != "pcap" && *format != "pcapng" {
				err = domain.Invalid("packet export requires --format pcap or pcapng")
				break
			}
			if args[0] == "capture" {
				request := capture.LiveRequest{Backend: *backend, Address: *target, DurationSeconds: *duration, PacketLimit: *limit}
				value, e := runCapture(ctx, application, request)
				data, err = contract.FromLiveResult(value), e
			} else {
				value, e := application.ImportCapture(ctx, *file)
				data, err = contract.FromImportResult(value), e
			}
			if err == nil && *output != "" {
				err = application.ExportCapture(ctx, *output, *format)
			}
		default:
			err = domain.Invalid("unknown command %q", args[0])
		}
	}
	if progressErr := finishProgress(); progressErr != nil {
		return ExitFailed
	}
	return writeResponse(call, out, errout, data, err)
}

func writeEnvelope(out io.Writer, data any, err error) int {
	response := envelope{SchemaVersion: domain.SchemaVersion, Version: domain.Version, Data: data}
	code := exitOK
	if err != nil {
		mapped := contract.FromError(err)
		response.Error = &mapped
		code = exitCode(mapped)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if enc.Encode(response) != nil {
		return exitFailure
	}
	return code
}

const (
	ExitOK          = 0
	ExitFailed      = 1
	ExitUsage       = 2
	ExitUnsupported = 3
	ExitPermission  = 4
	ExitDependency  = 5
	ExitNotFound    = 6
	ExitTimeout     = 7
	ExitConflict    = 8
	ExitAuth        = 9
	ExitCancelled   = 130
	exitOK          = ExitOK
	exitFailure     = ExitFailed
	exitInvalid     = ExitUsage
)

func exitCode(e contract.Error) int {
	return ExitCode(e.Code)
}

// ExitCode maps a stable error code to the documented process exit status.
func ExitCode(code string) int {
	switch code {
	case "":
		return ExitOK
	case domain.CodeInvalidInput:
		return ExitUsage
	case domain.CodeUnsupported:
		return ExitUnsupported
	case domain.CodePermissionRequired, domain.CodeLocationConsentRequired:
		return ExitPermission
	case domain.CodeDependencyMissing:
		return ExitDependency
	case domain.CodeNotFound:
		return ExitNotFound
	case domain.CodeTimeout:
		return ExitTimeout
	case domain.CodeConflict:
		return ExitConflict
	case domain.CodeUnauthorized, domain.CodeForbidden:
		return ExitAuth
	case domain.CodeCancelled:
		return ExitCancelled
	default:
		return ExitFailed
	}
}
