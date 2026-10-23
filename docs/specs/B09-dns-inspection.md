# B09 DNS inspection

| Field | Value |
| --- | --- |
| PLAN tasks | B09 (resolver comparison, SOA/NS consistency, read-only DNSSEC chain, DoH/DoT reachability, `miekg/dns` decision) |
| Status | Implemented (2026-10-23). Query checks, adapters and the Diagnostics panel have unit and browser evidence. Docker AC-10 is recorded in the evidence and is not closed from a host that cannot run the lab |
| Depends on | H02 (job manager), H03 (`app.HostNetwork` for the system DNS servers), H04, H05/H06, W01 + W04 (HTTP route) |
| Risk | Medium: a new DNS library, DNSSEC verification code paths, and Docker lab fixtures that must stay deterministic |

## 1. Goal

Add a read-only DNS inspection capability in a new package, `internal/dnsinspect`, which answers:

- Do the system resolver and explicit resolvers agree?
- Are the authoritative servers of a zone consistent (SOA serial and NS set)?
- Does the DNSSEC chain from a trust anchor down to the answer validate?
- Are DNS-over-HTTPS and DNS-over-TLS endpoints reachable?

A blocked port-53 egress is reported as its own result. The capability is exposed in the GUI, the web server and the CLI, and covered by Docker lab fixtures. This spec also makes the `miekg/dns` GitHub-versus-Codeberg decision.

## 2. Non-goals

- No zone transfer (AXFR/IXFR), not even against owned zones. PLAN keeps it for later.
- No subdomain brute force, wordlists, NSEC/NSEC3 walking or any enumeration (PLAN §11).
- No recursive resolver implementation. Queries go to the resolvers the user names, the system's configured servers, or the authoritative servers of the zone being inspected.
- No change to the existing `dns` diagnostic in `internal/network/dns.go`.
- No CT or OSINT lookups.
- No DNS cache.
- No persistence of names, resolvers or answers. History stores counts only.

## 3. Current state

Re-verify each fact before starting:

1. `internal/network/dns.go:10-62`: `lookupDNS(ctx, resolver *net.Resolver, host, recordType)` supports `""`, A, AAAA, CNAME, MX, NS, TXT and PTR through the stdlib resolver. It caps results at 256 values of 1,024 bytes each. No TTL, RCODE, AD bit or SOA is visible through `net.Resolver`.
2. `internal/network/network.go:90-136`: `Validate` limits DNS kinds to A/AAAA/CNAME/MX/NS/TXT/PTR and the timeout to 100..30000 ms.
3. `internal/network/dns_test.go:13-100`: unit fixtures use `golang.org/x/net/dns/dnsmessage` with a UDP socket on `127.0.0.1:0`.
4. `cmd/lab-fixture/main.go:65-97`: the Docker lab DNS fixture serves `fixture.test. A 192.0.2.42` on UDP `:5353` with `dnsmessage` and answers NXDOMAIN for everything else. HTTP is on `:8080` and TCP on `:9090`.
5. `docker/compose.yml`: services `fixture` and `test` share the `internal: true` network `lab`, with `cap_drop: [ALL]`. The runtime user is `65532`, so ports below 1024 cannot be bound.
6. `tests/integration/lab_test.go:20-60` (`//go:build integration`): the test reads `LAB_FIXTURE` and checks HTTP, discovery and DNS against the fixture.
7. `go.mod` pins `golang.org/x/net v0.60.0`. There is no DNS library besides `dnsmessage`.
8. `github.com/miekg/dns` (GitHub, BSD-3-Clause): the README states that v2 is now at `codeberg.org/miekg/dns`, that "this repo will only get specific fixes", and that it "will be archived" at some point. `github.com/hashicorp/mdns` (needed by B10) requires `github.com/miekg/dns v1.1.73`.
9. `app.HostNetwork` (H03) exposes the system DNS servers (today `platform.Adapter.DNS`, `internal/adapter/platform/platform.go:9`).

## 4. Design

### 4.1 Decision: `github.com/miekg/dns` v1 (GitHub)

This decision is recorded in `docs/dependencies.md` and summarized here:

| Criterion | GitHub v1.1.x | Codeberg v2 |
| --- | --- | --- |
| API stability | Stable for years; frozen except fixes | New API, pre-1.0 |
| Already in the graph | Yes, via `hashicorp/mdns` (B10) at v1.1.73 | No; adding it would mean two DNS libraries |
| DNSSEC helpers (`RRSIG.Verify`, `DNSKEY.ToDS`, `KeyTag`) | Present | Present (different API) |
| License | BSD-3-Clause | BSD-3-Clause (verify) |
| Maintenance | Fixes only; archive announced | Active |

**Decision:** pin `github.com/miekg/dns` at the version required by `hashicorp/mdns` or newer within v1.1.x. At research time that is v1.1.73. Record the exact version.

**Revisit triggers** (written into `docs/dependencies.md`):

- the GitHub repository is archived;
- a security fix ships only on Codeberg;
- `hashicorp/mdns` moves to v2.

Isolate every `miekg/dns` use in `internal/dnsinspect/wire.go`, so a later migration touches one file.

### 4.2 Types (`internal/dnsinspect`)

```go
package dnsinspect

const (
    MaxResolvers       = 4
    MaxDoH             = 2
    MaxDoT             = 2
    MaxNameservers     = 8
    MaxAddrsPerNS      = 2
    MaxTrustAnchors    = 4
    MaxChainDepth      = 8
    MaxQueries         = 64
    MaxAnswersReported = 64
    MaxResponseBytes   = 64 << 10
    MaxRDataText       = 1024
    Workers            = 8
    MinTimeoutMS       = 200
    MaxTimeoutMS       = 5000
    OverallDeadline    = 30 * time.Second
    EDNSBufferSize     = 1232
)

type Request struct {
    Name         string   `json:"name"`                    // FQDN or relative; network.ValidateHost; not an IP
    Type         string   `json:"type"`                    // A AAAA CNAME MX NS TXT SOA CAA DS DNSKEY; default A
    Resolvers    []string `json:"resolvers,omitempty"`     // "ip" or "ip:port" (IPv6 as "[ip]:port"), 0..4
    DoH          []string `json:"doh,omitempty"`           // "https://host[:port]/path", 0..2
    DoT          []string `json:"dot,omitempty"`           // "host" or "host:port" (default 853), 0..2
    Checks       []string `json:"checks"`                  // subset of "compare","soa_ns","dnssec","encrypted"; ≥1
    TrustAnchors []string `json:"trust_anchors,omitempty"` // DS RR text; default = IANA root anchors
    TimeoutMS    int      `json:"timeout_ms"`              // per query, 200..5000
}

type Report struct {
    Name       string            `json:"name"`
    Type       string            `json:"type"`
    Compare    *Comparison       `json:"compare,omitempty"`
    SOANS      *Consistency      `json:"soa_ns,omitempty"`
    DNSSEC     *DNSSECChain      `json:"dnssec,omitempty"`
    Encrypted  []EncryptedResult `json:"encrypted,omitempty"`
    Queries    int               `json:"queries"`         // total queries sent
    Limited    bool              `json:"limited"`         // MaxQueries reached or answers cut
}

type ResolverAnswer struct {
    Resolver  string   `json:"resolver"`   // "system" or "ip:port"
    Transport string   `json:"transport"`  // "os" | "udp" | "tcp"
    Status    string   `json:"status"`     // see §4.3 status values
    RCode     string   `json:"rcode,omitempty"` // "NOERROR" | "NXDOMAIN" | "SERVFAIL" | "REFUSED" | ...
    AD        bool     `json:"ad"`
    Answers   []string `json:"answers"`    // canonical RR text without TTL, sorted, ≤ MaxAnswersReported
    MinTTL    *uint32  `json:"min_ttl,omitempty"`
    ElapsedMS float64  `json:"elapsed_ms"`
}

type Comparison struct {
    Results []ResolverAnswer `json:"results"`
    Agree   bool             `json:"agree"`        // all "ok" results have identical Answers
    Groups  [][]string       `json:"groups"`       // resolvers grouped by identical answer sets
}

type AuthServer struct {
    NS        string   `json:"ns"`        // NS host name
    Address   string   `json:"address"`   // queried IP
    Status    string   `json:"status"`    // "ok" | "lame" | "timeout" | "refused" | "unreachable"
    Serial    *uint32  `json:"serial,omitempty"`
    NSSet     []string `json:"ns_set"`    // sorted, lowercased
    AA        bool     `json:"aa"`
}

type Consistency struct {
    Zone          string       `json:"zone"`
    Servers       []AuthServer `json:"servers"`
    SerialsAgree  bool         `json:"serials_agree"`
    NSSetsAgree   bool         `json:"ns_sets_agree"`
    Verdict       string       `json:"verdict"` // "consistent" | "serial_mismatch" | "ns_mismatch" | "lame_servers" | "no_servers"
}

type DNSSECLink struct {
    Zone       string `json:"zone"`
    DSMatches  bool   `json:"ds_matches"`   // a DS (or trust anchor) digest matches a DNSKEY
    KeyTag     uint16 `json:"key_tag"`
    Algorithm  string `json:"algorithm"`
    SigValid   bool   `json:"sig_valid"`    // RRSIG over DNSKEY RRset verifies and is within its validity period
    Status     string `json:"status"`       // "secure" | "insecure" | "bogus" | "indeterminate"
    Detail     string `json:"detail,omitempty"`
}

type DNSSECChain struct {
    Links      []DNSSECLink `json:"links"`       // anchor zone first
    AnswerSig  bool         `json:"answer_sig"`  // RRSIG over the answer RRset verifies with the zone key
    Verdict    string       `json:"verdict"`     // "secure" | "insecure" | "bogus" | "indeterminate"
    Resolver   string       `json:"resolver"`    // resolver used to fetch records (first explicit, else system)
}

type EncryptedResult struct {
    Endpoint  string  `json:"endpoint"`
    Protocol  string  `json:"protocol"` // "doh" | "dot"
    Status    string  `json:"status"`   // "ok" | "http_status" | "tls_error" | "timeout" | "unreachable" | "bad_response"
    HTTPCode  int     `json:"http_code,omitempty"`
    RCode     string  `json:"rcode,omitempty"`
    ElapsedMS float64 `json:"elapsed_ms"`
}

func Validate(r Request) error
func Inspect(ctx context.Context, r Request, systemServers []string) (Report, error)
```

`systemServers` comes from `app.HostNetwork` (configured DNS server IPs, deduplicated, capped at `MaxResolvers`). The capability never reads OS files itself.

### 4.3 Checks

All queries go through an `inspector` struct that holds the injected `exchange` functions (UDP/TCP/DoT/DoH), a query budget counter (`MaxQueries`, atomic) and the TLS roots (nil = system). The work runs on a worker pool of `Workers` goroutines over a bounded job channel (capacity `Workers`). Once the budget is spent, `Limited=true` and the remaining jobs report `Status "skipped"`.

**`compare`:**

1. The system view uses `net.DefaultResolver` with `PreferGo: false`, through the existing `lookupDNS` semantics (only A/AAAA/CNAME/MX/NS/TXT; for other types the system result is `Status "unsupported_type"`). The result has `Resolver "system"` and `Transport "os"`.
2. Each explicit resolver and each `systemServers` entry is queried with `miekg/dns`:
    - `Msg.SetQuestion(dns.Fqdn(name), type)`, `RecursionDesired=true`, EDNS0 with UDP size `EDNSBufferSize` and `DO=true`;
    - over UDP; on `TC=1`, retry the same query over TCP.
3. Status values:
    - `ok` (NOERROR with or without answers);
    - `nxdomain`, `servfail`, `refused` and `rcode_other`;
    - `timeout`;
    - `port53_unreachable`, from `classifyTransport` below;
    - `unreachable`.
4. `classifyTransport(port int, udpErr, tcpErr error) string` is pure:
    - when UDP timed out, also attempt a TCP connect to the same `ip:port` within the timeout;
    - if TCP also fails (timeout or refused) and `port == 53`, the status is `port53_unreachable`;
    - if both fail and `port != 53`, the status is `unreachable`;
    - if TCP succeeds, the status is `timeout` (the server is up, UDP is filtered or the server is silent).
5. Answers are normalized to the RR text with the TTL zeroed and the name lowercased (`rr.Header().Ttl = 0; strings.ToLower(rr.String())`). Each is truncated at `MaxRDataText`, sorted and capped at `MaxAnswersReported`.
6. `Agree` is true when every `ok` result has an identical `Answers` slice. `Groups` lists resolver names per distinct answer set, sorted.

**`soa_ns`:**

1. Find the zone by walking up from `Name`. For each suffix (at most `MaxChainDepth` labels), query SOA at the first resolver (explicit, else system server). The first suffix whose answer section holds an SOA owned by that suffix is the zone.
2. Query the NS records of the zone at the same resolver, sorted. Keep the first `MaxNameservers`. Resolve each NS to at most `MaxAddrsPerNS` addresses (A first, then AAAA).
3. For each address, query `SOA` and `NS` with `RecursionDesired=false`.
    - `AA=false` or `REFUSED` gives `lame`/`refused`.
    - Otherwise record `Serial` and `NSSet`.
4. The verdict is evaluated in this order:
    - `no_servers` if none answered;
    - `lame_servers` if any server is lame and the others agree;
    - `serial_mismatch` if serials differ;
    - `ns_mismatch` if NS sets differ;
    - else `consistent`.

**`dnssec`** (read-only verification; no trust decisions are made for the user):

1. Trust anchors:
    - parse `r.TrustAnchors` with `dns.NewRR`, at most `MaxTrustAnchors`, each a `*dns.DS`;
    - when empty, use the built-in IANA root anchors as constants in `anchors.go`:
        - `. IN DS 20326 8 2 E06D44B80B8F1D39A95C0B0D7C65D08458E880409BBC683457104237C7F8EC8D`;
        - `. IN DS 38696 8 2 683D2D0ACB8C9B712A1948B27F741219298D0A450D612C483AF444A4C0FB2B16`.

      Verify both against <https://data.iana.org/root-anchors/root-anchors.xml> during implementation and record the check.
2. The anchor zone is the owner name of the anchors (they must all share one owner; otherwise `invalid_input`). Build the zone list from the anchor zone down to the zone of `Name` (from the `soa_ns` zone discovery, run internally if `soa_ns` was not requested), at most `MaxChainDepth` links.
3. For each zone in order:
    - query `DNSKEY` (DO=1, CD=1) at the chosen resolver;
    - for the anchor zone, compare the anchors with `dnskey.ToDS(ds.DigestType)` for each `dnskey` with the SEP flag (`Flags&1 == 1`) and a matching `KeyTag()`;
    - for child zones, query `DS` for the child at the resolver (DO=1, CD=1) and compare the same way;
    - verify the `RRSIG` covering `DNSKEY` with the matched key: `sig.Verify(key, dnskeyRRset) == nil && sig.ValidityPeriod(now)`.
4. Link status:
    - `secure` when the DS matches and the signature is valid;
    - `insecure` when the parent returned a NOERROR/NODATA DS answer (no DS) — the chain stops and the verdict is `insecure`;
    - `bogus` on a DS mismatch or an invalid or expired signature;
    - `indeterminate` on timeout or SERVFAIL.
5. For the answer: query `Name`/`Type` with DO=1, CD=1 and verify the `RRSIG` covering the answer RRset with a zone-signing key of the last zone (any DNSKEY with a matching `KeyTag`). This sets `AnswerSig`.
6. The chain verdict is:
    - `secure` if all links are `secure` and `AnswerSig`;
    - `insecure` if an insecure delegation is reached;
    - `bogus` if any link is bogus or `AnswerSig` is false while all links are `secure`;
    - else `indeterminate`.
7. Supported algorithms are those that `miekg/dns` verifies (RSASHA256, RSASHA512, ECDSAP256SHA256, ECDSAP384SHA384 and ED25519). Others give `indeterminate` with detail "unsupported algorithm {n}".

**`encrypted`:**

- **DoH:**
  - `POST <url>` with header `Content-Type: application/dns-message`, `Accept: application/dns-message` and the packed query for `Name`/`Type` as the body (≤ 512 bytes);
  - `http.Client{Timeout: TimeoutMS, CheckRedirect: http.ErrUseLastResponse}`, `Transport{Proxy: nil, DisableKeepAlives: true, MaxResponseHeaderBytes: 16 << 10, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}}`;
  - read the body with `io.LimitReader(MaxResponseBytes+1)`; oversize gives `bad_response`;
  - status 200 plus a parsable DNS message gives `ok` with the `RCode`. Another status code gives `http_status` with `HTTPCode`. A TLS verification error gives `tls_error`.
- **DoT:** `dns.Client{Net: "tcp-tls", Timeout: TimeoutMS, TLSConfig: &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12, RootCAs: roots}}` to `host:port`, with the same status mapping.
- DoH URL validation: `url.Parse`, scheme `https`, no userinfo, host passes `network.ValidateHost`, port 1..65535, total length ≤ 2,048.

### 4.4 Validation rules

| Field | Rule |
| --- | --- |
| `Name` | `network.ValidateHost`; must not parse as an IP; ≤ 253 bytes |
| `Type` | one of `A AAAA CNAME MX NS TXT SOA CAA DS DNSKEY`; empty means `A` |
| `Resolvers` | ≤ 4; each `netip.ParseAddrPort` or `netip.ParseAddr` (port 53 implied); no zone for non-link-local; unique |
| `DoH` | ≤ 2; see §4.3 |
| `DoT` | ≤ 2; `host[:port]`; `ValidateHost`; port 1..65535 |
| `Checks` | non-empty subset; `encrypted` requires at least one `DoH` or `DoT`; duplicates rejected |
| `TrustAnchors` | ≤ 4; each parses as DS |
| `TimeoutMS` | 200..5000 |

Every violation is `invalid_input`. Network failures are per-result statuses, not errors. `Inspect` returns an error only for `invalid_input`, `cancelled` or `timeout` (overall deadline).

### 4.5 Application, adapters and UI

```go
// internal/app
func (a *Application) InspectDNS(ctx context.Context, r dnsinspect.Request) (dnsinspect.Report, error)
```

- It runs as an operation job with kind `dns_inspect` and label "DNS inspection".
- `systemServers` comes from `HostNetwork`; a `HostNetwork` error gives an empty list, never a failure.
- History summary: `"DNS inspection <state>: <q> queries, checks=<n>"`.
- Register the capability `dns_inspect`: `available` everywhere, provider `miekg/dns`, detail "Resolver comparison, SOA/NS, DNSSEC chain, DoH/DoT; read-only".

| Interface | Change |
| --- | --- |
| Wails (`internal/adapter/wailsui`) | `InspectDNS(r contract.DNSInspectRequest) (contract.DNSInspectReport, error)` |
| HTTP (`internal/adapter/httpapi`) | `POST /api/dns/inspect`, body ≤ 8 KiB |
| CLI (`internal/adapter/cli`) | `kali-tools dns-inspect --target <name> [--record A] [--resolvers ip[:port],…] [--doh url,…] [--dot host[:port],…] [--checks compare,soa_ns,dnssec,encrypted] [--trust-anchor "<DS RR>"] [--timeout 2000]`. Comma lists are bounded at 1,024 bytes per flag. The default `--checks` is `compare`. JSON envelope. Exit 2 on `invalid_input`. |
| DTO (`internal/adapter/contract`) | Mirrors §4.2 |

If H05/W01 have not landed, add these to `app.go` / `cmd/kali-tools` until H05 lands and record the HTTP route as a follow-up.

Frontend: `frontend/src/features/dns/DNSInspectPanel.tsx` as a "DNS inspection" tab on the Diagnostics page. It holds:

- the form (name, type, resolvers, DoH, DoT, check toggles);
- the comparison table with agree/disagree badges;
- the SOA/NS server table with a verdict;
- the DNSSEC chain as an ordered list of links with status pills;
- the encrypted endpoint table.

i18n keys (both catalogs): `dnsInspect`, `resolvers`, `doh`, `dot`, `checks`, `checkCompare`, `checkSoaNs`, `checkDnssec`, `checkEncrypted`, `agree`, `disagree`, plus every status and verdict string from §4.2 as `dns_<value>`. Example: `dns_port53_unreachable` = "Port 53 appears blocked: UDP and TCP to this resolver both failed".

### 4.6 Docker lab fixtures

Extend `cmd/lab-fixture` (it may import `github.com/miekg/dns`; it is a composition root). Every listener runs on ports ≥ 1024, and the `fixture` container stays non-root.

| Listener | Port | Content |
| --- | --- | --- |
| Existing UDP DNS | 5353 | unchanged (`fixture.test` A) |
| Auth server A | UDP+TCP 5354 | zones `consistent.test` (SOA serial 100, NS `ns1.consistent.test.`, `ns2.consistent.test.`), `mismatch.test` (serial 100), `signed.test` (signed, see below) |
| Auth server B | UDP+TCP 5355 | `consistent.test` (serial 100, same NS), `mismatch.test` (**serial 101**) |
| Recursive-like resolver | UDP+TCP 5356 | answers with `RA=1` for `fixture.test` A (192.0.2.42), `differs.test` A **192.0.2.43**, and forwards `*.consistent.test`, `*.mismatch.test` and `*.signed.test` from its own static copy of server A's data. NS records for those zones point to glue `ns1`→fixture IP:5354 and `ns2`→fixture IP:5355 (the port is carried by the test through a lab-only override, see below) |
| Silent UDP | 5357 | reads and never answers; TCP 5357 not listening |
| DoT | 8853 | TLS (self-signed lab CA) wrapping the 5356 resolver logic |
| DoH | 8443 `/dns-query` | HTTPS (same CA), RFC 8484 POST |
| Lab CA | HTTP 8080 `/lab-ca.pem` | PEM of the lab CA certificate, for the test only |

`signed.test` uses ED25519 (algorithm 15) keys derived from fixed seeds with `ed25519.NewKeyFromSeed`. There are two seeds, KSK and ZSK, written as constants in `cmd/lab-fixture/dnssec.go`. The fixture signs at start-up with an inception of now − 1 h and an expiration of now + 24 h. Its DS (digest type 2) is a constant the integration test uses as `TrustAnchors` (`signed.test. IN DS <tag> 15 2 <hex>`). Compute the constant once and pin it in both the fixture and the test.

Lab-only port override: the NS addresses of authoritative servers come from glue A records, which carry no port. The `dnsinspect` inspector therefore has an unexported field `authPort func(ip netip.Addr) uint16` (default: always 53). The integration test builds the inspector through an `export_test.go`-style helper `dnsinspect.NewForLab(authPorts map[string]uint16, roots *x509.CertPool)`, compiled only with the `integration` build tag in `internal/dnsinspect/lab_integration.go`. The production `Inspect` never reads it.

Integration test `TestPrivateLabDNSInspection` (`tests/integration`, `integration` tag) asserts:

1. `compare` of `differs.test` with resolvers `[fixture:5356, fixture:5353]` gives `Agree=false` with two groups.
2. `compare` of `fixture.test` with `[fixture:5357]` gives status `unreachable`: UDP times out and TCP 5357 is refused on a port other than 53. The `port53_unreachable` branch is covered by the unit test `TestClassifyTransport`, because the non-root fixture cannot bind port 53.
3. `soa_ns` on `consistent.test` gives `consistent`. On `mismatch.test` it gives `serial_mismatch`.
4. `dnssec` on `www.signed.test` A with the pinned anchor gives verdict `secure`. With a wrong anchor digest it gives `bogus`.
5. `encrypted` with DoH `https://fixture:8443/dns-query` and DoT `fixture:8853` (lab CA fetched from `/lab-ca.pem`) gives `ok` for both.

## 5. Files

Create:

- `internal/dnsinspect/dnsinspect.go`: types, constants, `Validate`, `Inspect`, the inspector and worker pool.
- `internal/dnsinspect/wire.go`: every `miekg/dns` call (query building, exchange, normalization).
- `internal/dnsinspect/compare.go`, `soans.go`, `dnssec.go`, `encrypted.go`, `anchors.go`.
- `internal/dnsinspect/classify.go`: `classifyTransport`.
- `internal/dnsinspect/*_test.go`: unit tests with in-process `miekg/dns` servers on `127.0.0.1:0` (`dns.Server{PacketConn: ...}`).
- `internal/dnsinspect/lab_integration.go` (`//go:build integration`): `NewForLab`.
- `cmd/lab-fixture/dnssec.go`, `cmd/lab-fixture/auth.go`, `cmd/lab-fixture/encrypted.go`: new fixture listeners.
- `tests/integration/dnsinspect_test.go`: lab test.
- `internal/app/dnsinspect.go` and `internal/app/dnsinspect_test.go`.
- `frontend/src/features/dns/DNSInspectPanel.tsx`.

Modify:

- `go.mod`, `go.sum`: `github.com/miekg/dns`.
- `cmd/lab-fixture/main.go`: start the new listeners; extend `--health` to check 5354 and 5356.
- `docker/compose.yml`: no port changes; the internal network is unchanged.
- `internal/app/capabilities.go`, driving adapters, DTOs, `bridge.ts`, `App.tsx`, `i18n.ts`, `workflows.spec.ts`.
- `docs/dependencies.md`, `docs/architecture.md`, `docs/tools-research.md`, `PLAN.md`.

Delete: none.

## 6. Implementation steps

1. Dependency and license review:
    - `go get github.com/miekg/dns@v1.1.73` (or the version `hashicorp/mdns` requires, if newer);
    - open `LICENSE` in the module cache and confirm BSD-3-Clause;
    - run `go mod why -m github.com/miekg/dns`;
    - run `govulncheck ./...`;
    - record the decision of §4.1 in `docs/dependencies.md`.
2. Write the types, `Validate` and the bound tests.
3. Write `wire.go` and the `compare` check with in-process servers. Add `classifyTransport` with a pure table test.
4. Write `soa_ns` with two in-process authoritative servers.
5. Write `dnssec` with an in-process signed zone (ED25519 seeds as in the lab) plus a bogus variant.
6. Write `encrypted` with an `httptest.NewTLSServer` DoH handler and a `dns.Server{Net: "tcp-tls"}` DoT server.
7. Add the app use case, capability, driving adapters and DTOs.
8. Extend the lab fixture and the integration test. Run the Docker lab.
9. Frontend panel, i18n, Playwright.
10. Update the docs.
11. Write `docs/verification/specs/B09.md`.

## 7. Acceptance criteria

- **AC-1.** Given invalid inputs (an IP as `Name`, type `ANY`, 5 resolvers, a duplicate resolver, a DoH with `http://`, a DoH with userinfo, `Checks` empty, `encrypted` without endpoints, a timeout of 199/5001, 5 trust anchors), when `Validate` runs, then each gives `invalid_input`. The boundary values 200/5000/4 resolvers/2 DoH are accepted.
- **AC-2.** Given two in-process resolvers returning different A records, when `compare` runs, then `Agree=false`, `Groups` has 2 entries, and the answers are TTL-normalized and sorted.
- **AC-3.** Given a truncated (`TC=1`) UDP answer, when queried, then the TCP retry happens and the result has `Transport "tcp"`.
- **AC-4.** Given `classifyTransport(53, timeout, refused)`, then the result is `port53_unreachable`. For `(5357, timeout, refused)` it is `unreachable`, and for `(53, timeout, nil)` it is `timeout`.
- **AC-5.** Given two authoritative servers with serials 100 and 101, then the verdict is `serial_mismatch`. Given equal serials and NS sets, `consistent`. Given one server answering `AA=0`, `lame_servers`.
- **AC-6.** Given a signed in-process zone and its DS as the anchor, when `dnssec` runs, then all links are `secure`, `AnswerSig=true` and the verdict is `secure`. Given a tampered RRSIG, `bogus`. Given a signature expired by the injected clock, `bogus`. Given no DS at the parent, `insecure`.
- **AC-7.** Given an `httptest` DoH server returning 200 with a valid message, then `ok`. Given 500, `http_status` with 500. Given an untrusted certificate, `tls_error`. Given a body over 64 KiB, `bad_response`.
- **AC-8.** Given a query budget of `MaxQueries` exhausted by many NS servers, then `Limited=true` and no more than 64 queries were sent (counted by the fake exchange).
- **AC-9.** Given a cancelled context, then `Inspect` returns `cancelled` and the worker goroutines exit (checked with a `sync.WaitGroup` in the test hook; no `time.Sleep`).
- **AC-10.** Given the Docker lab, when `TestPrivateLabDNSInspection` runs, then assertions 1–5 of §4.6 pass.
- **AC-11.** Given any inspection, when the use case completes, then the history summary contains no name, resolver or answer.

## 8. Tests

| Test | Package | Asserts | ACs |
| --- | --- | --- | --- |
| `TestValidateDNSInspectRequest` | `internal/dnsinspect` | AC-1 table | AC-1 |
| `TestCompareResolversDisagree` | same | AC-2 | AC-2 |
| `TestTruncatedAnswerRetriesTCP` | same | AC-3 | AC-3 |
| `TestClassifyTransport` | same | AC-4 table | AC-4 |
| `TestSOANSConsistency` | same | AC-5 table | AC-5 |
| `TestDNSSECChainVerdicts` | same | AC-6 table (secure, bogus sig, expired, insecure) | AC-6 |
| `TestEncryptedEndpoints` | same | AC-7 table | AC-7 |
| `TestQueryBudget` | same | AC-8 | AC-8 |
| `TestInspectCancellation` | same | AC-9 | AC-9 |
| `TestRootAnchorsParse` | same | Built-in anchors parse as DS for `.` with key tags 20326 and 38696 | §4.3 |
| `TestPrivateLabDNSInspection` | `tests/integration` | AC-10 | AC-10 |
| `TestInspectDNSHistoryRedaction` | `internal/app` | AC-11 | AC-11 |
| `TestCLIDNSInspectFlags` | `internal/adapter/cli` | Comma lists, bounds, exit codes | CLI |
| `TestDNSInspectRouteBodyLimit` | `internal/adapter/httpapi` | 8 KiB + 1 gives 413 | HTTP |
| Playwright `DNS inspection renders comparison, SOA/NS and DNSSEC verdicts` | `frontend/tests/workflows.spec.ts` | Contract fake | UI |

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
go vet -tags integration ./...
go test -race ./internal/dnsinspect/...
bash scripts/setup/setup-linux-docker.sh test
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

## 10. Documentation updates

- `docs/dependencies.md`:
  - add a row for `github.com/miekg/dns` (BSD-3-Clause, DNS wire protocol and DNSSEC verification for `dnsinspect`; also required by `hashicorp/mdns`);
  - add a "Decision 2026-xx-xx" paragraph with §4.1's table summary and revisit triggers;
  - remove "Codeberg migration pending" from the watch list in `docs/tools-research.md` §7 and replace it with a link.
- `docs/architecture.md` §2/§3: add `internal/dnsinspect`.
- `docs/tools-research.md` §11: close the "`miekg/dns`: GitHub v1 vs the Codeberg module" decision with a link.
- `PLAN.md` B09: tick both items after AC-1 to AC-11 pass, linking the evidence.
- `README.md`: CLI list adds `dns-inspect`.

## 11. Risks and rollback

- **The GitHub repository gets archived.** Mitigated: all use is isolated in `wire.go`, and revisit triggers are recorded.
- **DNSSEC false "secure".** Mitigated: tampered-signature and expired tests, the use of the library's `Verify` with no custom crypto, and verdicts labelled read-only. The UI says "observed validation; not a trust decision".
- **Lab flakiness from multiple listeners.** Mitigated: the health check covers the new ports, and tests use deadlines rather than sleeps.
- **The root anchors change** (KSK rollover). The constants have a source URL comment and a test. `TrustAnchors` allows override.
- **Rollback.** Revert the commits and run `go mod tidy`. The lab fixture changes are additive.

## 12. Quality and delivery assertions

### 12.1 Applicable quality gates

| Gate | Applies | Tightened threshold |
| --- | --- | --- |
| QG-1, QG-2, QG-3, QG-4, QG-5, QG-7, QG-9, QG-13, QG-14, QG-15 | yes | none |
| QG-6 | yes | `internal/dnsinspect` ≥ 85% statements |
| QG-8 | yes | +1 Playwright workflow |
| QG-10 | yes | `internal/dnsinspect` imports no `internal/adapter/*` or `internal/app`; `github.com/miekg/dns` is imported only by `internal/dnsinspect/wire.go`, `dnssec.go`, `anchors.go` and `cmd/lab-fixture` |
| QG-11 | yes | `TestInspectDNSHistoryRedaction` |
| QG-12 | yes | every constant in §4.2 has a limit and limit+1 test |

### 12.2 Delivery assertions

| ID | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | Library decision recorded | `grep -n "miekg/dns" docs/dependencies.md` | row plus decision paragraph with revisit triggers | reviewer |
| DA-2 | Single DNS library | `go list -m all \| grep -c 'miekg/dns'` | `1` (no `codeberg.org/miekg/dns`) | CI |
| DA-3 | No enumeration or AXFR | `grep -rnE 'AXFR\|IXFR\|TypeAXFR\|NSEC3?Walk\|wordlist' internal/dnsinspect` | no output | CI |
| DA-4 | Port-53 classification | `go test -run TestClassifyTransport ./internal/dnsinspect/` | PASS | CI |
| DA-5 | DNSSEC verdicts | `go test -run TestDNSSECChainVerdicts ./internal/dnsinspect/` | PASS (4 cases) | CI |
| DA-6 | Docker lab coverage | `TestPrivateLabDNSInspection` in the Docker CI job | PASS, 5 assertions | CI (Docker) |
| DA-7 | Query budget | `go test -run TestQueryBudget ./internal/dnsinspect/` | ≤ 64 exchanges counted | CI |

Evidence record: `docs/verification/specs/B09.md`.
