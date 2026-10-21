package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

type envelope struct {
	SchemaVersion int           `json:"schema_version"`
	Version       string        `json:"version"`
	Data          any           `json:"data,omitempty"`
	Error         *domain.Error `json:"error,omitempty"`
}

// cliDataPath preserves the CLI's disk-free behavior until C01.
const cliDataPath = ":memory:"

func run(ctx context.Context, args []string, getenv func(string) string, out, errout io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(errout, "Usage: kali-tools <version|doctor|interfaces|inventory|diagnose|discover|inspect|wifi|capture|import> [flags]\nAll output is versioned JSON; discover also accepts --format csv. Wi-Fi secrets are available only through the explicit desktop reveal action.")
		return 2
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
		return 2
	}
	if flags.NArg() != 0 {
		err = domain.Invalid("unexpected positional arguments")
	} else {
		application, cleanup, bootstrapErr := bootstrap.New(ctx, bootstrap.Config{DataPath: cliDataPath})
		if bootstrapErr != nil {
			err = bootstrapErr
		} else {
			defer func() { _ = cleanup() }()
			switch args[0] {
			case "version":
				data = map[string]string{"version": domain.Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH}
			case "doctor":
				inventory, _ := application.Tools(ctx)
				data = map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH, "capabilities": application.Capabilities(ctx), "tools": inventory, "capture": "Windows raw IPv4 requires administrator authorization; no external driver. Other platforms unavailable.", "wifi": "Windows WLAN API; explicit reveal only in desktop"}
			case "interfaces":
				data, err = application.Interfaces(ctx)
			case "inventory":
				data, err = application.Inventory(ctx)
			case "diagnose":
				data, err = application.Diagnose(ctx, network.Request{Kind: *kind, Target: *target, Port: *port, TimeoutMS: *timeout, RecordType: *recordType})
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
					data, err = application.InspectServices(ctx, *target, numbers)
					break
				}
				if err == nil {
					var results []network.PortResult
					results, err = application.Discover(ctx, *target, numbers, time.Duration(*timeout)*time.Millisecond)
					data = results
					if err == nil && *format == "csv" {
						writer := csv.NewWriter(out)
						if e := writer.Write([]string{"schema_version", "host", "port", "state"}); e != nil {
							return 1
						}
						for _, r := range results {
							if e := writer.Write([]string{"1", r.Host, strconv.Itoa(r.Port), r.State}); e != nil {
								return 1
							}
						}
						writer.Flush()
						if writer.Error() != nil {
							return 1
						}
						return 0
					}
				}
			case "wifi":
				data, err = application.CurrentWiFi(ctx)
			case "capture", "import":
				if *output != "" && *format != "pcap" && *format != "pcapng" {
					err = domain.Invalid("packet export requires --format pcap or pcapng")
					break
				}
				if args[0] == "capture" {
					request := capture.LiveRequest{Address: *target, DurationSeconds: *duration, PacketLimit: *limit}
					data, err = runCapture(ctx, application, request)
				} else {
					data, err = application.ImportCapture(ctx, *file)
				}
				if err == nil && *output != "" {
					err = application.ExportCapture(ctx, *output, *format)
				}
			default:
				err = domain.Invalid("unknown command %q", args[0])
			}
		}
	}
	response := envelope{SchemaVersion: domain.SchemaVersion, Version: domain.Version, Data: data}
	code := 0
	if err != nil {
		code = 1
		var typed *domain.Error
		if errors.As(err, &typed) {
			response.Error = typed
			if typed.Code == "invalid_input" {
				code = 2
			}
		} else if errors.Is(err, context.Canceled) {
			response.Error = &domain.Error{Code: "cancelled", Message: "Operation cancelled"}
		} else {
			response.Error = &domain.Error{Code: "failed", Message: err.Error()}
		}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if enc.Encode(response) != nil {
		return 1
	}
	return code
}

const captureWaitSlack = 10 * time.Second

func runCapture(ctx context.Context, application *app.Application, request capture.LiveRequest) (capture.LiveResult, error) {
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, _ := application.Subscribe(subCtx)
	if err := application.StartCapture(ctx, request); err != nil {
		return capture.LiveResult{}, err
	}
	return awaitCapture(ctx, events, func() { application.StopCapture(context.Background()) }, time.Duration(request.DurationSeconds)*time.Second+captureWaitSlack)
}

func awaitCapture(ctx context.Context, events <-chan job.Event, stop func(), limit time.Duration) (capture.LiveResult, error) {
	timer := time.NewTimer(limit)
	defer timer.Stop()
	stopOnce := sync.OnceFunc(stop)
	cancelled := ctx.Done()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return capture.LiveResult{}, job.ErrClosed
			}
			if event.Topic != job.TopicCapture {
				continue
			}
			outcome, ok := event.Data.(app.CaptureEvent)
			if ok {
				return outcome.Result, outcome.Err
			}
		case <-cancelled:
			stopOnce()
			cancelled = nil
		case <-timer.C:
			stopOnce()
			return capture.LiveResult{}, &domain.Error{Code: "timeout", Message: "Capture completion timed out"}
		}
	}
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	cancel()
	os.Exit(code)
}
