// Package tlsaudit holds bounded TLS audit requests and certificate reports.
package tlsaudit

import (
	"net/netip"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

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
