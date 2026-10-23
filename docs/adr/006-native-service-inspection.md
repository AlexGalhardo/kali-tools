# ADR 006: native TCP service inspection

Status: accepted. Supersedes [ADR 003](003-optional-nmap.md). Linux loopback checks passed; Windows, macOS and non-loopback qualification remain open.

On 2026-10-23 the owner cancelled Nmap service inspection. NPSL 0.95 treats software that executes Nmap and parses its results, and software that reads `nmap-service-probes` or `nmap-os-db`, as a derivative-work trigger. The adapter in ADR 003 did the first of those. The project does not accept that clause, request a waiver, or import user-produced Nmap XML.

Service inspection stays on `app.ToolRunner` and `Application.InspectServices`, so the Wails binding, `POST /api/inspect` and `kali-tools inspect` keep the same operation. The implementation dials TCP. It does not execute Nmap, link Nmap, or read Nmap data files. Inventory still reports TShark and does not run it.

Each requested endpoint gets one connection. A short read classifies a banner the peer sends first (SSH, FTP, SMTP, IMAP, POP3, VNC, MySQL, or HTTP). A silent peer gets one TLS handshake using the standard library, then one `GET /` HTTP/1.1 request if that handshake fails. The TLS handshake sets `InsecureSkipVerify` only so an untrusted certificate can still identify TLS; the certificate is not returned and is not a trust decision. The result records a service name, a capped product and version, and the method `banner`, `tls` or `http`. An open port that matches none of those checks is reported open with an empty service. A refused connection is `unreachable`. Port numbers are not treated as proof of a protocol.

Scope remains an explicit IPv4 prefix of /24 or narrower and at most 32 unique ports. The run has a two-minute deadline, 32 workers and a 10 ms pace. Banner and response reads are capped. History stores the provider name, the probe-set version and the host count. Targets and probe bytes are not stored.

The JSON field names stay `scanner`, `version`, `finished`, `hosts`, `service`, `state` and `addresses`. `scanner` is `native` and `version` is the probe-set revision `1`. The Go result type is `ServiceInspection`. Nested types are `InspectionFinished`, `InspectionState`, `InspectionAddress`, `InspectionService`, `InspectionPort` and `InspectionHost`, in both `app` and `contract`. Wails models are regenerated from those names. The value is not an Nmap document and it carries no XML tags. An earlier draft kept the Go type name `Nmap` only so generated models would not need a hand edit; that name is not part of the 1.0 schema.

Rejected replacements, with the reasons recorded in tools research §3.1: `praetorian-inc/fingerprintx` (Apache-2.0, but a vendored `crypto/ssh` fork and a heavy module graph), `zmap/zgrab2` (Apache-2.0 and ISC, but a survey framework that requires the caller to name the protocol and pulls `zcrypto` into handshake use), and any probe set copied from Nmap data files.

This does not claim Nmap's coverage. Protocols that are silent and are neither TLS nor HTTP stay unidentified. There is no SYN scan, OS fingerprint, brute force, exploit check, evasion, or UDP probe.

Sources: [NPSL 0.95 annotated](https://nmap.org/npsl/npsl-annotated.html), [fingerprintx v1.1.19](https://github.com/praetorian-inc/fingerprintx), [zgrab2 license](https://github.com/zmap/zgrab2/blob/master/LICENSE), [Go crypto/tls](https://pkg.go.dev/crypto/tls).
