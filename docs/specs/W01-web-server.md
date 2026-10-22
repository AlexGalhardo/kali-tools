# W01 Web server

| Field | Value |
| --- | --- |
| PLAN tasks | W01, W02, W03, W05, W06 |
| Status | Implementation and non-native acceptance verified 2026-10-22; native acceptance pending |
| Depends on | H01–H08, C01 (exit codes and `serve` dispatch); W04 is implemented together with step 6 |
| Risk | High: opens a TCP listener that can start network operations; any gap in W04 is remotely exploitable from a web page |

## 1. Goal

`kali-tools serve` serves the same React build as the desktop app to a local browser. The browser talks to the same `app.Application` through REST routes that mirror the application methods one to one, and receives job, metrics and capture events over Server-Sent Events. The frontend chooses Wails or HTTP at runtime, so feature code is transport-neutral. Native file dialogs become browser upload and download. Remote exposure is an explicit opt-in that requires TLS. Capability states shown in a browser are those of the server process. Playwright runs the shared workflows against both the Wails fake and a real `httpapi` test server.

## 2. Non-goals

- No security design here beyond wiring: every security control is specified in [W04](W04-web-security.md). Do not weaken or reorder it.
- No self-signed certificate generation. Remote mode requires a user-supplied certificate and key (PLAN W05 allows "user certificate"); generated certificates are a follow-up.
- No WebSocket, no CORS, no OpenAPI generator, no router dependency.
- No new application operations, validation rules or capability logic.
- No multi-user accounts, no persistent sessions, no daemon or service installer.
- No serving from the Docker image: the Docker image has no built frontend (§3.4); `serve` there fails with `dependency_missing`.
- No visual redesign. The App split moves code; markup, classes and strings stay identical.
- No Wails v3 migration.

## 3. Current state

Re-verify every fact. Facts marked (H) are expected outputs of H01–H08 and C01; if one is false, STOP and report.

Checkout corrections (2026-10-21): H05 moved dialogs to adapter/wailsui and bindings to wailsui/Bindings; the frontend baseline has 19 workflows. C01 reserves serve without a listener; the actual runServe dispatch is added by this spec step 6, as its Files/Steps require. The shared asset function uses fs.Sub with the constant valid path dist (its error is impossible), preserving 100% reachable statement coverage without a panic-only test seam.

Continuation correction (2026-10-22): steps 1–9 are implemented. The shared acceptance harness still lacked its planned HTTP driver. Complete it with the production server, cookie authentication and unchanged shared scenarios, using the two acceptance test files listed below. Boundary and native gates remain separately tracked.

### 3.1 Assets and desktop root

- `main.go:14-15` embeds `all:frontend/dist` into `var assets embed.FS`; `main.go:19` passes it to `assetserver.Options{Assets: assets}`.
- `bridge_smoke_test.go:62` also uses `assets`.
- `frontend/dist` contains only `.gitkeep` in Git; `frontend/package.json` `build` runs `tsc --noEmit && vite build && node scripts/preserve-dist.mjs`, and `frontend/scripts/preserve-dist.mjs` recreates `dist/.gitkeep`.
- No Go package exists under `frontend/`.

### 3.2 Native dialogs and events in the desktop facade

- `app.go:192` `ImportCapture` opens `wruntime.OpenFileDialog`; `app.go:196-197` returns `{packets: []}` when the dialog is cancelled.
- `app.go:266` `SaveCapture(format)` opens `wruntime.SaveFileDialog`, then `artifact.Save` with a 30 s deadline (`app.go:270`); returns `false` when cancelled.
- Events are emitted with `wruntime.EventsEmit` under the names `job` (`app.go:65`), `metrics` (`app.go:80`, every 2 s) and `capture` (`app.go:240`).
- (H) After H02/H04/H05, these flows live in `app.Application` and `internal/adapter/wailsui`: `ImportCapture(ctx, path)` and `ExportCapture(ctx, path, format)` take paths, and the Wails adapter shows the dialogs. `Subscribe(ctx) (<-chan job.Event, func() uint64)` delivers all three topics. `JobSnapshot(ctx, id)` exists.

### 3.3 Frontend

- `frontend/src/shared/api/bridge.ts:1` imports `../../../wailsjs/go/main/App` (the only `wailsjs` import in `src`).
- `bridge.ts:2-93` declares the DTO types; `bridge.ts:94-122` declares `Backend`; `bridge.ts:131` `connected()` is `Boolean(window.go?.main?.App)`; `bridge.ts:132-137` `backend()` throws "Native bridge unavailable…" without Wails; `bridge.ts:138-142` `onEvent` wraps `window.runtime.EventsOn`.
- `frontend/src/main.tsx` renders `<App />` synchronously.
- `frontend/src/app/App.tsx` has 1,256 lines and holds every page (`App.tsx:29-47` page list: `dashboard`, `interfaces`, `diagnostics`, `wifi`, `packets`, `toolkit`, `history`, `settings`). Helpers `download` (`:48-59`), `Stat` (`:1222`), `Empty` (`:1239`), `Field` (`:1242`). `execute` (`:148-159`) stores `String(e)` as the error; `errorText` (`:98-101`) extracts the code with the regex `(?:^|\s)([a-z_]+):`, so every transport must reject with an `Error` whose message starts with the error code followed by a colon and one space.
- `App.tsx:182-224` subscribes to `metrics`, `job` and `capture`; `App.tsx:277-281` reads `e.result` from `job` events.
- `frontend/src/features/` contains only `.gitkeep`.
- `frontend/vite.config.ts` has no proxy. `frontend/playwright.config.ts` defines one implicit project with `webServer` = `npm run dev` on `PW_PORT` (default 4173).
- `frontend/tests/workflows.spec.ts:3-25` `connect(page)` injects a fake `window.go.main.App` and `window.runtime.EventsOn`, then waits for "Fixture Ethernet". 17 tests; several replace `window.go.main.App.<Method>` at runtime (`:37`, `:52`, `:62`, `:99`, `:102`, `:125`, `:141`, `:151`) or use `document.documentElement.dataset` hooks (`:12-13`, `:76`, `:161`). `:31` asserts the "Native bridge unavailable. Launch the desktop app to access your network." text without Wails.

### 3.4 Server, CLI and Docker

- `internal/adapter/httpapi/` contains only `.gitkeep`.
- `cmd/kali-tools/main.go` has no `serve` command (`main.go:37` usage).
- (H, C01) `cmd/kali-tools/main.go` `run` dispatches `serve` to `runServe` (C01 §4.11); `cli.ExitCode(code)` exists.
- `docker/Dockerfile:15,22` builds the CLI with `go build ./cmd/kali-tools` without building the frontend.
- `internal/capture/offline.go:21` `MaxFileSize = 64 << 20`.
- `docs/architecture.md:202-220` (§6.2) describes the planned server; `docs/architecture-research.md:401-419` (§7.4) the REST+SSE pattern.

## 4. Design

### 4.1 Shared assets (W01)

```go
// frontend/embed.go
package frontend

import (
    "embed"
    "io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built frontend rooted at dist/. It is the only embed of the
// frontend build in the module; the desktop and the web server both use it.
func Dist() fs.FS {
    sub, err := fs.Sub(dist, "dist")
    if err != nil {
        panic(err) // unreachable: "dist" is a constant path inside the embed
    }
    return sub
}
```

- `main.go` removes its `//go:embed` and passes `frontend.Dist()` to `assetserver.Options{Assets: ...}`. `bridge_smoke_test.go` does the same.
- `cmd/kali-tools/serve.go` passes `frontend.Dist()` as `httpapi.Options.Assets`.
- `httpapi.New` fails with `dependency_missing` "frontend assets are not built; run bun run build in frontend/" when `fs.Stat(opts.Assets, "index.html")` fails.

### 4.2 Transport-neutral frontend (W02)

Files in `frontend/src/shared/api/`:

- `types.ts`: the DTO types and `Backend` moved verbatim from `bridge.ts:2-122`, plus `Topic` and `TransportKind`.
- `transport.ts`: `Transport`, `WailsTransport`, `HttpTransport`, `selectTransport()`. It is the **only** module in `src` that imports `wailsjs`.
- `bridge.ts`: the public seam. Feature code imports only from `bridge.ts`.

```ts
// types.ts (additions)
export type Topic = "job" | "metrics" | "capture";
export type TransportKind = "wails" | "http";

// transport.ts
export interface Transport {
  readonly kind: TransportKind;
  readonly backend: Backend;
  init(): Promise<boolean>; // true when calls can succeed
  on(topic: Topic, fn: (data: unknown) => void): () => void;
}
export class WailsTransport implements Transport { /* generated bindings + window.runtime.EventsOn */ }
export class HttpTransport implements Transport { /* fetch + EventSource */ }
export function selectTransport(): Transport {
  return typeof window.go === "object" && window.go !== null
    ? new WailsTransport()
    : new HttpTransport();
}

// bridge.ts
export * from "./types";
export function initBridge(): Promise<void>;          // selects + init; never throws
export function connected(): boolean;                 // result of init()
export function transportKind(): TransportKind;
export function backend(): Backend;                   // throws the existing message when !connected()
export function onEvent<T>(name: Topic, fn: (data: T) => void): () => void;
```

- `main.tsx` awaits `initBridge()` before `createRoot(...).render(...)`.
- `WailsTransport.init()` resolves `true`. Its `backend` is the generated module H05 produced (today `bridge.ts:1`). Its `on` keeps `window.runtime?.EventsOn(...) ?? (() => {})`.
- `HttpTransport` rules:
  - `init()`: if `location.hash` matches `^#token=([A-Za-z0-9_-]{43})$`, `POST /api/session` with `{"token": "<value>"}` and then `history.replaceState(null, "", location.pathname + location.search)` so the token leaves the address bar. Then `GET /api/version`; `true` only for a 200 JSON response.
  - Every call uses `fetch(path, {credentials: "same-origin"})`. JSON bodies send `Content-Type: application/json`.
  - A non-2xx response with a JSON body `{"error": {"code", "message"}}` rejects with `new Error(code + ": " + message)`. A network failure rejects with `Error("no_connection: kali-tools server is unreachable")`. A non-JSON response rejects with `Error("no_connection: unexpected response from server")`.
  - `ImportCapture()`: creates a hidden `<input type="file" accept=".pcap,.pcapng">`, clicks it, and resolves on `change` or `cancel`. Cancel resolves `{packets: [], limited: false, format: ""}` (same as `app.go:196-197`). A file larger than `67108864` bytes rejects with `Error("invalid_input: capture must be at most 64 MiB")` without uploading. Otherwise it sends `POST /api/capture/import` with the `File` as body and `Content-Type: application/octet-stream`.
  - `SaveCapture(format)`: `GET /api/capture/export?format=<format>`, reads the body as a `Blob`, downloads it through a temporary object URL with `download = "capture." + format`, revokes the URL after 1000 ms, resolves `true`.
  - `on(topic, fn)`: one shared `EventSource("/api/events")`, created on the first subscription and closed when the last handler unsubscribes. It dispatches by SSE event name. It tracks `lastSeq` from each message's `lastEventId` (the SSE `id:`), and a set of non-terminal job IDs seen in `job` events or returned by `StartDiagnostic` (at most 64; oldest evicted). On an SSE `resync` event, or when an event's `seq` is greater than `lastSeq + 1`, it calls `GET /api/jobs/{id}` for each tracked ID and dispatches to `job` handlers the synthetic event `{schema_version: 1, seq: lastSeq, job: <status>}` (the `contract.JobEvent` shape without `result`; a diagnostic result missed during a disconnect is not recoverable, the job state and history are). A `not_found` drops the ID.
- `frontend/scripts/check-wailsjs-imports.mjs` fails when any file under `src/` other than `src/shared/api/transport.ts` contains `wailsjs`. `package.json` `lint` becomes `biome check src && node scripts/check-wailsjs-imports.mjs`.
- `t.native` in both catalogs changes to "Not connected. Launch the desktop app, or open the URL printed by kali-tools serve." and its Portuguese translation.

### 4.3 App.tsx split order (W02)

Each step moves one page's JSX and page-local state into `features/<capability>/<Name>Page.tsx`, receives shared state through props, and runs the full Playwright suite. Markup, class names and strings do not change.

```ts
// app/shell.ts: the props every page receives
export type PageProps = {
  t: Catalog;                                     // typeof en
  busy: boolean;
  execute: (fn: () => Promise<void>) => Promise<void>;
  stateText: (state: string) => string;
  setError: (v: string) => void;
  setNotice: (v: string) => void;
};
```

1. `shared/ui/Stat.tsx`, `shared/ui/Empty.tsx`, `shared/ui/Field.tsx`, `shared/lib/download.ts` (pure helpers, no state).
2. `features/settings/SettingsPage.tsx` (settings form; `settings` state stays in `App` because it drives `t`).
3. `features/history/HistoryPage.tsx`.
4. `features/interfaces/InterfacesPage.tsx`.
5. `features/overview/OverviewPage.tsx` (page id `dashboard`; metrics subscription stays in `App`).
6. `features/diagnostics/DiagnosticsPage.tsx` (moves the diagnostic form state and the `job` result subscription at `App.tsx:277-281`).
7. `features/wifi/WiFiPage.tsx` (moves `secret`, `revealEpoch` and the blur/visibility/15 s clearing at `App.tsx:240-260`; the clear-on-navigation stays correct because the page unmounts).
8. `features/capture/CapturePage.tsx` (page id `packets`; moves packet, filter, capture-format state and the `capture` subscription).
9. `features/discovery/DiscoveryPage.tsx` (page id `toolkit`; moves discovery, inspection and tools state).

After step 9, `App.tsx` holds only the shell (navigation, language, error/notice banner, page switch) and is under 300 lines.

### 4.4 HTTP adapter (W03)

Package `internal/adapter/httpapi`:

```go
type Options struct {
    Addr        string       // default "127.0.0.1:0" is applied by the caller
    Assets      fs.FS        // frontend.Dist()
    AllowRemote bool
    TLSCert     string       // PEM path; both or neither
    TLSKey      string
    Logger      *slog.Logger // nil → slog.New(slog.DiscardHandler)
}

type Server struct { /* unexported: app, listener, http.Server, session, limits, logger */ }

// New validates opts (W04 §4.1, W05 §4.7), loads TLS, creates the session
// secrets and binds the listener. It does not serve.
func New(a *app.Application, opts Options) (*Server, error)

func (s *Server) URL() string             // launch URL with "#token=…"; the only token accessor
func (s *Server) Addr() net.Addr
func (s *Server) CertFingerprint() string // "AB:CD:…" SHA-256 of the leaf, "" without TLS
func (s *Server) Handler() http.Handler   // full middleware chain (for tests)

// Run serves until ctx is done, then calls http.Server.Shutdown with a 5 s
// deadline. It returns nil after a clean shutdown. It may be called once.
func (s *Server) Run(ctx context.Context) error
```

`http.Server` fields: `Handler: s.Handler()`, `ReadHeaderTimeout`, `MaxHeaderBytes`, `IdleTimeout` from W04 §4.6, `WriteTimeout: 0` (SSE), `BaseContext` returning the `Run` context so SSE handlers stop on shutdown, `ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelWarn)`.

Files:

```text
internal/adapter/httpapi/
├── server.go      Options, Server, New, Run, URL, Addr, CertFingerprint, Handler
├── routes.go      addRoutes: every route, one line each
├── handlers.go    one small handler per route
├── events.go      SSE handler
├── files.go       capture upload and download
├── encode.go      decodeJSON, writeJSON, writeError, statusFor (the single error mapping)
├── security.go    W04 middleware
├── session.go     W04 token and cookie
└── *_test.go
```

#### 4.4.1 Routes (routes.go)

Every row is one `mux.Handle` line. "Body limit" names the W04 constant.

| Method and pattern | Application call | Request | Success | Body limit |
| --- | --- | --- | --- | --- |
| `GET /healthz` | none | — | 200 `{"status":"ok"}` | 0 |
| `POST /api/session` | none (W04) | `{"token"}` | 204 + cookie | `maxSessionBody` |
| `GET /api/version` | none | — | 200 `{"version","os","arch","mode":"server"}` | 0 |
| `GET /api/capabilities` | `Capabilities` | — | 200 `[]Capability` | 0 |
| `GET /api/interfaces` | `Interfaces` | — | 200 `[]Interface` | 0 |
| `GET /api/inventory` | `Inventory` | — | 200 `Inventory` | 0 |
| `GET /api/tools` | `Tools` | — | 200 `[]Tool` | 0 |
| `POST /api/diagnose` | `Diagnose` | `DiagnosticRequest` | 200 `DiagnosticResult` | `maxJSONBody` |
| `POST /api/diagnostics` | `StartDiagnostic` | `DiagnosticRequest` | 202 `Job` | `maxJSONBody` |
| `GET /api/jobs/{id}` | `JobSnapshot` | — | 200 `Job` | 0 |
| `POST /api/jobs/{id}/cancel` | `CancelJob` | empty | 200 `{"cancelled": bool}` | 0 |
| `GET /api/events` | `Subscribe` | — | SSE stream | 0 |
| `GET /api/history` | `History` | — | 200 `[]Job` | 0 |
| `DELETE /api/history` | `ClearHistory` | empty | 204 | 0 |
| `GET /api/settings` | `Settings` | — | 200 `Settings` | 0 |
| `PUT /api/settings` | `SaveSettings` | `Settings` | 204 | `maxJSONBody` |
| `GET /api/wifi` | `CurrentWiFi` | — | 200 `WiFi` | 0 |
| `POST /api/wifi/reveal` | `RevealWiFi` | empty | 200 `{"state","value"}`, `Cache-Control: no-store` | 0 |
| `POST /api/discovery` | `Discover` | `{"cidr","ports"}` | 200 `[]PortResult` | `maxJSONBody` |
| `POST /api/inspect` | `InspectServices` | `{"cidr","ports"}` | 200 `ServiceInspection` | `maxJSONBody` |
| `POST /api/capture/start` | `StartCapture` | `{"address","duration_seconds","packet_limit"}` | 202 empty | `maxJSONBody` |
| `POST /api/capture/stop` | `StopCapture` | empty | 204 | 0 |
| `POST /api/capture/import` | `ImportCapture` | raw file, `application/octet-stream` | 200 `Capture` | `maxCaptureUpload` |
| `GET /api/capture/export?format=pcap\|pcapng` | `ExportCapture` | — | 200 file | 0 |
| `/api/` (catch-all) | none | — | 404 `not_found` | 0 |
| `GET /` | none | — | `http.FileServerFS(opts.Assets)` | 0 |

`Application.Close`, `StartMetrics`, `CanImportCapture` and `CanExportCapture` are lifecycle/preflight helpers and have no separate route. Test-only methods are excluded. `POST /api/discovery` passes the same probe timeout constant the Wails adapter passes (`app.go:180`: 500 ms). The JSON field names are those of `contract`/`bridge.ts`.

Handler rules:

- A handler decodes, calls the application with `r.Context()`, and encodes. It contains no `if` that another adapter also needs.
- JSON request routes require `Content-Type: application/json` (media type compared case-insensitively, parameters ignored), else 415 with code `invalid_input`. `decodeJSON` uses `http.MaxBytesReader`, `DisallowUnknownFields`, and rejects trailing data.
- "empty" bodies are read through `http.MaxBytesReader(w, r.Body, 0)`; any byte gives 413.
- Successful JSON responses are the bare DTO with `Content-Type: application/json; charset=utf-8`. Errors are `{"error": {"code", "message"}}` produced only by `writeError(w, r, err)`, which calls `contract.FromError(err)` and `statusFor(code)`.
- `{id}` must match `^[0-9a-f]{32}$`, else 400 `invalid_input`.
- `POST /api/wifi/reveal`: POST only (mux), session required (W04), `Cache-Control: no-store` and `Pragma: no-cache`, the result is never logged, never placed in an error message, and never stored.

```go
func statusFor(code string) int
```

| Code | Status | Code | Status |
| --- | --- | --- | --- |
| `invalid_input` | 400 | `dependency_missing` | 424 |
| `unauthorized` | 401 | `cancelled` | 499 |
| `forbidden` | 403 | `failed` | 500 |
| `permission_required` | 403 | `unsupported` | 501 |
| `location_consent_required` | 403 | `no_connection` | 502 |
| `not_found` | 404 | `certificate_invalid` | 502 |
| `conflict` | 409 | `timeout` | 504 |
| (unknown) | 500 | | |

Transport-level errors also use the error body: 413 (`invalid_input`, body too large), 415 (`invalid_input`, content type), 429 (`conflict`, W04 caps).

#### 4.4.2 Capture upload and download (files.go)

Web mode replaces the native dialogs (§3.2):

- Import: `os.CreateTemp("", "kali-tools-upload-*")` (mode 0600 by default), `io.Copy` from `http.MaxBytesReader(w, r.Body, maxCaptureUpload)`, close, `a.ImportCapture(r.Context(), path)`, always `os.Remove(path)` in a `defer`. A `*http.MaxBytesError` maps to 413 before the application is called. `maxCaptureUpload = app.MaxCaptureFileSize` (64 MiB).
- Export: `os.MkdirTemp("", "kali-tools-export-*")`, path `<dir>/capture.<format>`, `a.ExportCapture(ctx, path, format)` with a 30 s deadline (as `app.go:270`), open the file, set `Content-Type` (`application/vnd.tcpdump.pcap` for `pcap`, `application/x-pcapng` for `pcapng`; other formats are rejected by the application before any file exists), `Content-Disposition: attachment; filename="capture.<format>"`, `Cache-Control: no-store`, `io.Copy`, then `os.RemoveAll(dir)` in a `defer`. The `format` query value longer than 16 bytes is 400.
- At most one upload or export runs at a time (`maxFileTransfers = 1`); a second gets 429.

`internal/app/limits.go` adds `const MaxCaptureFileSize = capture.MaxFileSize`, so `httpapi` does not import `internal/capture`.

#### 4.4.3 Server-Sent Events (events.go)

```go
const (
    sseKeepAlive    = 15 * time.Second
    sseWriteTimeout = 10 * time.Second
    sseRetryMS      = 3000
)
```

1. Acquire one slot of the W04 subscriber cap; if none is free, 429 `conflict`.
2. Headers: `Content-Type: text/event-stream`, `Cache-Control: no-store`, `X-Accel-Buffering: no`. Write `retry: 3000\n\n` and flush through `http.NewResponseController(w)`.
3. `ch, dropped := a.Subscribe(r.Context())`.
4. If the request has a `Last-Event-ID` header that parses as `uint64`, write `event: resync\ndata: {"reason":"reconnect","last_seq":N}\n\n`.
5. Loop on `r.Context().Done()`, `ch`, and a 15 s ticker:
    - event: if `dropped()` grew since the last write, first write `event: resync\ndata: {"reason":"dropped","dropped":N}\n\n`. Then write `id: <Seq>\nevent: <Topic>\ndata: <json.Marshal(ev)>\n\n`.
    - ticker: write `: keep-alive\n\n`.
    - before each write call `rc.SetWriteDeadline(time.Now().Add(sseWriteTimeout))`; after it call `rc.Flush()`. Any write or flush error ends the handler.
6. A closed `ch` ends the handler.

The SSE event name and `data` come from `contract.EventPayload(ev)` (H06 §4.3), the same function the Wails forwarder uses, so both transports deliver one payload shape to `onEvent` handlers: `job` → `contract.JobEvent{schema_version, seq, job, result?}`, `metrics` → `contract.Metrics`, `capture` → `contract.CaptureEvent{result, error}`. When `EventPayload` returns `ok == false`, the event is skipped (its `Seq` still counts for gap detection on the client). The SSE `id:` is always `ev.Seq`, which is strictly increasing per manager across topics (H02 AC-4).

### 4.5 Capability honesty in server mode (W06)

- `GET /api/capabilities` calls `Application.Capabilities` in the server process on every request. The server never caches or rewrites states, so Wi-Fi location consent (`location_consent_required`, A05) and capture permission (`permission_required`) reflect the account that runs `serve`.
- `GET /api/version` returns `"mode": "server"`. When `transportKind() === "http"`, the shell shows a one-line notice with the new string `serverMode` ("Served by kali-tools serve. Capability states are those of the server process." plus the Portuguese translation).
- Native dialogs are never called in server mode (§4.4.2).

### 4.6 `kali-tools serve` (cmd root)

```go
// cmd/kali-tools/serve.go
func runServe(ctx context.Context, args []string, stdout, stderr io.Writer) int
```

| Flag | Type | Default | Meaning |
| --- | --- | --- | --- |
| `--listen` | string | `127.0.0.1:0` | `host:port`; port 0 picks a free port |
| `--allow-remote` | bool | false | acknowledges non-loopback exposure (W05) |
| `--tls-cert` | string | `""` | PEM certificate path |
| `--tls-key` | string | `""` | PEM private key path |
| `--json` | bool | false | print the startup record as a JSON envelope |
| `--no-color`, `--lang` | | | accepted, ignored |
| `--timeout`, `--detach` | | | rejected with `invalid_input` |

Flow: parse (errors exit 2 without echoing values, as C01) → `bootstrap.New` → `httpapi.New(a, Options{Addr, Assets: frontend.Dist(), AllowRemote, TLSCert, TLSKey, Logger: slog.New(slog.NewTextHandler(stderr, nil))})` → print the startup record → `s.Run(ctx)` → `cleanup()`. Errors exit with `cli.ExitCode(contract.FromError(err).Code)`. A signal-driven clean shutdown exits 0.

Startup record, human mode (stdout):

```text
kali-tools 0.1.0-alpha.1-dev serving on http://127.0.0.1:53111
Open: http://127.0.0.1:53111/#token=<43 characters>
Press Ctrl-C to stop.
```

With TLS, the scheme is `https` and a line `TLS certificate SHA-256: <fingerprint>` follows. JSON mode prints one envelope with `data: {"url","address","tls","remote","certificate_sha256"}`. The URL with the token is written only to stdout, never through the logger.

### 4.7 Remote exposure (W05)

Rules applied by `httpapi.New` (so tests cover them without the CLI):

1. Parse `Addr` with `net.SplitHostPort`. The host `localhost` is rewritten to `127.0.0.1`. An empty host, `0.0.0.0` or `::` is `invalid_input` "bind a specific address".
2. Loopback means the host parses as an IP with `IsLoopback()`. Anything else (other IPs, DNS names) is non-loopback.
3. Non-loopback requires `AllowRemote == true` **and** both `TLSCert` and `TLSKey`, else `invalid_input` "non-loopback listen requires --allow-remote, --tls-cert and --tls-key". The server never starts in that case.
4. Exactly one of `TLSCert`/`TLSKey` is `invalid_input`. TLS uses `tls.LoadX509KeyPair` and `tls.Config{MinVersion: tls.VersionTLS12}`. TLS is optional on loopback.
5. For non-loopback, `runServe` writes this warning to stderr before serving:

```text
WARNING: kali-tools is reachable from the network at https://<address>. Anyone who obtains the URL token can run network diagnostics, discovery and packet capture from this host as the current user, and can reveal the saved Wi-Fi key. Stop the server with Ctrl-C when finished.
```

Why remote exposure is opt-in: HTTP and TCP diagnostics, discovery and Nmap inspection deliberately reach arbitrary hosts chosen by the caller. That is the tool's purpose, so it is not filtered. A reachable server would turn the host into a network probe for anyone holding the token, so loopback is the default and every remote listener needs TLS, an explicit flag and a visible warning. W04 has the threat model.

### 4.8 Playwright against a real server (W02 assert)

- New Go program `tests/e2e/httpserver/main.go` with `//go:build e2e`. Flags `--assets <dir>` (served with `os.DirFS`) and `--listen 127.0.0.1:<port>`. It builds `*app.Application` from hand-written fakes in `tests/e2e/httpserver/fakes.go` (with `fakes_test.go` contract verification) for every driven port H03 declared (in-memory job/settings store; Wi-Fi provider returning "Fixture Network" and `synthetic-test-key`; tool runner with `nmap` as `dependency_missing`; host network returning the fixture inventory; capture provider). It also opens a TCP listener on `127.0.0.1:0` that accepts and closes connections, and writes a one-packet DNS PCAP into `os.MkdirTemp` with gopacket (as `cmd/kali-tools/main_test.go:109-125` does). It starts `httpapi.New(...).Run` and prints, in order:

```text
KT_URL=http://127.0.0.1:<port>/#token=<token>
KT_FIXTURE_TCP=127.0.0.1:<port>
KT_FIXTURE_PCAP=<absolute path>
KT_READY
```

- `frontend/tests/http-server.setup.ts` (Playwright `globalSetup`) spawns `go run -tags e2e ./tests/e2e/httpserver --assets frontend/dist --listen 127.0.0.1:<PW_HTTP_PORT>` from the repository root (`PW_HTTP_PORT` default 4174, validated like `PW_PORT`), reads stdout until `KT_READY` (60 s timeout), and exports the three values as environment variables. `globalTeardown` kills the process.
- `frontend/tests/support.ts` exports a `test` fixture whose `connect(page)` depends on `testInfo.project.name`: `wails-fake` injects the existing fake (moved verbatim from `workflows.spec.ts:3-25`); `http` navigates to `KT_URL` and waits for the connected indicator. Helpers `fillDiagnosticTarget(page)` (http: target `127.0.0.1`, port from `KT_FIXTURE_TCP`) and `importCapture(page)` (http: handles `filechooser` with `KT_FIXTURE_PCAP`).
- Projects in `playwright.config.ts`:

```ts
projects: [
  { name: "wails-fake", use: { baseURL: viteUrl }, grepInvert: /@http-only/ },
  { name: "http", use: { baseURL: httpUrl }, grepInvert: /@wails-only/, fullyParallel: false },
],
workers: process.env.CI ? 1 : undefined,
globalSetup: "./tests/http-server.setup.ts",
```

- Tags: tests that replace `window.go.main.App.*`, use `dataset` hooks, take screenshots, or assert fixture-only interface names get `@wails-only` in the title. "browser without Wails shows explicit limitation" is renamed "browser without a backend shows explicit limitation @wails-only" and expects the new `t.native` text. Shared tests (no tag): diagnostic job and history, Wi-Fi key clearing, packet import and filtering, PT-BR preferences, accessibility.
- New `@http-only` tests: token fragment exchanged and scrubbed from the URL; capture export triggers a browser download named `capture.pcapng`; capability rows equal `GET /api/capabilities`; served `index.html` bytes equal `frontend/dist/index.html`; zero `securitypolicyviolation` events across all pages; job events arrive over SSE.

## 5. Files

Create:

- `frontend/embed.go`, `frontend/embed_test.go`: shared assets.
- `frontend/src/shared/api/types.ts`, `frontend/src/shared/api/transport.ts`: transport seam.
- `frontend/src/app/shell.ts`, `frontend/src/shared/ui/{Stat,Empty,Field}.tsx`, `frontend/src/shared/lib/download.ts`.
- `frontend/src/features/{settings,history,interfaces,overview,diagnostics,wifi,capture,discovery}/<Name>Page.tsx`.
- `frontend/scripts/check-wailsjs-imports.mjs`.
- `frontend/tests/support.ts`, `frontend/tests/http-server.setup.ts`, `frontend/tests/http.spec.ts` (`@http-only` tests).
- `internal/adapter/httpapi/{server,routes,handlers,events,files,encode}.go` and tests (security files in W04).
- `internal/app/limits.go`.
- `tests/acceptance/http_driver_test.go`: HTTP driver for the existing diagnostics, discovery and history scenarios.
- `cmd/kali-tools/serve.go`, `cmd/kali-tools/serve_test.go`.
- `tests/e2e/httpserver/main.go`, `tests/e2e/httpserver/fakes.go` (with `fakes_test.go` contract verification).

Modify:

- `main.go`, `bridge_smoke_test.go`: `frontend.Dist()`.
- `tests/acceptance/acceptance_test.go`: register the HTTP driver alongside CLI, direct and Wails.
- `internal/bootstrap/roots_test.go`: permit the pure shared frontend asset package only in desktop composition (checkout correction discovered by the full suite).
- `frontend/src/shared/api/bridge.ts`, `frontend/src/main.tsx`, `frontend/src/app/App.tsx`, `frontend/src/shared/i18n/i18n.ts` (`native`, `serverMode`, session strings).
- `frontend/package.json` (`lint` script), `frontend/playwright.config.ts`, `frontend/tests/workflows.spec.ts` (imports `test` from `support.ts`; tags).
- `docs/architecture.md`, `README.md`, `docs/development.md`, `CHANGELOG.md`, `PLAN.md`.

Delete: `frontend/src/features/.gitkeep`, `internal/adapter/httpapi/.gitkeep`.

## 6. Implementation steps

Each step keeps `go test ./...` and the full Playwright suite green.

1. **Shared embed (W01).** Add `frontend/embed.go`; switch `main.go` and `bridge_smoke_test.go`; add `TestDistIsTheOnlyEmbed`. Run `go vet -tags desktop .`.
2. **Transport seam (W02).** Move types to `types.ts`; add `transport.ts` with `WailsTransport` only and `selectTransport` returning it when `window.go` exists; `initBridge` in `main.tsx`; `check-wailsjs-imports.mjs`. No behavior change.
3. **App split (W02).** Steps 1–9 of §4.3, one commit each, Playwright after each.
4. **httpapi read routes (W03).** `encode.go`, `statusFor`, `routes.go` with the GET routes, version and healthz; handler tests through `httptest.NewRecorder` on the mux only (no listener, no binary wiring).
5. **httpapi write routes, SSE and files (W03).** Remaining routes, `events.go`, `files.go`, `internal/app/limits.go`; tests.
6. **Server and `serve` (W03 + W04).** `server.go`, the complete W04 middleware and session, `cmd/kali-tools/serve.go`, loopback only. W04 must be Done in this same step; no commit contains `serve` without it.
7. **HttpTransport and Playwright http project (W02).** `HttpTransport`, i18n strings, `tests/e2e/httpserver`, `support.ts`, `http-server.setup.ts`, projects and tags, `http.spec.ts`.
8. **Remote exposure (W05).** §4.7 rules in `New`, TLS loading, fingerprint, warning, tests.
9. **Capability honesty (W06).** `mode` in `/api/version`, `serverMode` notice, capability test.
10. **Docs.** §10 updates.
11. **Evidence.** Write `docs/verification/specs/W01.md`.

## 7. Acceptance criteria

- **AC-1** Given a frontend build, when the desktop asset server and `httpapi` serve every file of `frontend.Dist()`, then the bytes are identical, and `frontend/embed.go` is the only `//go:embed` of the build in the module.
- **AC-2** Given `src/`, then only `src/shared/api/transport.ts` contains `wailsjs`, enforced by `bun run lint`.
- **AC-3** Given the split, then `App.tsx` is under 300 lines and each page is in `features/<capability>/`, with the full Playwright suite green after every split step.
- **AC-4** Given the route table in §4.4.1, then `routes.go` registers exactly those routes, and every `app.Application` method except `Close` is reached by exactly one route.
- **AC-5** Given each error code, when a handler fails with it, then the status matches §4.4.1 and the body is `{"error":{"code","message"}}`.
- **AC-6** Given an SSE client that disconnects before a job's terminal event and reconnects with `Last-Event-ID`, then it receives `resync`, and `GET /api/jobs/{id}` reports the terminal state.
- **AC-7** Given an idle SSE stream, then a `: keep-alive` comment arrives every 15 s (tested with an injected ticker interval), and the handler returns when the request context ends.
- **AC-8** Given a 64 MiB (`maxCaptureUpload`) upload, then it reaches `ImportCapture`; given 64 MiB + 1 byte, then the response is 413 and `ImportCapture` is not called; the temporary file is removed in both cases.
- **AC-9** Given an imported capture, when `GET /api/capture/export?format=pcap` runs, then the response is an attachment named `capture.pcap` whose bytes parse as PCAP with the same packet count, and the temporary directory is removed.
- **AC-10** Given `POST /api/wifi/reveal` with a session, then the response carries `Cache-Control: no-store`, and neither the server log nor history contains the synthetic key.
- **AC-11** Given `--listen 192.0.2.10:8443` without `--allow-remote`, or with it but without TLS files, then `New` fails with `invalid_input` and no listener is bound; with `0.0.0.0:8443`, then `invalid_input`.
- **AC-12** Given a loopback `serve`, then stdout contains exactly one `#token=` URL and stderr (logger) contains no token.
- **AC-13** Given the `http` Playwright project, then the shared workflows pass against the real `httpapi` test server, and the `@http-only` tests pass.
- **AC-14** Given server mode, then capability rows show exactly the states returned by `GET /api/capabilities` of the server process, and the server-mode notice is visible.
- **AC-15** Given a non-loopback TLS listener, then the stderr warning of §4.7 is printed and the URL scheme is `https`.
- **AC-16** [native] Given Windows 11, when `kali-tools serve` runs as a standard user and as a different account (`runas /user:<other>`), then the Wi-Fi and capture capability states shown in the browser are recorded for both in `docs/verification/`, and each differs from the desktop only through an explicit state.
- **AC-17** [native] Given the desktop build, then it opens no TCP listener (`netstat -ano` filtered by the desktop PID shows no LISTENING socket).

## 8. Tests

| Test | Package | Asserts | AC |
| --- | --- | --- | --- |
| `TestDistIsTheOnlyEmbed` | `frontend` | walks module `.go` files; only `frontend/embed.go` embeds `dist` | AC-1 |
| `TestAssetsServedByteIdentical` | `httpapi` | `fstest.MapFS` files served byte-identical, with W04 headers | AC-1 |
| `check-wailsjs-imports.mjs` | frontend lint | exit 1 when a second file imports `wailsjs` | AC-2 |
| `TestRoutesMatchApplication` | `httpapi` | route list equals §4.4.1; reflection over `*app.Application` methods minus `Close` maps 1:1 | AC-4 |
| `TestStatusForEveryCode` | `httpapi` | 14 codes + unknown | AC-5 |
| `TestJSONDecodeLimits` | `httpapi` | `maxJSONBody` accepted, +1 → 413; unknown field, trailing data, wrong content type | AC-5 |
| `TestSSEResyncRecoversTerminalState` | `httpapi` | AC-6 flow with a blocking fake diagnostic | AC-6 |
| `TestSSEKeepAliveAndShutdown` | `httpapi` | keep-alive comment; handler exits on context end | AC-7 |
| `TestCaptureUploadLimit` | `httpapi` | limit and limit+1; temp file removed | AC-8 |
| `TestCaptureExportDownload` | `httpapi` | headers, PCAP bytes, temp dir removed; second concurrent transfer → 429 | AC-9 |
| `TestRevealNoStoreAndNotLogged` | `httpapi` | headers; log buffer and history lack `synthetic-test-key` | AC-10 |
| `TestRemoteRequiresOptInAndTLS` | `httpapi` | table over §4.7 rules | AC-11 |
| `TestServePrintsTokenOnlyToStdout` | `cmd/kali-tools` | AC-12 | AC-12 |
| `TestRemoteWarningAndHTTPS` | `cmd/kali-tools` | test certificate generated into `t.TempDir()`; warning text on stderr; `https` URL | AC-15 |
| Playwright shared workflows, project `http` | frontend | AC-13 | AC-13 |
| `token fragment is exchanged and scrubbed @http-only` | frontend | cookie set; `location.hash === ""` | AC-13 |
| `capture export downloads a file @http-only` | frontend | download `capture.pcapng` | AC-9, AC-13 |
| `capability states come from the server @http-only` | frontend | rows equal `/api/capabilities`; notice visible | AC-14 |
| `served index matches build @http-only` | frontend | bytes equal | AC-1 |
| `no CSP violations @http-only` | frontend | zero `securitypolicyviolation` events on every page | AC-13 |

For `TestRemoteWarningAndHTTPS`, binding a real non-loopback address is not portable in CI. Test the validation path through `httpapi.New` with `192.0.2.10:0` (expects the bind to fail with `failed` after validation passes), and test the warning and `https` scheme through a `runServe` seam that takes a `listen` function; this seam is the only test hook allowed.

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
go test -race -count=1 ./internal/adapter/httpapi/... ./cmd/kali-tools/... ./frontend/...
go vet -tags e2e ./tests/e2e/...
cd frontend && bunx playwright test --project=http && bunx playwright test --project=wails-fake
go run ./cmd/kali-tools serve            # open the printed URL manually; Ctrl-C exits 0
go run ./cmd/kali-tools serve --listen 192.0.2.10:8443; echo "exit=$?"   # expect exit=2
node scripts/markdown.mjs
```

## 10. Documentation updates

- `docs/architecture.md`: §2 map (`frontend/embed.go`, `httpapi` files, `tests/e2e`), §6.1 assets, §6.2 rewritten from "planned" to the actual routes, SSE contract, remote rules and capability honesty; D7 marked resolved.
- `README.md`: a "Use it in a browser" section with `kali-tools serve`, the loopback default, the token URL and the remote opt-in warning.
- `docs/development.md`: Playwright projects, `PW_HTTP_PORT`, the e2e server.
- `docs/specs/README.md` §6.3 wording: "`shared/api/` is the only directory that talks to Go; feature code imports only `bridge.ts`" (reviewer applies; it is a convention text change).
- `CHANGELOG.md` `[Unreleased]`, `PLAN.md` W01/W02/W03/W05/W06 boxes with evidence links.

## 11. Risks and rollback

- **Security regression.** Mitigated by W04 being part of step 6 and by its blocking tests. Rollback: revert step 6; steps 1–5 add no listener.
- **SSE shape divergence between Wails and HTTP.** Caught by the shared Playwright workflows on both projects. If they diverge, fix the H05 forwarder, not the frontend handlers.
- **Large uploads in memory.** Uploads stream to a temp file; the browser holds at most one 64 MiB export blob.
- **CSP blocks a library style.** If the "no CSP violations" test fails because a dependency injects `<style>` elements, add `'unsafe-inline'` to `style-src` only, record it in W04 evidence, never to `script-src`.
- **Binary size.** The CLI now embeds the frontend (a few MiB). Accepted; Docker builds without a frontend fail `serve` explicitly.
- Rollback per numbered step; each is one commit, e.g. `feat(httpapi): routes and SSE (W03)`.

## 12. Quality and delivery assertions

### 12.1 Applicable quality gates

| Gate | Applies | Threshold for this spec |
| --- | --- | --- |
| QG-1, QG-2, QG-3, QG-4 | yes | as in README §5.1 |
| QG-5 | yes | `go test -race ./internal/adapter/httpapi/...` has no race report (SSE and caps are concurrent) |
| QG-6 | yes | `internal/adapter/httpapi` ≥ 85 % statements (tightened: security-critical adapter); `frontend` package 100 % (one function); `cmd/kali-tools` not lower than before (record both) |
| QG-7 | yes | as in README §5.1 |
| QG-8 | yes | Playwright count rises: 19 `wails-fake` tests kept, plus the shared tests in `http`, plus 6 `@http-only` tests |
| QG-9 | yes | binding drift empty after step 2 (no Go binding change expected) |
| QG-10 | yes | `httpapi` imports no `internal/adapter/{storage,tools,probe,platform,cli,wailsui}` and no `internal/{capture,network,wifi}` |
| QG-11 | yes | AC-10, AC-12 tests and W04 token tests |
| QG-12 | yes | `maxJSONBody`, `maxCaptureUpload`, `maxFileTransfers`, `sseKeepAlive`, `sseWriteTimeout`, tracked job IDs (64) each tested at limit and limit+1 |
| QG-13, QG-14, QG-15 | yes | as in README §5.1 |

### 12.2 Delivery assertions

| DA | Assertion | Command / test / artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | One embed serves both interfaces | `go test ./frontend -run TestDistIsTheOnlyEmbed -count=1` and `go test ./internal/adapter/httpapi -run TestAssetsServedByteIdentical -count=1` | both pass | CI |
| DA-2 | Feature code never imports `wailsjs` | `cd frontend && node scripts/check-wailsjs-imports.mjs` | exit 0; `grep -rl wailsjs frontend/src` prints only `transport.ts` | CI |
| DA-3 | App is split | `wc -l frontend/src/app/App.tsx`; `ls frontend/src/features` | under 300 lines; 8 feature folders | reviewer |
| DA-4 | Routes mirror the application 1:1 | `go test ./internal/adapter/httpapi -run TestRoutesMatchApplication -count=1` | pass | CI |
| DA-5 | Single error mapping | `grep -c "func statusFor" -r internal/adapter/httpapi` and `TestStatusForEveryCode` | exactly 1 definition; test passes | CI, reviewer |
| DA-6 | SSE never loses a terminal state | `go test -race ./internal/adapter/httpapi -run 'TestSSE' -count=1` | pass | CI |
| DA-7 | Upload/download bounded and cleaned | `go test ./internal/adapter/httpapi -run 'TestCapture' -count=1` | pass; no `kali-tools-upload-*`/`kali-tools-export-*` left in `os.TempDir()` by the test | CI |
| DA-8 | Remote is opt-in with TLS | `go test ./internal/adapter/httpapi -run TestRemoteRequiresOptInAndTLS -count=1`; `go run ./cmd/kali-tools serve --listen 192.0.2.10:8443` | test passes; command exits 2 | CI |
| DA-9 | Same workflows on both transports | `cd frontend && bunx playwright test` | both projects pass; shared test titles appear in both project reports | CI |
| DA-10 | Capability honesty | Playwright `capability states come from the server @http-only` | pass | CI |
| DA-11 | Server account states recorded | AC-16 procedure | recorded in `docs/verification/` with OS build and accounts | [native] |
| DA-12 | Desktop opens no listener | AC-17 procedure | no LISTENING socket for the desktop PID | [native] |

Evidence record: `docs/verification/specs/W01.md`.
