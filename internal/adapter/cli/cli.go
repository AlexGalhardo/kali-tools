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
	"strings"
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
		_, _ = fmt.Fprintln(errout, "Usage: kali-tools <version|doctor|interfaces|inventory|diagnose|discover|inspect|wifi|capture|import> [flags]\nAll output is versioned JSON; discover also accepts --format csv. Wi-Fi secrets are available only through the explicit desktop reveal action.")
		return exitInvalid
	}
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
			data = map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH, "capabilities": contract.FromCapabilities(application.Capabilities(ctx)), "tools": contract.FromTools(inventory), "capture": "Windows raw IPv4 requires administrator authorization; no external driver. Other platforms unavailable.", "wifi": "Windows WLAN API; explicit reveal only in desktop"}
		case "interfaces":
			value, e := application.Interfaces(ctx)
			data, err = contract.FromInterfaces(value), e
		case "inventory":
			value, e := application.Inventory(ctx)
			data, err = contract.FromInventory(value), e
		case "diagnose":
			value, e := application.Diagnose(ctx, network.Request{Kind: *kind, Target: *target, Port: *port, TimeoutMS: *timeout, RecordType: *recordType})
			data, err = contract.FromDiagnosticResult(value), e
		case "discover", "inspect":
			if *format != "json" && *format != "csv" {
				err = domain.Invalid("format must be json or csv")
				break
			}
			if len(*ports) > 191 || strings.Count(*ports, ",") >= 32 {
				err = domain.Invalid("1..32 ports required")
				break
			}
			var numbers []int
			for _, s := range strings.Split(*ports, ",") {
				n, e := strconv.Atoi(s)
				if e != nil {
					err = domain.Invalid("invalid port list")
					break
				}
				numbers = append(numbers, n)
			}
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
				request := capture.LiveRequest{Address: *target, DurationSeconds: *duration, PacketLimit: *limit}
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
	exitOK      = 0
	exitFailure = 1
	exitInvalid = 2
)

func exitCode(e contract.Error) int {
	if e.Code == domain.CodeInvalidInput {
		return exitInvalid
	}
	return exitFailure
}
