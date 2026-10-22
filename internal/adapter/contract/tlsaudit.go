package contract

import (
	"slices"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/tlsaudit"
)

func (r TLSAuditRequest) Request() tlsaudit.Request { return tlsaudit.Request(r) }

func FromTLSAudit(v tlsaudit.Report) TLSAuditReport {
	return TLSAuditReport{Host: v.Host, Port: v.Port, ServerName: v.ServerName,
		NegotiatedVersion: v.NegotiatedVersion, CipherSuite: v.CipherSuite, ALPN: v.ALPN,
		Verified: v.Verified, VerifyError: v.VerifyError, Verifier: v.Verifier, OCSPStapled: v.OCSPStapled,
		SCTs: TLSAuditSCTSummary(v.SCTs), FindingsLimited: v.FindingsLimited, LintEngine: v.LintEngine,
		Chain: mapSlice(v.Chain, func(c tlsaudit.Certificate) TLSAuditCertificate {
			out := TLSAuditCertificate(c)
			out.SANs = slices.Clone(c.SANs)
			out.IssuingCertificateURL = slices.Clone(c.IssuingCertificateURL)
			out.OCSPServers = slices.Clone(c.OCSPServers)
			out.CRLDistributionPoints = slices.Clone(c.CRLDistributionPoints)
			return out
		}),
		Protocols: mapSlice(v.Protocols, func(p tlsaudit.ProtocolResult) TLSAuditProtocolResult { return TLSAuditProtocolResult(p) }),
		Findings:  mapSlice(v.Findings, func(f tlsaudit.Finding) TLSAuditFinding { return TLSAuditFinding(f) }),
	}
}

type TLSAuditRequest struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	ServerName string `json:"server_name,omitempty"`
	TimeoutMS  int    `json:"timeout_ms"`
	Lint       bool   `json:"lint"`
}

type TLSAuditReport struct {
	Host              string                   `json:"host"`
	Port              int                      `json:"port"`
	ServerName        string                   `json:"server_name"`
	NegotiatedVersion string                   `json:"negotiated_version"`
	CipherSuite       string                   `json:"cipher_suite"`
	ALPN              string                   `json:"alpn"`
	Verified          bool                     `json:"verified"`
	VerifyError       string                   `json:"verify_error,omitempty"`
	Verifier          string                   `json:"verifier"`
	OCSPStapled       bool                     `json:"ocsp_stapled"`
	SCTs              TLSAuditSCTSummary       `json:"scts"`
	Chain             []TLSAuditCertificate    `json:"chain"`
	Protocols         []TLSAuditProtocolResult `json:"protocols"`
	Findings          []TLSAuditFinding        `json:"findings"`
	FindingsLimited   bool                     `json:"findings_limited"`
	LintEngine        string                   `json:"lint_engine,omitempty"`
}

type TLSAuditCertificate struct {
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

type TLSAuditSCTSummary struct {
	TLSExtension int `json:"tls_extension"`
	Embedded     int `json:"embedded"`
}

type TLSAuditProtocolResult struct {
	Version string `json:"version"`
	Status  string `json:"status"`
	Cipher  string `json:"cipher,omitempty"`
}

type TLSAuditFinding struct {
	Source    string `json:"source"`
	ID        string `json:"id"`
	Severity  string `json:"severity"`
	CertIndex int    `json:"cert_index"`
	Detail    string `json:"detail,omitempty"`
}
