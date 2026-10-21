# B11 HTTP metadata: phase timings, security headers and redirect chain

| Field | Value |
| --- | --- |
| PLAN tasks | B11 (`httptrace` phase timings, security-header presence report, redirect chain without following) |
| Status | Ready |
| Depends on | H04 (`app.Application`), H05/H06 (adapters, DTOs), W01 + W04 (HTTP route) |
| Risk | Low: an additive extension of an existing, well-tested diagnostic; the main risk is breaking the "redirects are not followed" guarantee |

## 1. Goal

Extend the existing `http` diagnostic in `internal/network` so that one request also reports:

- the connection phase timings from `net/http/httptrace` (DNS, connect, TLS, time to first byte, total);
- the presence and bounded values of standard security response headers;
- the redirect target of the response, **without the HTTP client following it**.

An optional, explicit hop-by-hop trace (at most 5 hops) issues each hop as a new, separately validated request. It detects loops and stops at the limit.

The existing `Result.Values` (`[Status, Proto]`) contract is unchanged. The new data is an additional `http` object, visible in the GUI, the web server and the CLI.

## 2. Non-goals

- No automatic redirect following by `http.Client`. `CheckRedirect` keeps returning `http.ErrUseLastResponse`.
- No header scoring or "grade". Presence and raw bounded values only.
- No `Set-Cookie`, `Authorization`, `WWW-Authenticate` or any other header value beyond the fixed allowlist in §4.2.
- No body inspection, title extraction, technology fingerprinting, favicon hashing, fuzzing or directory brute force (tools research §4.7).
- No methods other than `GET`, no request bodies, no custom request headers from the user.
- No proxy support. `Proxy: nil` stays.
- No change to the TLS diagnostic. TLS details belong to B08; this spec reports only the TLS phase duration.

## 3. Current state

Re-verify each fact before starting. The `network` package was being changed concurrently when this spec was written (the `Prober` port). Re-check the `Diagnose` signature.

1. `internal/network/network.go:53-59`: `Request{RecordType, Kind, Target, Port, TimeoutMS}`.
2. `internal/network/network.go:60-68`: `Result{ErrorCode, Kind, Target, Status, ElapsedMS, Values, Provider}`.
3. `internal/network/network.go:117-131`: for `Kind == "http"`, validation requires:
    - `len(Target) ≤ 8192`;
    - a URL with `http`/`https`, a hostname and no userinfo;
    - a port of 1..65535 when present;
    - a host that passes `ValidateHost`.
4. `internal/network/network.go:180-194` (inside `Diagnostics.Diagnose`, or `Diagnose` in the older form):
    - `http.Transport{MaxResponseHeaderBytes: 64 << 10, Proxy: nil, DisableKeepAlives: true, TLSHandshakeTimeout: TimeoutMS}`;
    - `http.Client{CheckRedirect: … return http.ErrUseLastResponse}`;
    - the body is drained up to 65,536 bytes;
    - `Values = []string{resp.Status, resp.Proto}`.
5. `internal/network/network_test.go:15-27` (`TestDiagnosticsFixtures`) asserts that a 302 is reported as `"302 Found"`, that `len(Values) == 2`, and that the redirect destination is **not** contacted.
6. `internal/network/network_test.go:46-56` (`TestHTTPTimeout`) checks the 100 ms bound. `network_test.go:103-118` (`TestHTTPHeaderAndPortBounds`) checks the 64 KiB header limit and port validation.
7. `internal/app/capabilities.go:9`: the `diagnostics` detail says "redirects are not followed".
8. `frontend/src/shared/api/bridge.ts:27-42`: `DiagnosticRequest`/`DiagnosticResult` TS types mirror the Go JSON.
9. `cmd/kali-tools/main.go:44-71`: `diagnose` takes `--kind`, `--target`, `--port`, `--timeout` and `--record`.

## 4. Design

### 4.1 Request and result extension (`internal/network`)

```go
const (
    MaxRedirectHops     = 5
    MaxHeaderValueBytes = 256
    MaxLocationBytes    = 8192
)

type Request struct {
    RecordType string `json:"record_type,omitempty"`
    Kind       string `json:"kind"`
    Target     string `json:"target"`
    Port       int    `json:"port"`
    TimeoutMS  int    `json:"timeout_ms"`
    Redirects  int    `json:"redirects,omitempty"` // http only: 0..MaxRedirectHops explicit hops to trace; default 0
}

type Result struct {
    ErrorCode string        `json:"error_code,omitempty"`
    Kind      string        `json:"kind"`
    Target    string        `json:"target"`
    Status    string        `json:"status"`
    ElapsedMS float64       `json:"elapsed_ms"`
    Values    []string      `json:"values"`
    Provider  string        `json:"provider"`
    HTTP      *HTTPMetadata `json:"http,omitempty"` // set only for kind "http" when a response was received
}

type HTTPMetadata struct {
    StatusCode      int              `json:"status_code"`
    Phases          HTTPPhases       `json:"phases"`
    SecurityHeaders []SecurityHeader `json:"security_headers"` // fixed order, always len == len(securityHeaderNames)
    Redirect        *RedirectTarget  `json:"redirect,omitempty"`
    Chain           []RedirectHop    `json:"chain,omitempty"`  // only when Request.Redirects > 0
    ChainStatus     string           `json:"chain_status"`     // "not_requested" | "complete" | "loop_detected" | "limit_reached" | "invalid_target" | "failed"
}

// Durations in milliseconds; nil when the phase did not occur (IP literal → no DNS, http → no TLS).
type HTTPPhases struct {
    DNSMS     *float64 `json:"dns_ms"`
    ConnectMS *float64 `json:"connect_ms"`
    TLSMS     *float64 `json:"tls_ms"`
    TTFBMS    *float64 `json:"ttfb_ms"`  // WroteRequest → GotFirstResponseByte
    TotalMS   float64  `json:"total_ms"` // request start → response headers parsed
}

type SecurityHeader struct {
    Name    string `json:"name"`
    Present bool   `json:"present"`
    Value   string `json:"value,omitempty"`   // first value, ≤ MaxHeaderValueBytes, then " [truncated]"
    Note    string `json:"note,omitempty"`    // "ignored_over_http" | "multiple_values"
}

type RedirectTarget struct {
    Location   string `json:"location"`   // resolved absolute URL, ≤ MaxLocationBytes
    Valid      bool   `json:"valid"`      // passes Validate for kind http
    SelfLoop   bool   `json:"self_loop"`  // normalized Location == normalized request URL
    CrossHost  bool   `json:"cross_host"` // host differs from the request host
    Downgrade  bool   `json:"downgrade"`  // https → http
}

type RedirectHop struct {
    URL        string           `json:"url"`
    StatusCode int              `json:"status_code"`
    Phases     HTTPPhases       `json:"phases"`
    Redirect   *RedirectTarget  `json:"redirect,omitempty"`
}
```

Validation changes in `Validate`:

- `Redirects` must be 0 for every kind except `http`, else `invalid_input` "redirects apply only to HTTP diagnostics".
- For `http`, `Redirects` must be 0..`MaxRedirectHops`, else `invalid_input` "redirects must be 0..5".

### 4.2 Security header allowlist (fixed order)

```go
var securityHeaderNames = []string{
    "Strict-Transport-Security",
    "Content-Security-Policy",
    "Content-Security-Policy-Report-Only",
    "X-Content-Type-Options",
    "X-Frame-Options",
    "Referrer-Policy",
    "Permissions-Policy",
    "Cross-Origin-Opener-Policy",
    "Cross-Origin-Embedder-Policy",
    "Cross-Origin-Resource-Policy",
}
```

`securityHeaders(h http.Header, scheme string) []SecurityHeader` is pure:

- `Present = len(h.Values(name)) > 0`.
- `Value` is the first value with control characters (`< 0x20` except tab, and `0x7f`) replaced by `?`. When it exceeds `MaxHeaderValueBytes` bytes, it is cut at a UTF-8 boundary and gets the suffix `" [truncated]"`.
- `Note = "multiple_values"` when more than one value exists.
- `Note = "ignored_over_http"` for `Strict-Transport-Security` when `scheme == "http"` (RFC 6797 §8.1). This takes precedence over `multiple_values`.

### 4.3 Phase timing

`traceRequest(ctx, req) (*http.Request, *phaseRecorder)` attaches `httptrace.WithClientTrace`. `phaseRecorder` holds a `sync.Mutex` and the timestamps from a monotonic `time.Now()`. It records:

| Hook | Field |
| --- | --- |
| `DNSStart` / `DNSDone` | first DNS start, matching done |
| `ConnectStart` | earliest start across happy-eyeballs attempts |
| `ConnectDone(err == nil)` | the first successful done |
| `TLSHandshakeStart` / `TLSHandshakeDone` | start, done |
| `WroteRequest` | written |
| `GotFirstResponseByte` | first byte |

`phases(start, headersAt)` converts to `HTTPPhases`:

- a phase is `nil` unless both of its timestamps exist;
- every value is `float64(d.Microseconds())/1000`;
- `TotalMS` runs from the start to the moment `client.Do` returned.

### 4.4 Request flow (`internal/network/http.go`, new file; `Diagnose` delegates)

Move the current HTTP branch of `Diagnose` into:

```go
func diagnoseHTTP(ctx context.Context, r Request) (values []string, meta *HTTPMetadata, err error)
func fetchOnce(ctx context.Context, client *http.Client, target string) (resp status, headers, phases, err)
```

1. Build one `http.Transport` (exactly today's settings) and one `http.Client` with `CheckRedirect: return http.ErrUseLastResponse` for the whole diagnostic. Defer `CloseIdleConnections`.
2. Hop 0 is the user's target. Call `fetchOnce`, which drains the body up to 65,536 bytes as today. Set `values = []string{resp.Status, resp.Proto}`, as today. Build `meta.StatusCode`, `meta.Phases` and `meta.SecurityHeaders`.
3. Redirect detection: if the status is one of 301, 302, 303, 307 or 308 and `Location` is non-empty:
    - resolve with `req.URL.Parse(location)`, after first checking `len(location) ≤ MaxLocationBytes` (otherwise `Redirect.Valid=false`, with the stored location truncated);
    - `Valid = Validate(Request{Kind: "http", Target: resolved.String(), TimeoutMS: r.TimeoutMS}) == nil`;
    - `SelfLoop = normalizeURL(resolved) == normalizeURL(req.URL)`;
    - `CrossHost = !strings.EqualFold(resolved.Hostname(), req.URL.Hostname())`;
    - `Downgrade = req.URL.Scheme == "https" && resolved.Scheme == "http"`.
4. If `r.Redirects == 0`, set `ChainStatus = "not_requested"` and stop. **No second request is made.**
5. Explicit trace (`r.Redirects > 0`): `seen := {normalizeURL(hop0)}`. Then, for `i := 1..r.Redirects`:
    - if the previous hop has no redirect, set `ChainStatus = "complete"` and stop;
    - if the previous redirect is `!Valid`, set `ChainStatus = "invalid_target"` and stop, without requesting it;
    - if `normalizeURL(next)` is in `seen`, set `ChainStatus = "loop_detected"` and stop, without requesting it;
    - otherwise `fetchOnce(ctx, client, next)` under the **same** overall `ctx` deadline (`TimeoutMS` covers the whole diagnostic, as today). Append a `RedirectHop`. A fetch error sets `ChainStatus = "failed"` and stops; the diagnostic itself still completes, because hop 0 succeeded;
    - after the loop, if the last hop still redirects, set `ChainStatus = "limit_reached"`; else `"complete"`.
6. `normalizeURL(u)`:
    - lowercase scheme and host;
    - drop the default ports (`:80` for http, `:443` for https);
    - an empty path becomes `/`;
    - keep the raw query;
    - drop the fragment.

Hop URLs and `Location` values can contain query tokens. They stay in the in-memory `Result` only. History (`app`) never stores them; the current summary `Diagnostic <state>` is unchanged.

### 4.5 Application, adapters and UI

There is no new application method: `Application.Diagnose`/`StartDiagnostic` (H04) already carry `network.Request` and `network.Result`. Update the `diagnostics` capability detail to "DNS, bounded TCP, HTTP (phase timings, security headers, redirect target; explicit hop trace ≤ 5) and TLS; redirects are never followed automatically".

| Interface | Change |
| --- | --- |
| Wails | No new binding. `contract.DiagnosticRequest` gains `redirects`; `contract.DiagnosticResult` gains `http`. Regenerate the bindings. |
| HTTP | `POST /api/diagnostics` accepts `redirects` and returns `http` (W01's route; no new route). |
| CLI | `kali-tools diagnose --kind http --target <url> [--redirects 0..5]`. The JSON envelope includes `data.http`. `--redirects` with another kind gives exit 2. |
| DTO | Mirrors §4.1 |

If H05/W01 have not landed, add `--redirects` to `cmd/kali-tools/main.go` and pass the field through `app.go` until H05 lands. Record the HTTP route as a follow-up.

Frontend:

- `bridge.ts`: add `redirects?: number` to `DiagnosticRequest`, and add `http?: HTTPMetadata` (with the TS types mirroring §4.1) to `DiagnosticResult`.
- New `frontend/src/features/diagnostics/HTTPMetadataView.tsx`, rendered under the diagnostic result when `result.http` exists:
  - a phase bar list (DNS, Connect, TLS, TTFB, Total, in ms; "—" when null);
  - a security-header table (name, present ✓/✗, value in `<code>` with wrapping, note);
  - a redirect card (Location as text, never a link, plus `cross_host`/`downgrade`/`self_loop` badges);
  - the chain table when present, with the chain status.
- The diagnostics form gains a numeric "Trace redirects (0–5)" input, shown only for kind `http`, default 0.
- i18n keys (both catalogs): `phases`, `phaseDns`, `phaseConnect`, `phaseTls`, `phaseTtfb`, `phaseTotal`, `securityHeaders`, `headerPresent`, `headerMissing`, `redirectTarget`, `crossHost`, `downgrade`, `selfLoop`, `traceRedirects`, `chain`, `chain_not_requested`, `chain_complete`, `chain_loop_detected`, `chain_limit_reached`, `chain_invalid_target`, `chain_failed`, `note_ignored_over_http`, `note_multiple_values`.

### 4.6 Limits

| Limit | Value | Constant |
| --- | --- | --- |
| Hops traced | 0..5 | `MaxRedirectHops` |
| Header value reported | 256 bytes | `MaxHeaderValueBytes` |
| Location length | 8,192 bytes | `MaxLocationBytes` |
| Response headers | 64 KiB | existing `MaxResponseHeaderBytes` |
| Body drained per hop | 64 KiB | existing |
| Overall time | `TimeoutMS` (100..30000) for all hops together | existing |

## 5. Files

Create:

- `internal/network/http.go`: `diagnoseHTTP`, `fetchOnce`, `phaseRecorder`, `securityHeaders`, `normalizeURL`, types and constants.
- `internal/network/http_test.go`: new tests (§8).
- `frontend/src/features/diagnostics/HTTPMetadataView.tsx`.

Modify:

- `internal/network/network.go`: `Request.Redirects`, `Result.HTTP`, validation rules, and the HTTP branch delegating to `diagnoseHTTP`.
- `internal/app/capabilities.go`: diagnostics detail text.
- The CLI `diagnose` flag `--redirects`; DTOs; `bridge.ts`; `App.tsx` (form input and view); `i18n.ts`; `workflows.spec.ts`; regenerated `frontend/wailsjs`.
- `docs/architecture.md`, `docs/tools-research.md`, `PLAN.md`.

Delete: none.

## 6. Implementation steps

1. Add the types, constants and `Validate` rules, with bound tests (`Redirects` −1/0/5/6 and non-http kinds).
2. Extract the HTTP branch into `diagnoseHTTP` with no behavior change. Run the existing tests; they must pass unchanged.
3. Add `securityHeaders` and `normalizeURL` as pure functions, test first.
4. Add `phaseRecorder` with `httptrace`. Test it against `httptest.NewServer` and `httptest.NewTLSServer`.
5. Add redirect detection and the explicit trace, test first (§8).
6. Add the CLI flag, the DTO fields and the binding regeneration.
7. Frontend view, form input, i18n, Playwright.
8. Update the docs and run the verification.
9. Write `docs/verification/specs/B11.md`.

## 7. Acceptance criteria

- **AC-1.** Given the existing test `TestDiagnosticsFixtures`, when run unchanged, then it passes: `Values == ["302 Found", <proto>]` and the destination server is not contacted.
- **AC-2.** Given a 302 with `Location: /next` and `Redirects=0`, then `http.redirect.location` is the absolute URL of `/next`, `valid=true`, `chain_status="not_requested"`, and exactly **one** request reached the test server (atomic counter).
- **AC-3.** Given a server with all 10 allowlisted headers, then all are `present=true` with values. Given a server with none, all are `present=false`, `len == 10` and the order is fixed. Given HSTS over `http://`, `note="ignored_over_http"`. Given a 300-byte CSP, the value is 256 bytes plus `" [truncated]"`.
- **AC-4.** Given a response that sets `Set-Cookie: session=synthetic-token`, then the serialized `Result` JSON does not contain `synthetic-token`.
- **AC-5.** Given `http://127.0.0.1:<port>` (IP literal, plain HTTP), then `phases.dns_ms == null`, `phases.tls_ms == null`, `connect_ms`/`ttfb_ms` are non-null, and `total_ms ≥ ttfb_ms`. Given an `httptest.NewTLSServer` target trusted through an injected client (test hook), `tls_ms` is non-null.
- **AC-6.** Given a chain A→B→C(200) with `Redirects=5`, then `chain` has 2 hops and `chain_status="complete"`.
- **AC-7.** Given a loop A→B→A with `Redirects=5`, then `chain_status="loop_detected"` after 1 traced hop, and A was requested exactly once.
- **AC-8.** Given an endless chain (each hop redirects to `/n+1`) with `Redirects=5`, then 5 hops are traced, `chain_status="limit_reached"` and the server saw exactly 6 requests.
- **AC-9.** Given a `Location: http://user:pass@x/` or `Location: ftp://x/`, then `redirect.valid=false`. With `Redirects=5`, `chain_status="invalid_target"` and no request to it.
- **AC-10.** Given `Redirects=6`, or `Redirects=1` with `Kind="tcp"`, then `invalid_input`.
- **AC-11.** Given a chain where hop 2 hangs and the overall `TimeoutMS=300`, then the diagnostic returns within 1 s, hop 0 data is present, and `chain_status="failed"`.
- **AC-12.** Given an `https://` target redirecting to `http://`, then `redirect.downgrade=true`. Given a different host, `cross_host=true`.

## 8. Tests

| Test | Package | Asserts | ACs |
| --- | --- | --- | --- |
| `TestDiagnosticsFixtures` (existing, unchanged) | `internal/network` | Not followed by default | AC-1 |
| `TestHTTPTimeout`, `TestHTTPHeaderAndPortBounds` (existing, unchanged) | `internal/network` | Bounds | regression |
| `TestHTTPRedirectTargetNotFollowed` | `internal/network` | AC-2 request counter == 1 | AC-2 |
| `TestSecurityHeadersReport` | `internal/network` | AC-3 table via the pure function and one live server | AC-3 |
| `TestHTTPMetadataOmitsCookies` | `internal/network` | AC-4 JSON scan | AC-4 |
| `TestHTTPPhases` | `internal/network` | AC-5 | AC-5 |
| `TestRedirectChainComplete` | `internal/network` | AC-6 | AC-6 |
| `TestRedirectChainLoop` | `internal/network` | AC-7 | AC-7 |
| `TestRedirectChainLimit` | `internal/network` | AC-8 | AC-8 |
| `TestRedirectInvalidTarget` | `internal/network` | AC-9 | AC-9 |
| `TestRedirectsValidation` | `internal/network` | AC-10 and the −1/0/5/6 bounds | AC-10 |
| `TestRedirectChainDeadline` | `internal/network` | AC-11 | AC-11 |
| `TestRedirectFlags` | `internal/network` | AC-12 (`normalizeURL` table included) | AC-12 |
| `TestCLIDiagnoseRedirectsFlag` | `internal/adapter/cli` | Exit 2 for a non-http kind; `data.http` present | CLI |
| Playwright `HTTP diagnostic shows phases, headers and redirect target` | `frontend/tests/workflows.spec.ts` | Contract fake | UI |

All servers are `httptest` servers. There are no sleeps; hangs use `<-r.Context().Done()`, as in `TestHTTPTimeout`.

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
go test -race -run 'HTTP|Redirect|SecurityHeaders|Diagnostics' ./internal/network/
bash scripts/setup/setup-linux-docker.sh test
```

The Docker lab integration test (`tests/integration/lab_test.go:32-35`) keeps asserting `204 No Content` for `/health`. Optionally add an assertion that `http.security_headers` has length 10.

## 10. Documentation updates

- `docs/architecture.md` §5.1/§6: a note that the HTTP diagnostic carries `http` metadata and never auto-follows redirects.
- `docs/tools-research.md` §4.7(c): mark the `httptrace` timings, the security headers and the redirect chain as implemented (B11), and state that the explicit trace is opt-in and capped at 5.
- `PLAN.md` B11: tick after AC-1 to AC-12 pass.
- `README.md`: `diagnose --redirects`.

## 11. Risks and rollback

- **Breaking the not-followed guarantee.** Guarded by the unchanged `TestDiagnosticsFixtures` and the request counter in AC-2. The `CheckRedirect` function is not modified.
- **Tokens in redirect URLs.** These are shown to the user only, never persisted. The UI renders them as text, not links, to avoid accidental navigation.
- **Concurrent `httptrace` callbacks** (happy eyeballs). The recorder is mutex-protected and covered by the race detector run.
- **Rollback.** Revert the commits. `Result.HTTP` is additive (`omitempty`), so old clients ignore it.

## 12. Quality and delivery assertions

### 12.1 Applicable quality gates

| Gate | Applies | Tightened threshold |
| --- | --- | --- |
| QG-1, QG-2, QG-3, QG-4, QG-7, QG-9, QG-13, QG-14, QG-15 | yes | none |
| QG-5 | yes | the race detector run over `internal/network` must include `TestHTTPPhases` |
| QG-6 | yes | `internal/network/http.go` ≥ 90%; `internal/network` overall not lower than before (record before/after) |
| QG-8 | yes | +1 Playwright workflow |
| QG-10 | yes | `internal/network` still imports no `internal/adapter/*` |
| QG-11 | yes | `TestHTTPMetadataOmitsCookies`; the job summary stays `Diagnostic <state>` |
| QG-12 | yes | `MaxRedirectHops`, `MaxHeaderValueBytes` and `MaxLocationBytes` each have limit and limit+1 tests |

### 12.2 Delivery assertions

| ID | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | Redirects never auto-followed | `go test -run 'TestDiagnosticsFixtures\|TestHTTPRedirectTargetNotFollowed' ./internal/network/` | PASS; request counter == 1 | CI |
| DA-2 | `CheckRedirect` unchanged | `grep -n "ErrUseLastResponse" internal/network/*.go` | exactly one client construction, returning `http.ErrUseLastResponse` | reviewer |
| DA-3 | Hop cap and loop detection | `go test -run 'TestRedirectChainLoop\|TestRedirectChainLimit' ./internal/network/` | PASS; server request counts 1 and 6 | CI |
| DA-4 | Header allowlist only | `go test -run TestSecurityHeadersReport ./internal/network/` and `grep -c '"Set-Cookie"' internal/network/http.go` | PASS; `0` | CI, reviewer |
| DA-5 | Backward-compatible `Values` | the existing lab test `TestPrivateLab` (`Values[0] == "204 No Content"`) | PASS | CI (Docker) |

Evidence record: `docs/verification/specs/B11.md`.
