package tlsaudit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestAuditLintControlAndLimits(t *testing.T) {
	r, roots := auditFixtureServer(t, tls.VersionTLS12, tls.VersionTLS13, false)
	calls := 0
	a := auditor{roots: roots, lint: func([]byte) ([]Finding, string, error) {
		calls++
		out := make([]Finding, 600)
		for i := range out {
			out[i] = Finding{Source: "zlint", ID: fmt.Sprintf("fixture_%03d", i), Severity: "warn"}
		}
		return out, "zlint fixture", nil
	}}
	r.Lint = true
	report, err := a.audit(t.Context(), r)
	if err != nil || !report.FindingsLimited || len(report.Findings) != MaxFindings || calls != 1 {
		t.Fatalf("%v %d %d", err, len(report.Findings), calls)
	}
	r.Lint = false
	report, err = a.audit(t.Context(), r)
	if err != nil || calls != 1 || report.LintEngine != "" {
		t.Fatal(err, calls, report.LintEngine)
	}
	r.Lint = true
	a.lint = func([]byte) ([]Finding, string, error) {
		return nil, "zlint fixture", fmt.Errorf("fixture lint failure")
	}
	report, err = a.audit(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range report.Findings {
		if f.ID == "lint_failed" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing lint_failed finding")
	}
}

func auditFixtureServer(t *testing.T, minimum, maximum uint16, expired bool) (Request, *x509.CertPool) {
	t.Helper()
	s := httptest.NewUnstartedServer(nil)
	s.TLS = &tls.Config{MinVersion: minimum, MaxVersion: maximum, CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA}}
	now := time.Now()
	if expired {
		now = now.Add(-100 * 24 * time.Hour)
	}
	cert, key := fixtureTLSCertificate(t, now)
	s.TLS.Certificates = []tls.Certificate{{Certificate: [][]byte{cert.Raw}, PrivateKey: key}}
	s.Config.ErrorLog = discardLogger()
	s.StartTLS()
	t.Cleanup(s.Close)
	host, port, err := net.SplitHostPort(s.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(s.Certificate())
	return Request{Host: host, Port: n, TimeoutMS: 500, Lint: false}, pool
}

func TestAuditTrustedFixture(t *testing.T) {
	r, roots := auditFixtureServer(t, tls.VersionTLS12, tls.VersionTLS13, false)
	report, err := (auditor{roots: roots}).audit(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Verified || report.Verifier != "go-x509" || len(report.Chain) != 1 {
		t.Fatalf("%+v", report)
	}
	for _, p := range report.Protocols {
		if (p.Version == "TLS 1.2" || p.Version == "TLS 1.3") && p.Status != "accepted" {
			t.Fatal(p)
		}
	}
	for _, f := range report.Findings {
		if f.Severity == "error" {
			t.Fatal(f)
		}
	}
	if len(report.Chain[0].SANs) == 0 {
		t.Fatal("missing sans")
	}
}

func TestAuditReportsExpiredWithoutFailing(t *testing.T) {
	r, roots := auditFixtureServer(t, tls.VersionTLS12, tls.VersionTLS13, true)
	report, err := (auditor{roots: roots}).audit(t.Context(), r)
	if err != nil || report.Verified || report.VerifyError != "expired" {
		t.Fatalf("%+v %v", report, err)
	}
}

func TestAuditSelfSignedAndUntrusted(t *testing.T) {
	r, _ := auditFixtureServer(t, tls.VersionTLS12, tls.VersionTLS13, false)
	report, err := (auditor{roots: x509.NewCertPool()}).audit(t.Context(), r)
	if err != nil || report.VerifyError != "unknown_authority" {
		t.Fatalf("%+v %v", report, err)
	}
}

func TestAuditHostnameMismatch(t *testing.T) {
	r, roots := auditFixtureServer(t, tls.VersionTLS12, tls.VersionTLS13, false)
	r.ServerName = "other.test"
	report, err := (auditor{roots: roots}).audit(t.Context(), r)
	if err != nil || report.VerifyError != "hostname_mismatch" {
		t.Fatalf("%+v %v", report, err)
	}
}

func TestAuditTimeoutAndCancel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := listener.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = io.Copy(io.Discard, c)
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	r := Request{Host: host, Port: n, TimeoutMS: 500}
	start := time.Now()
	_, err = Audit(t.Context(), r)
	de, ok := errors.AsType[*domain.Error](err)
	if !ok || de.Code != domain.CodeTimeout || time.Since(start) > 1500*time.Millisecond {
		t.Fatalf("%v", err)
	}
	<-done
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = Audit(ctx, r)
	de, ok = errors.AsType[*domain.Error](err)
	if !ok || de.Code != domain.CodeCancelled {
		t.Fatal(err)
	}
}

func TestAuditRejectsOversizedPresentedChain(t *testing.T) {
	cert, key := fixtureTLSCertificate(t, time.Now())
	chain := make([][]byte, MaxChain+1)
	for i := range chain {
		chain[i] = cert.Raw
	}
	s := httptest.NewUnstartedServer(nil)
	s.Config.ErrorLog = discardLogger()
	s.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: chain, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	s.StartTLS()
	defer s.Close()
	host, port, err := net.SplitHostPort(s.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Audit(t.Context(), Request{Host: host, Port: n, TimeoutMS: 500})
	de, ok := errors.AsType[*domain.Error](err)
	if !ok || de.Code != domain.CodeInvalidInput {
		t.Fatal(err)
	}
}
