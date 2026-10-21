# H04 Application facade

| Field | Value |
| --- | --- |
| PLAN tasks | H04 |
| Status | Ready |
| Depends on | H01, H02, H03 |
| Risk | Medium: every desktop binding and every CLI command is re-routed; the CLI capture path changes from a direct call to start, subscribe and wait |

## 1. Goal

Turn `app.Service` into the single application value, `app.Application`. It exposes every operation that `app.go` (Wails) or `cmd/kali-tools` (CLI) performs today. Both driving roots then call only `*app.Application`, and import only `internal/app`, `internal/domain` and the packages that currently serve as data types (`job`, `network`, `capture`, `wifi`). The direct calls to `platform`, `tools`, `wifi` and `capture` from the roots disappear (debt D2), and the web server (W03) can reuse the same value with no new logic.

## 2. Non-goals

- Do not move the Wails bindings to `internal/adapter/wailsui`, or the CLI tree to `internal/adapter/cli` (H05).
- Do not introduce `internal/adapter/contract` DTOs or error mapping (H06). The roots keep returning `job.Status`, `app.Settings`, `network.*`, `capture.*`, `wifi.*` and `app.*` types.
- Do not add Wails binding methods. In particular, do not bind `JobSnapshot` or `Subscribe` (W03 and H05 use them). The generated `frontend/wailsjs` must not change.
- Do not add CLI commands or flags (C01). Do not make the CLI use a persistent data path (C01).
- Do not change validation rules, limits, error codes or JSON keys.
- Do not create `internal/porttest` or `tests/acceptance` (H07).

## 3. Current state

These facts were verified on 2026-10-21 against the pre-H01 tree. After H01–H03 land:

- `app.Service` lives in `internal/app/service.go`, `metrics.go` and `capture.go`, with `New(deps Deps) *Service` (H03).
- The transitional `app.Application{Service *Service}` lives in `internal/app/application.go` (H01).
- `bootstrap.New` returns `&app.Application{Service: service}`.

Re-verify before starting.

Remaining direct calls in the Wails root `app.go`, by pre-H01 line:

| Line | Binding | Direct call |
| --- | --- | --- |
| 123 | `Capabilities` | `app.Capabilities()` (package function, `internal/app/capabilities.go:9`) |
| 124 | `Interfaces` | `network.Interfaces()` |
| 125 | `Inventory` | `platform.Inventory()` |
| 130 | `Tools` | `tools.Inventory(a.ctx)` |
| 136 | `Diagnose` | `a.service.Diagnostics.Diagnose(a.ctx, r)` |
| 174-175 | `CurrentWiFi` / `RevealWiFi` | `wifi.Current()` / `wifi.Reveal()` |
| 180 | `Discover` | `a.service.Discover(a.ctx, cidr, ports, 500*time.Millisecond)` |

The capture bindings delegate to `Service` since H02. History and settings delegate since H03.

Remaining direct calls in the CLI root `cmd/kali-tools/main.go`:

| Line | Command | Direct call |
| --- | --- | --- |
| 65 | `doctor` | `app.Capabilities()`, `tools.Inventory(ctx)` |
| 67 | `interfaces` | `network.Interfaces()` |
| 69 | `inventory` | `platform.Inventory()` |
| 95 | `inspect` | `tools.RunNmap(ctx, ...)` (no history) |
| 100 | `discover` | `network.Discover(ctx, ..., time.Duration(*timeout)*time.Millisecond)` (no history) |
| 120 | `wifi` | `wifi.Current()` |
| 130-139 | `capture`, `import` | `capture.Live`, `capture.LiveSession`, `capture.ReadFile`, `capture.ReadArtifactFile` |
| 141-146 | export | `artifact.Save(ctx, *output, *format)` with no timeout; a nil artifact gives `"capture has no packets to export"` |

Since H01, `diagnose` uses `application.Service.Diagnostics`.

Other facts:

- `cmd/kali-tools/main.go:152-167` is the error mapping: `invalid_input` gives exit 2, any other `*domain.Error` gives exit 1 with its code, `context.Canceled` gives code `cancelled`, and anything else gives code `failed`.
- `bridge_smoke_test.go:59-69` calls `NewApp()` and `app.startup(ctx)`, then invokes `Interfaces`, `Settings`, `Version`, `CancelJob("missing")`, `SaveCapture("invalid")` (expects rejection) and `InspectServices("0.0.0.0/0",[80])` (expects rejection).
- The `wifi` CLI command never reveals secrets (usage text at `cmd/kali-tools/main.go:37`).

## 4. Design

### 4.1 `internal/app.Application`

Rename `Service` to `Application` and delete the H01 wrapper in `application.go`. Move the type and its constructor into `application.go`.

```go
package app

// DefaultDiscoveryTimeout is the per-endpoint TCP timeout used by the desktop
// (was the literal at app.go:180). The CLI passes its --timeout flag instead.
const DefaultDiscoveryTimeout = 500 * time.Millisecond

type Application struct {
    // all fields unexported: deps Deps, diagnostics network.Diagnostics,
    // jobs *job.Manager, plus the H02 lifecycle, metrics and capture fields
}

func New(deps Deps) *Application

func (a *Application) Capabilities(ctx context.Context) []domain.Capability
func (a *Application) Interfaces(ctx context.Context) ([]network.InterfaceInfo, error)
func (a *Application) Inventory(ctx context.Context) (HostSnapshot, error)
func (a *Application) Tools(ctx context.Context) ([]Tool, error)
func (a *Application) Diagnose(ctx context.Context, r network.Request) (network.Result, error)
func (a *Application) StartDiagnostic(ctx context.Context, r network.Request) (job.Status, error)
func (a *Application) CancelJob(ctx context.Context, id job.ID) bool
func (a *Application) JobSnapshot(ctx context.Context, id job.ID) (job.Status, bool)
func (a *Application) Subscribe(ctx context.Context) (<-chan job.Event, func() uint64)
func (a *Application) History(ctx context.Context) ([]job.Status, error)
func (a *Application) ClearHistory(ctx context.Context) error
func (a *Application) Settings(ctx context.Context) (Settings, error)
func (a *Application) SaveSettings(ctx context.Context, v Settings) error
func (a *Application) CurrentWiFi(ctx context.Context) (wifi.Network, error)
func (a *Application) RevealWiFi(ctx context.Context) (wifi.Secret, error)
func (a *Application) Discover(ctx context.Context, cidr string, ports []int, timeout time.Duration) ([]network.PortResult, error)
func (a *Application) InspectServices(ctx context.Context, cidr string, ports []int) (Nmap, error)
func (a *Application) StartMetrics(ctx context.Context)
func (a *Application) StartCapture(ctx context.Context, r capture.LiveRequest) error
func (a *Application) StopCapture(ctx context.Context)
func (a *Application) CanImportCapture(ctx context.Context) error
func (a *Application) ImportCapture(ctx context.Context, path string) (capture.ImportResult, error)
func (a *Application) CanExportCapture(ctx context.Context, format string) error
func (a *Application) ExportCapture(ctx context.Context, path, format string) error
func (a *Application) Close() error
```

`StartMetrics`, `CanImportCapture` and `CanExportCapture` are additions to the shared method list:

- `StartMetrics` lets only interfaces that display rates start the 2 s sampler. The CLI never starts it.
- `CanImportCapture` and `CanExportCapture` let the desktop reject a request before it opens a native file dialog (H02 §4.3).

`Close() error` takes no context. It is the cleanup half of `bootstrap.New`.

### 4.2 Method rules

Every method follows these rules:

- `ctx` is the caller's context.
  - Synchronous calls pass it to providers.
  - Job lifetimes never derive from it. They use the manager (H02).
  - The synchronous `Discover` and `InspectServices` cancel their job when `ctx` ends (`context.AfterFunc`, H02).
- Behavior per method:
  - `Capabilities` returns the current registry. Rename the package function `Capabilities()` to the unexported `capabilities()`. W06 will make the result depend on `deps`.
  - `Interfaces` calls `network.Interfaces()`. This is a capability call, not a port.
  - `Inventory` calls `deps.Host.Inventory(ctx)`. A nil `Host` gives `domain.Unsupported("host network provider is not configured")`.
  - `Tools` calls `deps.Tools.Inventory(ctx), nil`. A nil `Tools` gives `nil, domain.Unsupported("tool runner is not configured")`.
  - `Diagnose` is synchronous and does **not** write history, exactly as today.
  - `StartDiagnostic` is the former `Service.Start`.
  - `CancelJob` and `JobSnapshot` delegate to `job.Manager.Cancel` and `job.Manager.Snapshot`. `Subscribe` delegates to `job.Manager.Subscribe` with the caller's `ctx`.
  - `History`, `ClearHistory`, `Settings` and `SaveSettings` delegate to `deps.Jobs` and `deps.Settings`.
  - `CurrentWiFi` calls `deps.WiFi.Current(ctx)`.
  - `RevealWiFi` calls `deps.WiFi.Reveal(ctx)`. Its result is returned to the caller only. It is never logged, published as an event, persisted or included in an error message. A nil `WiFi` gives `domain.Unsupported("Wi-Fi provider is not configured")`.
  - `Discover` and `InspectServices` keep the H02 summaries, which hold counts only.
  - `ExportCapture` applies `CaptureExportTimeout` (30 s) for every caller, including the CLI, which had no bound before.
- After `Close`:
  - `StartDiagnostic`, `Discover` and `InspectServices` return `job.ErrClosed`.
  - `StartCapture`, `CanImportCapture` and `ImportCapture` return `domain.Invalid("application is shutting down")`.
  - `Subscribe` returns a closed channel.

### 4.3 Bootstrap

`bootstrap.New` returns `app.New(app.Deps{...})` directly (the H03 wiring), not a wrapper. Its signature does not change: `func New(ctx context.Context, cfg Config) (*app.Application, func() error, error)`.

### 4.4 Wails root

`main.go` owns the composition:

```go
// main.go
func NewApp() *App {
    return &App{boot: func(ctx context.Context) (*app.Application, func() error, error) {
        return bootstrap.New(ctx, bootstrap.Config{})
    }}
}
```

`app.go` only adapts. It keeps the same exported method set and signatures, so the bindings do not drift.

```go
type App struct {
    initMu  sync.RWMutex
    closed  bool
    ctx     context.Context // Wails runtime context (allowed exception, README §6.1)
    cancel  context.CancelFunc
    app     *app.Application
    cleanup func() error
    initErr error
    boot    func(context.Context) (*app.Application, func() error, error)
}
```

- `startup`:
  1. Call `a.boot(a.ctx)`. On error, set `a.initErr` and return.
  2. Subscribe: `events, _ := a.app.Subscribe(a.ctx)`.
  3. Start the forwarder with `wailsEvent` (H02, unchanged).
  4. Call `a.app.StartMetrics(a.ctx)`.
- `shutdown` keeps its order: mark closed, call `a.cancel()`, then `a.cleanup()`.
- Each binding performs the `ready()` check and delegates with `a.ctx`.
  - `CancelJob(id string)` converts with `job.ID(id)`.
  - `Discover(cidr, ports)` passes `app.DefaultDiscoveryTimeout`.
  - `ImportCapture()` calls `CanImportCapture`, opens the dialog (an empty path returns `capture.ImportResult{Packets: []capture.Packet{}}`), then calls `a.app.ImportCapture(a.ctx, path)`.
  - `SaveCapture(format)` calls `CanExportCapture(a.ctx, format)`, opens the dialog (an empty path or a dialog error is returned as today), then calls `a.app.ExportCapture(a.ctx, path, format)` and returns `true`.
  - `StartCapture(r)` calls `a.app.StartCapture(a.ctx, r)`. Validation happens inside, before the ready check, as today.
- `app.go` imports from the module only `internal/app`, `internal/domain`, `internal/job`, `internal/network`, `internal/capture` and `internal/wifi`. `main.go` additionally imports `internal/bootstrap`.

### 4.5 CLI root

`run(ctx, args, getenv, out, errout)` keeps the H01 structure and the `cliDataPath` constant. Every command goes through `application`:

| Command | Call |
| --- | --- |
| `version` | unchanged (no application call) |
| `doctor` | `application.Capabilities(ctx)`; `tools, _ := application.Tools(ctx)`; static capture/Wi-Fi text unchanged |
| `interfaces` | `application.Interfaces(ctx)` |
| `inventory` | `application.Inventory(ctx)` |
| `diagnose` | `application.Diagnose(ctx, network.Request{...})` |
| `discover` | `application.Discover(ctx, *target, numbers, time.Duration(*timeout)*time.Millisecond)`; CSV writing unchanged |
| `inspect` | `application.InspectServices(ctx, *target, numbers)` |
| `wifi` | `application.CurrentWiFi(ctx)` (never `RevealWiFi`) |
| `capture` | `runCapture` (below), then the optional export |
| `import` | `data, err = application.ImportCapture(ctx, *file)`, then the optional export |
| export (`--output`) | `application.ExportCapture(ctx, *output, *format)`; the `--format` check before any work stays first |

The `capture` command subscribes before it starts, so the single completion event cannot be missed:

```go
// captureWaitSlack bounds the wait beyond the requested duration.
const captureWaitSlack = 10 * time.Second

func runCapture(ctx context.Context, application *app.Application, r capture.LiveRequest) (capture.LiveResult, error) {
    subCtx, stop := context.WithCancel(context.Background()) // must outlive ctx to receive the final event
    defer stop()
    events, _ := application.Subscribe(subCtx)
    if err := application.StartCapture(ctx, r); err != nil {
        return capture.LiveResult{}, err
    }
    return awaitCapture(ctx, events, func() { application.StopCapture(context.Background()) },
        time.Duration(r.DurationSeconds)*time.Second+captureWaitSlack)
}

// awaitCapture returns the first "capture" event's result and its typed error
// (CaptureEvent.Err). When ctx ends it calls stop once and keeps waiting; after
// limit it calls stop and returns domain error code "timeout".
func awaitCapture(ctx context.Context, events <-chan job.Event, stop func(), limit time.Duration) (capture.LiveResult, error)
```

- `awaitCapture` reads `ev.Data.(app.CaptureEvent)` and ignores every other topic.
- A closed channel returns `job.ErrClosed`.
- Typed provider errors (`permission_required`, `unsupported`, `invalid_input`) reach the envelope unchanged, and so does `context.Canceled` (code `cancelled`). The existing mapping at `main.go:152-167` is unchanged.
- `cmd/kali-tools/main.go` imports from the module only `internal/bootstrap`, `internal/app`, `internal/domain`, `internal/job`, `internal/network` and `internal/capture`.

Intentional CLI differences:

| Case | Before | After |
| --- | --- | --- |
| `discover` / `inspect` | no job row | write count-only job rows to the CLI's in-memory store; invisible, nothing is written to disk |
| Export of a file with zero packets | `"capture has no packets to export"` | `"import or finish a capture before saving"` (still `invalid_input`, exit 2) |
| Export duration | unbounded | bounded at 30 s |
| `capture` and `import` without `--output` | no artifact kept | keep the bounded artifact in memory for the process lifetime |

### 4.6 Preserved constraints

| Constraint | How H04 keeps it |
| --- | --- |
| Max 8 concurrent jobs | `job.Limits{}` inside `app.New` (unchanged) |
| Terminal-event delivery | `job.TerminalWait`; `capture` completion uses it, and the CLI subscribes before starting |
| No secrets or targets in history | `RevealWiFi` has no side effect besides the provider call; summaries hold counts only |
| Wails event names | `job`, `metrics`, `capture` via the forwarder (unchanged) |
| CLI JSON envelope with `schema_version` | `envelope` and the error mapping are unchanged |

## 5. Files

Create:

- `internal/app/application_test.go`: package `app_test`, using the H03 fakes.
- `cmd/kali-tools/capture_test.go`: `awaitCapture` unit tests.

Modify:

- `internal/app/application.go`: becomes the home of `Application`, `New` and the simple delegating methods. Remove the H01 wrapper.
- `internal/app/service.go`: renamed to `jobs.go`, with `StartDiagnostic`, `Discover`, `InspectServices`, `CancelJob`, `JobSnapshot` and `Subscribe` (use `git mv`).
- `internal/app/metrics.go`, `capture.go`, `export_test.go`: rename the receiver type and add `ctx` parameters.
- `internal/app/capabilities.go`: unexport the package function.
- Every `internal/app` test: `Service` becomes `Application`, `s.Start` becomes `a.StartDiagnostic(ctx, ...)`, `s.Jobs.Subscribe` becomes `a.Subscribe(ctx)`, `s.Cancel(id)` becomes `a.CancelJob(ctx, job.ID(id))`. Mechanical only; the assertions do not change.
- `internal/bootstrap/bootstrap.go`, `bootstrap_test.go`: return `*app.Application` directly; tests use `application.Diagnose` and `application.History` instead of `.Service`.
- `internal/bootstrap/roots_test.go`: add `TestDrivingRootsImportOnlyApplication`.
- `main.go`: `NewApp` with `boot`; import `bootstrap`.
- `app.go`: delegate everything; drop the `platform`, `tools`, `bootstrap` and `time` imports where they become unused.
- `cmd/kali-tools/main.go`: commands through `application`, `runCapture` and `awaitCapture`.
- `.golangci.yml`: change the errcheck exclusion `internal/app.(*Service).Close` to `internal/app.(*Application).Close`.
- `docs/architecture.md`: see §10.

Delete:

- `internal/app/service.go` (moved to `jobs.go`).

## 6. Implementation steps

Each step leaves `go vet ./... && go vet -tags desktop . && go test ./...` green.

1. Rename `Service` to `Application` (`git mv internal/app/service.go internal/app/jobs.go`). Merge the H01 wrapper away. Update `bootstrap`, `app.go`, the CLI and the tests mechanically (`application.Service.X` becomes `application.X`). Update `.golangci.yml`.
2. Add the ctx-first methods of §4.1 that do not exist yet (`Capabilities`, `Interfaces`, `Inventory`, `Tools`, `Diagnose`, `CurrentWiFi`, `RevealWiFi`, `JobSnapshot`, `Subscribe`, `StartMetrics`) and change the existing methods to take `ctx` first. Add `application_test.go`. Run `go test -race ./internal/app`.
3. Wails: move `NewApp` to `main.go` with `boot`, and make every binding delegate (§4.4). Run `go vet -tags desktop .` and `node scripts/task.mjs bindings`; expect no drift.
4. CLI: route every command through `application`. Add `runCapture` and `awaitCapture` with `capture_test.go`. Run `go test ./cmd/kali-tools`. All existing CLI tests must pass unchanged.
5. Add `TestDrivingRootsImportOnlyApplication`. Run `go test ./internal/bootstrap`.
6. Run the Docker lab and, on Windows, the bridge smoke test.
7. Update the docs (§10).
8. Write `docs/verification/specs/H04.md`, following README §5.3, with the real outputs of §9 and §12.

## 7. Acceptance criteria

- **AC-1:** Given an `Application` built from H03 fakes, when `Inventory`, `Tools`, `CurrentWiFi`, `RevealWiFi`, `Settings`, `SaveSettings`, `History` and `ClearHistory` are called, then each returns exactly what its fake port returns, and passes the caller's `ctx` (the fake records `ctx.Value(testKey)`).
- **AC-2:** Given a `fakeWiFi` whose `Reveal` returns `Secret{State: "available", Value: "synthetic-psk"}`, when `RevealWiFi` is called, then the value is returned. Afterwards:
  - no status in `fakeJobStore` contains `synthetic-psk`;
  - no event received by a subscriber (JSON-encoded) contains `synthetic-psk`.
- **AC-3:** Given nil `Host`, `Tools` or `WiFi` deps, the corresponding methods return a `*domain.Error` with code `unsupported`.
- **AC-4:** Given `StartDiagnostic` and a subscriber:
  - `JobSnapshot(id)` returns the same ID while the job runs and after it finishes;
  - `CancelJob(ctx, id)` cancels it (terminal state `cancelled`);
  - `CancelJob(ctx, "missing")` returns false.
- **AC-5:** Given 8 running diagnostics, a 9th `StartDiagnostic` fails, and the shutdown contract holds. This is the existing `TestConcurrentDiagnosticsBoundAndShutdown`, renamed calls only.
- **AC-6:** Given `Close`, the methods of §4.2 return the documented errors, and a second `Close` returns the first result.
- **AC-7:** Given `awaitCapture`:
  - with an event whose `CaptureEvent.Err` is `&domain.Error{Code: "permission_required"}`, it returns that error (`errors.As` succeeds with that code);
  - with `ctx` cancelled first, it calls `stop` exactly once and returns the subsequent event's `context.Canceled`;
  - with no event within `limit`, it calls `stop` and returns code `timeout`;
  - with a closed channel, it returns `job.ErrClosed`.
- **AC-8:** Given the CLI, every existing test in `cmd/kali-tools/main_test.go` passes with unchanged assertions: the contract table, HTTP diagnose, malformed flags, discovery validation, JSON and CSV exports, capture conversion and payload absence.
- **AC-9:** Given `kali-tools capture --target 127.0.0.1`, the CLI exits 2 with code `invalid_input` (validation inside `StartCapture`), and no event wait occurs.
- **AC-10:** Given the source files, when the import check runs, then:
  - `app.go` imports, within the module, only `internal/{app,domain,job,network,capture,wifi}`;
  - `cmd/kali-tools/main.go` imports only `internal/{bootstrap,app,domain,job,network,capture}`;
  - neither file imports anything under `internal/adapter/`.
- **AC-11:** Given `node scripts/task.mjs bindings`, there is no drift under `frontend/wailsjs`.
- **AC-12 [native]:** Given the Windows desktop, `TestNativeBridge` passes. Manual checks: interfaces, inventory, tools, diagnostics with job states, discovery, inspection (with Nmap installed), Wi-Fi metadata, explicit reveal, live capture with save, and import.
- **AC-13:** Given the Docker lab, `TestCLIExecutable` passes unchanged.

## 8. Tests

| Test | Package | Asserts | AC |
| --- | --- | --- | --- |
| `TestApplicationDelegatesToPorts` | `internal/app` (`application_test.go`) | Table over the eight methods; values and ctx propagation | AC-1 |
| `TestRevealNeverReachesJobStoreOrEvents` | `internal/app` | `synthetic-psk` absent from stored statuses and JSON events | AC-2 |
| `TestMissingProvidersAreUnsupported` | `internal/app` | Extended from H03 with `Inventory`, `Tools`, `CurrentWiFi`, `RevealWiFi` | AC-3 |
| `TestJobSnapshotAndCancel` | `internal/app` | Snapshot, cancel, unknown ID | AC-4 |
| `TestConcurrentDiagnosticsBoundAndShutdown`, `TestCancellationAndRedaction`, `TestTerminalEventSurvivesFullEventBuffer`, `TestDiscoverySummarySurvivesRestartWithoutTargets` | `internal/app` | Existing assertions; renamed calls only | AC-5 |
| `TestCloseIsIdempotent` | `internal/app` | Extended: post-close errors per §4.2 | AC-6 |
| `TestAwaitCapturePreservesTypedError`, `TestAwaitCaptureStopsOnCancel`, `TestAwaitCaptureTimesOut`, `TestAwaitCaptureClosedChannel` | `cmd/kali-tools` (`capture_test.go`) | §4.5 contract; use `limit` values of at most 200 ms and channels, no `time.Sleep` | AC-7 |
| `TestCLIContract`, `TestCLIHTTPAndMalformedFlags`, `TestDiscoveryRejectsInvalidOptionsBeforeScanning`, `TestDiscoveryExports`, `TestCLICaptureConversion`, `TestRunDoesNotWriteUserConfig` | `cmd/kali-tools` | Unchanged | AC-8 |
| `TestCLICaptureRejectsLoopback` | `cmd/kali-tools` | Exit 2, `invalid_input` | AC-9 |
| `TestDrivingRootsImportOnlyApplication` | `internal/bootstrap` (`roots_test.go`) | `go/parser` import allowlists of AC-10 | AC-10 |
| `TestNativeBridge` | root (native tags) | Unchanged | AC-12 |
| `TestCLIExecutable` | `tests/integration` | Unchanged | AC-13 |

All `internal/job`, `internal/network`, `internal/capture`, `internal/wifi` and `internal/adapter/*` tests must pass unchanged.

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
go test -race ./internal/app ./cmd/kali-tools ./internal/bootstrap
go list -f '{{join .Imports "\n"}}' ./cmd/kali-tools | grep internal/adapter      # must print nothing
go list -tags desktop -f '{{join .Imports "\n"}}' . | grep internal/adapter      # must print nothing
node scripts/task.mjs bindings                       # no drift
bash scripts/setup/setup-linux-docker.sh test        # Docker lab
node scripts/task.mjs bridge                         # [native] Windows only
```

## 10. Documentation updates

- `docs/architecture.md`:
  - §1/§3: the driving roots call only `app.Application`.
  - §4.2: remove the row "`app.go` / CLI: direct calls to `platform`, `wifi`, `capture`, `tools.Inventory`".
  - §5.1: show `App.StartDiagnostic` calling `app.Application.StartDiagnostic`.
  - §6.3: list the CLI commands as routed through `app.Application`.
  - §8: mark D2 resolved for H04 (the extraction into `adapter/cli` and `adapter/wailsui` remains H05).
- `docs/architecture-research.md` §6.1: note that the implemented `Application` is a single struct with methods, not handler fields.
- `PLAN.md`: tick the H04 sub-item with evidence. Leave the parent unticked until AC-12 [native] is recorded.

## 11. Risks and rollback

- **The CLI capture now depends on event delivery.** Mitigation:
  - it subscribes before `StartCapture`;
  - the capture topic uses the 2 s terminal wait;
  - the wait is bounded by duration plus 10 s;
  - `awaitCapture` has unit tests.
- **Binding drift from a changed signature.** Mitigation: AC-11. Every `App` method keeps its pre-H04 signature.
- **Accidental secret exposure through `RevealWiFi`.** Mitigation: AC-2. `RevealWiFi` has no logging or event path.
- **Rollback:** revert the H04 commits. Nothing persistent changes. If the bindings drifted, regenerate them from the reverted code.

## 12. Quality and delivery assertions

### 12.1 Applicable gates

| Gate | Applies | Spec threshold |
| --- | --- | --- |
| QG-1 Formatting | yes | empty diff |
| QG-2 Lint | yes | `0 issues.`; errcheck exclusion renamed |
| QG-3 Vet on all targets | yes | exit 0, including `go vet -tags desktop .` |
| QG-4 Tests | yes | all pass; existing CLI assertions unchanged |
| QG-5 Race | yes | no race reports in `./internal/app`, `./cmd/kali-tools`, `./internal/bootstrap` |
| QG-6 Coverage | yes, tightened | `internal/app` ≥ 80 %; `cmd/kali-tools` not below the recorded pre-spec value |
| QG-7 Vulnerabilities | yes | no new dependency |
| QG-8 Frontend | yes | all pass; workflow count unchanged |
| QG-9 Binding drift | yes | `node scripts/task.mjs bindings` reports no drift |
| QG-10 Dependency rule | yes | DA-1, DA-2, DA-3 |
| QG-11 Secrets | yes | `TestRevealNeverReachesJobStoreOrEvents`; `TestCLICaptureConversion` payload absence |
| QG-12 Bounds | yes | `captureWaitSlack` and `CaptureExportTimeout` tested by `TestAwaitCaptureTimesOut` and the H02 export test; `DefaultDiscoveryTimeout` is within the 100..5000 ms range validated by `network.ValidateDiscovery` |
| QG-13 Commit | yes | `refactor(app): single application facade (H04)` |
| QG-14 Docs | yes | architecture §1, §3, §4.2, §5.1, §6.3, §8; research §6.1 note; CHANGELOG entry |
| QG-15 Markdown | yes | `Markdown checks passed` |

### 12.2 Delivery assertions

| DA | Assertion | Check | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | The CLI root imports no adapter | `go list -f '{{join .Imports "\n"}}' ./cmd/kali-tools \| grep -c internal/adapter` | `0` | CI |
| DA-2 | The desktop root imports no adapter | `go list -tags desktop -f '{{join .Imports "\n"}}' . \| grep -c internal/adapter` | `0` | CI |
| DA-3 | Import allowlists hold per file | `go test ./internal/bootstrap -run 'TestDrivingRootsImportOnlyApplication' -v` | `PASS` | CI |
| DA-4 | Every desktop operation exists on the application | `go doc ./internal/app Application \| grep -cE '^func \(a \*Application\) (Capabilities\|Interfaces\|Inventory\|Tools\|Diagnose\|StartDiagnostic\|CancelJob\|JobSnapshot\|Subscribe\|History\|ClearHistory\|Settings\|SaveSettings\|CurrentWiFi\|RevealWiFi\|Discover\|InspectServices\|StartCapture\|StopCapture\|ImportCapture\|ExportCapture\|Close)\('` | `22` | reviewer |
| DA-5 | The Wi-Fi secret never leaks | `go test ./internal/app -run 'TestRevealNeverReachesJobStoreOrEvents' -v` | `PASS` | CI |
| DA-6 | The CLI JSON contract is unchanged | `go test ./cmd/kali-tools -run 'TestCLIContract\|TestDiscoveryExports\|TestCLICaptureConversion' -v` | `PASS` | CI |
| DA-7 | The bindings did not drift | `node scripts/task.mjs bindings` | exit 0, no drift | reviewer (CI after H05) |
| DA-8 | The Docker lab CLI passes | `bash scripts/setup/setup-linux-docker.sh test` | exit 0 | reviewer |
| DA-9 | The desktop is fully functional | `node scripts/task.mjs bridge` plus the manual list in AC-12 | all pass | [native] |

### 12.3 Evidence record

`docs/verification/specs/H04.md`
