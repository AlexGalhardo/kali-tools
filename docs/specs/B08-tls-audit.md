# B08 TLS and certificate audit

| Field | Value |
| --- | --- |
| PLAN tasks | B08 (chain report, protocol matrix, zlint findings; optional zcrypto legacy observation) |
| Status | Dependency review complete (2026-10-22); implementation pending |
| Depends on | H02 (job manager), H04 (`app.Application`), H05/H06 (adapters, DTOs), W01 + W04 (HTTP route) |
| Risk | Medium: a new dependency tree (zlint, zcrypto, logrus, publicsuffix-go, go-toml) and probes that intentionally accept untrusted chains for observation |

## 1. Goal

Add a TLS audit capability that reports what a TLS endpoint presents and accepts, without exploiting anything. It lives in a new capability package, `internal/tlsaudit`, named so it does not shadow `crypto/tls`. It reports:

- a certificate chain report (validity, SAN coverage, key and signature algorithms, AIA/OCSP/CRL URLs, stapling, SCT counts);
- a protocol support matrix (TLS 1.0–1.3), probed with `MinVersion = MaxVersion` per attempt;
- zlint findings for every presented certificate.

The capability is exposed in the GUI, the web server and the CLI. Optional legacy-protocol observation through zcrypto is a documented follow-up behind a labelled switch; it is not implemented here.

## 2. Non-goals

- No exploit or vulnerability checks: no Heartbleed-style reads, no renegotiation attacks, no padding oracles, no cipher downgrade attacks, no ROBOT.
- No full cipher-suite enumeration (only the negotiated suite per version), and no JA3/JA4 fingerprinting. JA4+ is excluded by PLAN §11.
- No CT log queries (OSINT, PLAN §11) and no OCSP/CRL network fetches. URLs are only reported.
- No zcrypto handshake code (follow-up §4.8). zcrypto enters only as zlint's certificate parser.
- No testssl.sh adapter.
- No change to the existing `network` TLS diagnostic (`internal/network/tls.go`), which keeps `MinVersion: TLS12` and strict verification.
- No STARTTLS protocols (SMTP/IMAP/XMPP).
- No persistence of hostnames, certificates or findings. History stores counts only.

## 3. Current state

Re-verify each fact before starting:

1. `internal/network/tls.go:12-33`: `diagnoseTLS(ctx, host, port, roots)` dials with `tls.Dialer{Config: &tls.Config{MinVersion: tls.VersionTLS12, ServerName, RootCAs: roots}}`. It returns at most 16 strings `subject=… issuer=… expires=…`, with subject and issuer cut at 1,024 characters.
2. `internal/network/network.go:325-351`: `diagnosticErrorCode` maps `x509.UnknownAuthorityError`, `x509.HostnameError` and `x509.CertificateInvalidError` to `certificate_invalid`.
3. `internal/network/network.go:70-89`: `ValidateHost` rejects empty hosts, hosts over 253 characters, the characters `/\ \r\n\t @?#` and bad labels. It accepts IP literals, including zoned IPv6.
4. `internal/network/tls_test.go` builds fixtures with `httptest.NewTLSServer` and self-generated ECDSA certificates (trusted, untrusted, expired).
5. `docs/tools-research.md` §4.4 lists the building blocks:
    - stdlib `crypto/tls` + `crypto/x509`;
    - `zmap/zlint` (Apache-2.0);
    - `zmap/zcrypto` (Apache-2.0, research-grade, no releases);
    - on Windows, `SystemCertPool` delegates to the platform verifier.
6. `PLAN.md` B08 names the package `internal/tls`. This spec uses `internal/tlsaudit` on purpose and updates PLAN.md.
7. `go.mod` has none of `github.com/zmap/*`, `github.com/sirupsen/logrus`, `github.com/weppos/publicsuffix-go` or `github.com/pelletier/go-toml`.
8. Upstream `github.com/zmap/zlint/v3` (latest tag v3.7.2 at research time) requires Go ≥ 1.25.0. It depends on:
    - `github.com/zmap/zcrypto` (pseudo-version);
    - `github.com/sirupsen/logrus`;
    - `github.com/pelletier/go-toml v1.9.5`;
    - `github.com/weppos/publicsuffix-go` (pseudo-version);
    - `golang.org/x/crypto`, `x/net`, `x/text` and `x/sys`.

    Its API is `zlint.LintCertificate(c *zcryptox509.Certificate) *zlint.ResultSet` and `zlint.LintCertificateEx(c, lint.Registry)`. `lint.LintStatus` values: `Reserved=0`, `NA=1`, `NE=2`, `Pass=3`, `Notice=4`, `Warn=5`, `Error=6`, `Fatal=7`.

## 4. Design

### 4.1 Package and types (`internal/tlsaudit`)

```go
package tlsaudit

const (
    MaxChain         = 16
    MaxCertDER       = 64 << 10
    MaxSANs          = 256
    MaxURLsPerField  = 16
    MaxFindings      = 512
    MaxDetailBytes   = 512
    MaxSCTs          = 64
    ExpiresSoonDays  = 30
    MinTimeoutMS     = 500
    MaxTimeoutMS     = 15000
    OverallDeadline  = 60 * time.Second
)

type Request struct {
    Host       string `json:"host"`                  // network.ValidateHost
    Port       int    `json:"port"`                  // 1..65535
    ServerName string `json:"server_name,omitempty"` // SNI; defaults to Host unless Host is an IP; network.ValidateHost
    TimeoutMS  int    `json:"timeout_ms"`            // per attempt, 500..15000
    Lint       bool   `json:"lint"`                  // run zlint (default true in adapters)
}

type Report struct {
    Host              string           `json:"host"`
    Port              int              `json:"port"`
    ServerName        string           `json:"server_name"`
    NegotiatedVersion string           `json:"negotiated_version"` // "TLS 1.3"
    CipherSuite       string           `json:"cipher_suite"`       // tls.CipherSuiteName
    ALPN              string           `json:"alpn"`
    Verified          bool             `json:"verified"`
    VerifyError       string           `json:"verify_error,omitempty"` // "unknown_authority" | "hostname_mismatch" | "expired" | "not_yet_valid" | "invalid"
    Verifier          string           `json:"verifier"`               // "go-x509" | "windows-platform" | "darwin-platform"
    OCSPStapled       bool             `json:"ocsp_stapled"`
    SCTs              SCTSummary       `json:"scts"`
    Chain             []Certificate    `json:"chain"`
    Protocols         []ProtocolResult `json:"protocols"`
    Findings          []Finding        `json:"findings"`
    FindingsLimited   bool             `json:"findings_limited"`
    LintEngine        string           `json:"lint_engine,omitempty"` // "zlint v3.x.y"
}

type Certificate struct {
    Index                 int       `json:"index"` // 0 = leaf
    Subject               string    `json:"subject"`
    Issuer                string    `json:"issuer"`
    SerialHex             string    `json:"serial_hex"`
    NotBefore             time.Time `json:"not_before"`
    NotAfter              time.Time `json:"not_after"`
    DaysRemaining         int       `json:"days_remaining"` // negative when expired
    SANs                  []string  `json:"sans"`           // DNS names, IPs, URIs, emails; ≤ MaxSANs
    SANsLimited           bool      `json:"sans_limited"`
    KeyAlgorithm          string    `json:"key_algorithm"` // "RSA" | "ECDSA" | "Ed25519" | "unknown"
    KeyBits               int       `json:"key_bits"`      // RSA modulus bits, ECDSA curve bits, 256 for Ed25519
    SignatureAlgorithm    string    `json:"signature_algorithm"`
    IsCA                  bool      `json:"is_ca"`
    SelfSigned            bool      `json:"self_signed"`
    IssuingCertificateURL []string  `json:"aia_issuers"`
    OCSPServers           []string  `json:"ocsp_servers"`
    CRLDistributionPoints []string  `json:"crl_distribution_points"`
    EmbeddedSCTs          int       `json:"embedded_scts"`
    SHA256                string    `json:"sha256"` // hex of DER
}

type SCTSummary struct {
    TLSExtension int `json:"tls_extension"` // len(ConnectionState.SignedCertificateTimestamps)
    Embedded     int `json:"embedded"`      // leaf extension 1.3.6.1.4.1.11129.2.4.2
}

type ProtocolResult struct {
    Version string `json:"version"` // "TLS 1.0" | "TLS 1.1" | "TLS 1.2" | "TLS 1.3"
    Status  string `json:"status"`  // "accepted" | "rejected" | "client_unsupported" | "timeout" | "error"
    Cipher  string `json:"cipher,omitempty"`
}

type Finding struct {
    Source    string `json:"source"`   // "kali-tools" | "zlint"
    ID        string `json:"id"`       // stable id, e.g. "expired", "e_sub_cert_aia_missing"
    Severity  string `json:"severity"` // "info" | "warn" | "error" | "fatal"
    CertIndex int    `json:"cert_index"` // -1 for endpoint-level findings
    Detail    string `json:"detail,omitempty"` // ≤ MaxDetailBytes, no certificate bytes
}

func Validate(r Request) error
// Audit runs every step sequentially under one overall deadline.
func Audit(ctx context.Context, r Request) (Report, error)
```

`Audit` has injection points for tests through an unexported struct:

```go
type auditor struct {
    roots  *x509.CertPool          // nil = system roots / platform verifier
    now    func() time.Time
    lint   func(der []byte) ([]Finding, string, error) // zlint adapter; nil skips linting
    dialer *net.Dialer
}
```

`Audit` uses `auditor{now: time.Now, lint: zlintFindings}`. Tests construct `auditor` with fixture roots.

### 4.2 Algorithm

1. Run `Validate`:
    - `network.ValidateHost(Host)`;
    - the port must be 1..65535;
    - `ServerName` must be empty or pass `ValidateHost` and must not be an IP literal;
    - `TimeoutMS` must be in `MinTimeoutMS..MaxTimeoutMS`.

    Errors are `invalid_input`. Then derive `ctx` with `OverallDeadline`.
2. **Observation handshake.** Dial with `tls.Dialer`:
    - `Config{ServerName: sni, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true, NextProtos: []string{"h2", "http/1.1"}}`;
    - a per-attempt timeout of `TimeoutMS`.

    `InsecureSkipVerify` is required to observe untrusted chains. Verification is done explicitly in step 3, and **no application data is ever sent**; the connection is closed right after `ConnectionState()`. Annotate the line with `//nolint:gosec // audit observes untrusted chains; verification is explicit (B08)`.

    Failures:
    - a dial or handshake failure here maps through the existing `network`-style classification: `timeout`, `cancelled`, or `failed` with the message "TLS handshake failed";
    - a failure here ends the audit with that error.
3. **Verification.**
    - Parse `state.PeerCertificates`; more than `MaxChain` certificates or one over `MaxCertDER` gives `invalid_input` "certificate chain exceeds limits".
    - Build `x509.VerifyOptions{DNSName: sniOrIP, Intermediates: pool(chain[1:]), Roots: a.roots, CurrentTime: a.now()}` and call `leaf.Verify(opts)`.
    - Map the error to `VerifyError`:
        - `UnknownAuthorityError` → `unknown_authority`;
        - `HostnameError` → `hostname_mismatch`;
        - `CertificateInvalidError` with `Reason == x509.Expired` → `expired` when `now > NotAfter`, else `not_yet_valid`;
        - other errors → `invalid`.
    - `Verifier` is `"go-x509"` when `roots != nil` or GOOS is not windows/darwin; it is `"windows-platform"` or `"darwin-platform"` when `roots == nil` on those GOOS.
4. **Chain report.** For each certificate (index order as presented), fill `Certificate`:
    - `KeyBits`: `rsa.PublicKey.N.BitLen()`, `ecdsa.PublicKey.Curve.Params().BitSize`, or 256 for Ed25519;
    - `SelfSigned`: `bytes.Equal(RawSubject, RawIssuer)` **and** `CheckSignatureFrom(itself) == nil`;
    - `EmbeddedSCTs`: parsed by `countEmbeddedSCTs(ext []byte) (int, error)`, bounded by `MaxSCTs`. The format is an ASN.1 OCTET STRING containing a TLS `opaque SerializedSCT<1..2^16-1>` list, where each entry has a 2-byte length prefix. A malformed extension gives count 0 and finding `sct_extension_malformed` (warn);
    - strings truncated with the existing `%.1024s` rule, and lists capped at `MaxURLsPerField`/`MaxSANs`.

    Also set `OCSPStapled = len(state.OCSPResponse) > 0`, `SCTs.TLSExtension = min(len(state.SignedCertificateTimestamps), MaxSCTs)`, `NegotiatedVersion = tls.VersionName(state.Version)`, `CipherSuite = tls.CipherSuiteName(state.CipherSuite)` and `ALPN = state.NegotiatedProtocol`.
5. **Protocol matrix.** For each `v` in `[TLS10, TLS11, TLS12, TLS13]`, sequentially, run a new handshake with `MinVersion = MaxVersion = v`, `InsecureSkipVerify: true`, the same SNI and a per-attempt timeout of `TimeoutMS`.
    - For v < TLS 1.3, set `CipherSuites` to the IDs of `tls.CipherSuites()` followed by `tls.InsecureCipherSuites()`, so servers that only offer RSA key exchange or 3DES are observed instead of misreported.
    - Map the outcome to a status:
        - handshake ok gives `accepted` with the cipher;
        - a server alert (the error string contains `remote error: tls:`) or EOF during the handshake gives `rejected`;
        - `clientSupports(v)` false (§4.3) gives `client_unsupported`, and no network attempt is made for that version;
        - a deadline gives `timeout`;
        - anything else gives `error`.
    - Close every connection immediately and send no application data.
6. **Built-in findings** (`Source: "kali-tools"`):

    | ID | Severity | Condition |
    | --- | --- | --- |
    | `expired` | error | leaf `now > NotAfter` |
    | `not_yet_valid` | error | leaf `now < NotBefore` |
    | `expires_soon` | warn | leaf `0 ≤ DaysRemaining < ExpiresSoonDays` |
    | `hostname_mismatch` | error | `VerifyError == "hostname_mismatch"` |
    | `untrusted_chain` | error | `VerifyError == "unknown_authority"` |
    | `self_signed_leaf` | warn | leaf `SelfSigned` |
    | `weak_rsa_key` | error | RSA `KeyBits < 2048` (any cert) |
    | `weak_ecdsa_key` | error | ECDSA `KeyBits < 256` |
    | `weak_signature` | error | `SignatureAlgorithm` is MD5- or SHA1-based, for any non-root cert |
    | `legacy_protocol_accepted` | warn | TLS 1.0 or 1.1 `accepted` |
    | `no_tls13` | info | TLS 1.3 not `accepted` |
    | `no_ocsp_stapling` | info | `!OCSPStapled` and the leaf has `OCSPServers` |
    | `no_scts` | info | `SCTs.TLSExtension + SCTs.Embedded == 0` |

7. **zlint** (when `r.Lint`): for each certificate, call `a.lint(cert.Raw)`. Append findings, capping the total at `MaxFindings` (`FindingsLimited`). A zlint error for one certificate adds the finding `lint_failed` (warn) with `CertIndex`; the audit continues.
8. Sort findings by severity (fatal, error, warn, info), then `CertIndex`, then `ID`.

### 4.3 TLS version capability self-test

Go may refuse to offer TLS 1.0/1.1 in some toolchain or GODEBUG configurations. `clientSupports(v uint16) bool` runs once per process (`sync.OnceValue`): a loopback handshake between an in-process `tls.Server` and `tls.Client` both pinned to `v`, using an ephemeral self-signed ECDSA P-256 certificate and a 2 s deadline over `net.Pipe`. If it fails, the matrix entry is `client_unsupported` and no network attempt is made. This keeps "the server rejected TLS 1.0" honest.

### 4.4 zlint adapter (`internal/tlsaudit/zlint.go`)

```go
import (
    zx509 "github.com/zmap/zcrypto/x509"
    "github.com/zmap/zlint/v3"
    "github.com/zmap/zlint/v3/lint"
)

// zlintFindings parses DER with zcrypto and returns findings with Status >= Notice.
func zlintFindings(der []byte) ([]Finding, string, error)
```

- Call `zx509.ParseCertificate(der)`. On error, return it; the caller maps it to `lint_failed`.
- Call `rs := zlint.LintCertificate(c)`. For each `name, res := range rs.Results`, where `res.Status >= lint.Notice`, emit a `Finding`:
  - `Source "zlint"`;
  - `ID name`;
  - `Severity` from the status (`Notice`→`info`, `Warn`→`warn`, `Error`→`error`, `Fatal`→`fatal`);
  - `Detail` set to `res.Details` truncated to `MaxDetailBytes`.
- Iterate in sorted name order so output is deterministic.
- The engine string is `"zlint " + zlintModuleVersion`. Read it from `runtime/debug.ReadBuildInfo()` for the module `github.com/zmap/zlint/v3`, falling back to `"zlint v3"`.
- zlint uses logrus. Before the first lint, set `logrus.SetOutput(io.Discard)` once (`sync.Once`) so no certificate data reaches stderr. This is a process-global side effect and must be documented in a comment.

### 4.5 Dependency and license review (mandatory before step 4 of §6)

1. Run `go get github.com/zmap/zlint/v3@<latest v3 tag>` (v3.7.2 or newer at implementation time) and record the exact version.
2. Check whether `go.mod` raises `golang.org/x/*` versions. Re-run `govulncheck ./...` and `govulncheck -tags desktop .`.
3. For each new module in `go list -m -json all` (expected: `zmap/zlint/v3`, `zmap/zcrypto`, `sirupsen/logrus`, `pelletier/go-toml`, `weppos/publicsuffix-go`), open the LICENSE (and NOTICE) in the module cache and record the SPDX id:
    - zlint: Apache-2.0; v3.7.2 contains LICENSE but no NOTICE;
    - zcrypto: Apache-2.0 AND ISC AND MIT (verified pinned module LICENSE);
    - logrus: MIT;
    - go-toml v1: MIT AND Apache-2.0 (verified pinned module LICENSE);
    - publicsuffix-go: MIT (verify).

    Any non-permissive license (GPL/AGPL/SSPL/unknown) stops the spec and needs an owner decision.
4. Confirm that the release notices generator (`scripts/notices.mjs` or its current name) picks up the Apache NOTICE files and that `THIRD_PARTY_NOTICES.txt` contains them.
5. Measure the binary size of `go build -trimpath -ldflags="-s -w" ./cmd/kali-tools` before and after, and record both. If the growth exceeds 8 MiB, raise it to the owner before merging.
6. Add the rows to `docs/dependencies.md` (§10).

### 4.6 Application, adapters and UI

```go
// internal/app
func (a *Application) AuditTLS(ctx context.Context, r tlsaudit.Request) (tlsaudit.Report, error)
```

- It runs as an operation job with kind `tls_audit` and label "TLS audit" (H02 job manager), and counts against the 8-operation cap.
- History summary: `"TLS audit <state>: <n> certificates, <e> errors, <w> warnings"`. Never the host.
- Register the capability `tls_audit`: `available` on all OSes, provider `go/crypto+zlint`, detail "Chain, protocol matrix (TLS 1.0–1.3) and zlint; no exploit checks".

| Interface | Change |
| --- | --- |
| Wails (`internal/adapter/wailsui`) | `AuditTLS(r contract.TLSAuditRequest) (contract.TLSAuditReport, error)` |
| HTTP (`internal/adapter/httpapi`) | `POST /api/tls/audit`, JSON body ≤ 4 KiB, response the report |
| CLI (`internal/adapter/cli`) | `kali-tools tls-audit --target <host> [--port 443] [--server-name <sni>] [--timeout 5000] [--lint=true]`, JSON envelope; exit 0 even with findings, exit 1 on handshake failure, exit 2 on invalid input |
| DTO (`internal/adapter/contract`) | Mirrors §4.1 with the same JSON names |

If H05/W01 have not landed, add these to `app.go` / `cmd/kali-tools` until H05 lands and record the HTTP route as a follow-up.

Frontend: `frontend/src/features/tls/TLSAuditPanel.tsx`, opened from the Diagnostics page as a "TLS audit" tab. It contains:

- a form (host, port, SNI, lint toggle);
- a summary card (negotiated version, cipher, verified with verifier, stapling, SCTs);
- the protocol matrix as a 4-row table;
- the chain as an accordion per certificate;
- the findings table sorted by severity, 100 rows per page.

i18n keys (both catalogs): `tlsAudit`, `serverName`, `runLint`, `negotiated`, `verifiedBy`, `ocspStapled`, `scts`, `protocolMatrix`, `chain`, `findings`, `findingsLimited`, plus the status and severity keys `tls_accepted`, `tls_rejected`, `tls_client_unsupported`, `tls_timeout`, `tls_error`, `severity_info`, `severity_warn`, `severity_error` and `severity_fatal`. Finding IDs from zlint are shown verbatim (English), with `detail`.

### 4.7 Errors

| Situation | Code |
| --- | --- |
| Invalid request | `invalid_input` |
| Dial or handshake timeout | `timeout` |
| Context cancelled | `cancelled` |
| TCP refused, handshake failure on the observation dial | `failed` ("TLS handshake failed") |
| Chain over limits | `invalid_input` |

An untrusted, expired or mismatched certificate is **not** an error. It is reported as `Verified=false` plus findings.

### 4.8 Follow-up (not in this spec): zcrypto legacy observation

Behind a labelled switch, `Request.LegacyObservation bool`, shown in the UI as "Legacy observation (research-grade zcrypto client; not for trust decisions)": an isolated adapter `internal/adapter/zcryptotls` would probe SSLv3 and export/RC4 suites with `github.com/zmap/zcrypto/tls`. It would never be used for kali-tools' own connections, and the result would carry `Source: "zcrypto"`. It needs its own spec, because zcrypto has no releases and is explicitly not for production. Record it in `docs/tools-research.md` §11.

## 5. Files

Create:

- `internal/tlsaudit/tlsaudit.go`: types, constants, `Validate`, `Audit`, `auditor`.
- `internal/tlsaudit/chain.go`: certificate report, SCT counting, built-in findings.
- `internal/tlsaudit/matrix.go`: protocol matrix and `clientSupports`.
- `internal/tlsaudit/zlint.go`: zlint adapter and the logrus silencing.
- `internal/tlsaudit/tlsaudit_test.go`, `chain_test.go`, `matrix_test.go`, `zlint_test.go`.
- `internal/tlsaudit/fixtures_test.go`: in-test certificate factory (ECDSA P-256, RSA 1024/2048, self-signed, expired, mismatched, SHA1-signed).
- `internal/app/tlsaudit.go` and `internal/app/tlsaudit_test.go`.
- `frontend/src/features/tls/TLSAuditPanel.tsx`.

Modify:

- `go.mod`, `go.sum`: zlint v3 and its transitive modules.
- `internal/app/capabilities.go`: `tls_audit`.
- Driving adapters and DTOs; `frontend/src/shared/api/bridge.ts`, `frontend/src/app/App.tsx`, `frontend/src/shared/i18n/i18n.ts`, `frontend/tests/workflows.spec.ts`.
- `docs/dependencies.md`, `docs/architecture.md`, `docs/tools-research.md`, `PLAN.md`, `docs/licenses/` (add the Apache-2.0 text if not present).

Delete: none.

## 6. Implementation steps

1. Run the dependency and license review (§4.5, steps 1–4) **before** writing code. Stop if any license is not permissive.
2. Write the types, `Validate` and its bound tests.
3. Write the chain report and the built-in findings with fixture certificates (no network).
4. Write the observation handshake and verification against `httptest` TLS servers.
5. Write the protocol matrix and `clientSupports`, with fixture servers pinned to single versions.
6. Write the zlint adapter. Test it with a fixture leaf that zlint must flag, for example a missing AIA on a "subscriber" certificate. Assert at least one `zlint` finding and deterministic order.
7. Add the app use case, capability, driving adapters and DTOs.
8. Frontend panel, i18n, Playwright.
9. Measure the binary size (§4.5 step 5). Update the docs.
10. Write `docs/verification/specs/B08.md`.

## 7. Acceptance criteria

- **AC-1.** Given invalid requests (empty host, port 0/65536, timeout 499/15001, `ServerName` that is an IP, host containing `/`), when `Validate` runs, then `invalid_input` is returned. Given the boundary values 500 and 15000, they are accepted.
- **AC-2.** Given an `httptest` TLS server with a certificate trusted through `auditor.roots`, when audited, then:
  - `Verified=true`;
  - `Verifier="go-x509"`;
  - TLS 1.2 and 1.3 are `accepted`;
  - there is no error-severity built-in finding;
  - the chain has 1 certificate with SANs that include `127.0.0.1`.
- **AC-3.** Given an expired trusted certificate, when audited, then the audit **does not fail**: `Verified=false`, `VerifyError="expired"`, and finding `expired` (error).
- **AC-4.** Given a self-signed certificate not in the roots, then `VerifyError="unknown_authority"` and the findings include `untrusted_chain` and `self_signed_leaf`.
- **AC-5.** Given a certificate for `other.test` and `ServerName="fixture.test"`, then `VerifyError="hostname_mismatch"` and finding `hostname_mismatch` is present.
- **AC-6.** Given an RSA-1024 leaf, then finding `weak_rsa_key` is present. Given an RSA-2048 leaf, it is absent.
- **AC-7.** Given a server pinned to `MinVersion=MaxVersion=TLS 1.0` (or 1.1), when the matrix runs, then that version is `accepted` (or `client_unsupported` when `clientSupports` is false), TLS 1.2 and 1.3 are `rejected`, and `legacy_protocol_accepted` is present only when a legacy version was `accepted`.
- **AC-8.** Given a server that accepts TCP and never answers, then the audit returns `timeout` within `TimeoutMS + 1 s`. Given a cancelled context, `cancelled`.
- **AC-9.** Given `Lint=true` on a fixture leaf lacking the AIA extension, then at least one finding with `Source="zlint"` exists, the findings are sorted, and `LintEngine` starts with `zlint`. Given `Lint=false`, there are no zlint findings.
- **AC-10.** Given a chain of 17 certificates (fake server), then `invalid_input`. Given 600 synthetic findings, 512 are returned and `FindingsLimited=true`.
- **AC-11.** Given any audit, when the app use case completes, then the history summary contains no hostname, IP or certificate subject (QG-11).
- **AC-12 [native].** Given the owner's Windows 11 host with internet access, when `kali-tools tls-audit --target example.com` runs, then `Verifier="windows-platform"`, `Verified=true` and TLS 1.2 or 1.3 is `accepted`. Record the output.

## 8. Tests

| Test | Package | Asserts | ACs |
| --- | --- | --- | --- |
| `TestValidateTLSAuditRequestBounds` | `internal/tlsaudit` | Table including boundaries | AC-1 |
| `TestAuditTrustedFixture` | same | AC-2 fields | AC-2 |
| `TestAuditReportsExpiredWithoutFailing` | same | AC-3 | AC-3 |
| `TestAuditSelfSignedAndUntrusted` | same | AC-4 | AC-4 |
| `TestAuditHostnameMismatch` | same | AC-5 | AC-5 |
| `TestWeakKeyFindings` | same | RSA 1024 vs 2048, ECDSA P-224 is rejected by Go so it is tested via the pure `keyFindings` function | AC-6 |
| `TestProtocolMatrixLegacyOnlyServer` | same | AC-7; the server uses an explicit `MinVersion`/`MaxVersion` and the CBC-SHA ECDHE suite | AC-7 |
| `TestClientSupportsSelfTest` | same | Returns true for TLS 1.2/1.3; deterministic across two calls | AC-7 |
| `TestAuditTimeoutAndCancel` | same | AC-8 | AC-8 |
| `TestZlintFindingsDeterministic` | same | AC-9 | AC-9 |
| `TestCountEmbeddedSCTs` | same | Valid list of 2; malformed lengths; 65 entries capped at 64 | QG-12 |
| `TestChainAndFindingLimits` | same | AC-10 | AC-10 |
| `TestAuditTLSHistoryRedaction` | `internal/app` | AC-11 with a fake job store | AC-11 |
| `TestCLITLSAuditExitCodes` | `internal/adapter/cli` | 0 with findings, 1 on handshake failure, 2 on invalid input | CLI |
| `TestTLSAuditRouteBodyLimit` | `internal/adapter/httpapi` | 4 KiB + 1 body gives 413 | HTTP |
| Playwright `TLS audit shows matrix, chain and sorted findings` | `frontend/tests/workflows.spec.ts` | Contract fake | UI |

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
go test -race ./internal/tlsaudit/...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go list -m -json all | node -e "let s='';process.stdin.on('data',d=>s+=d).on('end',()=>{console.log(s.match(/\"Path\": \"[^\"]+\"/g).length)})"
go build -trimpath -ldflags="-s -w" -o /tmp/kt-after ./cmd/kali-tools && ls -l /tmp/kt-after
```

## 10. Documentation updates

- `docs/dependencies.md`:
  - add rows for `github.com/zmap/zlint/v3` (Apache-2.0, certificate linting);
  - `github.com/zmap/zcrypto` (Apache-2.0 / BSD-3-Clause, zlint parser only, pseudo-version, research-grade);
  - `github.com/sirupsen/logrus` (MIT, transitive, output silenced);
  - `github.com/pelletier/go-toml` (MIT, transitive);
  - `github.com/weppos/publicsuffix-go` (MIT, transitive).

  Use the verified SPDX ids from §4.5.
- `docs/architecture.md`:
  - §2 and §3: add `internal/tlsaudit` as a capability package;
  - §7.1: replace the example "for example TLS audit" with a link to this spec.
- `docs/tools-research.md` §4.4: mention the package name `internal/tlsaudit`; §11: the zcrypto follow-up.
- `PLAN.md` B08:
  - rename `internal/tls` to `internal/tlsaudit`;
  - tick the chain/matrix/zlint item after AC-1 to AC-11 pass;
  - leave the zcrypto item open, as a follow-up.
- `README.md`: CLI list adds `tls-audit`.

## 11. Risks and rollback

- **zcrypto has no releases** (pseudo-versions) and is research-grade. Mitigation: it is used only for parsing inside zlint, and it is pinned by `go.sum`. A revisit trigger lives in `docs/dependencies.md`.
- **Global logrus output.** Silencing changes the process-wide logrus logger. No other kali-tools dependency uses logrus today; verify with `go mod why github.com/sirupsen/logrus`.
- **Toolchain refuses TLS 1.0/1.1.** This is handled by `client_unsupported`, so the matrix never claims a server rejection it did not observe.
- **Binary size and init time.** zlint registers hundreds of lints in `init`. Measured in §4.5 step 5.
- **Rollback.** Revert the commits and run `go mod tidy`. No persisted data changes.

## 12. Quality and delivery assertions

### 12.1 Applicable quality gates

| Gate | Applies | Tightened threshold |
| --- | --- | --- |
| QG-1, QG-2, QG-3, QG-4, QG-5, QG-8, QG-9, QG-13, QG-14, QG-15 | yes | QG-8: +1 workflow |
| QG-6 | yes | `internal/tlsaudit` ≥ 85% statements |
| QG-7 | yes | govulncheck on the default and desktop tags after adding zlint: no reachable vulnerability |
| QG-10 | yes | `internal/tlsaudit` imports no `internal/adapter/*`, `internal/app` or `net/http` |
| QG-11 | yes | history and logs contain no host, SNI, subject or certificate bytes (`TestAuditTLSHistoryRedaction`) |
| QG-12 | yes | each constant in §4.1 has a limit and limit+1 test (`TestValidateTLSAuditRequestBounds`, `TestCountEmbeddedSCTs`, `TestChainAndFindingLimits`) |

### 12.2 Delivery assertions

| ID | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | Package name does not shadow stdlib | `go list ./internal/... \| grep -x '.*/internal/tls'` | no output | CI |
| DA-2 | Licenses recorded | `docs/dependencies.md` rows for the 5 modules with SPDX ids; `THIRD_PARTY_NOTICES.txt` in the CI release artifact contains "zlint" | rows present; grep count ≥ 1 | reviewer, CI |
| DA-3 | No exploit checks exist | `grep -rniE 'heartbeat\|heartbleed\|renegotiat\|padding.?oracle' internal/tlsaudit` | no output | CI |
| DA-4 | Untrusted chains are reported, not failed | `go test -run 'TestAuditReportsExpiredWithoutFailing\|TestAuditSelfSignedAndUntrusted' ./internal/tlsaudit/` | PASS | CI |
| DA-5 | Matrix honesty | `go test -run 'TestProtocolMatrixLegacyOnlyServer\|TestClientSupportsSelfTest' ./internal/tlsaudit/` | PASS | CI |
| DA-6 | Binary size recorded | before/after sizes in the evidence record | both numbers present; growth ≤ 8 MiB or an owner sign-off linked | reviewer |
| DA-7 | Existing TLS diagnostic unchanged | `git diff <base> -- internal/network/tls.go` | empty | reviewer |
| DA-8 [native] | Platform verifier on Windows | Procedure: on Windows 11, run `kali-tools tls-audit --target example.com --port 443` | `verifier == "windows-platform"`, `verified == true` | [native] |

For DA-8, record the Windows build, network type, date (UTC) and operator.

Evidence record: `docs/verification/specs/B08.md`.
