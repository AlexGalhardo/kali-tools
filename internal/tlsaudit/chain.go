package tlsaudit

import (
	"bytes"
	"cmp"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func validateChain(chain []*x509.Certificate) error {
	if len(chain) == 0 || len(chain) > MaxChain {
		return domain.Invalid("certificate chain exceeds limits")
	}
	for _, cert := range chain {
		if cert == nil || len(cert.Raw) > MaxCertDER {
			return domain.Invalid("certificate chain exceeds limits")
		}
	}
	return nil
}

func certificateReport(cert *x509.Certificate, index int, now time.Time) (Certificate, []Finding) {
	hash := sha256.Sum256(cert.Raw)
	out := Certificate{
		Index: index, Subject: fmt.Sprintf("%.1024s", cert.Subject.String()),
		Issuer: fmt.Sprintf("%.1024s", cert.Issuer.String()), SerialHex: cert.SerialNumber.Text(16),
		NotBefore: cert.NotBefore, NotAfter: cert.NotAfter,
		DaysRemaining: int(math.Floor(cert.NotAfter.Sub(now).Hours() / 24)),
		SANs:          []string{}, KeyAlgorithm: "unknown", SignatureAlgorithm: cert.SignatureAlgorithm.String(),
		IsCA: cert.IsCA, SelfSigned: bytes.Equal(cert.RawSubject, cert.RawIssuer) && cert.CheckSignatureFrom(cert) == nil,
		IssuingCertificateURL: boundedStrings(cert.IssuingCertificateURL, MaxURLsPerField),
		OCSPServers:           boundedStrings(cert.OCSPServer, MaxURLsPerField),
		CRLDistributionPoints: boundedStrings(cert.CRLDistributionPoints, MaxURLsPerField),
		SHA256:                hex.EncodeToString(hash[:]),
	}
	switch key := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		out.KeyAlgorithm, out.KeyBits = "RSA", key.N.BitLen()
	case *ecdsa.PublicKey:
		out.KeyAlgorithm, out.KeyBits = "ECDSA", key.Curve.Params().BitSize
	case ed25519.PublicKey:
		out.KeyAlgorithm, out.KeyBits = "Ed25519", 256
	}
	total := len(cert.DNSNames) + len(cert.IPAddresses) + len(cert.URIs) + len(cert.EmailAddresses)
	out.SANsLimited = total > MaxSANs
	for _, name := range cert.DNSNames[:min(len(cert.DNSNames), MaxSANs)] {
		out.SANs = append(out.SANs, fmt.Sprintf("%.1024s", name))
	}
	for _, ip := range cert.IPAddresses[:min(len(cert.IPAddresses), MaxSANs-len(out.SANs))] {
		out.SANs = append(out.SANs, ip.String())
	}
	for _, uri := range cert.URIs[:min(len(cert.URIs), MaxSANs-len(out.SANs))] {
		out.SANs = append(out.SANs, fmt.Sprintf("%.1024s", uri.String()))
	}
	for _, email := range cert.EmailAddresses[:min(len(cert.EmailAddresses), MaxSANs-len(out.SANs))] {
		out.SANs = append(out.SANs, fmt.Sprintf("%.1024s", email))
	}
	findings := []Finding{}
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 4, 2}) {
			continue
		}
		n, err := countEmbeddedSCTs(ext.Value)
		if err != nil {
			findings = append(findings, finding("sct_extension_malformed", "warn", index))
		} else {
			out.EmbeddedSCTs = n
		}
		break
	}
	return out, findings
}

func boundedStrings(values []string, limit int) []string {
	out := make([]string, 0, min(len(values), limit))
	for _, value := range values[:min(len(values), limit)] {
		out = append(out, fmt.Sprintf("%.1024s", value))
	}
	return out
}

func countEmbeddedSCTs(ext []byte) (int, error) {
	invalid := errors.New("malformed sct extension")
	if len(ext) > MaxCertDER {
		return 0, invalid
	}
	var list []byte
	rest, err := asn1.Unmarshal(ext, &list)
	if err != nil || len(rest) != 0 || len(list) < 2 {
		return 0, invalid
	}
	size := int(binary.BigEndian.Uint16(list))
	if size == 0 || size != len(list)-2 {
		return 0, invalid
	}
	count := 0
	for offset := 2; offset < len(list); {
		if len(list)-offset < 2 {
			return 0, invalid
		}
		n := int(binary.BigEndian.Uint16(list[offset:]))
		offset += 2
		if n == 0 || n > len(list)-offset {
			return 0, invalid
		}
		offset += n
		count++
	}
	return min(count, MaxSCTs), nil
}

func finding(id, severity string, index int) Finding {
	return Finding{Source: "kali-tools", ID: id, Severity: severity, CertIndex: index}
}

func keyFindings(cert Certificate) []Finding {
	if cert.KeyAlgorithm == "RSA" && cert.KeyBits < 2048 {
		return []Finding{finding("weak_rsa_key", "error", cert.Index)}
	}
	if cert.KeyAlgorithm == "ECDSA" && cert.KeyBits < 256 {
		return []Finding{finding("weak_ecdsa_key", "error", cert.Index)}
	}
	return []Finding{}
}

func builtInFindings(r Report, now time.Time) []Finding {
	out := []Finding{}
	if len(r.Chain) == 0 {
		return out
	}
	leaf := r.Chain[0]
	checks := []struct {
		id, severity string
		applies      bool
	}{
		{id: "expired", severity: "error", applies: now.After(leaf.NotAfter)},
		{id: "not_yet_valid", severity: "error", applies: now.Before(leaf.NotBefore)},
		{id: "expires_soon", severity: "warn", applies: leaf.DaysRemaining >= 0 && leaf.DaysRemaining < ExpiresSoonDays},
		{id: "hostname_mismatch", severity: "error", applies: r.VerifyError == "hostname_mismatch"},
		{id: "untrusted_chain", severity: "error", applies: r.VerifyError == "unknown_authority"},
		{id: "self_signed_leaf", severity: "warn", applies: leaf.SelfSigned},
		{id: "no_ocsp_stapling", severity: "info", applies: !r.OCSPStapled && len(leaf.OCSPServers) > 0},
		{id: "no_scts", severity: "info", applies: r.SCTs.TLSExtension+r.SCTs.Embedded == 0},
	}
	for _, check := range checks {
		if check.applies {
			out = append(out, finding(check.id, check.severity, 0))
		}
	}
	for _, cert := range r.Chain {
		out = append(out, keyFindings(cert)...)
		if !cert.SelfSigned && (strings.Contains(cert.SignatureAlgorithm, "MD5") || strings.Contains(cert.SignatureAlgorithm, "SHA1")) {
			out = append(out, finding("weak_signature", "error", cert.Index))
		}
	}
	hasTLS13, hasLegacy := false, false
	for _, p := range r.Protocols {
		if p.Status != "accepted" {
			continue
		}
		if p.Version == "TLS 1.3" {
			hasTLS13 = true
		}
		if p.Version == "TLS 1.0" || p.Version == "TLS 1.1" {
			hasLegacy = true
		}
	}
	if hasLegacy {
		out = append(out, finding("legacy_protocol_accepted", "warn", -1))
	}
	if !hasTLS13 {
		out = append(out, finding("no_tls13", "info", -1))
	}
	return out
}

func (r *Report) addFindings(findings ...Finding) {
	for _, f := range findings {
		if len(r.Findings) == MaxFindings {
			r.FindingsLimited = true
			return
		}
		if len(f.Detail) > MaxDetailBytes {
			end := MaxDetailBytes
			for end > 0 && !utf8.RuneStart(f.Detail[end]) {
				end--
			}
			f.Detail = f.Detail[:end]
		}
		r.Findings = append(r.Findings, f)
	}
}

func (r *Report) sortFindings() {
	rank := map[string]int{"fatal": 0, "error": 1, "warn": 2, "info": 3}
	slices.SortFunc(r.Findings, func(a, b Finding) int {
		if c := cmp.Compare(rank[a.Severity], rank[b.Severity]); c != 0 {
			return c
		}
		if c := cmp.Compare(a.CertIndex, b.CertIndex); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
}
