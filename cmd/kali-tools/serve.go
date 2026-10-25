package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/AlexGalhardo/kali-tools/frontend"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/httpapi"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

// serveArguments recognizes only the command position, leaving flag validation to
// the command parser. Other commands retain the shared CLI parser.
func serveArguments(args []string) ([]string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--lang" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if arg != "serve" {
			return nil, false
		}
		result := append([]string{}, args[:i]...)
		return append(result, args[i+1:]...), true
	}
	return nil, false
}

// writeTokenFile stores the launch URL, which carries the credential, with owner-only permissions.
func writeTokenFile(path, launchURL string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return domain.Invalid("cannot write --token-file")
	}
	if err := file.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		_ = file.Close()
		return domain.Invalid("cannot restrict --token-file")
	}
	_, werr := file.WriteString(launchURL + "\n")
	if err := errors.Join(werr, file.Close()); err != nil {
		return domain.Invalid("cannot write --token-file")
	}
	return nil
}

func runServe(ctx context.Context, args []string, out, errout io.Writer, listen func(*app.Application, httpapi.Options) (*httpapi.Server, error)) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	address := flags.String("listen", "127.0.0.1:0", "specific host:port")
	remote := flags.Bool("allow-remote", false, "allow a non-loopback TLS listener")
	cert := flags.String("tls-cert", "", "TLS certificate")
	key := flags.String("tls-key", "", "TLS private key")
	allowFrom := flags.String("allow-from", "", "comma-separated CIDRs allowed to connect (requires --allow-remote)")
	tokenFile := flags.String("token-file", "", "write the launch URL to this file (mode 0600) instead of printing it")
	jsonMode := flags.Bool("json", false, "JSON startup record")
	flags.Bool("no-color", false, "accepted for compatibility")
	flags.String("lang", "en", "accepted for compatibility")
	help := flags.Bool("help", false, "show help")
	flags.BoolVar(help, "h", false, "show help")
	fail := func(err error) int {
		e := contract.FromError(err)
		_, _ = fmt.Fprintf(errout, "%s: %s\n", e.Code, e.Message)
		return cli.ExitCode(e.Code)
	}
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return fail(domain.Invalid("invalid serve arguments"))
	}
	if *help {
		_, err := fmt.Fprintln(out, "Usage: kali-tools serve [--listen host:port] [--allow-remote --tls-cert file --tls-key file [--allow-from cidr[,cidr]]] [--token-file file] [--json]\nOpen the printed token URL, or the URL in --token-file. Press Ctrl-C to stop.")
		if err != nil {
			return 1
		}
		return 0
	}
	a, cleanup, err := bootstrap.New(ctx, bootstrap.Config{})
	if err != nil {
		return fail(err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			_ = fail(err)
		}
	}()
	var allowed []string
	if *allowFrom != "" {
		allowed = strings.Split(*allowFrom, ",")
	}
	s, err := listen(a, httpapi.Options{Addr: *address, Assets: frontend.Dist(), AllowRemote: *remote, TLSCert: *cert, TLSKey: *key, AllowFrom: allowed, Logger: slog.New(slog.NewTextHandler(errout, nil))})
	if err != nil {
		return fail(err)
	}
	host, _, _ := net.SplitHostPort(*address)
	ip := net.ParseIP(host)
	isRemote := host != "localhost" && (ip == nil || !ip.IsLoopback())
	tlsEnabled := s.CertFingerprint() != ""
	if isRemote {
		_, _ = fmt.Fprintf(errout, "WARNING: kali-tools is reachable from the network at https://%s. Anyone who obtains the URL token can run network diagnostics, discovery and packet capture from this host as the current user, and can reveal the saved Wi-Fi key. Stop the server with Ctrl-C when finished.\n", s.Addr())
	}
	if *tokenFile != "" {
		err = writeTokenFile(*tokenFile, s.URL())
	}
	switch {
	case err != nil:
	case *jsonMode:
		data := map[string]any{"address": s.Addr().String(), "tls": tlsEnabled, "remote": isRemote, "certificate_sha256": s.CertFingerprint()}
		if *tokenFile != "" {
			data["token_file"] = *tokenFile
		} else {
			data["url"] = s.URL()
		}
		err = json.NewEncoder(out).Encode(map[string]any{"schema_version": domain.SchemaVersion, "version": domain.Version, "data": data})
	default:
		launch := "Open: " + s.URL()
		if *tokenFile != "" {
			launch = "Launch URL written to " + *tokenFile
		}
		_, err = fmt.Fprintf(out, "kali-tools %s serving on %s\n%s\nPress Ctrl-C to stop.\n", domain.Version, strings.Split(s.URL(), "/#")[0], launch)
		if err == nil && tlsEnabled {
			_, err = fmt.Fprintf(out, "TLS certificate SHA-256: %s\n", s.CertFingerprint())
		}
	}
	if err != nil {
		stop, cancel := context.WithCancel(context.Background())
		cancel()
		_ = s.Run(stop)
		return fail(err)
	}
	if err := s.Run(ctx); err != nil {
		return fail(err)
	}
	return 0
}
