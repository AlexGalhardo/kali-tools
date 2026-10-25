package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const (
	minTimeout     = 100 * time.Millisecond
	maxTimeout     = 10 * time.Minute
	maxPortsArg    = 191
	maxPortsCommas = 31
)

func parsePorts(value string) ([]int, error) {
	if len(value) > maxPortsArg || strings.Count(value, ",") > maxPortsCommas {
		return nil, domain.Invalid("1..32 ports required")
	}
	ports := []int{}
	for _, part := range strings.Split(value, ",") {
		port, err := strconv.Atoi(part)
		if err != nil {
			return nil, domain.Invalid("invalid port list")
		}
		ports = append(ports, port)
	}
	return ports, nil
}

type globals struct {
	JSON    bool
	Timeout time.Duration
	NoColor bool
	Lang    string
	Detach  bool
}

type invocation struct {
	command     command
	g           globals
	legacy      []string
	help        bool
	flagFailure bool
	arg         string
	values      map[string]string
	set         map[string]bool
}

func parseTimeout(value string) (time.Duration, error) {
	if milliseconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if milliseconds < minTimeout.Milliseconds() || milliseconds > maxTimeout.Milliseconds() {
			return 0, domain.Invalid("timeout must be 100ms..10m")
		}
		return time.Duration(milliseconds) * time.Millisecond, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < minTimeout || duration > maxTimeout {
		return 0, domain.Invalid("timeout must be 100ms..10m")
	}
	return duration, nil
}

func registerGlobals(fs *flag.FlagSet, g *globals) {
	fs.BoolVar(&g.JSON, "json", g.JSON, "versioned JSON output")
	fs.BoolVar(&g.NoColor, "no-color", g.NoColor, "disable ANSI colors")
	fs.BoolVar(&g.Detach, "detach", g.Detach, "unsupported: no daemon mode")
	fs.StringVar(&g.Lang, "lang", g.Lang, "result language: en or pt-BR")
	fs.Func("timeout", "operation timeout: milliseconds or Go duration", func(value string) error {
		var err error
		g.Timeout, err = parseTimeout(value)
		return err
	})
}

func jsonRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if slices.Contains([]string{"--json", "-json", "--json=true", "-json=true"}, arg) {
			return true
		}
	}
	return false
}

var unknownFlag = regexp.MustCompile(`flag provided but not defined: -([a-zA-Z][a-zA-Z0-9-]{0,40})$`)
var invalidFlag = regexp.MustCompile(`(?:for flag -|flag needs an argument: -)([a-zA-Z][a-zA-Z0-9-]{0,40})(?::|$)`)

func safeFlagError(err error) error {
	if match := unknownFlag.FindStringSubmatch(err.Error()); len(match) == 2 {
		return domain.Invalid("unknown flag --%s", match[1])
	}
	if match := invalidFlag.FindStringSubmatch(err.Error()); len(match) == 2 {
		return domain.Invalid("invalid value for --%s", match[1])
	}
	return domain.Invalid("invalid command flags")
}

func parseInvocation(args []string) (invocation, error) {
	call := invocation{g: globals{JSON: jsonRequested(args), Lang: "en"}}
	root := flag.NewFlagSet("kali-tools", flag.ContinueOnError)
	root.SetOutput(io.Discard)
	registerGlobals(root, &call.g)
	if err := root.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			call.help = true
			return call, nil
		}
		call.flagFailure = true
		return call, safeFlagError(err)
	}
	args = root.Args()
	if len(args) == 0 {
		return call, domain.Invalid("command required")
	}
	if args[0] == "help" {
		call.help = true
		args = args[1:]
		if len(args) == 0 {
			return call, nil
		}
	}
	index, consumed := -1, 1
	for i, c := range commands {
		if c.path == args[0] && consumed == 1 {
			index = i
		}
		if len(args) > 1 && c.path == args[0]+" "+args[1] {
			index, consumed = i, 2
		}
	}
	if index < 0 {
		return call, domain.Invalid("unknown command %q", args[0])
	}
	call.command = commands[index]
	args = args[consumed:]
	if (call.command.path == "jobs show" || call.command.path == "jobs cancel") && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		call.arg, args = args[0], args[1:]
	}
	positionalKind := ""
	if call.command.path == "diagnose" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		positionalKind, args = args[0], args[1:]
		if positionalKind == "trace" {
			positionalKind = "traceroute"
		}
	}
	fs := flag.NewFlagSet(call.command.path, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	registerGlobals(fs, &call.g)
	for _, name := range call.command.flags {
		switch name {
		case "lint":
			fs.Bool(name, true, "run certificate linting")
		case "all", "confirm", "owned", "interfaces", "authorized", "accept":
			fs.Bool(name, false, name)
		case "port":
			port := 443
			if call.command.path == "snmp" {
				port = 161
			}
			fs.Int(name, port, "port")
		case "duration":
			seconds := 5
			if call.command.path == "local-discover" {
				seconds = 3
			}
			if call.command.path == "throughput" {
				seconds = 0
			}
			fs.Int(name, seconds, "seconds")
		case "redirects":
			fs.Int(name, 0, "HTTP redirect hops to trace, 0..5")
		case "limit":
			fs.Int(name, 1000, "packet limit")
		case "retention-days":
			fs.Int(name, 0, "history retention days")
		default:
			value := ""
			switch name {
			case "kind":
				value = "tcp"
			case "ports":
				value = "80,443"
			case "format":
				value = "json"
			case "family":
				value = "all"
			case "checks":
				value = "compare"
			case "record":
				if call.command.path == "dns-inspect" {
					value = "A"
				}
			case "protocols":
				value = "mdns,ssdp"
			case "ssdp-target":
				value = "ssdp:all"
			case "version":
				if call.command.path == "snmp" {
					value = "v2c"
				}
			case "auth":
				value = "SHA256"
			}
			fs.String(name, value, name)
		}
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			call.help = true
			return call, nil
		}
		call.flagFailure = true
		return call, safeFlagError(err)
	}
	if fs.NArg() != 0 {
		return call, domain.Invalid("unexpected positional arguments")
	}
	if call.help {
		return call, nil
	}
	if call.g.Detach {
		return call, domain.Invalid("--detach is not supported: kali-tools has no daemon mode")
	}
	if call.g.Lang != "en" && call.g.Lang != "pt-BR" {
		return call, domain.Invalid("language must be en or pt-BR")
	}
	if (call.command.path == "jobs show" || call.command.path == "jobs cancel") && !call.help && !validJobID.MatchString(call.arg) {
		return call, domain.Invalid("job ID must be 32 lowercase hexadecimal characters")
	}
	call.values = make(map[string]string, len(call.command.flags))
	for _, name := range call.command.flags {
		call.values[name] = fs.Lookup(name).Value.String()
	}
	if call.command.path == "wifi reveal" && call.values["confirm"] != "true" {
		return call, domain.Invalid("wifi reveal prints the saved network key; re-run with --confirm")
	}
	if (call.command.path == "discover" || call.command.path == "inspect") && call.values["authorized"] != "true" {
		return call, domain.Invalid("%s sends traffic to every host in the scope; re-run with --authorized to confirm you have written authorization for it", call.command.path)
	}
	if strings.HasPrefix(call.command.path, "capture") || call.command.path == "import" {
		if call.values["output"] != "" && call.values["format"] != "pcap" && call.values["format"] != "pcapng" {
			return call, domain.Invalid("packet export requires --format pcap or pcapng")
		}
		if (call.command.path == "capture import" || call.command.path == "import") && call.values["file"] == "" {
			return call, domain.Invalid("capture import requires --file")
		}
	}
	call.legacy = []string{call.command.path}
	switch call.command.path {
	case "capture live":
		call.legacy[0] = "capture"
	case "capture import":
		call.legacy[0] = "import"
	}
	call.set = make(map[string]bool)
	var parseErr error
	fs.Visit(func(f *flag.Flag) {
		call.set[f.Name] = true
		if f.Name == "authorized" {
			return
		}
		if !slices.Contains(call.command.flags, f.Name) {
			return
		}
		value := f.Value.String()
		if f.Name == "kind" && value == "trace" {
			value = "traceroute"
		}
		if f.Name == "kind" && positionalKind != "" && value != positionalKind {
			parseErr = domain.Invalid("kind given twice")
		}
		if f.Name == "format" && value == "csv" && call.g.JSON && (call.command.path == "discover" || call.command.path == "neighbors") {
			parseErr = domain.Invalid("CSV cannot be combined with --json")
		}
		if f.Name == "format" && value == "json" && call.command.path == "neighbors" {
			call.g.JSON = true
		}
		call.legacy = append(call.legacy, "--"+f.Name, value)
	})
	if positionalKind != "" {
		call.legacy = append(call.legacy, "--kind", positionalKind)
	}
	if call.g.Timeout != 0 && call.command.path != "version" {
		if call.command.path == "capture" || call.command.path == "capture live" {
			return call, domain.Invalid("capture live uses --duration")
		}
		call.legacy = append(call.legacy, "--timeout", strconv.FormatInt(call.g.Timeout.Milliseconds(), 10))
	}
	return call, parseErr
}

var validJobID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// NeedsApp avoids opening storage for help, version and rejected invocations.
func NeedsApp(args []string) bool {
	call, err := parseInvocation(args)
	return err == nil && !call.help && call.command.needsApp
}

func writeHelp(w io.Writer, call invocation) error {
	if call.command.path != "" {
		_, err := fmt.Fprintf(w, "Usage: kali-tools %s [flags]\n%s\nFlags: --json --timeout --no-color --lang", call.command.path, call.command.synopsis)
		if err != nil {
			return err
		}
		for _, name := range call.command.flags {
			if _, err := fmt.Fprintf(w, " --%s", name); err != nil {
				return err
			}
		}
		_, err = fmt.Fprintln(w)
		return err
	}
	if _, err := fmt.Fprintln(w, "Usage: kali-tools [global flags] <command> [flags]\nCommands:"); err != nil {
		return err
	}
	for _, c := range commands {
		if _, err := fmt.Fprintf(w, "  %-16s %s\n", c.path, c.synopsis); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "Global flags: --json --timeout --no-color --lang")
	return err
}
