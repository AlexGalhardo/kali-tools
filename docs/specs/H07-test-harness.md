# H07 Contract and acceptance test harness

| Field | Value |
| --- | --- |
| PLAN tasks | H07 |
| Status | Implementation and non-native acceptance complete 2026-10-21; native AC-10/DA-7 pending ([evidence](../verification/specs/H07.md#continuation-2026-10-21)) |
| Depends on | H03, H04, H05, H06 |
| Risk | Low: tests and test-only packages; production code changes only where a fake reveals a real contract gap, and then STOP and report |

## 1. Goal

Fakes cannot drift from real adapters, and behavior is proven once for every interface. `internal/porttest` holds one exported contract suite per driven port; each suite runs against the real adapter and against its in-memory fake from `internal/porttest/fake`. `tests/acceptance` holds interface-agnostic specifications (diagnostics, discovery, history) executed through interchangeable drivers: the direct application, the CLI adapter and the Wails bindings adapter. W01 later adds an HTTP driver without changing the specifications.

## 2. Non-goals

- Do not write the HTTP driver (W01) or start any HTTP server other than local `httptest` fixtures used as diagnostic targets.
- Do not add CLI commands. `history` arrives in C01; until then the CLI driver reports history as unsupported (§4.4).
- Do not change port interfaces, adapter behavior or `app.Application`. If a fake cannot pass a suite because the port is underspecified, STOP and report.
- Do not claim native acceptance: suites against OS adapters (`HostNetwork`, `WiFiProvider`, `Prober`, `LiveProvider`) assert only honest-state properties on hosts without the capability.
- Do not use `time.Sleep` for synchronization; no unit-test deadline above 3 s.
- Do not add third-party test libraries.

Continuation scope (2026-10-21): the owner requested resuming from the latest commits and reconciling PLAN with SDD. The previously reported ICMP gap is the resumption point: permit `internal/adapter/probe/probe.go` to call the existing `network.ValidateHost` before native delegation, and wire the specified real `TestProberContract`. No port or native implementation changes are needed. The original stop-and-report requirement was fulfilled by the H07 checkpoint; this correction is recorded explicitly rather than extending other adapter behavior.

## 3. Current state

Facts at commit `8cbf25e` (re-verify):

- `internal/porttest/` and `tests/acceptance/` contain only `.gitkeep` (`docs/architecture.md:61`, `:79`).
- `internal/app/service_test.go` builds the service on a real SQLite store (`storage.Open(filepath.Join(t.TempDir(), "db"))`, `service_test.go:20`); there is no fake store and no contract suite (D5).
- `internal/adapter/storage/storage_test.go` covers recovery and retention (`:13`), concurrent reads (`:50`), migration rollback (`:78`), settings bounds (`:96`) and the legacy path (`:122`). `History` is limited to 200 rows (`storage.go:119`), `Clear` and `Retain` keep `queued` and `running` jobs (`storage.go:150`, `:156`), settings default to `{"en", 30}` with retention 1–365 and language `en` or `pt-BR` (`storage.go:166-188`).
- CLI behavior tests run the CLI in-process (`cmd/kali-tools/main_test.go`); after H05 they live in `internal/adapter/cli/cli_test.go` and call `cli.Run`.
- Driven ports at the time this spec starts (H03; verify with `go doc`): `app.JobStore`, `app.SettingsStore`, `app.HostNetwork`, `app.ToolRunner`, `app.WiFiProvider`, `network.Prober`, `capture.LiveProvider`. At `8cbf25e` only `capture.LiveProvider` exists (`internal/capture/live.go:27-29`: `Capture(context.Context, LiveRequest) (LiveResult, error)`); `network` still imports `internal/adapter/probe` (`internal/network/network.go:18`, `:151-152`), which H03 removes.
- Tool states produced by `internal/adapter/tools/tools.go:20-36`: `dependency_missing`, `failed`, `available`, `unsupported`; `RunNmap` rejects a scope before executing anything (`internal/adapter/tools/nmap.go` via `nmapArguments`) and returns `&domain.Error{Code: tool.State}` when Nmap is not usable (`tools.go:55-58`).
- `internal/network.ValidateDiscovery` rejects anything other than an IPv4 prefix of /24 or narrower (`network.go:214-218`).
- H06 delivered `internal/adapter/contract` (DTOs, `FromError`) and `wailsui.NewWithRuntime`.

Port method sets this spec expects from H03 (if H03 named a method differently, use H03's name and record it under "Deviations"; the behaviors below do not change):

| Port | Methods (expected) |
| --- | --- |
| `app.JobStore` | `PutJob(ctx, job.Status) error`, `History(ctx) ([]job.Status, error)`, `Clear(ctx) error`, `Retain(ctx, before time.Time) error` |
| `app.SettingsStore` | `Settings(ctx) (app.Settings, error)`, `SaveSettings(ctx, app.Settings) error` |
| `app.HostNetwork` | `Inventory(ctx) (<snapshot>, error)` with adapters and counters |
| `app.ToolRunner` | `Inventory(ctx) []<tool>`, `InspectServices(ctx, cidr string, ports []int) (<inspection>, error)` |
| `app.WiFiProvider` | `Current(ctx) (wifi.Network, error)`, `Reveal(ctx) (wifi.Secret, error)` |
| `network.Prober` | the H03 echo/trace method(s) returning values or a `*domain.Error` |
| `capture.LiveProvider` | `Capture(ctx, capture.LiveRequest) (capture.LiveResult, error)` |

## 4. Design

### 4.1 Package layout

```text
internal/porttest/            package porttest: exported contract suites (non-test files that import "testing")
    jobstore.go               JobStoreContract
    settings.go               SettingsStoreContract
    host.go                   HostNetworkContract
    tools.go                  ToolRunnerContract
    wifi.go                   WiFiProviderContract
    prober.go                 ProberContract
    capture.go                LiveProviderContract
    states.go                 shared assertions (honest-state helper)
internal/porttest/fake/       package fake: in-memory fakes, one per port
    store.go                  Store (JobStore + SettingsStore)
    host.go, tools.go, wifi.go, prober.go, capture.go
tests/acceptance/             package acceptance, test files only
    driver_test.go            Driver interface, errUnsupported
    app_test.go               newApplication(t)
    direct_driver_test.go     directDriver
    cli_driver_test.go        cliDriver
    wails_driver_test.go      wailsDriver
    diagnostics_test.go       diagnostics specification
    discovery_test.go         discovery specification
    history_test.go           history specification
    acceptance_test.go        TestAcceptance: specifications × drivers
```

Only `_test.go` files may import `internal/porttest` or `internal/porttest/fake` (enforced by H08). `tests/acceptance` has no non-test files.

### 4.2 Contract suites (`internal/porttest`)

Every suite has this shape and runs each case as a `t.Run` subtest with a fresh port value:

```go
func JobStoreContract(t *testing.T, newStore func(t *testing.T) app.JobStore)
func SettingsStoreContract(t *testing.T, newStore func(t *testing.T) app.SettingsStore)
func HostNetworkContract(t *testing.T, newHost func(t *testing.T) app.HostNetwork)
func ToolRunnerContract(t *testing.T, newRunner func(t *testing.T) app.ToolRunner)
func WiFiProviderContract(t *testing.T, newProvider func(t *testing.T) app.WiFiProvider)
func ProberContract(t *testing.T, newProber func(t *testing.T) network.Prober)
func LiveProviderContract(t *testing.T, newProvider func(t *testing.T) capture.LiveProvider)

// Limits mirrored from the real store; the suite asserts them at the boundary.
const (
    HistoryLimit        = 200
    MinRetentionDays    = 1
    MaxRetentionDays    = 365
    contractDeadline    = 3 * time.Second
)

// RequireHonestState fails unless err is nil or a *domain.Error whose code is in allowed.
func RequireHonestState(t *testing.T, err error, allowed ...string)
```

`HistoryLimit`, `MinRetentionDays` and `MaxRetentionDays` must equal the constants of the real store; if the store still uses literals (`storage.go:119`, `:184`), export them there as named constants in the same commit (`storage.HistoryLimit`, `storage.MinRetentionDays`, `storage.MaxRetentionDays`) and reference them from a storage test that asserts equality with the `porttest` values. This is the only production change allowed.

Cases (subtest names in backticks):

`JobStoreContract`:

- `put_then_history`: `PutJob` queued, running, completed for one ID; `History` returns one row with the last state, `Finished` set, `Created` equal (`time.Equal`) to the first put, UTC.
- `history_newest_first_and_bounded`: put `HistoryLimit+1` completed jobs with strictly increasing `Created`; `History` returns exactly `HistoryLimit` rows, newest first. A second store with exactly `HistoryLimit` jobs returns all of them.
- `clear_keeps_active`: one job per state (`queued`, `running`, `completed`, `failed`, `cancelled`); after `Clear`, only `queued` and `running` remain.
- `retain_drops_old_terminal_jobs`: terminal and active jobs before and after a cut time; after `Retain(cut)`, old terminal jobs are gone, old active jobs and newer jobs remain.
- `cancelled_context_writes_nothing`: `PutJob` with an already cancelled context returns a non-nil error and `History` (with a live context) is empty.
- `status_has_no_target_fields`: `reflect` over `job.Status` lists exactly `ID`, `Kind`, `State`, `Created`, `Finished`, `Summary`; a store cannot persist targets, keys or payloads because the type has no field for them.
- `round_trip_is_verbatim`: a summary containing `synthetic-psk` written by the test is returned unchanged (the store neither adds nor strips data; redaction happens upstream and is covered by app tests).

`SettingsStoreContract`:

- `defaults`: fresh store returns `{Language: "en", RetentionDays: 30}`.
- `save_then_load`: `{pt-BR, 365}` round-trips.
- `retention_bounds`: `MinRetentionDays` and `MaxRetentionDays` save; `MinRetentionDays-1` and `MaxRetentionDays+1` fail with `domain.CodeInvalidInput`.
- `language_allow_list`: `fr` fails with `domain.CodeInvalidInput`.
- `rejected_save_keeps_previous`: after a failed save, `Settings` returns the last valid value.

`HostNetworkContract`:

- `inventory_or_honest_state`: within `contractDeadline`, either no error and every counter has a non-empty `ID` and every adapter a non-empty `Name`, or `RequireHonestState(err, unsupported, permission_required)`.
- `cancelled_context_returns`: a cancelled context returns within `contractDeadline`.

`ToolRunnerContract`:

- `inventory_lists_known_tools`: exactly the names `nmap` and `tshark`; each state is one of `available`, `dependency_missing`, `failed`, `unsupported`; when state is not `available`, `Version` is empty.
- `rejects_scope_before_execution`: `InspectServices(ctx, "0.0.0.0/0", []int{80})` fails with `invalid_input` within `contractDeadline`.
- `unavailable_tool_is_a_typed_state`: when `nmap` is not `available`, `InspectServices(ctx, "127.0.0.1/32", []int{80})` fails with the same code as the inventory state.

`WiFiProviderContract`:

- `current_or_honest_state`: no error with non-empty `State`, or `RequireHonestState(err, unsupported, permission_required, no_connection, location_consent_required)`.
- `reveal_states`: no error with `State` one of `available`, `permission_required`, `no_saved_key`, `open_network`, `enterprise` (and `Value` empty unless `available`), or an honest-state error as above. The suite never prints `Value`.

`ProberContract`:

- `rejects_invalid_target`: target `a;whoami` fails with `invalid_input` or `unsupported` and never with `failed`.
- `loopback_or_honest_state`: echo to `127.0.0.1` with a 1 s budget either returns at least one value or `RequireHonestState(err, unsupported, permission_required, timeout)`.

`LiveProviderContract`:

- `honest_state_without_permission`: a valid request for the first non-loopback local IPv4 (skip with a reason when none) with `DurationSeconds: 1`, `PacketLimit: 1` returns within `DurationSeconds + 2 s` either a result with `len(Packets) <= PacketLimit` or `RequireHonestState(err, unsupported, permission_required)`.
- `result_carries_no_payload_field`: `reflect` over `capture.Packet` has no `[]byte` field.

### 4.3 Fakes (`internal/porttest/fake`)

```go
package fake

// Store is an in-memory JobStore and SettingsStore guarded by a sync.Mutex.
type Store struct { /* jobs map[string]job.Status; settings app.Settings; ... */ }
func NewStore() *Store

type Host struct {
    Snapshot <snapshot>  // returned when Err is nil
    Err      error
}
type Tools struct {
    Tools  []<tool>
    Result <inspection>
    Err    error
}
type WiFi struct {
    Network wifi.Network
    Secret  wifi.Secret
    Err     error
}
type Prober struct {
    Values []string
    Err    error
}
type LiveCapture struct {
    Result capture.LiveResult
    Err    error
}
```

Rules: every fake method checks `ctx.Err()` first and returns it wrapped with `%w`; the store enforces the same limits and validation as the real store (it imports the `porttest` constants or the storage constants through `porttest`, never `internal/adapter/storage`); returned slices are copies; validation that belongs to the domain (`network.ValidateDiscovery`, `capture.ValidateLiveRequest`) is called by the fake exactly where the real adapter calls it. A fake that needs different behavior per test exposes exported fields, not setters with logic.

If H03 or H04 created fakes inside `internal/app` test files, move them into `fake` and update those tests to import it. Tests that import `fake` must be in an external test package (`package app_test`), because `fake` imports `app`.

Wiring of suites (one test per pair):

| Test (file) | Real adapter | Fake |
| --- | --- | --- |
| `TestJobStoreContract` (`internal/adapter/storage/contract_test.go`) | `storage.Open(filepath.Join(t.TempDir(), "db"))` with `t.Cleanup(Close)` | — |
| `TestSettingsStoreContract` (same file) | same | — |
| `TestFakeStoreContracts` (`internal/porttest/fake/fake_test.go`) | — | `fake.NewStore()` for both suites |
| `TestHostNetworkContract` (`internal/adapter/platform/contract_test.go`) | H03 platform adapter | — |
| `TestToolRunnerContract` (`internal/adapter/tools/contract_test.go`) | H03 tools adapter | — |
| `TestProberContract` (`internal/adapter/probe/contract_test.go`) | `probe.ICMP{}` (or the H03 name) | — |
| `TestWiFiProviderContract` (`internal/wifi/contract_test.go`, `package wifi_test`) | H03 native provider | — |
| `TestLiveProviderContract` (`internal/capture/contract_test.go`, `package capture_test`) | H03 live provider | — |
| `TestFakePortContracts` (`internal/porttest/fake/fake_test.go`) | — | `Host`, `Tools`, `WiFi`, `Prober`, `LiveCapture`, each configured twice: once returning data, once returning an honest-state error |

`internal/wifi` and `internal/capture` tests importing `porttest` (which imports `app`) are allowed because they are test files in external test packages; H08 encodes that.

### 4.4 Acceptance harness (`tests/acceptance`)

```go
package acceptance

var errUnsupported = errors.New("driver does not support this operation yet")

// Driver is one way of reaching the application. Every method returns either a
// result or a *contract.Error (never both), or errUnsupported.
type Driver interface {
    Name() string
    Diagnose(ctx context.Context, r contract.DiagnosticRequest) (contract.DiagnosticResult, error)
    Discover(ctx context.Context, cidr string, ports []int, timeout time.Duration) ([]contract.PortResult, error)
    History(ctx context.Context) ([]contract.Job, error)
    ClearHistory(ctx context.Context) error
}

// newApplication builds the real application exactly like the composition roots,
// with the data location in t.TempDir(), and registers the cleanup.
func newApplication(t *testing.T) *app.Application

type driverFactory func(t *testing.T, a *app.Application) Driver
```

Drivers:

| Driver | Implementation | Unsupported until |
| --- | --- | --- |
| `direct` | calls `a.Diagnose`, `a.Discover`, `a.History`, `a.ClearHistory`; maps with `contract.From*`; errors with `contract.FromError` | — |
| `cli` | calls `cli.Run(ctx, a, args, &stdout, &stderr)`; `diagnose --kind K --target T --port P --timeout MS [--record R]`; `discover --target CIDR --ports a,b --timeout MS`; decodes `{schema_version, version, data json.RawMessage, error *contract.Error}`; asserts `schema_version == 1`, exit 2 iff `error.code == invalid_input`, exit 0 iff no error | `History`, `ClearHistory` return `errUnsupported` until C01 adds `history list` and `history clear`; C01 must replace them and delete this row |
| `wails` | `b := wailsui.NewWithRuntime(a, wailsui.Runtime{Emit: func(context.Context, string, ...any) {}})`, `b.Startup(ctx)`, `t.Cleanup(func() { b.Shutdown(ctx) })`; calls `b.Diagnose`, `b.Discover` (the binding's fixed 500 ms timeout; the `timeout` argument is ignored), `b.History`, `b.ClearHistory`; errors via `errors.As` to `contract.Error` | — |
| `http` | added by W01 in `tests/acceptance/http_driver_test.go` | W01 |

Specifications (each a `func(t *testing.T, d Driver)`; each scenario builds its own application and driver):

- `DiagnosticsSpec`:
  - `http_target_reports_status`: an `httptest` server answering 204; `Diagnose{Kind: "http", Target: server.URL, TimeoutMS: 3000}` returns `Status` `completed` and a value containing `204 No Content`.
  - `rejects_command_like_target`: `Diagnose{Kind: "tcp", Target: "a;whoami", Port: 443, TimeoutMS: 3000}` returns `invalid_input`; `History` (where supported) has no row.
  - `timeout_bounds`: `TimeoutMS` 99 and 30001 return `invalid_input`; 100 and 30000 are accepted (target `127.0.0.1`, closed port obtained by opening and closing a listener).
- `DiscoverySpec`:
  - `finds_open_loopback_port`: a `net.Listen("tcp", "127.0.0.1:0")` listener; `Discover("127.0.0.1/32", []int{port}, 500ms)` returns exactly `{127.0.0.1, port, open}`.
  - `rejects_wide_scope`: `0.0.0.0/0` and `10.0.0.0/23` return `invalid_input` and no result.
  - `port_count_bounds`: 32 unique ports accepted, 33 return `invalid_input`.
- `HistorySpec` (skips with `t.Skip` and the reason "history: CLI command arrives in C01" when the driver returns `errUnsupported`):
  - `discovery_summary_without_target`: after the discovery scenario, `History` holds one job with `Kind` `discovery`, `State` `completed`, `Summary` `Discovery completed: 1 endpoints checked, 1 open`, and no field of the marshalled job contains `127.0.0.1`.
  - `clear_removes_terminal_jobs`: after `ClearHistory`, `History` is empty.

```go
func TestAcceptance(t *testing.T) {
    drivers := map[string]driverFactory{"direct": newDirect, "cli": newCLI, "wails": newWails}
    specs := map[string]func(*testing.T, Driver){
        "diagnostics": DiagnosticsSpec, "discovery": DiscoverySpec, "history": HistorySpec,
    }
    // for each spec name, for each driver name: t.Run(spec+"/"+driver, ...)
}
```

Iterate in sorted key order so output is deterministic. History therefore runs through `direct` and `wails` (two drivers) and diagnostics and discovery through all three.

## 5. Files

Create:

- `internal/porttest/doc.go`: package comment (test-only package; imported only from `_test.go` files).
- `internal/porttest/jobstore.go`, `settings.go`, `host.go`, `tools.go`, `wifi.go`, `prober.go`, `capture.go`, `states.go`: suites and helpers.
- `internal/porttest/fake/store.go`, `host.go`, `tools.go`, `wifi.go`, `prober.go`, `capture.go`, `fake_test.go`: fakes and their suite runs.
- `internal/adapter/storage/contract_test.go`, `internal/adapter/platform/contract_test.go`, `internal/adapter/tools/contract_test.go`, `internal/adapter/probe/contract_test.go`, `internal/wifi/contract_test.go`, `internal/capture/contract_test.go`: real-adapter suite runs.
- `tests/acceptance/*_test.go`: the files of §4.1.

Modify:

- `internal/adapter/storage/storage.go`: only to export `HistoryLimit`, `MinRetentionDays`, `MaxRetentionDays` if they are still literals (§4.2).
- `internal/app/*_test.go`: only if H03/H04 fakes move into `fake` (§4.3).
- `internal/adapter/probe/probe.go`: only the continuation correction described in §2.
- `docs/architecture.md`, `docs/architecture-research.md`, `PLAN.md`: §10.

Delete: `internal/porttest/.gitkeep`, `tests/acceptance/.gitkeep`.

## 6. Implementation steps

1. **Store suites.** Write `JobStoreContract`, `SettingsStoreContract` and `RequireHonestState`; run them against the real store. Export the store limits if needed. `go test ./internal/adapter/storage` passes.
2. **Fake store.** Write `fake.Store` and `TestFakeStoreContracts`. Move any existing app-test fakes into `fake`. `go test ./internal/porttest/... ./internal/app/...` passes.
3. **OS and tool port suites.** Write the five remaining suites and fakes, then the real-adapter test files. On the development host, record which subtests returned data and which returned an honest state. `go test ./...` passes on Windows; `GOOS=linux go vet ./...` and `GOOS=darwin go vet ./...` pass.
4. **Acceptance drivers.** Write `Driver`, `newApplication`, the three drivers and `TestAcceptance` with the diagnostics specification only. `go test ./tests/acceptance` passes.
5. **Discovery and history specifications.** Add them. `go test -race ./tests/acceptance` passes on CI Linux.
6. **Docs.** Apply §10. Run the full §9 block.
7. **Evidence.** Write `docs/verification/specs/H07.md` with the README §5.3 structure, including the per-host table of honest-state versus data subtests from step 3.

Commit: `test(porttest): add port contract suites and acceptance drivers (H07)`, or one commit per step with the same scope.

## 7. Acceptance criteria

- AC-1: Given the real SQLite store and `fake.Store`, when `JobStoreContract` and `SettingsStoreContract` run, then both pass every subtest.
- AC-2: Given `HistoryLimit` and the retention limits, when the suites run, then the boundary value and boundary + 1 are both exercised and the porttest constants equal the storage constants.
- AC-3: Given every driven port listed in §3, when `go test ./...` runs, then each port has exactly one suite in `internal/porttest`, run by one real-adapter test and by `TestFakeStoreContracts` or `TestFakePortContracts`.
- AC-4: Given a host without a capability (for example Linux CI for Wi-Fi, ICMP and live capture), when the OS port suites run against the real adapter, then they pass through the honest-state branch and never accept a `failed` code where an explicit state is required.
- AC-5: Given `TestAcceptance`, when it runs, then `diagnostics` and `discovery` pass through `direct`, `cli` and `wails`, and `history` passes through `direct` and `wails` and is skipped for `cli` with the C01 reason.
- AC-6: Given a discovery on `127.0.0.1/32`, when history is listed through any supporting driver, then the job summary has counts only and no marshalled field contains `127.0.0.1`.
- AC-7: Given the CLI driver, when an operation fails with `invalid_input`, then the process exit code is 2, and for any other error it is 1.
- AC-8: Given the source tree, when non-test files are searched, then no file outside `internal/porttest` imports `internal/porttest` or `internal/porttest/fake`.
- AC-9: Given the test suite, when searched, then no new test in this spec calls `time.Sleep`, and every wait uses a deadline of at most 3 s.
- AC-10 [native]: Given Windows with Wi-Fi, ICMP permission and an administrator session, when the OS port suites run, then the data branches pass (recorded in the evidence file with OS build and hardware).

## 8. Tests

| Test | Package | Asserts | AC |
| --- | --- | --- | --- |
| `TestJobStoreContract`, `TestSettingsStoreContract` | `internal/adapter/storage` | §4.2 cases on SQLite | AC-1, AC-2 |
| `TestStoreLimitsMatchContract` | `internal/adapter/storage` (`package storage_test`) | `storage.HistoryLimit == porttest.HistoryLimit` and the retention pair | AC-2 |
| `TestFakeStoreContracts` | `internal/porttest/fake` | §4.2 store cases on the fake | AC-1 |
| `TestFakePortContracts` | `internal/porttest/fake` | the five OS and tool suites on both fake configurations | AC-3 |
| `TestHostNetworkContract`, `TestToolRunnerContract`, `TestProberContract`, `TestWiFiProviderContract`, `TestLiveProviderContract` | respective adapter packages | §4.2 cases on real adapters | AC-3, AC-4, AC-10 |
| `TestAcceptance/<spec>/<driver>` | `tests/acceptance` | §4.4 scenarios | AC-5, AC-6, AC-7 |
| `TestRequireHonestState` | `internal/porttest` (`package porttest_test`) | nil passes; allowed domain code passes; other code, plain error and wrapped non-domain error fail (using a recording `testing.TB` fake is not possible with `*testing.T`, so test the pure helper `honestState(err error, allowed ...string) error` that `RequireHonestState` wraps) | AC-4 |

AC-8 and AC-9 are checked by the commands in §9.

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
go test -race -timeout 120s ./internal/porttest/... ./internal/adapter/... ./tests/acceptance/...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
```

Spec-specific checks:

```bash
go test -v -run 'TestAcceptance' ./tests/acceptance | grep -E '^\s+--- (PASS|SKIP)'
go test -run 'Contract' -v ./... | grep -cE '^=== RUN\s+Test.*Contract$'
grep -rln --include='*.go' --exclude='*_test.go' 'internal/porttest' . | grep -v '^./internal/porttest/' && exit 1 || true
git diff --name-only HEAD~1 -- '*_test.go' | xargs grep -n 'time.Sleep' && exit 1 || true
```

Binding regeneration is not affected by this spec; `node scripts/task.mjs bindings` must still exit 0 on native CI.

## 10. Documentation updates

- `docs/architecture.md`: §2 map (`porttest` and `porttest/fake` filled, `tests/acceptance` filled); §4.3 last paragraph (suites exist, table of suite ↔ adapter ↔ fake); §8 D5 resolved (with H03).
- `docs/architecture-research.md` §10: replace the illustrative suite example with the real `JobStoreContract` signature and note the `fake` subpackage.
- `PLAN.md` H07: tick boxes with evidence links; note that the HTTP driver is W01 and CLI history is C01.

## 11. Risks and rollback

- Suites against OS adapters can be flaky on hosts with partial capabilities (for example ICMP allowed but slow). Each case has a `contractDeadline`; a timeout is an allowed honest state only where listed.
- Import cycles: a test inside `package app` importing `fake` fails to compile. Keep such tests in `package app_test`.
- The real-store suite uses a temp file; Windows can hold SQLite handles briefly. Always `t.Cleanup(store.Close)` before `t.TempDir` cleanup runs (register cleanup after creating the temp dir).
- The CLI driver skip for history must not outlive C01. C01's spec removes it; the skip message names C01 so a search finds it.
- Rollback: `git revert` the H07 commit(s). Only test code and, at most, three exported constants are affected.

## 12. Quality and delivery assertions

Applicable global gates (README §5.1):

| Gate | Threshold for H07 |
| --- | --- |
| QG-1, QG-2, QG-3, QG-4, QG-7, QG-13, QG-14, QG-15 | as defined; commit messages contain `(H07)` |
| QG-4 | every `t.Skip` added names its reason; the only allowed skips are the C01 CLI-history skip, "no non-loopback IPv4" and the honest-state branches documented in §4.2 |
| QG-5 | `go test -race` on `internal/porttest/...` and `tests/acceptance/...` clean on CI Linux |
| QG-6 | `internal/porttest/fake` ≥ 80 % statements (new package); `internal/porttest` suites are exercised by the fakes and real adapters (record the figure); existing adapter packages not lower than before (record before/after) |
| QG-8 | unchanged Playwright count |
| QG-11 | `status_has_no_target_fields` and `discovery_summary_without_target` pass |
| QG-12 | `HistoryLimit` and retention limits tested at limit and limit + 1 in both store suites |

Delivery assertions:

| DA | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | Each fake passes the same suite as its real adapter | `go test ./internal/porttest/... ./internal/adapter/storage -run 'Contract'` | pass | CI |
| DA-2 | Every driven port has a suite | `ls internal/porttest/*.go` lists the seven suite files of §4.1 | 7 suite files | reviewer |
| DA-3 | Diagnostics, discovery and history run through at least two drivers | `go test -v -run TestAcceptance ./tests/acceptance` | `--- PASS` for 3 diagnostics, 3 discovery and 2 history subtests; exactly 1 `--- SKIP` (cli history) | CI |
| DA-4 | Porttest is test-only | the `grep` in §9 | no match | CI, reviewer |
| DA-5 | No sleep-based synchronization added | the `time.Sleep` grep in §9 | no match | reviewer |
| DA-6 | Store limits are shared constants | `go test ./internal/adapter/storage -run TestStoreLimitsMatchContract` | pass | CI |
| DA-7 | Data branches of OS suites on real hardware | `go test -v -run 'Contract' ./internal/wifi ./internal/capture ./internal/adapter/probe ./internal/adapter/platform` on Windows as administrator with Wi-Fi | each suite reports the data branch | [native] |

Evidence record: `docs/verification/specs/H07.md`.
