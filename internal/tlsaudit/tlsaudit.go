// Package tlsaudit holds bounded TLS audit requests and certificate reports.
package tlsaudit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

type auditor struct {
	roots  *x509.CertPool
	now    func() time.Time
	lint   func([]byte) ([]Finding, string, error)
	dialer *net.Dialer
}

func Audit(ctx context.Context, r Request) (Report, error) {
	return (auditor{now: time.Now, lint: zlintFindings}).audit(ctx, r)
}

func (a auditor) audit(ctx context.Context, r Request) (Report, error) {
	if err := Validate(r); err != nil {
		return Report{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, OverallDeadline)
	defer cancel()
	state, err := a.observe(ctx, r, 0)
	if err != nil {
		return Report{}, handshakeError(err)
	}
	if err := validateChain(state.PeerCertificates); err != nil {
		return Report{}, err
	}
	now := time.Now()
	if a.now != nil {
		now = a.now()
	}
	name := r.ServerName
	if name == "" {
		name = r.Host
		if ip, err := netip.ParseAddr(name); err == nil {
			name = ip.WithZone("").String()
		}
	}
	pool := x509.NewCertPool()
	for _, cert := range state.PeerCertificates[1:] {
		pool.AddCert(cert)
	}
	_, verifyErr := state.PeerCertificates[0].Verify(x509.VerifyOptions{DNSName: name, Roots: a.roots, Intermediates: pool, CurrentTime: now})
	report := Report{Host: r.Host, Port: r.Port, ServerName: serverName(r), Verified: verifyErr == nil,
		VerifyError: verificationError(verifyErr, state.PeerCertificates[0], now), Verifier: "go-x509",
		NegotiatedVersion: tls.VersionName(state.Version), CipherSuite: tls.CipherSuiteName(state.CipherSuite),
		ALPN: state.NegotiatedProtocol, OCSPStapled: len(state.OCSPResponse) > 0,
		SCTs:  SCTSummary{TLSExtension: min(len(state.SignedCertificateTimestamps), MaxSCTs)},
		Chain: []Certificate{}, Findings: []Finding{}, Protocols: []ProtocolResult{}}
	if a.roots == nil && (runtime.GOOS == "windows" || runtime.GOOS == "darwin") {
		report.Verifier = runtime.GOOS + "-platform"
	}
	for i, cert := range state.PeerCertificates {
		if ctx.Err() != nil {
			return Report{}, handshakeError(ctx.Err())
		}
		value, findings := certificateReport(cert, i, now)
		report.Chain = append(report.Chain, value)
		report.addFindings(findings...)
		if i == 0 {
			report.SCTs.Embedded = value.EmbeddedSCTs
		}
		if r.Lint && a.lint != nil {
			findings, engine, err := a.lint(cert.Raw)
			report.LintEngine = engine
			if err != nil {
				report.addFindings(finding("lint_failed", "warn", i))
				continue
			}
			for j := range findings {
				findings[j].CertIndex = i
			}
			report.addFindings(findings...)
		}
	}
	report.Protocols = a.protocolMatrix(ctx, r)
	if ctx.Err() != nil {
		return Report{}, handshakeError(ctx.Err())
	}
	report.addFindings(builtInFindings(report, now)...)
	report.sortFindings()
	return report, nil
}

func serverName(r Request) string {
	if r.ServerName != "" {
		return r.ServerName
	}
	if _, err := netip.ParseAddr(r.Host); err == nil {
		return ""
	}
	return r.Host
}

func (a auditor) observe(ctx context.Context, r Request, version uint16) (tls.ConnectionState, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	config := &tls.Config{ServerName: serverName(r), MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS13,
		InsecureSkipVerify: true, //nolint:gosec // audit observes untrusted chains; verification is explicit (B08)
		NextProtos:         []string{"h2", "http/1.1"}}
	if version != 0 {
		config.MinVersion, config.MaxVersion = version, version
		config.CipherSuites = observationCiphers()
	}
	dialer := tls.Dialer{NetDialer: a.dialer, Config: config}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(r.Host, strconv.Itoa(r.Port)))
	if err != nil {
		return tls.ConnectionState{}, err
	}
	defer conn.Close()
	return conn.(*tls.Conn).ConnectionState(), nil
}

func handshakeError(err error) error {
	code := domain.CodeFailed
	if errors.Is(err, context.Canceled) {
		code = domain.CodeCancelled
	}
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
		code = domain.CodeTimeout
	}
	return &domain.Error{Code: code, Message: "TLS handshake failed"}
}

func verificationError(err error, leaf *x509.Certificate, now time.Time) string {
	if err == nil {
		return ""
	}
	if _, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		return "unknown_authority"
	}
	if _, ok := errors.AsType[x509.HostnameError](err); ok {
		return "hostname_mismatch"
	}
	if invalid, ok := errors.AsType[x509.CertificateInvalidError](err); ok && invalid.Reason == x509.Expired {
		if now.After(leaf.NotAfter) {
			return "expired"
		}
		return "not_yet_valid"
	}
	return "invalid"
}

const (
	MaxChain        = 16
	MaxCertDER      = 64 << 10
	MaxSANs         = 256
	MaxURLsPerField = 16
	MaxFindings     = 512
	MaxDetailBytes  = 512
	MaxSCTs         = 64
	ExpiresSoonDays = 30
	MinTimeoutMS    = 500
	MaxTimeoutMS    = 15000
	OverallDeadline = 60 * time.Second
)

type Request struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	ServerName string `json:"server_name,omitempty"`
	TimeoutMS  int    `json:"timeout_ms"`
	Lint       bool   `json:"lint"`
}

type Report struct {
	Host              string           `json:"host"`
	Port              int              `json:"port"`
	ServerName        string           `json:"server_name"`
	NegotiatedVersion string           `json:"negotiated_version"`
	CipherSuite       string           `json:"cipher_suite"`
	ALPN              string           `json:"alpn"`
	Verified          bool             `json:"verified"`
	VerifyError       string           `json:"verify_error,omitempty"`
	Verifier          string           `json:"verifier"`
	OCSPStapled       bool             `json:"ocsp_stapled"`
	SCTs              SCTSummary       `json:"scts"`
	Chain             []Certificate    `json:"chain"`
	Protocols         []ProtocolResult `json:"protocols"`
	Findings          []Finding        `json:"findings"`
	FindingsLimited   bool             `json:"findings_limited"`
	LintEngine        string           `json:"lint_engine,omitempty"`
}

type Certificate struct {
	Index                 int       `json:"index"`
	Subject               string    `json:"subject"`
	Issuer                string    `json:"issuer"`
	SerialHex             string    `json:"serial_hex"`
	NotBefore             time.Time `json:"not_before"`
	NotAfter              time.Time `json:"not_after"`
	DaysRemaining         int       `json:"days_remaining"`
	SANs                  []string  `json:"sans"`
	SANsLimited           bool      `json:"sans_limited"`
	KeyAlgorithm          string    `json:"key_algorithm"`
	KeyBits               int       `json:"key_bits"`
	SignatureAlgorithm    string    `json:"signature_algorithm"`
	IsCA                  bool      `json:"is_ca"`
	SelfSigned            bool      `json:"self_signed"`
	IssuingCertificateURL []string  `json:"aia_issuers"`
	OCSPServers           []string  `json:"ocsp_servers"`
	CRLDistributionPoints []string  `json:"crl_distribution_points"`
	EmbeddedSCTs          int       `json:"embedded_scts"`
	SHA256                string    `json:"sha256"`
}

type SCTSummary struct {
	TLSExtension int `json:"tls_extension"`
	Embedded     int `json:"embedded"`
}

type ProtocolResult struct {
	Version string `json:"version"`
	Status  string `json:"status"`
	Cipher  string `json:"cipher,omitempty"`
}

type Finding struct {
	Source    string `json:"source"`
	ID        string `json:"id"`
	Severity  string `json:"severity"`
	CertIndex int    `json:"cert_index"`
	Detail    string `json:"detail,omitempty"`
}

func Validate(r Request) error {
	if err := network.ValidateHost(r.Host); err != nil {
		return err
	}
	if r.Port < 1 || r.Port > 65535 {
		return domain.Invalid("port must be 1..65535")
	}
	if r.TimeoutMS < MinTimeoutMS || r.TimeoutMS > MaxTimeoutMS {
		return domain.Invalid("timeout must be 500..15000 ms")
	}
	if r.ServerName == "" {
		return nil
	}
	if err := network.ValidateHost(r.ServerName); err != nil {
		return domain.Invalid("invalid server name")
	}
	if _, err := netip.ParseAddr(r.ServerName); err == nil {
		return domain.Invalid("server name must not be an IP address")
	}
	return nil
}
