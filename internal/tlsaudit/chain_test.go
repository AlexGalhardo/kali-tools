package tlsaudit

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCountEmbeddedSCTs(t *testing.T) {
	encode := func(n int) []byte {
		list := make([]byte, 2+3*n)
		binary.BigEndian.PutUint16(list, uint16(3*n))
		for i := range n {
			list[2+3*i+1] = 1
			list[2+3*i+2] = 42
		}
		raw, err := asn1.Marshal(list)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	tests := []struct {
		name    string
		raw     []byte
		want    int
		invalid bool
	}{
		{name: "two", raw: encode(2), want: 2},
		{name: "at limit", raw: encode(64), want: 64},
		{name: "above limit", raw: encode(65), want: 64},
		{name: "invalid asn1", raw: []byte{4, 5, 0}, invalid: true},
		{name: "missing list length", raw: []byte{4, 1, 0}, invalid: true},
		{name: "empty list", raw: encode(0), invalid: true},
		{name: "short list", raw: []byte{4, 4, 0, 3, 0, 1}, invalid: true},
		{name: "missing entry length", raw: []byte{4, 3, 0, 1, 0}, invalid: true},
		{name: "zero entry length", raw: []byte{4, 4, 0, 2, 0, 0}, invalid: true},
		{name: "short entry", raw: []byte{4, 5, 0, 3, 0, 2, 42}, invalid: true},
		{name: "trailing bytes", raw: append(encode(2), 0), invalid: true},
		{name: "oversized input", raw: make([]byte, MaxCertDER+1), invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := countEmbeddedSCTs(tt.raw)
			if got != tt.want || (err != nil) != tt.invalid {
				t.Fatalf("count = %d, %v; want %d, invalid=%v", got, err, tt.want, tt.invalid)
			}
		})
	}
}

func TestWeakKeyFindings(t *testing.T) {
	tests := []struct {
		name string
		cert Certificate
		want string
	}{
		{name: "rsa 1024", cert: Certificate{KeyAlgorithm: "RSA", KeyBits: 1024}, want: "weak_rsa_key"},
		{name: "rsa 2047", cert: Certificate{KeyAlgorithm: "RSA", KeyBits: 2047}, want: "weak_rsa_key"},
		{name: "rsa 2048", cert: Certificate{KeyAlgorithm: "RSA", KeyBits: 2048}},
		{name: "ecdsa 224", cert: Certificate{KeyAlgorithm: "ECDSA", KeyBits: 224}, want: "weak_ecdsa_key"},
		{name: "ecdsa 255", cert: Certificate{KeyAlgorithm: "ECDSA", KeyBits: 255}, want: "weak_ecdsa_key"},
		{name: "ecdsa 256", cert: Certificate{KeyAlgorithm: "ECDSA", KeyBits: 256}},
		{name: "ed25519", cert: Certificate{KeyAlgorithm: "Ed25519", KeyBits: 256}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyFindings(tt.cert)
			if tt.want == "" {
				if len(got) != 0 {
					t.Fatalf("unexpected findings: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].ID != tt.want || got[0].Severity != "error" {
				t.Fatalf("got %+v, want %s", got, tt.want)
			}
		})
	}
}

func TestCertificateReport(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	leaf := fixtureCertificate(t, now)
	cert, findings := certificateReport(leaf, 0, now)
	if cert.Subject != "CN=fixture.test" || cert.Issuer != cert.Subject || cert.SerialHex != "2a" {
		t.Fatalf("identity: %+v", cert)
	}
	if !cert.SelfSigned || !cert.IsCA || cert.KeyAlgorithm != "ECDSA" || cert.KeyBits != 256 {
		t.Fatalf("key/signature: %+v", cert)
	}
	if cert.DaysRemaining != 90 || len(cert.SHA256) != 64 || len(findings) != 0 {
		t.Fatalf("report: %+v, findings: %+v", cert, findings)
	}
	if !slices.Equal(cert.SANs, []string{"fixture.test", "127.0.0.1"}) {
		t.Fatalf("sans: %v", cert.SANs)
	}
	cert.SANs[0] = "changed"
	if leaf.DNSNames[0] != "fixture.test" {
		t.Fatal("report aliases certificate")
	}
	leaf.Signature[0] ^= 1
	cert, _ = certificateReport(leaf, 0, now)
	if cert.SelfSigned {
		t.Fatal("invalid self-signature accepted")
	}
}

func TestCertificateReportBounds(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{MaxSANs, MaxSANs + 1} {
		t.Run(fmt.Sprintf("sans %d", n), func(t *testing.T) {
			leaf := fixtureCertificate(t, now)
			leaf.DNSNames = make([]string, n)
			for i := range leaf.DNSNames {
				leaf.DNSNames[i] = strings.Repeat("x", 1025)
			}
			leaf.IPAddresses = nil
			cert, _ := certificateReport(leaf, 0, now)
			if len(cert.SANs) != MaxSANs || cert.SANsLimited != (n > MaxSANs) || len(cert.SANs[0]) != 1024 {
				t.Fatalf("san bounds: %d, limited=%v", len(cert.SANs), cert.SANsLimited)
			}
		})
	}
	leaf := fixtureCertificate(t, now)
	uri, err := url.Parse("https://fixture.test/")
	if err != nil {
		t.Fatal(err)
	}
	leaf.URIs = []*url.URL{uri}
	leaf.EmailAddresses = []string{"fixture@example.test"}
	leaf.IPAddresses = []net.IP{net.IPv6loopback}
	for _, n := range []int{MaxURLsPerField, MaxURLsPerField + 1} {
		leaf.IssuingCertificateURL = make([]string, n)
		leaf.OCSPServer = make([]string, n)
		leaf.CRLDistributionPoints = make([]string, n)
		for i := range n {
			leaf.IssuingCertificateURL[i] = strings.Repeat("u", 1025)
			leaf.OCSPServer[i] = "https://ocsp.fixture.test"
			leaf.CRLDistributionPoints[i] = "https://crl.fixture.test"
		}
		cert, _ := certificateReport(leaf, 0, now)
		if len(cert.IssuingCertificateURL) != 16 || len(cert.OCSPServers) != 16 || len(cert.CRLDistributionPoints) != 16 {
			t.Fatal("url lists exceed limit")
		}
		if len(cert.IssuingCertificateURL[0]) != 1024 || !slices.Contains(cert.SANs, uri.String()) || !slices.Contains(cert.SANs, "fixture@example.test") {
			t.Fatalf("bounded strings/san fields: %+v", cert)
		}
	}
	leaf.Extensions = append(leaf.Extensions, sctExtension([]byte{4, 1, 0}))
	cert, findings := certificateReport(leaf, 0, now)
	if cert.EmbeddedSCTs != 0 || len(findings) != 1 || findings[0].ID != "sct_extension_malformed" {
		t.Fatalf("malformed sct: %+v, %+v", cert, findings)
	}
	validSCT, err := asn1.Marshal([]byte{0, 3, 0, 1, 42})
	if err != nil {
		t.Fatal(err)
	}
	leaf.Extensions[len(leaf.Extensions)-1] = sctExtension(validSCT)
	cert, findings = certificateReport(leaf, 0, now)
	if cert.EmbeddedSCTs != 1 || len(findings) != 0 {
		t.Fatalf("valid sct: %+v, %+v", cert, findings)
	}
	weakKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []any{&rsa.PublicKey{N: leaf.SerialNumber, E: 65537}, &weakKey.PublicKey, ed25519.PublicKey(make([]byte, 32)), nil} {
		leaf.PublicKey = key
		cert, _ := certificateReport(leaf, 0, now)
		if key == nil && cert.KeyAlgorithm != "unknown" {
			t.Fatal(cert.KeyAlgorithm)
		}
	}
}

func TestBuiltInFindings(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	base := Certificate{NotBefore: now.Add(-time.Hour), NotAfter: now.Add(90 * 24 * time.Hour), DaysRemaining: 90}
	tests := []struct {
		name   string
		change func(*Report)
		want   string
	}{
		{name: "expired", change: func(r *Report) { r.Chain[0].NotAfter = now.Add(-time.Second); r.Chain[0].DaysRemaining = -1 }, want: "expired"},
		{name: "future", change: func(r *Report) { r.Chain[0].NotBefore = now.Add(time.Second) }, want: "not_yet_valid"},
		{name: "soon", change: func(r *Report) { r.Chain[0].DaysRemaining = 29 }, want: "expires_soon"},
		{name: "mismatch", change: func(r *Report) { r.VerifyError = "hostname_mismatch" }, want: "hostname_mismatch"},
		{name: "untrusted", change: func(r *Report) { r.VerifyError = "unknown_authority" }, want: "untrusted_chain"},
		{name: "self signed", change: func(r *Report) { r.Chain[0].SelfSigned = true }, want: "self_signed_leaf"},
		{name: "weak signature", change: func(r *Report) { r.Chain[0].SignatureAlgorithm = "SHA1-RSA" }, want: "weak_signature"},
		{name: "weak md5", change: func(r *Report) { r.Chain[0].SignatureAlgorithm = "MD5-RSA" }, want: "weak_signature"},
		{name: "weak ecdsa sha1", change: func(r *Report) { r.Chain[0].SignatureAlgorithm = "ECDSA-SHA1" }, want: "weak_signature"},
		{name: "weak key", change: func(r *Report) { r.Chain[0].KeyAlgorithm = "RSA"; r.Chain[0].KeyBits = 1024 }, want: "weak_rsa_key"},
		{name: "legacy", change: func(r *Report) {
			r.Protocols = append(r.Protocols, ProtocolResult{Version: "TLS 1.0", Status: "accepted"})
		}, want: "legacy_protocol_accepted"},
		{name: "no tls13", change: func(r *Report) { r.Protocols[0].Status = "client_unsupported" }, want: "no_tls13"},
		{name: "no stapling", change: func(r *Report) { r.Chain[0].OCSPServers = []string{"https://ocsp.fixture.test"} }, want: "no_ocsp_stapling"},
		{name: "no scts", change: func(r *Report) { r.SCTs.Embedded = 0 }, want: "no_scts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Report{Chain: []Certificate{base}, Protocols: []ProtocolResult{{Version: "TLS 1.3", Status: "accepted"}}, SCTs: SCTSummary{Embedded: 1}}
			tt.change(&r)
			got := builtInFindings(r, now)
			if len(got) != 1 || got[0].ID != tt.want || got[0].Source != "kali-tools" {
				t.Fatalf("got %+v, want %s", got, tt.want)
			}
		})
	}
	base.DaysRemaining = 30
	base.SelfSigned = true
	base.SignatureAlgorithm = "SHA1-RSA"
	r := Report{Chain: []Certificate{base}, OCSPStapled: true, SCTs: SCTSummary{TLSExtension: 1}, Protocols: []ProtocolResult{{Version: "TLS 1.3", Status: "accepted"}, {Version: "TLS 1.1", Status: "rejected"}}}
	got := builtInFindings(r, now)
	if len(got) != 1 || got[0].ID != "self_signed_leaf" {
		t.Fatalf("root/threshold false positives: %+v", got)
	}
	if got := builtInFindings(Report{}, now); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestChainAndFindingLimits(t *testing.T) {
	for _, n := range []int{MaxChain, MaxChain + 1} {
		chain := make([]*x509.Certificate, n)
		for i := range chain {
			chain[i] = &x509.Certificate{Raw: make([]byte, MaxCertDER)}
		}
		if err := validateChain(chain); (err != nil) != (n > MaxChain) {
			t.Fatalf("chain length %d: %v", n, err)
		}
	}
	if err := validateChain([]*x509.Certificate{{Raw: make([]byte, MaxCertDER+1)}}); err == nil {
		t.Fatal("oversized der accepted")
	}
	if err := validateChain(nil); err == nil {
		t.Fatal("empty chain accepted")
	}
	if err := validateChain([]*x509.Certificate{nil}); err == nil {
		t.Fatal("nil certificate accepted")
	}
	for _, n := range []int{MaxFindings, MaxFindings + 1, 600} {
		r := Report{Findings: []Finding{}}
		for range n {
			r.addFindings(Finding{Source: "zlint", ID: "fixture", Severity: "warn", Detail: strings.Repeat("é", MaxDetailBytes)})
		}
		if len(r.Findings) != MaxFindings || r.FindingsLimited != (n > MaxFindings) {
			t.Fatalf("finding bound %d: %d, %v", n, len(r.Findings), r.FindingsLimited)
		}
		if len(r.Findings[0].Detail) > MaxDetailBytes {
			t.Fatal("detail exceeds bound")
		}
	}
	for _, n := range []int{MaxDetailBytes, MaxDetailBytes + 1} {
		r := Report{Findings: []Finding{}}
		r.addFindings(Finding{Detail: strings.Repeat("a", n)})
		if len(r.Findings[0].Detail) != MaxDetailBytes {
			t.Fatal("detail boundary")
		}
	}
	runes := Report{Findings: []Finding{}}
	runes.addFindings(Finding{Detail: strings.Repeat("a", 511) + "é"})
	if len(runes.Findings[0].Detail) != 511 || !utf8.ValidString(runes.Findings[0].Detail) {
		t.Fatal("split utf8 detail")
	}
	invalidUTF8 := Report{Findings: []Finding{}}
	invalidUTF8.addFindings(Finding{Detail: strings.Repeat("\x80", MaxDetailBytes+1)})
	if len(invalidUTF8.Findings[0].Detail) > MaxDetailBytes {
		t.Fatal("invalid utf8 exceeds bound")
	}
	r := Report{Findings: []Finding{{Severity: "info"}, {Severity: "error", CertIndex: 1}, {Severity: "fatal"}, {Severity: "warn"}, {Severity: "error", CertIndex: 0, ID: "z"}, {Severity: "error", CertIndex: 0, ID: "a"}}}
	r.sortFindings()
	if r.Findings[0].Severity != "fatal" || r.Findings[1].ID != "a" || r.Findings[2].ID != "z" || r.Findings[3].CertIndex != 1 || r.Findings[4].Severity != "warn" || r.Findings[5].Severity != "info" {
		t.Fatalf("order: %+v", r.Findings)
	}
}

func FuzzCountEmbeddedSCTs(f *testing.F) {
	f.Add([]byte{4, 5, 0, 3, 0, 1, 42})
	f.Add([]byte{4, 4, 0, 2, 0, 0})
	f.Fuzz(func(t *testing.T, raw []byte) {
		n, err := countEmbeddedSCTs(raw)
		if n < 0 || n > MaxSCTs || (err != nil && n != 0) {
			t.Fatalf("invalid count: %d, %v", n, err)
		}
	})
}
