# kali-tools architecture

Status: binding reference for agents and contributors. Last structural review: 2026-10-21.

Read this before adding a package, moving a file, adding an interface or wiring a new interface (GUI, web, CLI). The reasoning and sources are in [architecture research](architecture-research.md). Tool and platform facts are in [tools research](tools-research.md). Delivery status is in [PLAN.md](../PLAN.md).

## 1. System in one picture

kali-tools is one Go core with three driving interfaces. It uses the hexagonal (ports and adapters) style: dependencies point inward, toward the capability packages.

```text
          DRIVING SIDE (primary actors)                      DRIVEN SIDE (secondary actors)
 ┌────────────────────────────────────────┐            ┌──────────────────────────────────────┐
 │ Wails desktop  main.go + app.go        │            │ adapter/storage   SQLite (modernc)   │
 │ Web server     adapter/httpapi (plan)  │──calls──►  │ adapter/tools     Nmap/TShark exec   │
 │ CLI            cmd/kali-tools          │   ▲        │ adapter/probe     Windows ICMP       │
 └────────────────────────────────────────┘   │        │ adapter/platform  routes, counters   │
                 │                            │        └──────────────────────────────────────┘
                 ▼                            │                         ▲ implements ports
 ┌──────────────────────────────────────────────────────────────────────┴───┐
 │ internal/app        use cases, job lifecycle, history (declares ports)   │
 │ internal/job        job manager + bounded event fan-out (plan)           │
 │ internal/network    interfaces, diagnostics, discovery, rates            │
 │ internal/capture    capture sessions, PCAP/PCAPNG, decoding              │
 │ internal/wifi       Wi-Fi metadata and explicit secret retrieval         │
 │ internal/domain     shared kernel: errors, capability, versions          │
 └──────────────────────────────────────────────────────────────────────────┘
```

The three interfaces must show the same capabilities, results and errors. All three receive the same application value and differ only in transport and presentation.

## 2. Repository map

```text
kali-tools/
├── main.go, app.go, doc.go         Wails composition root and desktop facade (must stay at root; see §6.1)
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
│   ├── app/                        application layer: Service (jobs, history, discovery, Nmap inspection), capability registry
│   ├── job/                        (empty) target home of the job manager and event fan-out
│   ├── network/                    capability: interfaces, DNS/TCP/HTTP/TLS/ping/trace diagnostics, TCP discovery, counter deltas
│   ├── capture/                    capability: live capture (Windows raw IPv4), PCAP/PCAPNG import/export, gopacket decoding
│   ├── wifi/                       capability: current Wi-Fi metadata and explicit key reveal (Windows)
│   ├── adapter/                    all adapters, named by technology
│   │   ├── storage/                driven: SQLite store, migrations, settings, retention
│   │   ├── tools/                  driven: optional executables (Nmap/TShark), bounded process runner, Nmap XML parser
│   │   ├── probe/                  driven: native Windows ICMP echo/traceroute
│   │   ├── platform/               driven: OS routes, DNS servers and interface counters (windows/linux/other)
│   │   ├── wailsui/                (empty) target home of the Wails bindings and event forwarder
│   │   ├── httpapi/                (empty) target home of the web server: REST, SSE, security middleware
│   │   └── cli/                    (empty) target home of the CLI command tree
│   ├── bootstrap/                  (empty) target home of the shared wiring used by every composition root
│   └── porttest/                   (empty) target home of the contract test suites for driven ports
│
├── frontend/                       React + TypeScript + Base UI + Tailwind v4 (Vite, Bun)
│   ├── index.html                  entry HTML (loads src/main.tsx)
│   ├── src/
│   │   ├── main.tsx                React entry point
│   │   ├── app/App.tsx             application shell, routes/pages (to be split into features/)
│   │   ├── features/               (empty) target home of per-capability UI modules
│   │   ├── shared/api/bridge.ts    THE transport seam: typed calls to Go (Wails today; HTTP+SSE planned)
│   │   ├── shared/i18n/i18n.ts     English and PT-BR catalogs
│   │   ├── shared/ui/Icon.tsx      shared presentational components
│   │   └── styles/style.css        design tokens and styles (Tailwind source root is src/)
│   ├── tests/workflows.spec.ts     Playwright workflows with contract-faithful fakes
│   ├── wailsjs/                    GENERATED Wails bindings; never edit by hand
│   └── dist/                       build output (embedded by main.go; only .gitkeep tracked)
│
├── tests/
│   ├── integration/                Docker lab tests (`integration` build tag)
│   ├── acceptance/                 (empty) target home of interface-agnostic acceptance specifications
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
| Application | `internal/app`, `internal/job` | `domain`, capability packages, `job` | `adapter/*`, Wails, `net/http`, `os/exec` |
| Driven adapters | `internal/adapter/{storage,tools,probe,platform}` | `domain`, capability packages, OS/driver libraries | driving adapters, `cmd`, `app` (except for types) |
| Driving adapters | `internal/adapter/{wailsui,httpapi,cli}`, today `app.go` and `cmd/kali-tools` | `app`, `domain`, DTO types | driven adapters directly (they go through `app`) |
| Composition roots | `main.go`, `cmd/*`, `internal/bootstrap` | everything | nothing imports them |

Rules that hold in every layer, from AGENTS.md and PLAN.md:

- No shell command concatenation. Executables run with an argument array, a deadline, output caps and descendant cancellation.
- Network operations, parser input, event queues and subprocess output are bounded.
- Wi-Fi secrets and packet payloads never enter history, logs or the database.
- Capability limitations are explicit states (`available`, `dependency_missing`, `permission_required`, `unsupported`, `failed`, and `location_consent_required` when added). Never fake a success.
- Platform code uses build constraints and an explicit unsupported fallback.

## 4. Ports and adapters

### 4.1 Where interfaces live

A port is a small Go interface declared **in the package that consumes it**. Adapters return concrete types and never import the port's package just to "implement" it. Add an interface only at a real boundary (a driven dependency, or a test stand-in for one). Inside a layer, call functions directly. See [architecture research](architecture-research.md) §3.2 and §4.6.

### 4.2 Current driven ports and implementations

| Consumer | Port (current form) | Implementation |
| --- | --- | --- |
| `app.Service` | `*storage.Store` concrete (no interface yet) | `adapter/storage` |
| `app.Service` | `Clock` interface | `app.SystemClock` |
| `app.Service` | Nmap inspection via `tools` functions | `adapter/tools` |
| `capture` | `LiveProvider` interface | `live_windows.go` / `live_other.go` |
| `network.Diagnostics` | `network.Prober` interface (injected by composition roots) | `probe.ICMP` in `adapter/probe` |
| `app.go` / CLI | direct calls to `platform.Snapshot`, `wifi`, `capture`, `tools.Inventory` | respective packages |

### 4.3 Target driven ports (PLAN phase H)

| Port | Declared in | Implementations |
| --- | --- | --- |
| `JobStore`, `SettingsStore` | `app` | `adapter/storage`, in-memory fake |
| `Prober` (ICMP echo/trace) — **done 2026-10-21** | `network` | `probe.ICMP` (windows), unsupported fallback |
| `HostNetwork` (routes, DNS servers, counters) | `app` | `adapter/platform` |
| `ToolRunner` / `ServiceInspector` | `app` | `adapter/tools` |
| `WiFiProvider` | `app` | `internal/wifi` native providers |
| `CaptureProvider` | `capture` (exists as `LiveProvider`) | Windows raw socket; future Npcap/libpcap/Pktmon |

Every port gets a contract suite in `internal/porttest`, run against both the real adapter and its fake.

## 5. Runtime flows

### 5.1 Starting a diagnostic job (current desktop path)

```text
React (bridge.ts) ──StartDiagnostic(req)──► app.go (Wails binding)
   app.go ──► app.Service.Start(ctx, req)
                ├─ network.Validate(req)              domain validation, typed errors
                ├─ storage.PutJob(queued)             adapter (SQLite), no secrets/payloads
                └─ goroutine: Service.Diagnostics.Diagnose(ctx)   injected Prober, bounded deadline, cancellable
                      └─ emit(Job) ──► app.go forwarder ──► runtime.EventsEmit("job") ──► React
CancelJob(id) ──► app.Service.Cancel(id) ──► context cancel ──► job state cancelled
```

### 5.2 Same flow in the target architecture (all interfaces)

```text
Wails binding ─┐
HTTP handler  ─┼─► app.Application.Diagnostics.Handle(ctx, req) ─► job.Manager.Start(...)
CLI command   ─┘                                                     │
                                                                     ▼ bounded fan-out
Wails forwarder (EventsEmit "job") ◄── job.Manager.Subscribe ──► SSE /api/events ──► CLI stdout
```

The job context derives from the manager (application lifetime), never from a request. Each event carries a monotonic `seq`, so a reconnecting client can call `Snapshot` and resynchronize.

### 5.3 Events

| Event | Producer today | Payload | Rule |
| --- | --- | --- | --- |
| `job` | `app.Service` via `app.go` | job state | Stable job ID and state machine `queued → running → completed/failed/cancelled` |
| `metrics` | ticker in `app.go` | interface rates | Batched; timestamp and units visible; reset never produces a negative spike |
| `capture` | `app.go` capture goroutine | capture result or error | Bounded packet count, retained bytes and duration |

The target moves the `metrics` and `capture` loops out of `app.go` into `app`/`job`, so the web server and the CLI can stream them too.

## 6. Interfaces (driving adapters)

### 6.1 Desktop (Wails v2), the primary interface

- `main.go` (build tags `desktop || bindings`) embeds `frontend/dist` and calls `wails.Run` with the `App` facade bound.
- **Why it stays at the repository root:**
  - Wails v2 builds the project-root package next to `wails.json`.
  - `//go:embed` cannot reference parent directories.
  - The generated binding namespace (`frontend/wailsjs/go/main/App`) follows the Go package name.
- The target keeps `main.go` thin and moves binding methods into `internal/adapter/wailsui`. Moving them changes the generated namespace, so regenerate the bindings and update `bridge.ts` in the same change.
- Bindings translate and delegate only. Business logic in `app.go` (metrics loop, capture session state) is debt tracked in PLAN.md.

### 6.2 Web server (planned): same UI in a browser

- **Delivery:** `kali-tools serve` subcommand of the CLI binary. One binary serves headless SSH hosts and local browsers.
- **Package:** `internal/adapter/httpapi`.
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

### 6.3 CLI

- `cmd/kali-tools/main.go` uses stdlib `flag`. Commands: `version`, `doctor`, `interfaces`, `inventory`, `diagnose`, `discover`/`inspect`, `wifi`, `capture`/`import`.
- `--json` output includes `schema_version`. Invalid input exits with a stable non-zero code.
- The target moves the command tree into `internal/adapter/cli` (with signature `Run(ctx, app, args, stdout, stderr) int`), routes every command through `app.Application`, and adds `serve`, `history`, `settings`, `export` and `jobs` commands, so the CLI is complete for SSH use.

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
| D2 | `app.go` and the CLI call `platform`, `wifi`, `capture` and `tools` directly instead of going through `app` | Logic and wiring are duplicated per interface; the web server would duplicate them again | H04, H05 |
| D3 | `app.go` owns the metrics ticker and capture session state | Business behavior in a driving adapter; unavailable to web and CLI | H02, H04 |
| D4 | Binding and CLI results expose `storage.Job` and `storage.Settings` | Adapter types used as DTOs; storage changes leak to the UI contract | H06 |
| D5 | `app.Service` depends on `*storage.Store` concretely | Use cases cannot run with a fake store; there is no contract test | H03, H07 |
| D6 | `capture`, `network`, `wifi` mix domain rules with OS or network I/O in one package | Harder to test without hardware | H03 (split only where a port is needed) |
| D7 | `frontend/src/app/App.tsx` (about 1,250 lines) holds every page | Hard to evolve per feature | W02 |
| D8 | No mechanical check of the dependency rule | Rules decay silently | H08 |

## 9. Verification commands

```bash
go vet ./... && go vet -tags desktop .
GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...                      # add -race where the toolchain supports it
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
node scripts/task.mjs fuzz         # parser fuzz smoke
bash scripts/setup/setup-linux-docker.sh test   # Docker CLI lab (or the Windows/macOS variant)
```

Formatting is enforced by the Lefthook pre-commit (`golangci-lint fmt`: gofmt -s + goimports with local prefix, re-staged automatically), followed by `golangci-lint run` and `go vet`; pre-push runs `golangci-lint fmt --diff`, tests and govulncheck.

The structural reorganization of 2026-10-21 was verified with:

- `go vet` (default, desktop tag, linux, darwin) and `go test ./...` on Windows;
- frontend typecheck, lint and build;
- 17/17 Playwright workflows;
- `bash -n` on the moved setup scripts.
