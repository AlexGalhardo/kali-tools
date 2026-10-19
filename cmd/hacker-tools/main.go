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
	"time"

	"github.com/AlexGalhardo/hacker-tools/internal/app"
	"github.com/AlexGalhardo/hacker-tools/internal/capture"
	"github.com/AlexGalhardo/hacker-tools/internal/domain"
	"github.com/AlexGalhardo/hacker-tools/internal/network"
	"github.com/AlexGalhardo/hacker-tools/internal/platform"
	"github.com/AlexGalhardo/hacker-tools/internal/tools"
	"github.com/AlexGalhardo/hacker-tools/internal/wifi"
)

type envelope struct {
	SchemaVersion int           `json:"schema_version"`
	Version       string        `json:"version"`
	Data          any           `json:"data,omitempty"`
	Error         *domain.Error `json:"error,omitempty"`
}

func run(ctx context.Context, args []string, out, errout io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errout, "Usage: hacker-tools <version|doctor|interfaces|inventory|diagnose|discover|wifi|capture|import> [flags]\nAll output is versioned JSON; discover also accepts --format csv. Wi-Fi secrets are available only through the explicit desktop reveal action.")
		return 2
	}
	var data any
	var err error
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(errout)
	target := flags.String("target", "", "host, URL or CIDR")
	kind := flags.String("kind", "tcp", "dns, tcp, http or tls")
	port := flags.Int("port", 443, "TCP port")
	timeout := flags.Int("timeout", 3000, "timeout milliseconds")
	file := flags.String("file", "", "capture path")
	ports := flags.String("ports", "80,443", "comma separated ports")
	format := flags.String("format", "json", "json or csv")
	duration := flags.Int("duration", 5, "capture duration seconds")
	limit := flags.Int("limit", 1000, "capture packet limit")
	if e := flags.Parse(args[1:]); e != nil {
		return 2
	}
	if flags.NArg() != 0 {
		err = domain.Invalid("unexpected positional arguments")
	} else {
		switch args[0] {
		case "version":
			data = map[string]string{"version": domain.Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH}
		case "doctor":
			data = map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH, "capabilities": app.Capabilities(), "tools": tools.Inventory(ctx), "capture": "Windows raw IPv4 requires administrator authorization; no external driver. Other platforms unavailable.", "wifi": "Windows WLAN API; explicit reveal only in desktop"}
		case "interfaces":
			data, err = network.Interfaces()
		case "inventory":
			data, err = platform.Inventory()
		case "diagnose":
			data, err = network.Diagnose(ctx, network.Request{Kind: *kind, Target: *target, Port: *port, TimeoutMS: *timeout})
		case "discover":
			var numbers []int
			for _, s := range strings.Split(*ports, ",") {
				n, e := strconv.Atoi(s)
				if e != nil {
					err = domain.Invalid("invalid port list")
					break
				}
				numbers = append(numbers, n)
			}
			if err == nil {
				var results []network.PortResult
				results, err = network.Discover(ctx, *target, numbers, time.Duration(*timeout)*time.Millisecond)
				data = results
				if err == nil && *format == "csv" {
					writer := csv.NewWriter(out)
					writer.Write([]string{"schema_version", "host", "port", "state"})
					for _, r := range results {
						writer.Write([]string{"1", r.Host, strconv.Itoa(r.Port), r.State})
					}
					writer.Flush()
					if writer.Error() != nil {
						return 1
					}
					return 0
				}
				if *format != "json" && *format != "csv" {
					err = domain.Invalid("format must be json or csv")
				}
			}
		case "wifi":
			data, err = wifi.Current()
		case "capture":
			data, err = capture.Live(ctx, capture.LiveRequest{Address: *target, DurationSeconds: *duration, PacketLimit: *limit})
		case "import":
			data, err = capture.ReadFile(*file)
		default:
			err = domain.Invalid("unknown command %q", args[0])
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
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
