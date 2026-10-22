# kali-tools architecture

Status: binding reference for agents and contributors. Last structural review: 2026-10-21.

Read this before adding a package, moving a file, adding an interface or wiring a new interface (GUI, web, CLI). The reasoning and sources are in [architecture research](architecture-research.md). Tool and platform facts are in [tools research](tools-research.md). Delivery status is in [PLAN.md](../PLAN.md).

## 1. System in one picture

kali-tools is one Go core with three driving interfaces. It uses the hexagonal (ports and adapters) style: dependencies point inward, toward the capability packages.

```text
          DRIVING SIDE (primary actors)                      DRIVEN SIDE (secondary actors)
 ┌────────────────────────────────────────┐            ┌──────────────────────────────────────┐
 │ Wails desktop  main.go + wailsui        │            │ adapter/storage   SQLite (modernc)   │
 │ Web server     adapter/httpapi         │──calls──►  │ adapter/tools     Nmap/TShark exec   │
 │ CLI            cmd/kali-tools          │   ▲        │ adapter/probe     Windows ICMP       │
 └────────────────────────────────────────┘   │        │ adapter/platform  routes, counters   │
                 │                            │        └──────────────────────────────────────┘
                 ▼                            │                         ▲ implements ports
 ┌──────────────────────────────────────────────────────────────────────┴───┐
 │ internal/app        use cases, job lifecycle, history (declares ports)   │
 │ internal/job        job manager + bounded event fan-out                  │
 │ internal/network    interfaces, diagnostics, discovery, rates            │
 │ internal/capture    capture sessions, PCAP/PCAPNG, decoding              │
 │ internal/wifi       Wi-Fi metadata and explicit secret retrieval         │
 │ internal/domain     shared kernel: errors, capability, versions          │
 └──────────────────────────────────────────────────────────────────────────┘
```

The three interfaces must show the same capabilities, results and errors. All three receive the same application value and differ only in transport and presentation.

H04 implements that shared value as `app.Application`, one concrete struct with private dependencies and methods for every operation. Desktop bindings and CLI commands call it directly; job state and provider details are not exposed as mutable fields. `bootstrap.New` returns the application plus its cleanup without a transitional Service wrapper.

## 2. Repository map

```text
kali-tools/
├── main.go, doc.go                 Wails composition root (must stay at root; see §6.1)
├── bridge_smoke_test.go            native Wails bridge smoke test (desktop tag)
├── wails.json                      Wails v2 project file
├── go.mod, go.sum                  single Go module github.com/AlexGalhardo/kali-tools
│
├── cmd/
│   ├── kali-tools/                 CLI composition root; future `serve` subcommand starts the web server
│   └── lab-fixture/                deterministic HTTP/DNS/TCP fixture server for the Docker lab
│
├── internal/                       all Go logic; not importable by other modules
│   ├── domain/                     shared kernel: Error{Code,Message}, Capability, Version, SchemaVersion
│   ├── app/                        application layer: Application (jobs, history, discovery, Nmap inspection), capability registry
│   ├── job/                        manager-owned job lifecycle, bounded subscriptions and ordered event fan-out
│   ├── network/                    capability: interfaces, DNS/TCP/HTTP/TLS/ping/trace diagnostics, TCP discovery, counter deltas
│   ├── capture/                    capability: live capture (Windows raw IPv4), PCAP/PCAPNG import/export, gopacket decoding
│   ├── wifi/                       capability: current Wi-Fi metadata and explicit key reveal (Windows)
│   ├── adapter/                    all adapters, named by technology
│   │   ├── contract/               shared frontend DTOs, explicit mappers and safe error mapping
│   │   ├── storage/                driven: SQLite store, migrations, settings, retention
│   │   ├── tools/                  driven: optional executables (Nmap/TShark), bounded process runner, Nmap XML parser
│   │   ├── probe/                  driven: native Windows ICMP echo/traceroute
│   │   ├── platform/               driven: OS routes, DNS servers and interface counters (windows/linux/other)
│   │   ├── wailsui/                Wails bindings, native dialogs and event forwarder
│   │   ├── httpapi/                REST, SSE, security middleware and bounded file transfers
│   │   └── cli/                    CLI parsing, commands, output and bounded capture wait
│   ├── bootstrap/                  shared wiring used by every composition root
│   └── porttest/                   shared driven-port suites; fake/ contains contract-tested in-memory adapters
│
├── frontend/                       React + TypeScript + Base UI + Tailwind v4 (Vite, Bun)
│   ├── index.html                  entry HTML (loads src/main.tsx)
│   ├── src/
│   │   ├── main.tsx                React entry point
│   │   ├── app/App.tsx             application shell, routes/pages (to be split into features/)
│   │   ├── features/               (empty) target home of per-capability UI modules
│   │   ├── shared/api/bridge.ts    THE transport seam: typed calls to Go (Wails or authenticated HTTP+SSE)
│   │   ├── shared/i18n/i18n.ts     English and PT-BR catalogs
│   │   ├── shared/ui/Icon.tsx      shared presentational components
│   │   └── styles/style.css        design tokens and styles (Tailwind source root is src/)
│   ├── tests/workflows.spec.ts     Playwright workflows with contract-faithful fakes
│   ├── wailsjs/                    GENERATED Wails bindings; never edit by hand
│   └── dist/                       build output (embedded by main.go; only .gitkeep tracked)
│
├── tests/
│   ├── integration/                Docker lab tests (`integration` build tag)
│   ├── acceptance/                 direct, CLI and headless Wails acceptance drivers and shared specifications
│   └── fixtures/                   (empty) synthetic PCAP and tool-output fixtures
│
├── scripts/                        Node task runners (task, lab, package, release, notices, markdown, setup)
│   └── setup/                      per-OS setup entry points, with and without Docker (bash)
├── docker/                         CLI lab Dockerfile and Compose file
├── build/                          Wails platform assets (icons, manifests); release staging is ignored
├── docs/
│   ├── architecture.md             this file
│   ├── architecture-research.md    Clean/Hexagonal Go research and decisions
│   ├── tools-research.md           tool landscape, licenses, platform limits
│   ├── brainstorming.md            owner decisions
│   ├── dependencies.md             dependency and license inventory
│   ├── development.md              developer workflow
│   ├── adr/                        architecture decision records
│   ├── ci/                         CI/CD operating guide
│   ├── releases/, verification/    release notes and recorded evidence
│   └── licenses/                   third-party license texts
├── .agents/skills/                 project skills (impeccable, karpathy-guidelines, ponytail*, book-to-skill,
│                                   clean-architecture-in-practice); revisions in sources.json
├── .github/workflows/              active, pinned CI
└── PLAN.md, AGENTS.md, README.md, CHANGELOG.md, CONTRIBUTING.md, SECURITY.md, DESIGN.md, PRODUCT.md
```

Folders marked *(empty)* contain only `.gitkeep`. They mark target locations from PLAN.md phase H. Do not put placeholder Go files in them; add code only as part of the PLAN task that fills them.

## 3. Layers and responsibilities

| Layer | Packages | May import | Must not import |
| --- | --- | --- | --- |
| Shared kernel | `internal/domain` | stdlib | anything in the module |
| Capabilities (inner hexagon) | `internal/network`, `internal/capture`, `internal/wifi` | `domain`, stdlib, pure libraries (gopacket) | `adapter/*`, `app`, Wails, `database/sql` |
| Application | `internal/app`, `internal/job` | `domain`, capability packages, `job` | `adapter/*`, Wails, `net/http`, `os/exec`, `database/sql` |
| Driven adapters | `internal/adapter/{storage,tools,probe,platform}` | `domain`, capability packages, OS/driver libraries | driving adapters, `cmd`, `app` (except for types) |
| Driving adapters | `internal/adapter/{wailsui,cli,httpapi}` | `app`, `domain`, `adapter/contract` DTO types | driven adapters directly (they go through `app`) |
| Frontend contract | `internal/adapter/contract` | `domain`, `app`, `job`, capability packages | other adapters, Wails, `net/http`, `os/exec`, `database/sql` |
| Composition roots | `main.go`, `cmd/*`, `internal/bootstrap` | everything | nothing imports them |

The matrix is enforced by depguard rules in `.golangci.yml` (rule names `domain`, `capabilities`, `application`, `driven-adapters`, `driving-adapters`, `wails`, `bootstrap`, `porttest`); `node scripts/task.mjs arch` proves each rule fires. External domain tests may import their own exact package; production self-imports are Go import cycles.

Rules that hold in every layer, from AGENTS.md and PLAN.md:

- No shell command concatenation. Executables run with an argument array, a deadline, output caps and descendant cancellation.
- Network operations, parser input, event queues and subprocess output are bounded.
- Wi-Fi secrets and packet payloads never enter history, logs or the database.
- Capability limitations are explicit states (`available`, `dependency_missing`, `permission_required`, `unsupported`, `failed`, and `location_consent_required`). Never fake a success.
- Platform code uses build constraints and an explicit unsupported fallback.

## 4. Ports and adapters

### 4.1 Where interfaces live

A port is a small Go interface declared **in the package that consumes it**. Adapters return concrete types and never import the port's package just to "implement" it. Add an interface only at a real boundary (a driven dependency, or a test stand-in for one). Inside a layer, call functions directly. See [architecture research](architecture-research.md) §3.2 and §4.6.

### 4.2 Current driven ports and implementations

| Consumer | Port (current form) | Implementation |
| --- | --- | --- |
| `app.Application` | `JobStore`, `SettingsStore` | `*storage.Store`, hand-written in-memory test fakes |
| `app.Application` | `job.Manager` with injected clock function | `internal/job`; `bootstrap.Config.Now` or UTC system clock |
| `app.Application` | `HostNetwork` | `platform.Host` |
| `app.Application` | `ToolRunner` (including service inspection) | `tools.Runner` |
| `app.Application` | `WiFiProvider` | `wifi.Native` |
| `app.Application` | `CaptureProvider` alias of `capture.LiveProvider` | `capture.Native` selected by build constraints |
| `network.Diagnostics` | `network.Prober` interface (injected by `bootstrap.New` for both roots) | `probe.ICMP` in `adapter/probe` |

### 4.3 Implemented driven ports (H03)

| Port | Declared in | Implementations |
| --- | --- | --- |
| `JobStore`, `SettingsStore` — done 2026-10-21 | `app` | `adapter/storage`, in-memory fake |
| `Prober` (ICMP echo/trace) — **done 2026-10-21** | `network` | `probe.ICMP` (windows), unsupported fallback |
| `HostNetwork` (routes, DNS servers, counters) — done 2026-10-21 | `app` | `platform.Host` |
| `ToolRunner` (includes `ServiceInspector`) — done 2026-10-21 | `app` | `tools.Runner` |
| `WiFiProvider` — done 2026-10-21 | `app` | `wifi.Native` |
| `CaptureProvider` — done 2026-10-21 | `capture.LiveProvider`, alias in `app` | `capture.Native` returns a live result and retained artifact |

H03 compile-time assertions live only in `internal/bootstrap/assert.go`. No inner package imports an adapter, checked across all GOOS source files by `TestCoreDoesNotImportAdapters`. `app.New(Deps)` requires job and settings stores; bootstrap owns SQLite and returns a `sync.Once` cleanup that closes the service before the store. Application-owned settings, host and tool types retain their existing JSON tags; tool XML tags remain in the application model; contract inspection DTOs carry JSON fields only. H07 adds seven reusable suites in `internal/porttest` and in-memory implementations in `internal/porttest/fake`. The ICMP port adapter validates targets with `network.ValidateHost` before DNS/native delegation; the real adapter now runs the same `ProberContract` as the fake. Verification and native qualification status are recorded in [H07 evidence](verification/specs/H07.md).

H07 suite wiring:

| Suite | Real implementation | Shared fake |
| --- | --- | --- |
| `JobStoreContract`, `SettingsStoreContract` | `storage.Store` | `fake.Store` |
| `HostNetworkContract` | `platform.Host` | `fake.Host` |
| `ToolRunnerContract` | `tools.Runner` | `fake.Tools` |
| `WiFiProviderContract` | `wifi.Native` | `fake.WiFi` |
| `ProberContract` | `probe.ICMP` | `fake.Prober` |
| `LiveProviderContract` | `capture.Native` | `fake.LiveCapture` |

Only test files outside `internal/porttest` import the suites or fakes. `tests/acceptance` creates a fresh application per scenario; diagnostics, discovery and history run through direct, CLI and Wails drivers after C01 step 4. Bootstrap still wires real SQLite in memory for interface acceptance; persistence remains covered by real-file storage/bootstrap tests. The HTTP driver remains W01. No Wails runtime or native dialogs are launched by these drivers.

H07 native OS-port suites passed on Windows build 26300 with active Wi-Fi and administrator capture on 2026-10-21. The capture suite selects an active non-loopback interface; disconnected interfaces may retain addresses that Windows cannot bind. This qualifies H07 OS-port data branches only, not desktop IPC or other platform acceptance.

## 5. Runtime flows

### 5.1 Starting a diagnostic job (current desktop path)

```text
React (bridge.ts) ──StartDiagnostic(req)──► wailsui.Bindings (Wails binding)
   wailsui.Bindings ──► app.Application.StartDiagnostic(ctx, req)
                ├─ network.Validate(req)              domain validation, typed errors
                └─ job.Manager.Start                  manager-owned context, maximum 8 active jobs
                     ├─ OnChange ──► storage.PutJob    bounded writes, no secrets/payloads
                     └─ Application.Diagnose  injected Prober, bounded deadline, cancellable
                          └─ Subscribe ──► wailsui forwarder ──► runtime.EventsEmit("job") ──► React
CancelJob(id) ──► app.Application.CancelJob(ctx, id) ──► job.Manager.Cancel ──► job state cancelled
```

### 5.2 Same flow in the target architecture (all interfaces)

```text
Wails binding ─┐
HTTP handler  ─┼─► app.Application.StartDiagnostic(ctx, req) ─► job.Manager.Start(...)
CLI command   ─┘                                                     │
                                                                     ▼ bounded fan-out
Wails forwarder (EventsEmit "job") ◄── job.Manager.Subscribe ──► SSE /api/events ──► CLI stdout
```

The job context derives from the manager (application lifetime), never from a request. Each event carries a monotonic `seq`, so a reconnecting client can call `Snapshot` and resynchronize.

### 5.3 Events

| Event | Producer today | Payload | Rule |
| --- | --- | --- | --- |
| `job` | `job.Manager` via `adapter/wailsui` | `{schema_version, seq, job, result?}` | Stable ID and state machine `queued → running → completed/failed/cancelled` |
| `metrics` | `app.Application` metrics loop | interface rates | Samples every 2 s; reset never produces a negative spike |
| `capture` | `app.Application` capture session | `{result, error}` | Bounded packet count, retained bytes and duration; no capture payloads in history |

H02 moves all three streams through `job.Manager`. `Subscribe(ctx)` uses a default buffer of 256 (maximum 4096) and reports dropped events. Each manager assigns monotonically increasing `seq` values; non-terminal delivery drops without blocking, while terminal job and capture events share a maximum 2 s delivery wait per publication. `Snapshot` and `Wait` retain at most 256 finished statuses. `Application.Close` stops capture and metrics and closes the manager; bootstrap then closes storage.

## 6. Interfaces (driving adapters)

### 6.1 Desktop (Wails v2), the primary interface

- `frontend.Dist()` owns the sole embed of `frontend/dist`; `main.go` (build tags `desktop || bindings`) obtains `app.Application` from bootstrap and binds `wailsui.Bindings` to `wails.Run`.
- `run(ctx, args, getenv, stdout, stderr) int` owns composition and deferred cleanup. `OnStartup` and `OnShutdown` receive `b.Startup` and `b.Shutdown` directly, so Wails exempts those lifecycle methods from its generated API. The binding adapter owns no storage or application cleanup.
- `internal/adapter/wailsui` owns the 20 frontend methods, native dialogs and cancellable forwarder. `Runtime` functions are injected for headless tests. Startup subscribes before opting into metrics; shutdown waits at most 3 s for its forwarder. The old root `app.go` is deleted.
- **Why it stays at the repository root:**
  - Wails v2 builds the project-root package next to `wails.json`.
  - Wails keeps its project root; the shared `frontend` Go package owns the embedded build used by both delivery modes.
- The generated namespace is `frontend/wailsjs/go/wailsui/Bindings`, following the bound receiver's package. `transport.ts`, Playwright fakes and native smoke calls use that namespace. The binding drift task regenerates it and rejects differences from Git.
- All bound request/result structs belong to `adapter/contract`; explicit mappers preserve JSON field order and nil slices while cloning mutable data. `fail` delegates error mapping to `contract.FromError`, preserving rejected-promise strings. Events use `contract.EventPayload`.
- Metrics and capture lifecycle live in `internal/app` after H02. Wails retains native dialogs and a payload translator; it forwards a manager subscription without owning producer state.

### 6.2 Web server: same UI in a browser

- **Delivery:** `kali-tools serve` subcommand of the CLI binary. One binary serves headless SSH hosts and local browsers.
- **Package:** `internal/adapter/httpapi`. Routes use the shared `adapter/contract` DTOs, `EventPayload` and `FromError`.
  - REST endpoints mirror the binding methods one-to-one.
  - Server-Sent Events at `/api/events` carry `job`, `metrics` and `capture` events.
  - The frontend `embed.FS` is shared with the desktop build.
- **Frontend:** `src/shared/api/bridge.ts` selects its transport at runtime: Wails bindings when `window.go` exists, otherwise `fetch` + `EventSource`. Feature code never imports `wailsjs` directly.
- **Security is mandatory before the first release that contains `serve`** (research §8):
  - loopback bind by default;
  - Host allowlist;
  - a per-launch token, carried by a URL fragment and exchanged for an HttpOnly SameSite=Strict cookie;
  - `http.NewCrossOriginProtection`;
  - no CORS;
  - request size and time limits;
  - caps on jobs and subscribers;
  - CSP.
  - Exposing the server on the LAN requires an explicit flag plus TLS plus a warning.
- **Capability honesty:** a server running as another user or as a service may lack Wi-Fi location consent or capture permission. It must report those states, not fail silently.

The only embedded asset source is `frontend/embed.go`; `Dist()` serves the same bytes to Wails and HTTP. All API routes except `POST /api/session` require cookie or bearer authentication. Health and static assets remain public. Lifecycle/preflight helpers have no separate routes. SSE subscribes before flushing, emits retry/sequence/resync and closes on cancellation; terminal job state is recoverable from `/api/jobs/{id}`. Capture import/export accepts bytes and format, never server paths.

Middleware order is recovery ? redacted access log ? security headers ? Host allowlist ? CrossOriginProtection ? Fetch Metadata ? in-flight cap ? session ? mux. Limits: JSON 64 KiB, session JSON 1 KiB, upload 64 MiB, one file transfer, eight SSE clients, 16 in-flight API requests, five-second header/shutdown deadlines, 120-second idle/upload bounds, 15-second SSE keepalive and ten-second SSE write deadline. CSP allows only same-origin scripts/styles; no inline exemption was needed. Remote listeners require specific binding plus explicit opt-in and TLS 1.2 minimum. See [W04 threat model](specs/W04-web-security.md) and [verification](verification/specs/W04.md). Native account/browser acceptance remains open.

### 6.3 CLI

C01 provides the command tree, strict flags, stable exit codes, human output by default and explicit versioned JSON. Result labels, states and warnings support English and PT-BR; help stays English. A bounded progress subscriber forwards job-manager events to stderr only in human mode, tracking active jobs for cancellation. Diagnostics subscribe before StartDiagnostic, cancel through CancelJob and await terminal state with a three-second grace. Capture cancellation returns partial results without exporting. The root handles interrupt/SIGTERM, restores default handling for a second signal, forwards NO_COLOR and opens the shared persistent SQLite graph only when NeedsApp requires it. Jobs/capture artifacts remain process-local; retained history/settings survive invocations. Golden fixtures, JSON path/type fixtures and [the CLI contract](cli.md) freeze presentation and compatibility. Native SSH acceptance remains pending. The binary root dispatches serve to the HTTP composition in cmd/kali-tools/serve.go.

- `cmd/kali-tools/main.go` only composes and cleans up. `internal/adapter/cli.Run(ctx, application, args, stdout, stderr) int` owns stdlib flag parsing and the commands: `version`, `doctor`, `interfaces`, `inventory`, `diagnose`, `discover`/`inspect`, `wifi`, `capture`/`import`.
- All capability commands route through `app.Application`. CLI capture subscribes before starting and waits at most the requested duration plus 10 s; cancellation stops capture and waits for its typed completion. Import/export use application-managed artifacts and the shared 30 s export bound. Discovery and inspection keep count-only history in the CLI's in-memory store.
- The root exposes `run(ctx, args, getenv, stdout, stderr) int` and obtains its application from bootstrap before calling the adapter. It uses shared persistent per-user storage after C01. Environment reads remain in roots; local FlagSet parsing is permitted only in the CLI driving adapter. Byte-exact output fixtures were committed before extraction and moved unchanged.
- `--json` output includes `schema_version`. Invalid input exits with a stable non-zero code.
- C01 implements the command tree and headless ergonomics; W01 adds `serve` with its mandatory W04 security. The H05 extraction adds no command or flag.

## 7. How to extend

### 7.1 Add a new tool or capability (for example TLS audit)

1. Check scope and licenses in [tools research](tools-research.md). Offensive categories are out of scope.
2. Create `internal/<capability>/` with domain types, validating constructors, limits and pure logic. Import only stdlib, `domain` and pure libraries.
3. If it needs the OS, a process or a library with side effects, declare a small port in the consumer and implement it in `internal/adapter/<technology>/`, with build-tagged files and an unsupported fallback.
4. Add a use case to `internal/app`. Register its capability state.
5. Expose it in **all three** interfaces: a Wails binding, an HTTP route and a CLI command. Each must stay thin.
6. Add the UI under `frontend/src/features/<capability>/`, calling only `shared/api/bridge.ts`, with translations in both catalogs.
7. Tests:
   - domain table tests;
   - use-case tests with fakes;
   - a contract suite for the new port;
   - an acceptance spec run through each driving adapter;
   - Playwright for the UI.
8. Update this file, PLAN.md, `docs/dependencies.md` and, for a significant trade-off, an ADR.

### 7.2 Add a platform implementation

Add `<name>_<goos>.go` beside the existing files, keeping the same constructor and method set. Keep the `_other.go` fallback returning an explicit unsupported error. Run `GOOS=windows|linux|darwin go vet ./...`. Never mark native acceptance from cross-compilation.

### 7.3 Moving or renaming packages

Use `git mv`, then rewrite import paths and run `gofmt`. Run `go vet ./...` for all three GOOS targets plus `go vet -tags desktop .`, then `go test ./...`. Update `.golangci.yml`, `scripts/task.mjs`, `docker/Dockerfile` and the docs that mention the old path. Historical evidence in `docs/verification/` is a record and keeps its original paths.

## 8. Known deviations (debt to remove)

| # | Deviation | Why it matters | PLAN task |
| --- | --- | --- | --- |
| D1 | ~~`internal/network` imports `internal/adapter/probe`~~ — resolved 2026-10-21 by the `network.Prober` port | — | H03 (done) |
| D2 | Resolved 2026-10-21: all desktop and CLI operations route through `app.Application`; driving adapters are extracted into `adapter/cli` and `adapter/wailsui` | Native acceptance remains pending | H04, H05 |
| D3 | Resolved 2026-10-21: `app.Application` owns metrics and capture; all producers publish through `job.Manager` | Native desktop acceptance remains pending under the owner's headless workflow | H02 |
| D4 | Resolved 2026-10-21: CLI and Wails map application values to separate `adapter/contract` DTOs; one safe `FromError` preserves stable codes | JSON, redaction, signature and headless workflow tests pass; native IPC acceptance pending | H06 |
| D5 | Resolved: storage ports have shared suites run against SQLite and in-memory fakes; H07 also wires the ICMP regression after target validation | Native OS-port data-branch qualification remains pending | H03, H07 |
| D6 | `capture`, `network`, `wifi` mix domain rules with OS or network I/O in one package | Harder to test without hardware | H03 (split only where a port is needed) |
| D7 | Resolved: App is below 300 lines; eight feature pages own local state | Verified by typecheck and 30 browser tests | W01/W02 |
| D8 | Resolved by eight depguard rules and deliberate violations via `node scripts/task.mjs arch` | Hosted proof recorded in H08 evidence | H08 |

## 9. Verification commands

```bash
go vet ./... && go vet -tags desktop .
GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...                      # add -race where the toolchain supports it
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
node scripts/task.mjs fuzz         # parser fuzz smoke
node scripts/task.mjs arch         # prove eight dependency rules reject violations
bash scripts/setup/setup-linux-docker.sh test   # Docker CLI lab (or the Windows/macOS variant)
```

Formatting is enforced by the Lefthook pre-commit (`golangci-lint fmt`: gofmt -s + goimports with local prefix, re-staged automatically), followed by `golangci-lint run` and `go vet`; pre-push runs `golangci-lint fmt --diff`, tests and govulncheck.

The structural reorganization of 2026-10-21 was verified with:

- `go vet` (default, desktop tag, linux, darwin) and `go test ./...` on Windows;
- frontend typecheck, lint and build;
- 17/17 Playwright workflows;
- `bash -n` on the moved setup scripts.

H06 CLI output uses the same contract DTOs and `contract.FromError`; exit status 2 remains reserved for `invalid_input`. Unknown internal errors become `failed: Operation failed`; authored messages are bounded to 512 UTF-8 bytes with control characters removed. `WiFiSecret` exposes its value only through explicit JSON reveal and redacts Go formatting.
