# W04 Web security

| Field | Value |
| --- | --- |
| PLAN tasks | W04 |
| Status | Implementation and non-native acceptance verified 2026-10-22; native browser acceptance pending |
| Depends on | H01–H08; implemented inside W01 step 6 (no commit contains `kali-tools serve` without this spec Done) |
| Risk | High: the server can start network diagnostics, discovery and capture, and can reveal the saved Wi-Fi key; it is reachable from any web page the user visits |

## 1. Goal

`kali-tools serve` is safe to run on a workstation with a normal browser. A web page the user visits cannot read data from it, cannot make it act (CSRF), and cannot reach it through DNS rebinding. Only the person who received the launch URL can use it. Requests, bodies, streams and concurrent work are bounded, the token never appears in logs or responses, and every control is proven by a named test. This spec is the blocking baseline from [architecture research §8](../architecture-research.md).

## 2. Non-goals

- No user accounts, passwords, OAuth or multi-user authorization. One launch token per server process.
- No CORS support of any kind, and no `AddTrustedOrigin` on the cross-origin protection.
- No rate limiting of the token exchange (a 256-bit token is not brute-forceable; concurrency caps still apply).
- No self-signed certificate generation and no HSTS (remote TLS is specified in W01 §4.7).
- No protection against other processes of the same OS user (they can read the terminal, the process memory and the browser profile). This is an accepted risk in the threat model.
- No WebSocket endpoint.

## 3. Current state

- `internal/adapter/httpapi/` contains only `.gitkeep`; no listener exists in the module (`grep -rn "net.Listen\|ListenAndServe" --include=*.go .` finds only tests and `cmd/lab-fixture`).
- Go is 1.27.2 (`go.mod:3`), so `http.NewCrossOriginProtection` (Go 1.25), `slog.DiscardHandler` (Go 1.24) and `http.Request.Pattern` (Go 1.23) are available.
- `docs/architecture-research.md:421-466` (§8) lists the mandatory layers and a sketch `secure(next, port, token)`; `docs/architecture.md:210-219` (§6.2) repeats them as blocking.
- `PLAN.md` W04 (§7.2) lists the asserts: reject a foreign Host, a cross-origin POST, a missing or invalid token and oversized bodies; the token never appears in logs, history or responses.
- The application already caps concurrent operations at 8 (`internal/app/service.go`, `len(s.jobs) >= 8`; H02 moves it into `job.Manager` with `DefaultMaxConcurrent = 8`).
- `internal/capture/offline.go:21` `MaxFileSize = 64 << 20`; W01 adds `app.MaxCaptureFileSize` equal to it.
- `frontend/src/app/App.tsx` builds downloads with `Blob` object URLs (`App.tsx:48-59`) and uses no inline `<script>`; `frontend/src/styles/style.css:1` imports only Tailwind (no remote fonts).
- (W01) `httpapi.Server`, `Options`, routes and handlers are specified in W01 §4.4; this spec owns `security.go`, `session.go` and the constants below.

## 4. Design

### 4.1 Listener and Host allowlist

- The default `--listen` is `127.0.0.1:0` (W01 §4.6). `0.0.0.0`, `::` and empty hosts are always rejected; non-loopback needs W01 §4.7 opt-in and TLS.
- After binding, `New` computes the allowlist from the **actual** port `P`:
  - loopback listener: `127.0.0.1:P`, `localhost:P`, `[::1]:P`;
  - non-loopback listener: exactly `<listen host>:P` (IPv6 hosts in brackets), as written in `--listen` after `net.SplitHostPort`.
- `hostAllowed(r)` lowercases `r.Host` and checks set membership. An empty Host, a missing port or any other value gets 403 `forbidden` "host not allowed". This applies to **every** path, including static assets and `/healthz`, because a rebinding page could otherwise read assets and probe the API.

### 4.2 Launch token and session

```go
const (
    tokenBytes     = 32 // crypto/rand
    sessionCookie  = "kali_session_" // + decimal port, so two servers on one host do not clash
    maxSessionBody = 1 << 10 // 1 KiB
)

type session struct {
    launch string // base64.RawURLEncoding of 32 random bytes (43 chars); printed in the URL fragment
    cookie string // base64.RawURLEncoding of another 32 random bytes; cookie value
    name   string // sessionCookie + port
    secure bool   // TLS listener
}

func newSession(port int, secure bool) (*session, error) // crypto/rand.Read; error → New fails
func (s *session) valid(r *http.Request) bool
func (s *session) exchange(w http.ResponseWriter, r *http.Request) // POST /api/session
```

- `Server.URL()` is `<scheme>://<host>:<port>/#token=<launch>`; the fragment is never sent to the server or in a `Referer`.
- `POST /api/session`: `Content-Type: application/json`, body ≤ `maxSessionBody`, exactly `{"token": "<string>"}`. Compare with `subtle.ConstantTimeCompare([]byte(got), []byte(s.launch)) == 1`. On success: `Set-Cookie: <name>=<cookie>; Path=/; HttpOnly; SameSite=Strict` plus `; Secure` when TLS, no `Max-Age`/`Expires` (session cookie), status 204. On failure: 401 `unauthorized` "invalid token" and one log line `session exchange rejected` with no token value.
- The launch token stays valid for the life of the process, so a second tab or a restarted browser can exchange it again. There is no logout; stopping the server invalidates everything.
- `valid(r)` is true when either:
  - the cookie `<name>` exists and `subtle.ConstantTimeCompare` with `s.cookie` is 1; or
  - the header `Authorization: Bearer <value>` compares equal to `s.launch` (for curl and scripts).
- Every `/api/*` route except `POST /api/session` requires `valid(r)`, **including** `GET /api/events` (EventSource cannot set headers, so the browser relies on the cookie). Failure is 401 `unauthorized` "session required". `/healthz` and static assets do not require a session (they expose no data; Host allowlist still applies).

### 4.3 Cross-origin requests

- `cop := http.NewCrossOriginProtection()`; `cop.SetDenyHandler(...)` writes the W01 error body with 403 `forbidden` "cross-origin request rejected". It is applied to every request; it blocks unsafe methods (`POST`, `PUT`, `DELETE`, `PATCH`) whose `Sec-Fetch-Site` is not `same-origin`/`none`, or whose `Origin` host differs from `Host` when `Sec-Fetch-Site` is absent. `AddTrustedOrigin` and `AddInsecureBypassPattern` are never called.
- Fetch-metadata check for reads: an `/api/*` request with `Sec-Fetch-Site` present and not `same-origin` or `none` gets 403 `forbidden`, for every method. This blocks cross-site GETs (including `EventSource`) even if a cookie were sent.
- No state changes on GET: the only GET routes are those in W01 §4.4.1, all read-only (`/api/capture/export` writes only a temporary file that it deletes).
- No CORS: the server never writes any `Access-Control-*` header. `OPTIONS` requests get the mux 405.

### 4.4 Response headers

Set on every response by `securityHeaders`:

```text
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
X-Frame-Options: DENY
Cross-Origin-Opener-Policy: same-origin
Cross-Origin-Resource-Policy: same-origin
Permissions-Policy: camera=(), microphone=(), geolocation=()
```

`/api/*` responses also get `Cache-Control: no-store`. If a dependency injects `<style>` elements and the W01 "no CSP violations" Playwright test fails, the only allowed change is adding `'unsafe-inline'` to `style-src`, recorded in the evidence. `script-src` never changes.

### 4.5 Middleware order

```go
func (s *Server) Handler() http.Handler {
    mux := http.NewServeMux()
    addRoutes(mux, s.app, s) // W01 routes.go
    var h http.Handler = mux
    h = s.requireSession(h)   // /api/* except POST /api/session
    h = s.limitRequests(h)    // /api/* except /api/events: maxInflight semaphore
    h = s.fetchMetadata(h)    // /api/*: Sec-Fetch-Site check
    h = s.cop.Handler(h)      // http.NewCrossOriginProtection
    h = s.hostAllowlist(h)    // every path
    h = securityHeaders(h)    // every response, including rejections
    h = s.accessLog(h)
    h = recoverPanics(h, s.log)
    return h
}
```

Order rationale: headers are set before any rejection so 401/403 responses also carry them; the Host check runs before the cross-origin check and the session check, so rebinding requests learn nothing about the API.

### 4.6 Limits

```go
const (
    readHeaderTimeout = 5 * time.Second
    idleTimeout       = 120 * time.Second
    maxHeaderBytes    = 16 << 10            // 16 KiB
    shutdownTimeout   = 5 * time.Second
    maxJSONBody       = 64 << 10            // 64 KiB, every JSON request route
    maxCaptureUpload  = app.MaxCaptureFileSize // 64 MiB
    bodyReadTimeout   = 30 * time.Second    // rc.SetReadDeadline for JSON bodies
    uploadReadTimeout = 120 * time.Second   // rc.SetReadDeadline for capture upload
    maxSSESubscribers = 8
    maxInflight       = 16                  // concurrent non-SSE /api requests
    maxFileTransfers  = 1                   // concurrent upload or export (used by W01 files.go)
)
```

- `maxSessionBody` (1 KiB) is declared in `session.go` (§4.2).
- `http.Server{ReadHeaderTimeout: readHeaderTimeout, IdleTimeout: idleTimeout, MaxHeaderBytes: maxHeaderBytes, WriteTimeout: 0}`. `WriteTimeout` stays 0 because it would kill SSE; SSE uses per-write deadlines (W01 §4.4.3).
- Every request body is wrapped in `http.MaxBytesReader` with the route's limit (0 for routes without a body). Exceeding it gives 413 `invalid_input` "request body too large".
- Caps use buffered-channel semaphores acquired with `select { case sem <- struct{}{}: default: 429 }`. A full cap is 429 `conflict` "too many concurrent requests" (or "too many event streams" for SSE). No request waits for a slot.
- Concurrent jobs: the application cap (8, H02) applies unchanged; the server adds no job cap of its own, and `maxInflight` bounds synchronous long operations (Discover, InspectServices, Diagnose).

Verification clarification (2026-10-22): `MaxHeaderBytes` delegates to Go's HTTP parser, which reserves 4096 additional bytes for buffered read-ahead. Wire tests cover 16 KiB, the effective 20 KiB boundary and boundary+1; this is not a custom exact 16 KiB wire cap. Deadline delegation is verified by recording the 30/120-second read deadlines and their reset on success/failure, real header/idle connection closure, synthetic-clock SSE and forced-shutdown boundary tests. These checks verify application configuration and delegation without reimplementing the standard library's timers.

### 4.7 Logging

- `accessLog` writes one `slog.Info("http request", "method", r.Method, "route", r.Pattern, "status", code, "duration_ms", ms)` per request. It never logs `r.URL`, the query, headers, cookies or bodies.
- The token and cookie value are never passed to the logger, never included in an error message, never returned by any route, and never written to history or SQLite. `Server.URL()` is the only accessor; `runServe` writes it to stdout.
- Reveal results, capture bytes and diagnostic targets are never logged.
- `recoverPanics` logs `slog.Error("panic recovered", "route", r.Pattern)` without the panic value (it may hold request data) and returns 500 `failed`.

### 4.8 Threat model

| ID | Threat | Attacker position | Control (section) | Residual risk |
| --- | --- | --- | --- | --- |
| T1 | DNS rebinding: an attacker domain re-resolves to 127.0.0.1 and its page reads the API | any web page | Host allowlist on every path (§4.1); session cookie is host-only and not sent to the attacker origin (§4.2) | none known |
| T2 | CSRF: a page POSTs to start a diagnostic, discovery, capture or reveal | any web page | `CrossOriginProtection` (§4.3); `SameSite=Strict` cookie (§4.2); JSON content type required (W01) | none known |
| T3 | Cross-site read of GET data or SSE (`<img>`, `<script>`, `EventSource`) | any web page | fetch-metadata check (§4.3); `SameSite=Strict`; no CORS headers; `Cross-Origin-Resource-Policy`; `nosniff` (§4.4) | none known |
| T4 | Unauthenticated local caller (curl, another browser profile) | local network stack | launch token + session cookie on every `/api` route including SSE (§4.2) | a process of the same OS user can read the terminal or browser profile (accepted, §2) |
| T5 | Token theft through logs, Referer, history or error messages | log readers, third-party sites | token only in the URL fragment; `Referrer-Policy: no-referrer`; fragment scrubbed with `history.replaceState` (W01 §4.2); no-log rule (§4.7) | browser history may keep the URL if opened before scrubbing (accepted; token dies with the process) |
| T6 | Timing attack on token comparison | local caller | `subtle.ConstantTimeCompare` (§4.2) | length is public (43 chars) |
| T7 | Clickjacking of the reveal button | any web page | `frame-ancestors 'none'`, `X-Frame-Options: DENY` (§4.4) | none known |
| T8 | XSS through injected script or content-type confusion | data shown in the UI | strict CSP with no inline script (§4.4); `nosniff`; React escaping | a vulnerable dependency could still render attacker data; CSP blocks script execution |
| T9 | Resource exhaustion: slowloris, huge headers, huge bodies, many streams, many requests | local or remote caller | `ReadHeaderTimeout`, `MaxHeaderBytes`, `MaxBytesReader`, read deadlines, SSE/inflight/file caps, app job cap (§4.6) | bounded, not eliminated |
| T10 | Network exposure: LAN attacker reaches the server | LAN | loopback default; remote needs `--allow-remote` + TLS + warning (W01 §4.7) | a token holder on the LAN has full tool access, by design |
| T11 | Abuse as a network probe: diagnostics and discovery reach arbitrary hosts | token holder | intended tool behavior, therefore no target filtering; exposure limited by T4/T10 controls; app bounds on scope (/24, 32 ports) and rate | a token holder can scan what the host can reach (documented) |
| T12 | Wi-Fi key leak over HTTP | network observer, caches | POST only, session required, `Cache-Control: no-store`, never logged (W01 §4.4.1); TLS required off loopback | loopback HTTP is plaintext but never leaves the host |
| T13 | Path traversal or arbitrary file access through import/export | token holder | no path parameters; uploads go to `os.CreateTemp`, exports to `os.MkdirTemp`, both deleted (W01 §4.4.2); static files from an `fs.FS` | none known |
| T14 | Server under another account shows misleading capability states | user | states computed in the server process (W01 §4.5) | none known |

### 4.9 Error statuses used by this spec

| Condition | Status | Code |
| --- | --- | --- |
| Host not allowed | 403 | `forbidden` |
| Cross-origin unsafe request or cross-site fetch metadata | 403 | `forbidden` |
| Missing or invalid session | 401 | `unauthorized` |
| Body too large | 413 | `invalid_input` |
| Cap reached | 429 | `conflict` |
| Panic | 500 | `failed` |

## 5. Files

Create:

- `internal/adapter/httpapi/security.go`: `securityHeaders`, `hostAllowlist`, `fetchMetadata`, `limitRequests`, `accessLog`, `recoverPanics`, limit constants.
- `internal/adapter/httpapi/session.go`: `session`, `newSession`, `valid`, `exchange`, `requireSession`.
- `internal/adapter/httpapi/security_test.go`, `internal/adapter/httpapi/session_test.go`.

Modify:

- `internal/adapter/httpapi/server.go` (W01): call `newSession`, compute the allowlist, build the chain of §4.5, set the `http.Server` limits.
- `docs/architecture.md` §6.2, `SECURITY.md` (a "Web server mode" section with the threat model summary), `CHANGELOG.md`, `PLAN.md` W04.

## 6. Implementation steps

All inside W01 step 6; each keeps tests green.

1. `security.go` limits and `securityHeaders`, with `TestSecurityHeadersOnEveryResponse`.
2. `hostAllowlist` and allowlist computation in `New`, with `TestForeignHostRejected` and `TestAllowedHosts`.
3. `session.go`: token generation, exchange, cookie, `requireSession`, with the session tests.
4. Cross-origin protection and fetch-metadata check, with `TestCrossOriginPostRejected` and `TestCrossSiteFetchRejected`.
5. Body limits, read deadlines, semaphores, `http.Server` limits, with the limit tests.
6. `accessLog`, `recoverPanics`, and `TestTokenNeverLogged`.
7. Wire `serve` (W01 step 6) only after steps 1–6 pass.
8. Docs: architecture, `SECURITY.md`, CHANGELOG.
9. Write `docs/verification/specs/W04.md`.

## 7. Acceptance criteria

- **AC-1** Given a running server on port P, when a request carries `Host: evil.example:P` (DNS rebinding), `Host: 127.0.0.1:P+1`, or an empty Host, then the response is 403 `forbidden` for `/`, `/healthz` and `/api/capabilities`.
- **AC-2** Given `Host: 127.0.0.1:P`, `localhost:P` and `[::1]:P` on a loopback server, then each is accepted.
- **AC-3** Given a valid session cookie, when a `POST /api/diagnostics` carries `Sec-Fetch-Site: cross-site`, or carries `Origin: https://evil.example` without `Sec-Fetch-Site`, then the response is 403 and no job starts.
- **AC-4** Given a valid session, when `GET /api/history` carries `Sec-Fetch-Site: cross-site`, then the response is 403.
- **AC-5** Given no cookie and no bearer, or an invalid cookie, or a wrong bearer, when any `/api/*` route except `POST /api/session` is called, then the response is 401 `unauthorized`, and no application method is called.
- **AC-6** Given no cookie, when `GET /api/events` is opened, then the response is 401 and no subscription is created.
- **AC-7** Given the launch token, when `POST /api/session` is called with it, then the response is 204 with a cookie that has `HttpOnly`, `SameSite=Strict`, `Path=/`, no expiry, and `Secure` only under TLS; with a wrong token, then 401 and no cookie.
- **AC-8** Given each JSON route, when the body is exactly `maxJSONBody` bytes of valid JSON padding, then it is not rejected for size; with `maxJSONBody + 1` bytes, then 413. The same holds for `maxSessionBody` and, in W01, `maxCaptureUpload`.
- **AC-9** Given any response, including 401, 403, 404 and 413, then the headers of §4.4 are present and no `Access-Control-*` header exists, even when the request has an `Origin` header.
- **AC-10** Given `maxSSESubscribers` open streams, when one more opens, then 429; given `maxInflight` blocked requests, when one more arrives, then 429.
- **AC-11** Given a server whose logger writes to a buffer, when the session exchange (valid and invalid), SSE, Wi-Fi reveal and a panic route run, then the buffer contains neither the launch token, nor the cookie value, nor `synthetic-test-key`, and no response body contains the launch token or cookie value.
- **AC-12** Given a client that sends headers one byte at a time, then the connection is closed after `readHeaderTimeout` (test uses an injected 200 ms value).
- **AC-13** Given the token comparison, then `session.valid` and `exchange` use `subtle.ConstantTimeCompare` (reviewer check plus `TestSessionUsesConstantTimeCompare`, which greps the source).
- **AC-14** Given the Playwright `http` project, then no `securitypolicyviolation` event fires on any page (W01 test), confirming the CSP does not break the UI.
- **AC-15** [native] Given Windows 11 with Edge and Chrome, when a page on another origin (served by `cmd/lab-fixture` or `python -m http.server`) runs `fetch("http://127.0.0.1:P/api/history", {credentials: "include"})` and a form POST to `/api/diagnostics`, then both fail and the server log shows 403 or 401. Record in `docs/verification/`.

## 8. Tests

All in package `httpapi` (internal test package for the unexported middleware; requests through `Server.Handler()` built by `New` on `127.0.0.1:0` with a fake-backed application from the W01 test helper).

| Test | Asserts | AC |
| --- | --- | --- |
| `TestForeignHostRejected` | DNS-rebinding Host, wrong port, empty Host → 403 on three paths | AC-1 |
| `TestAllowedHosts` | three loopback forms accepted; non-loopback allowlist is exactly the listen host | AC-2 |
| `TestCrossOriginPostRejected` | `Sec-Fetch-Site: cross-site` and foreign `Origin` → 403; job count unchanged | AC-3 |
| `TestCrossSiteFetchRejected` | cross-site GET → 403 | AC-4 |
| `TestMissingOrInvalidTokenRejected` | table over every `/api` route × {none, bad cookie, bad bearer} → 401; fake app call counter 0 | AC-5 |
| `TestEventsRequireSession` | SSE without cookie → 401; subscriber count 0 | AC-6 |
| `TestSessionExchange` | cookie attributes with and without TLS; wrong token → 401 | AC-7 |
| `TestOversizedBodyRejected` | limit and limit+1 for `maxJSONBody`, `maxSessionBody` | AC-8 |
| `TestSecurityHeadersOnEveryResponse` | headers on 200/401/403/404/413; no `Access-Control-*` with `Origin` set; `OPTIONS` → 405 | AC-9 |
| `TestSSESubscriberCap` | `maxSSESubscribers` streams open, next → 429; after one closes, a new one succeeds | AC-10 |
| `TestInflightCap` | `maxInflight` blocking requests, next → 429 | AC-10 |
| `TestTokenNeverLogged` | AC-11 | AC-11 |
| `TestReadHeaderTimeout` | slow header client closed | AC-12 |
| `TestSessionUsesConstantTimeCompare` | source of `session.go` contains `subtle.ConstantTimeCompare` and no `==` comparison of token strings | AC-13 |

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
```

Spec-specific:

```bash
go test -race -count=1 ./internal/adapter/httpapi/...
go test -count=1 ./internal/adapter/httpapi -run 'TestForeignHost|TestAllowedHosts|TestCrossOrigin|TestCrossSite|TestMissingOrInvalidToken|TestEventsRequireSession|TestSessionExchange|TestOversizedBody|TestSecurityHeaders|TestSSESubscriberCap|TestInflightCap|TestTokenNeverLogged|TestReadHeaderTimeout|TestSessionUsesConstantTimeCompare' -v
govulncheck ./...
# Manual smoke, loopback server on port P:
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: evil.example:P' http://127.0.0.1:P/              # 403
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:P/api/capabilities                        # 401
curl -s -o /dev/null -w '%{http_code}\n' -H 'Authorization: Bearer <token>' http://127.0.0.1:P/api/capabilities   # 200
node scripts/markdown.mjs
```

## 10. Documentation updates

- `docs/architecture.md` §6.2: security chain order, limits table, threat model link.
- `SECURITY.md`: "Web server mode" section: loopback default, token handling, remote opt-in, what a token holder can do (T10, T11), how to report issues.
- `README.md` browser section (W01): one paragraph on the token URL and never sharing it.
- `CHANGELOG.md` `[Unreleased]`; `PLAN.md` W04 boxes with evidence links.

## 11. Risks and rollback

- **A control is bypassed by a future route.** Mitigated by the middleware chain applying to the whole mux and by `TestMissingOrInvalidTokenRejected` iterating over every registered route (it reads the route list from `routes.go`, so a new route is covered automatically).
- **Go `CrossOriginProtection` behavior changes.** Pinned toolchain; `TestCrossOriginPostRejected` fails on regression.
- **CSP breaks the UI.** Only `style-src 'unsafe-inline'` may be added (§4.4).
- **Two servers on one host.** Port-suffixed cookie names avoid clashes.
- Rollback: revert the W01 step 6 commit; that removes `serve` and the listener together with this code. Never ship `serve` with any part of this spec reverted.

## 12. Quality and delivery assertions

### 12.1 Applicable quality gates

| Gate | Applies | Threshold for this spec |
| --- | --- | --- |
| QG-1, QG-2, QG-3, QG-4 | yes | as in README §5.1 |
| QG-5 | yes | `go test -race ./internal/adapter/httpapi/...` clean (caps and SSE are concurrent) |
| QG-6 | yes | `security.go` and `session.go` ≥ 95 % statements (tightened: security-critical); `httpapi` package ≥ 85 % (W01) |
| QG-7 | yes | `govulncheck ./...` reports no reachable vulnerability in `net/http` or `crypto/*` |
| QG-8 | yes | W01 Playwright `no CSP violations @http-only` passes |
| QG-10 | yes | `httpapi` imports `crypto/subtle` and `crypto/rand`; never `math/rand` (`go list -f '{{.Imports}}' ./internal/adapter/httpapi` shows no `math/rand`) |
| QG-11 | yes | `TestTokenNeverLogged` with the launch token, cookie and `synthetic-test-key` |
| QG-12 | yes | every constant in §4.6 tested at limit and limit+1 (time limits through injected values) |
| QG-13, QG-14, QG-15 | yes | as in README §5.1 |

### 12.2 Delivery assertions

Every threat in §4.8 maps to at least one DA.

| DA | Assertion | Command / test / artifact | Pass threshold | Verifier | Threats |
| --- | --- | --- | --- | --- | --- |
| DA-1 | Foreign Host refused on every path | `go test ./internal/adapter/httpapi -run 'TestForeignHostRejected\|TestAllowedHosts' -count=1` | pass | CI | T1 |
| DA-2 | Cross-origin unsafe requests refused | `go test ./internal/adapter/httpapi -run TestCrossOriginPostRejected -count=1` | pass; job count unchanged | CI | T2 |
| DA-3 | Cross-site reads refused, no CORS | `go test ./internal/adapter/httpapi -run 'TestCrossSiteFetchRejected\|TestSecurityHeadersOnEveryResponse' -count=1` | pass; zero `Access-Control-*` headers | CI | T3, T7, T8 |
| DA-4 | Every `/api` route requires the session, SSE included | `go test ./internal/adapter/httpapi -run 'TestMissingOrInvalidTokenRejected\|TestEventsRequireSession\|TestSessionExchange' -count=1` | pass; the route table in the test equals the registered routes | CI | T4 |
| DA-5 | Token never logged or returned | `go test ./internal/adapter/httpapi -run TestTokenNeverLogged -count=1` | pass | CI | T5, T12 |
| DA-6 | Constant-time comparison | `go test ./internal/adapter/httpapi -run TestSessionUsesConstantTimeCompare -count=1`; reviewer reads `session.go` | pass; no `==` on token strings | CI, reviewer | T6 |
| DA-7 | CSP does not break the UI | `cd frontend && bunx playwright test --project=http -g "no CSP violations"` | pass | CI | T8 |
| DA-8 | Bounded resources | `go test ./internal/adapter/httpapi -run 'TestOversizedBodyRejected\|TestSSESubscriberCap\|TestInflightCap\|TestReadHeaderTimeout' -count=1` | pass | CI | T9 |
| DA-9 | Remote exposure is opt-in with TLS | `go test ./internal/adapter/httpapi -run TestRemoteRequiresOptInAndTLS -count=1` (W01) | pass | CI | T10 |
| DA-10 | Probe abuse is documented | `SECURITY.md` "Web server mode" states what a token holder can do and why remote is opt-in | section present | reviewer | T11 |
| DA-11 | Reveal response is not cacheable | `go test ./internal/adapter/httpapi -run TestRevealNoStoreAndNotLogged -count=1` (W01) | pass | CI | T12 |
| DA-12 | No path parameters, temp files cleaned | `go test ./internal/adapter/httpapi -run 'TestCapture\|TestRoutesMatchApplication' -count=1` (W01); reviewer confirms no route takes a path | pass | CI, reviewer | T13 |
| DA-13 | Server capability states are honest | Playwright `capability states come from the server @http-only` (W01) | pass | CI | T14 |
| DA-14 | Real browsers refuse cross-origin access | AC-15 procedure in Edge and Chrome | both blocked, recorded | [native] | T1, T2, T3 |

Evidence record: `docs/verification/specs/W04.md`.
