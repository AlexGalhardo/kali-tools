//go:build integration

package integration

import (
	"context"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/dnsinspect"
)

const labSignedDS = "signed.test. IN DS 38590 15 2 b3d09e28ce56fd36df2da483c42ee8de3987d26747f553a6f15baa5a760485e4"

func TestPrivateLabDNSInspection(t *testing.T) {
	host := os.Getenv("LAB_FIXTURE")
	if host == "" {
		t.Fatal("LAB_FIXTURE is required; use the private Docker lab")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	ip := fixtureIPv4(t, ctx, host)
	pool := labRoots(t, ctx, host)
	dnsinspect.NewForLab(map[string]uint16{
		"ns1.consistent.test": 5354,
		"ns2.consistent.test": 5355,
		"ns1.mismatch.test":   5354,
		"ns2.mismatch.test":   5355,
	}, pool)

	compared, err := dnsinspect.Inspect(ctx, dnsinspect.Request{
		Name: "differs.test", Type: "A", Checks: []string{"compare"}, TimeoutMS: 1000,
		Resolvers: []string{net.JoinHostPort(ip, "5356"), net.JoinHostPort(ip, "5300")},
	}, nil)
	if err != nil || compared.Compare == nil || compared.Compare.Agree || len(compared.Compare.Groups) != 2 {
		t.Fatalf("differs.test compare: %v %+v", err, compared.Compare)
	}

	blocked, err := dnsinspect.Inspect(ctx, dnsinspect.Request{
		Name: "fixture.test", Type: "A", Checks: []string{"compare"}, TimeoutMS: 800,
		Resolvers: []string{net.JoinHostPort(ip, "5357")},
	}, nil)
	if err != nil || blocked.Compare == nil || !hasStatus(blocked.Compare.Results, "5357", "unreachable") {
		t.Fatalf("silent resolver: %v %+v", err, blocked.Compare)
	}

	consistent, err := dnsinspect.Inspect(ctx, dnsinspect.Request{
		Name: "consistent.test", Type: "A", Checks: []string{"soa_ns"}, TimeoutMS: 1000,
		Resolvers: []string{net.JoinHostPort(ip, "5356")},
	}, nil)
	if err != nil || consistent.SOANS == nil || consistent.SOANS.Verdict != "consistent" {
		t.Fatalf("consistent: %v %+v", err, consistent.SOANS)
	}
	mismatch, err := dnsinspect.Inspect(ctx, dnsinspect.Request{
		Name: "mismatch.test", Type: "A", Checks: []string{"soa_ns"}, TimeoutMS: 1000,
		Resolvers: []string{net.JoinHostPort(ip, "5356")},
	}, nil)
	if err != nil || mismatch.SOANS == nil || mismatch.SOANS.Verdict != "serial_mismatch" {
		t.Fatalf("mismatch: %v %+v", err, mismatch.SOANS)
	}

	secure, err := dnsinspect.Inspect(ctx, dnsinspect.Request{
		Name: "www.signed.test", Type: "A", Checks: []string{"dnssec"}, TimeoutMS: 1000,
		Resolvers: []string{net.JoinHostPort(ip, "5356")}, TrustAnchors: []string{labSignedDS},
	}, nil)
	if err != nil || secure.DNSSEC == nil || secure.DNSSEC.Verdict != "secure" || !secure.DNSSEC.AnswerSig {
		t.Fatalf("secure: %v %+v", err, secure.DNSSEC)
	}
	bogusAnchor := strings.TrimSuffix(labSignedDS, "4") + "5"
	bogus, err := dnsinspect.Inspect(ctx, dnsinspect.Request{
		Name: "www.signed.test", Type: "A", Checks: []string{"dnssec"}, TimeoutMS: 1000,
		Resolvers: []string{net.JoinHostPort(ip, "5356")}, TrustAnchors: []string{bogusAnchor},
	}, nil)
	if err != nil || bogus.DNSSEC == nil || bogus.DNSSEC.Verdict != "bogus" {
		t.Fatalf("bogus: %v %+v", err, bogus.DNSSEC)
	}

	encrypted, err := dnsinspect.Inspect(ctx, dnsinspect.Request{
		Name: "fixture.test", Type: "A", Checks: []string{"encrypted"}, TimeoutMS: 1000,
		DoH: []string{"https://" + net.JoinHostPort(host, "8443") + "/dns-query"},
		DoT: []string{net.JoinHostPort(host, "8853")},
	}, nil)
	if err != nil || len(encrypted.Encrypted) != 2 || encrypted.Encrypted[0].Status != "ok" || encrypted.Encrypted[1].Status != "ok" {
		t.Fatalf("encrypted: %v %+v", err, encrypted.Encrypted)
	}
}

func fixtureIPv4(t *testing.T, ctx context.Context, host string) string {
	t.Helper()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil || len(ips) == 0 || ips[0].To4() == nil {
		t.Fatal("fixture IPv4 lookup failed", err)
	}
	return ips[0].To4().String()
}

func labRoots(t *testing.T, ctx context.Context, host string) *x509.CertPool {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, "8080")+"/lab-ca.pem", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal("lab CA fetch failed", resp.StatusCode, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(body) {
		t.Fatal("lab CA PEM was rejected")
	}
	return pool
}

func hasStatus(results []dnsinspect.ResolverAnswer, port, status string) bool {
	for _, result := range results {
		if strings.HasSuffix(result.Resolver, ":"+port) && result.Status == status {
			return true
		}
	}
	return false
}
