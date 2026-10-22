package cli_test

import (
	"bytes"
	"crypto/tls"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
)

func TestCLITLSAuditExitCodes(t *testing.T) {
	s := httptest.NewTLSServer(nil)
	defer s.Close()
	host, port, err := net.SplitHostPort(s.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	a := newTestApp(t)
	for _, tt := range []struct {
		name string
		args []string
		want int
	}{
		{name: "findings", args: []string{"tls-audit", "--target", host, "--port", port, "--lint=false", "--json"}, want: 0},
		{name: "human", args: []string{"tls-audit", "--target", host, "--port", port, "--lint=false", "--lang", "pt-BR"}, want: 0},
		{name: "invalid", args: []string{"tls-audit", "--target", "bad/host", "--json"}, want: 2},
		{name: "failed", args: []string{"tls-audit", "--target", host, "--port", "0", "--json"}, want: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			got := cli.Run(t.Context(), a, tt.args, &out, &stderr)
			if got != tt.want {
				t.Fatalf("exit %d want %d: %s %s", got, tt.want, &out, &stderr)
			}
		})
	}
	s.Close()
	var out, stderr bytes.Buffer
	if got := cli.Run(t.Context(), a, []string{"tls-audit", "--target", host, "--port", port, "--json"}, &out, &stderr); got != 1 {
		t.Fatalf("closed server exit %d: %s", got, &out)
	}
}

func TestTLSAuditTimeoutIsPerAttempt(t *testing.T) {
	s := httptest.NewUnstartedServer(nil)
	s.TLS = &tls.Config{MinVersion: tls.VersionTLS12, GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) { time.Sleep(150 * time.Millisecond); return nil, nil }}
	s.StartTLS()
	defer s.Close()
	host, port, err := net.SplitHostPort(s.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	got := cli.Run(t.Context(), newTestApp(t), []string{"tls-audit", "--target", host, "--port", port, "--timeout", "500", "--lint=false", "--json"}, &out, &stderr)
	if got != 0 {
		t.Fatalf("per-attempt timeout applied to entire audit: %d %s %s", got, &out, &stderr)
	}
}
