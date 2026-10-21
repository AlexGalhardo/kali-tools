# H03 Driven ports

| Field | Value |
| --- | --- |
| PLAN tasks | H03 |
| Status | Implementation complete 2026-10-21; native AC-11 pending ([evidence](../verification/specs/H03.md)) |
| Depends on | H01, H02 |
| Risk | Medium: type moves change the generated Wails models (namespaces only), and storage gains an import of `internal/app` types |

## 1. Goal

Remove every import of `internal/adapter/*` from the inner packages:

- `internal/app`;
- `internal/network`;
- `internal/capture`;
- `internal/wifi`;
- `internal/job`.

To do that, declare small driven ports in the package that consumes them. Wire the real adapters in `internal/bootstrap` only. `app.Service` can then run entirely on hand-written fakes (debt D5), and the composition roots hold the only adapter imports. User-visible behavior and JSON shapes do not change.

## 2. Non-goals

- Do not rename `Service` to `Application`, and do not add the Wi-Fi, inventory or tools use-case methods (H04). `app.go` and the CLI may still call `platform`, `tools`, `wifi` and `capture` directly in H03.
- Do not introduce transport DTOs or error mapping (H06). The moved types keep their JSON tags byte for byte.
- Do not write `internal/porttest` contract suites (H07). H03 adds compile-time assertions and fakes only.
- Do not add depguard (H08). H03 adds a test-based import check.
- Do not split `capture`, `network` or `wifi` into domain and OS packages beyond the thin `Native` provider types (D6 stays open).
- Do not change the storage schema, SQL statements, limits or validation.
- Do not change the frontend, except for regenerated files under `frontend/wailsjs` and, only if `bun run typecheck` requires it, type-only imports in `frontend/src/shared/api/bridge.ts`.

## 3. Current state

Verified on 2026-10-21. Line numbers are those before H01/H02. Re-locate the symbols after H01 and H02 land, and stop if a symbol no longer matches.

### 3.1 Done: `network.Prober` (reference pattern)

- `internal/network/network.go:138-143` declares the port in its consumer:

  ```go
  type Prober interface {
      Provider() string
      Run(ctx context.Context, target string, trace bool) ([]string, error)
  }
  ```

- `internal/network/network.go:146-148` declares `type Diagnostics struct{ Prober Prober }`. Lines 163-168 return `domain.Unsupported` when `Prober == nil` and otherwise use `Provider()` and `Run(...)`.
- `internal/adapter/probe/probe.go:6-10` declares `type ICMP struct{}`, whose methods delegate to the build-tagged `Run`/`Provider` (`probe_windows.go`, `probe_other.go:11-15`). The adapter does not import `network`.
- The roots inject it: `app.go:54` and `cmd/kali-tools/main.go:71`. After H01, only `internal/bootstrap` injects it.
- Tests: `TestICMPWithoutProberIsUnsupported` (`internal/network/network_test.go:132`) and `TestICMPUsesInjectedProber` (`:147`, with the hand-written `fakeProber`).
- `docs/architecture.md:146` and `:258` already record D1 as resolved.

### 3.2 Remaining

- `internal/app/service.go:12-13` imports `internal/adapter/storage` and `internal/adapter/tools`. After H02, `Service` still holds `Store *storage.Store` and calls `tools.RunNmap` (`service.go:182`). It also has the function fields `Counters` and `Live` (added by H02).
- Storage types:
  - `storage.Job` is at `internal/adapter/storage/storage.go:21-28`; after H02 its JSON equals `job.Status`.
  - `storage.Settings{Language "language", RetentionDays "retention_days"}` is at `storage.go:160-163`.
  - The methods are `PutJob(ctx, Job) error` (108), `History(ctx) ([]Job, error)` (118), `Clear(ctx) error` (147), `Retain(ctx, time.Time) error` (153), `Settings(ctx) (Settings, error)` (165), `SaveSettings(ctx, Settings) error` (189) and `Close() error` (107).
- Platform types and functions:
  - `platform.Adapter` and `platform.Snapshot{Adapters, Counters []network.Counter}` are at `internal/adapter/platform/platform.go:5-14`.
  - `Inventory() (Snapshot, error)` takes no context. It is implemented at `platform_windows.go:16`, `platform_linux.go:17` and `platform_other.go:7`.
- Tools types and functions:
  - `tools.Tool` is at `internal/adapter/tools/tools.go:13-18`.
  - The `tools.Nmap*` result types (with `xml` and `json` tags) are at `tools.go:46-79`.
  - The package functions are `Inventory(ctx) []Tool` (`tools.go:42`) and `RunNmap(ctx, cidr, ports) (Nmap, error)` (`nmap.go:48`).
- Wi-Fi functions:
  - `wifi.Current() (Network, error)` and `wifi.Reveal() (Secret, error)` take no context (`wifi_other.go:7,10`, `wifi_windows.go:99,120`).
  - The types are at `wifi.go:3-15`.
- Capture provider:
  - `capture.LiveProvider{ Capture(context.Context, LiveRequest) (LiveResult, error) }` is declared at `internal/capture/live.go:27-29`. Nothing implements or uses it (`grep -rn LiveProvider` finds only the declaration).
  - Live sessions use the function `capture.LiveSession(ctx, r) (LiveResult, *Artifact, error)` (`live.go:33-37`). It is the only way to obtain the artifact needed for export.
- Generated bindings: `frontend/wailsjs/go/main/App.d.ts` imports the model namespaces `storage`, `platform`, `tools`, `network`, `capture`, `wifi` and `domain`. `frontend/src/shared/api/bridge.ts` declares its own structural types and does not reference `storage.`, `platform.` or `tools.` (verified with grep).
- A Go external test package may import a package that imports the package under test. An **internal** test file of `package app` may not import `storage` once `storage` imports `app`, because that is an import cycle. After H02, `service_test.go` is already `package app_test`.

## 4. Design

### 4.1 Ports and app-owned types (`internal/app/ports.go`, `internal/app/types.go`)

```go
package app

// JobStore persists job statuses. Implemented by *storage.Store.
type JobStore interface {
    PutJob(ctx context.Context, s job.Status) error
    History(ctx context.Context) ([]job.Status, error) // newest first, at most 200
    Clear(ctx context.Context) error                   // removes finished jobs only
    Retain(ctx context.Context, before time.Time) error
}

// SettingsStore persists user preferences. Implemented by *storage.Store.
type SettingsStore interface {
    Settings(ctx context.Context) (Settings, error)
    SaveSettings(ctx context.Context, v Settings) error
}

// HostNetwork reads OS routes, DNS servers and interface counters.
// Implemented by platform.Host.
type HostNetwork interface {
    Inventory(ctx context.Context) (HostSnapshot, error)
}

// ToolRunner discovers and runs optional executables. Implemented by tools.Runner.
type ToolRunner interface {
    Inventory(ctx context.Context) []Tool
    RunNmap(ctx context.Context, cidr string, ports []int) (Nmap, error)
}

// WiFiProvider reads the current Wi-Fi connection and, on explicit request,
// its key. Implemented by wifi.Native. Secrets never reach JobStore.
type WiFiProvider interface {
    Current(ctx context.Context) (wifi.Network, error)
    Reveal(ctx context.Context) (wifi.Secret, error)
}

// CaptureProvider runs one bounded live capture. The port is declared by
// internal/capture (LiveProvider); this alias keeps app.Deps readable.
type CaptureProvider = capture.LiveProvider

type Deps struct {
    Jobs        JobStore
    Settings    SettingsStore
    Diagnostics network.Diagnostics
    Host        HostNetwork
    Tools       ToolRunner
    WiFi        WiFiProvider
    Capture     CaptureProvider
    Clock       func() time.Time // nil = time.Now().UTC()
}
```

Types moved into `internal/app`, with JSON tags copied byte for byte:

```go
// Settings moves from storage.Settings.
type Settings struct {
    Language      string `json:"language"`
    RetentionDays int    `json:"retention_days"`
}

// HostAdapter and HostSnapshot move from platform.Adapter / platform.Snapshot.
type HostAdapter struct {
    Index    uint32   `json:"index"`
    Name     string   `json:"name"`
    Gateways []string `json:"gateways"`
    DNS      []string `json:"dns"`
}

type HostSnapshot struct {
    Adapters []HostAdapter     `json:"adapters"`
    Counters []network.Counter `json:"counters"`
}
```

Also move `Tool`, `Nmap`, `NmapFinished`, `NmapState`, `NmapAddress`, `NmapService`, `NmapPort` and `NmapHost` from `internal/adapter/tools/tools.go:13-18,46-79` into `internal/app/tools.go`. Copy them verbatim, including every `xml:"..."` tag and `XMLName xml.Name`. The xml tags are the parse contract of `tools.ParseNmap`. H06 separates transport DTOs from them.

### 4.2 Capture port signature change

`capture.LiveProvider` is unused today. Change it so the provider returns the retained artifact, and add the native provider:

```go
// internal/capture/live.go
type LiveProvider interface {
    Capture(ctx context.Context, r LiveRequest) (LiveResult, *Artifact, error)
}

// Native is the OS live-capture provider selected by build constraints
// (live_windows.go / live_other.go).
type Native struct{}

func (Native) Capture(ctx context.Context, r LiveRequest) (LiveResult, *Artifact, error) {
    return LiveSession(ctx, r)
}
```

### 4.3 Adapters and providers

#### `internal/adapter/storage`

It imports `internal/app` and `internal/job` for types only. This is allowed by docs/architecture.md:112 ("app (except for types)").

```go
type Job = job.Status        // alias keeps storage tests and SQL code compiling
type Settings = app.Settings // alias; validateSettings stays here
```

- `PutJob` passes `string(j.ID)` and `string(j.State)` to `ExecContext`.
- `History` scans `id` and `state` into local `string` variables and assigns `job.ID(id)` and `job.State(state)`.
- The SQL text does not change.

#### `internal/adapter/platform/platform.go`

```go
type Adapter = app.HostAdapter
type Snapshot = app.HostSnapshot

// Host adapts the build-tagged Inventory to app.HostNetwork.
type Host struct{}

func (Host) Inventory(ctx context.Context) (app.HostSnapshot, error) {
    if err := ctx.Err(); err != nil {
        return app.HostSnapshot{}, err
    }
    return Inventory()
}
```

The OS files (`platform_windows.go`, `platform_linux.go`, `platform_other.go`) stay unchanged.

#### `internal/adapter/tools/tools.go`

- Replace the struct definitions with aliases: `type Tool = app.Tool`, `type Nmap = app.Nmap` and so on for the six `Nmap*` types.
- Add the runner:

```go
// Runner adapts the package functions to app.ToolRunner.
type Runner struct{}

func (Runner) Inventory(ctx context.Context) []app.Tool { return Inventory(ctx) }
func (Runner) RunNmap(ctx context.Context, cidr string, ports []int) (app.Nmap, error) {
    return RunNmap(ctx, cidr, ports)
}
```

#### `internal/wifi/wifi.go`

It is not build-tagged.

```go
// Native adapts the build-tagged Current/Reveal to app.WiFiProvider.
type Native struct{}

func (Native) Current(ctx context.Context) (Network, error) {
    if err := ctx.Err(); err != nil {
        return Network{}, err
    }
    return Current()
}

func (Native) Reveal(ctx context.Context) (Secret, error) {
    if err := ctx.Err(); err != nil {
        return Secret{}, err
    }
    return Reveal()
}
```

### 4.4 `app.Service` on ports

```go
func New(deps Deps) *Service // replaces New(store, diagnostics, clock) from H02

type Service struct {
    Diagnostics network.Diagnostics // kept exported: app.go:136 and the CLI use it until H04
    Jobs        *job.Manager
    // unexported: deps Deps, plus the H02 lifecycle and capture fields
}

func (s *Service) History(ctx context.Context) ([]job.Status, error)
func (s *Service) ClearHistory(ctx context.Context) error
func (s *Service) Settings(ctx context.Context) (Settings, error)
func (s *Service) SaveSettings(ctx context.Context, v Settings) error
func (s *Service) Start(parent context.Context, r network.Request) (job.Status, error) // was storage.Job
```

- Remove the `Store`, `Counters` and `Live` fields.
- `OnChange` calls `deps.Jobs.PutJob(ctx, st)` directly. No conversion is needed.
- Metrics sample `deps.Host.Inventory(ctx)` and use `.Counters`. `StartMetrics` is a no-op when `deps.Host == nil`.
- `StartCapture` uses `deps.Capture.Capture(ctx, r)`. When `deps.Capture == nil`, it returns `domain.Unsupported("live capture provider is not configured")`.
- `InspectServices` returns `(Nmap, error)` and uses `deps.Tools.RunNmap`. When `deps.Tools == nil`, it returns `domain.Unsupported("tool runner is not configured")`.
- `Close` no longer closes the store. The store belongs to bootstrap.
- `deps.WiFi` is stored for H04. There is no Wi-Fi method in H03.
- Nil `deps.Jobs` or `deps.Settings` makes `New` panic with a clear message. These are programming errors in a composition root.
- After H03, `internal/app` imports nothing under `internal/adapter/`.

### 4.5 Bootstrap wiring and compile-time assertions

`internal/bootstrap/bootstrap.go`:

```go
service := app.New(app.Deps{
    Jobs:        store,
    Settings:    store,
    Diagnostics: network.Diagnostics{Prober: probe.ICMP{}},
    Host:        platform.Host{},
    Tools:       tools.Runner{},
    WiFi:        wifi.Native{},
    Capture:     capture.Native{},
    Clock:       now,
})
application := &app.Application{Service: service}
cleanup := func() error { return errors.Join(application.Close(), store.Close()) } // idempotent via sync.Once
```

- Wrap `cleanup` in `sync.Once` and store its result, so repeated calls return the first result.
- On the retention failure path, call `cleanup` (not only `application.Close`).

`internal/bootstrap/assert.go` is the only place where compile-time assertions live:

```go
var (
    _ app.JobStore        = (*storage.Store)(nil)
    _ app.SettingsStore   = (*storage.Store)(nil)
    _ app.HostNetwork     = platform.Host{}
    _ app.ToolRunner      = tools.Runner{}
    _ app.WiFiProvider    = wifi.Native{}
    _ app.CaptureProvider = capture.Native{}
    _ network.Prober      = probe.ICMP{}
)
```

### 4.6 Driving roots in H03

- `app.go`:
  - `History`, `ClearHistory`, `Settings` and `SaveSettings` call the new `Service` methods.
  - The binding return types become `job.Status`, `[]job.Status` and `app.Settings`.
  - `Inventory`, `Tools` and `InspectServices` keep their code. Their types are now aliases of `app.*`.
  - `app.go` no longer imports `internal/adapter/storage`.
- `cmd/kali-tools/main.go` needs no change beyond compiling, because the types are aliases.
- Regenerate the bindings with `node scripts/task.mjs bindings` (or `wails generate module -tags desktop`) and commit the regenerated `frontend/wailsjs`. The expected namespace changes are:

| Before | After |
| --- | --- |
| `storage.Job` | `job.Status` |
| `storage.Settings` | `app.Settings` |
| `platform.Snapshot` | `app.HostSnapshot` |
| `tools.Tool` | `app.Tool` |
| `tools.Nmap` | `app.Nmap` |

  Field names and JSON keys are identical.

### 4.7 Preserved constraints

| Constraint | How H03 keeps it |
| --- | --- |
| Max 8 concurrent jobs | Unchanged (`job.Limits{}` in `app.New`) |
| Terminal-event delivery | Unchanged (`job.TerminalWait`) |
| No secrets or targets in history | `JobStore` receives only `job.Status`; `WiFiProvider.Reveal` output never flows to `JobStore` (TestRevealNeverReachesJobStore arrives in H04; H03 has no Reveal use case) |
| Wails event names `job`, `metrics`, `capture` | Unchanged |
| CLI envelope with `schema_version` | Unchanged; the CLI JSON is byte-identical because only Go type names change |

## 5. Files

Create:

- `internal/app/ports.go`: the ports and `Deps`.
- `internal/app/types.go`: `Settings`, `HostAdapter`, `HostSnapshot`.
- `internal/app/tools.go`: `Tool` and the `Nmap*` types (moved).
- `internal/app/fakes_test.go`: `package app_test`; hand-written in-memory fakes `fakeJobStore` (`map[job.ID]job.Status`, `History` ordered by `Created` descending, limit 200), `fakeSettings`, `fakeHost`, `fakeTools`, `fakeWiFi`, `fakeCapture`.
- `internal/bootstrap/assert.go`: the compile-time assertions.

Modify:

- `internal/app/service.go`, `metrics.go`, `capture.go`: use `deps`; add the history and settings methods; `New(deps Deps)`.
- `internal/app/service_test.go`, `metrics_test.go`, `capture_test.go`: build the service with `app.New(app.Deps{...})`. Tests that used `storage.Open` keep the real store and close it themselves (`t.Cleanup(func(){ _ = store.Close() })`). Tests of metrics and capture use `fakeHost` and `fakeCapture` instead of `Counters` and `Live`.
- `internal/adapter/storage/storage.go`: aliases and explicit conversions (§4.3).
- `internal/adapter/platform/platform.go`: aliases and `Host`.
- `internal/adapter/tools/tools.go`: aliases and `Runner`.
- `internal/capture/live.go`: `LiveProvider` signature and `Native`.
- `internal/wifi/wifi.go`: `Native`.
- `internal/bootstrap/bootstrap.go`: `Deps` wiring and cleanup that closes the store.
- `internal/bootstrap/roots_test.go`: add `TestCoreDoesNotImportAdapters`.
- `app.go`: history and settings through `Service`; drop the `storage` import.
- `frontend/wailsjs/**`: regenerated, never hand-edited.
- `docs/architecture.md`: see §10.

Delete: nothing.

## 6. Implementation steps

Each step leaves `go vet ./... && go vet -tags desktop . && go test ./...` green.

1. Add `internal/app/types.go` and `internal/app/tools.go` (copied types). In `platform.go` and `tools.go`, replace the definitions with aliases. Run `go test ./internal/adapter/... ./cmd/...`.
2. Add `app.Settings`. In storage, alias `Settings` and `Job` and add the explicit string conversions. Run `go test ./internal/adapter/storage`. All five storage tests must pass unchanged.
3. Add `internal/app/ports.go`, `platform.Host`, `tools.Runner`, `wifi.Native`, and the `capture.LiveProvider` change with `capture.Native`. Add `internal/bootstrap/assert.go`. Run `go vet` for all GOOS targets.
4. Switch `app.Service` to `New(deps Deps)`, remove the adapter imports, add the history and settings methods, and update `internal/bootstrap` and the `internal/app` tests. Run `go test -race ./internal/app ./internal/bootstrap`.
5. Update `app.go` (history and settings through `Service`) and regenerate the bindings. Run `cd frontend && bun run typecheck && bun run build && bunx playwright test`.
6. Add `TestCoreDoesNotImportAdapters`. Run `go test ./internal/bootstrap`.
7. Update the docs (§10).
8. Write `docs/verification/specs/H03.md`, following README §5.3, with the real outputs of §9 and §12.

## 7. Acceptance criteria

- **AC-1 (Done 2026-10-21):** Given `network.Diagnostics{}` with no `Prober`, when a `ping` runs, then the result is `failed` with `error_code` `unsupported`. Given a fake `Prober`, a `traceroute` uses it with `trace == true`.
- **AC-2:** Given the non-test Go files of `internal/app`, `internal/job`, `internal/network`, `internal/capture` and `internal/wifi` (all GOOS variants), when their imports are parsed, then none starts with `github.com/AlexGalhardo/kali-tools/internal/adapter/`.
- **AC-3:** Given `internal/bootstrap/assert.go`, when the module compiles for `windows`, `linux` and `darwin`, then every adapter satisfies its port.
- **AC-4:** Given `app.New(app.Deps{...})` built only from the fakes in `fakes_test.go`, when a diagnostic job runs to completion, then `fakeJobStore` holds exactly one status, in state `completed`, with summary `Diagnostic completed`. The target string (`synthetic-target.invalid`) does not appear in any stored status.
- **AC-5:** Given fakes, when `Discover` runs against a local listener, then the stored summary is exactly `"Discovery completed: 1 endpoints checked, 1 open"`, with no address. When `InspectServices` runs with a `fakeTools` that returns `Nmap{Version: "7.95", Hosts: 2 hosts}`, then the summary is `"Service inspection completed: Nmap 7.95: 2 hosts"`.
- **AC-6:** Given `Deps{Tools: nil}`, `InspectServices` returns a `*domain.Error` with code `unsupported` after validation. Given `Deps{Capture: nil}`, `StartCapture` with a valid request returns code `unsupported`.
- **AC-7:** Given `fakeHost` and a manual tick, `metrics` events carry rates computed from `fakeHost` counters. Given `fakeCapture`, the capture session behaves as H02 AC-15.
- **AC-8:** Given the real store, `History`, `Clear`, `Retain`, `Settings` and `SaveSettings` keep their behavior. All five `internal/adapter/storage` tests pass unchanged, and the JSON of `job.Status` loaded from SQLite equals the pre-H03 `storage.Job` JSON for the same row.
- **AC-9:** Given bootstrap cleanup called twice, both calls return the same result, and the store is closed exactly once.
- **AC-10:** Given regenerated bindings, `frontend/wailsjs` differs only by the namespaces listed in §4.6. `bun run typecheck`, `bun run build` and every Playwright workflow pass.
- **AC-11 [native]:** Given the Windows desktop, history, settings, inventory, the tools page, inspection and capture behave as before. `TestNativeBridge` passes.

## 8. Tests

| Test | Package | Asserts | AC |
| --- | --- | --- | --- |
| `TestICMPWithoutProberIsUnsupported`, `TestICMPUsesInjectedProber` | `internal/network` | Existing, unchanged | AC-1 |
| `TestCoreDoesNotImportAdapters` | `internal/bootstrap` (`roots_test.go`) | `go/parser` with `parser.ImportsOnly` over `../app`, `../job`, `../network`, `../capture`, `../wifi`, skipping `_test.go` | AC-2 |
| (compile) `assert.go` | `internal/bootstrap` | `go vet` with `GOOS=windows`, `GOOS=linux` and `GOOS=darwin` | AC-3 |
| `TestDiagnosticJobWithFakes` | `internal/app` | Fake store contents, summary, no target | AC-4 |
| `TestDiscoverySummarySurvivesRestartWithoutTargets` | `internal/app` | Existing assertions; real store, closed by the test | AC-5 |
| `TestInspectServicesUsesToolRunner` | `internal/app` | Summary text, no CIDR in history | AC-5 |
| `TestMissingProvidersAreUnsupported` | `internal/app` | `unsupported` codes | AC-6 |
| `TestMetricsPublishesOnTick`, `TestCaptureSessionStates` | `internal/app` | Rewired to `fakeHost` and `fakeCapture`, same assertions | AC-7 |
| `TestPersistenceRecoveryAndRetention`, `TestConcurrentReadsAndCancelledWrite`, `TestMigrationRollback`, `TestStoredSettingsAreValidatedAndBounded`, `TestRenamedApplicationPreservesExistingData` | `internal/adapter/storage` | Unchanged | AC-8 |
| `TestJobStatusJSONMatchesLegacyRow` | `internal/adapter/storage` | `json.Marshal` of a loaded row equals the literal `{"id":"1","kind":"tcp","state":"completed","created":"...","finished":"...","summary":"x"}` | AC-8 |
| `TestCleanupIsIdempotent` | `internal/bootstrap` | Extended: a second call returns the first result | AC-9 |
| Playwright `frontend/tests/workflows.spec.ts` | frontend | All workflows | AC-10 |

These existing tests must keep passing:

- `internal/app`: `TestCancellationAndRedaction`, `TestConcurrentDiagnosticsBoundAndShutdown`, `TestCloseIsIdempotent`, `TestTerminalEventSurvivesFullEventBuffer`;
- all `internal/job` tests;
- all `cmd/kali-tools` tests;
- `internal/adapter/tools`: `TestNmapParser`, `FuzzNmap` seed corpus, `TestSupportedToolVersions`, `TestNmapScopeAndStructuredOutput`, `TestCommandBoundsAndErrors`;
- `internal/capture`: `TestCaptureRequestBounds`, `TestArtifactRoundTripAndOwnedBytes`.

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
GOOS=windows go vet ./...
go list -deps ./internal/app ./internal/job ./internal/network ./internal/capture ./internal/wifi | grep internal/adapter   # must print nothing
go test -race ./internal/app ./internal/bootstrap ./internal/adapter/storage
node scripts/task.mjs bindings && git diff --stat -- frontend/wailsjs   # only the namespace changes of §4.6
node scripts/task.mjs fuzz
node scripts/task.mjs bridge   # [native] Windows only
```

## 10. Documentation updates

- `docs/architecture.md`:
  - §4.2: replace the table with the implemented ports (`JobStore`, `SettingsStore`, `HostNetwork`, `ToolRunner`, `WiFiProvider`, `CaptureProvider` = `capture.LiveProvider`, `network.Prober`) and their implementations (`*storage.Store`, `platform.Host`, `tools.Runner`, `wifi.Native`, `capture.Native`, `probe.ICMP`).
  - §4.3: mark every row done; `ServiceInspector` is folded into `ToolRunner`.
  - §8: mark D5 resolved for the port (contract suites remain H07). D6 stays open.
- `PLAN.md`: tick the three H03 sub-items with evidence. The `Prober` part may already be ticked.

## 11. Risks and rollback

- **Binding namespace churn.** Mitigation: regenerate with the task runner; Playwright and typecheck pass; the JSON is unchanged (AC-8, AC-10).
- **Moving the `xml`-tagged `Nmap` types into `app`** puts parser tags in the application layer. This is accepted until H06. `FuzzNmap` and `TestNmapParser` guard the parse contract.
- **An import cycle if a future `package app` internal test imports `storage`.** Mitigation: keep the adapter-backed tests in `package app_test`, and have internal tests use the fakes only.
- **Rollback:** revert the H03 commits. Nothing persistent changes, and the bindings are regenerated from the reverted Go code.

## 12. Quality and delivery assertions

### 12.1 Applicable gates

| Gate | Applies | Spec threshold |
| --- | --- | --- |
| QG-1 Formatting | yes | empty diff |
| QG-2 Lint | yes | `0 issues.` |
| QG-3 Vet on all targets | yes, tightened | also `GOOS=windows go vet ./...` (the compile-time assertions cover every OS file) |
| QG-4 Tests | yes | all pass |
| QG-5 Race | yes | no race reports in `./internal/app`, `./internal/bootstrap`, `./internal/adapter/storage` |
| QG-6 Coverage | yes | `internal/app` ≥ 80 % or not below the recorded pre-spec value, whichever is higher; `internal/adapter/storage`, `platform`, `tools` not below the pre-spec values |
| QG-7 Vulnerabilities | yes | no new dependency |
| QG-8 Frontend | yes | all pass; workflow count unchanged |
| QG-9 Binding drift | yes | after regeneration, `git diff --exit-code frontend/wailsjs` is empty against the committed result |
| QG-10 Dependency rule | yes | DA-1 and DA-2 |
| QG-11 Secrets | yes | AC-4 (`synthetic-target.invalid` absent from stored statuses) |
| QG-12 Bounds | n.a. | no new bounded resource; existing storage limit (200 rows) unchanged |
| QG-13 Commit | yes | `refactor(app): consumer-side driven ports (H03)` |
| QG-14 Docs | yes | architecture §4.2, §4.3, §8; CHANGELOG entry |
| QG-15 Markdown | yes | `Markdown checks passed` |

### 12.2 Delivery assertions

| DA | Assertion | Check | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | No inner package reaches an adapter | `go list -deps ./internal/app ./internal/job ./internal/network ./internal/capture ./internal/wifi \| grep -c internal/adapter` | `0` | CI |
| DA-2 | The same holds for every GOOS file | `go test ./internal/bootstrap -run 'TestCoreDoesNotImportAdapters' -v` | `PASS` | CI |
| DA-3 | Every adapter satisfies its port on every OS | `GOOS=windows go vet ./internal/bootstrap && GOOS=linux go vet ./internal/bootstrap && GOOS=darwin go vet ./internal/bootstrap` | exit 0 | CI |
| DA-4 | Use cases run fully on fakes | `go test ./internal/app -run 'TestDiagnosticJobWithFakes\|TestInspectServicesUsesToolRunner\|TestMissingProvidersAreUnsupported' -v` | `PASS` | CI |
| DA-5 | Persisted JSON is unchanged | `go test ./internal/adapter/storage -run 'TestJobStatusJSONMatchesLegacyRow' -v` | `PASS` | CI |
| DA-6 | Only binding namespaces changed | `git diff --word-diff -- frontend/wailsjs` reviewed against §4.6 | no field or JSON-key change | reviewer |
| DA-7 | Compile-time assertions live only in bootstrap or tests | `grep -rn '^\s*_ app\.' --include=*.go internal \| grep -v internal/bootstrap/ \| grep -v _test.go` | prints nothing | reviewer |
| DA-8 | The desktop works on the new types | `node scripts/task.mjs bridge` and a manual history/settings/inventory/tools check | `PASS` | [native] |

### 12.3 Evidence record

`docs/verification/specs/H03.md`
