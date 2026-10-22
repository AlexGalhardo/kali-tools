package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/httpapi"
	"github.com/AlexGalhardo/kali-tools/internal/app"
)

func TestServeFlagsAndShutdown(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--timeout", "1s"}, {"--detach"}, {"--listen", "0.0.0.0:1"}, {"--listen", "192.0.2.10:8443"}, {"--unknown=synthetic-secret"}, {"unexpected"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			var out, stderr bytes.Buffer
			code := runServe(t.Context(), args, &out, &stderr, httpapi.New)
			want := 2
			if args[0] == "--help" {
				want = 0
			}
			if code != want || strings.Contains(stderr.String(), "synthetic-secret") {
				t.Fatalf("code=%d stderr=%s", code, &stderr)
			}
		})
	}
	for _, jsonMode := range []bool{false, true} {
		t.Setenv("APPDATA", t.TempDir())
		var out, stderr bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		listen := func(a *app.Application, opts httpapi.Options) (*httpapi.Server, error) {
			s, err := httpapi.New(a, opts)
			cancel()
			return s, err
		}
		args := []string{"--listen", "127.0.0.1:0"}
		if jsonMode {
			args = append(args, "--json")
		}
		if code := runServe(ctx, args, &out, &stderr, listen); code != 0 || !strings.Contains(out.String(), "#token=") {
			t.Fatalf("code=%d out=%s stderr=%s", code, &out, &stderr)
		}
		if strings.Contains(stderr.String(), "#token=") {
			t.Fatal("token leaked to stderr")
		}
	}
}

func TestServeDispatch(t *testing.T) {
	for _, args := range [][]string{{"serve", "--help"}, {"--json", "serve", "--help"}, {"--lang", "pt-BR", "serve", "--help"}} {
		var out, stderr bytes.Buffer
		if code := run(t.Context(), args, func(string) string { return "1" }, &out, &stderr); code != 0 || !strings.Contains(out.String(), "--listen") {
			t.Fatalf("code=%d out=%s err=%s", code, &out, &stderr)
		}
	}
}

type rejectedOutput struct{}

func (rejectedOutput) Write([]byte) (int, error) { return 0, errors.New("synthetic output failure") }

func TestServeOutputFailuresAndTLSWarning(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if code := runServe(t.Context(), []string{"--help"}, rejectedOutput{}, io.Discard, httpapi.New); code != 1 {
		t.Fatal(code)
	}
	for _, args := range [][]string{{}, {"--json"}} {
		if code := runServe(t.Context(), args, rejectedOutput{}, io.Discard, httpapi.New); code != 1 {
			t.Fatal(code)
		}
	}
	fixture := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer fixture.Close()
	pair := fixture.TLS.Certificates[0]
	key, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
		t.Fatal(err)
	}
	for _, remote := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		var out, stderr bytes.Buffer
		args := []string{"--listen", "127.0.0.1:0", "--tls-cert", certPath, "--tls-key", keyPath}
		if remote {
			args[1] = "192.0.2.10:0"
			args = append(args, "--allow-remote")
		}
		listen := func(a *app.Application, opts httpapi.Options) (*httpapi.Server, error) {
			if remote && !opts.AllowRemote {
				t.Fatal("missing opt-in")
			}
			opts.Addr = "127.0.0.1:0"
			s, err := httpapi.New(a, opts)
			cancel()
			return s, err
		}
		if code := runServe(ctx, args, &out, &stderr, listen); code != 0 || !strings.Contains(out.String(), "https://") || !strings.Contains(out.String(), "TLS certificate SHA-256:") {
			t.Fatalf("code=%d out=%s stderr=%s", code, &out, &stderr)
		}
		if remote && !strings.Contains(stderr.String(), "WARNING:") {
			t.Fatal("missing remote warning")
		}
	}
	if _, ok := serveArguments([]string{"--json"}); ok {
		t.Fatal("missing command accepted")
	}
}

func TestServeBootstrapFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", path)
	t.Setenv("XDG_CONFIG_HOME", path)
	t.Setenv("HOME", path)
	if code := runServe(t.Context(), nil, io.Discard, io.Discard, httpapi.New); code != 1 {
		t.Fatal(code)
	}
}
