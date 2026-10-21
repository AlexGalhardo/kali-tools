# H01 Bootstrap

| Field | Value |
| --- | --- |
| PLAN tasks | H01 |
| Status | Ready |
| Depends on | none |
| Risk | Medium: both composition roots change at once, and the desktop startup path can only be fully proven by the native bridge smoke test |

## 1. Goal

Create `internal/bootstrap`, the single place where the application graph is wired. It opens storage, applies retention and injects the driven adapters that exist today. The Wails desktop root (`main.go` + `app.go`) and the CLI root (`cmd/kali-tools`) both obtain their application value from `bootstrap.New`. Both roots expose the testable `run(ctx, args, getenv, stdout, stderr) int` seam. No package below a composition root reads flags or environment variables. Users see no difference: same bindings, same CLI output, same exit codes.

## 2. Non-goals

- Do not create `internal/job`, consumer-side ports or the full `app.Application` facade. Those are H02, H03 and H04.
- Do not move Wails bindings into `internal/adapter/wailsui`, and do not move the CLI command tree into `internal/adapter/cli` (H05).
- Do not change any Wails binding method signature. `frontend/wailsjs` must not change.
- Do not add CLI commands, flags or environment variables. `getenv` is passed through but H01 reads no variable.
- Do not make the CLI persist history to the user's configuration directory (decided in C01).
- Do not add a DI framework. Wire with plain constructors.
- Do not change the metrics ticker, the job forwarder or the capture session in `app.go` (H02).

## 3. Current state

Verified on 2026-10-21 against the working tree that already contains the `network.Prober` port. Re-verify each line before you start; if one does not hold, stop and report.

- `internal/bootstrap/` contains only `.gitkeep` (docs/architecture.md:60).
- Desktop wiring lives inside the Wails startup hook, in `app.go:37-85` (`(*App).startup`):
  - `app.go:44` resolves the path with `storage.DefaultPath()`.
  - `app.go:49` opens the store with `storage.Open(path)`.
  - `app.go:54` builds `app.New(store, network.Diagnostics{Prober: probe.ICMP{}})`.
  - `app.go:55-58` reads settings; when that read succeeds it calls `store.Retain(a.ctx, time.Now().AddDate(0, 0, -settings.RetentionDays))` and stores the result in `a.initErr`. A settings read error skips retention silently.
  - `app.go:59-84` starts the `job` forwarder and the 2 s `metrics` ticker (unchanged by H01).
- `app.go:86-104` (`shutdown`) closes the service with `a.service.Close()` (line 102), which also closes the store (`internal/app/service.go:150`).
- `app.go:110-121` (`readyLocked`) checks `closed`, then `initErr`, then `service == nil`.
- `app.go:36` declares `func NewApp() *App { return &App{} }`. `bridge_smoke_test.go:59` calls `NewApp()` and `bridge_smoke_test.go:64` calls `app.startup(ctx)` directly, with `APPDATA` redirected by `t.Setenv` (line 43).
- `main.go:17-21`: `main()` calls `NewApp()` and `wails.Run(...)`, and uses `log.Fatal` on error. There is no `run` function.
- The CLI root `cmd/kali-tools/main.go`:
  - `run(ctx context.Context, args []string, out, errout io.Writer) int` is at line 35. It has no `getenv` parameter.
  - It does not open storage at all.
  - `diagnose` builds its own graph: `network.Diagnostics{Prober: probe.ICMP{}}` at line 71.
  - `main()` at lines 175-179 uses `signal.NotifyContext(context.Background(), os.Interrupt)` and `os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))`.
- `cmd/kali-tools/main_test.go` calls `run(t.Context(), args, &out, &stderr)` at lines 30, 50, 56, 67, 85, 128 and 141.
- The Docker lab test stage runs the CLI as `USER 65532:65532` with no `HOME` (docker/Dockerfile:10). `os.UserConfigDir()` would therefore point to an unwritable directory. `tests/integration/lab_test.go` (`TestCLIExecutable`) runs `version`, `diagnose` and `discover` through the real binary and expects them to succeed.
- `storage.Open(":memory:")` is supported (`internal/adapter/storage/storage.go:54`), and the store uses one connection (`SetMaxOpenConns(1)`, line 63).
- Searching `internal/` and `cmd/` for `os.Getenv`, `os.LookupEnv`, `os.Environ`, `os.ExpandEnv` and `"flag"` finds only `cmd/kali-tools/main.go:8` (`"flag"`).
- The `network.Prober` port is already injected by both roots (H03 partial, done 2026-10-21). This spec moves that injection into bootstrap.

## 4. Design

### 4.1 Package `internal/bootstrap`

```go
// Package bootstrap wires the application graph shared by every composition
// root (Wails desktop, CLI, and later the web server). It is the only package
// besides main packages allowed to import driven adapters.
package bootstrap

import (
    "context"
    "fmt"
    "time"

    "github.com/AlexGalhardo/kali-tools/internal/adapter/probe"
    "github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
    "github.com/AlexGalhardo/kali-tools/internal/app"
    "github.com/AlexGalhardo/kali-tools/internal/network"
)

// Config holds the composition-root inputs. Only main packages fill it.
type Config struct {
    DataPath string           // "" = storage.DefaultPath(); ":memory:" = in-memory SQLite
    Now      func() time.Time // nil = time.Now().UTC()
}

// New opens storage, applies retention and injects the driven adapters.
// The returned cleanup is idempotent and must be called exactly once by the
// owner when the process stops. On error, New releases everything it opened
// and returns a nil application and a nil cleanup.
func New(ctx context.Context, cfg Config) (*app.Application, func() error, error)
```

Algorithm of `New`, in this order:

1. `now := cfg.Now`; if `nil`, use `func() time.Time { return time.Now().UTC() }`.
2. `path := cfg.DataPath`. If it is `""`, call `storage.DefaultPath()`. On error, return `fmt.Errorf("bootstrap: data path: %w", err)`.
3. `store, err := storage.Open(path)`. On error, return `fmt.Errorf("bootstrap: open storage: %w", err)`.
4. `service := app.New(store, network.Diagnostics{Prober: probe.ICMP{}})`.
5. `application := &app.Application{Service: service}`.
6. Retention, which preserves `app.go:55-58` exactly:
   - `settings, err := store.Settings(ctx)`.
   - If `err == nil`, call `store.Retain(ctx, now().AddDate(0, 0, -settings.RetentionDays))`.
   - If `Retain` fails, call `application.Close()` and return `fmt.Errorf("bootstrap: retention: %w", err)`.
   - If `Settings` fails, skip retention and do **not** return an error.
7. Return `application, application.Close, nil`.

`New` reads no environment variable and no flag. The `os.UserConfigDir()` call inside `storage.DefaultPath()` is the documented default and is allowed.

### 4.2 Transitional `app.Application`

Add `internal/app/application.go`:

```go
package app

// Application is the single value every composition root receives from
// internal/bootstrap. H01 form: it wraps the existing Service. H04 replaces
// the Service field with methods and introduces New(deps Deps).
type Application struct {
    Service *Service
}

// Close releases the application. It is idempotent (Service.Close is).
func (a *Application) Close() error { return a.Service.Close() }
```

`app.New(s *storage.Store, d network.Diagnostics) *Service` keeps its current signature in H01.

### 4.3 Wails root (`main.go`, `app.go`), which stays at the repository root

```go
// main.go
func main() {
    os.Exit(run(context.Background(), os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

// run starts the desktop window. ctx, args and getenv are the composition-root
// seam; H01 does not read them.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
    desktop := NewApp()
    if err := wails.Run(&options.App{ /* the exact options of main.go:19 */ }); err != nil {
        _, _ = fmt.Fprintln(stderr, err)
        return 1
    }
    return 0
}
```

- Do **not** install a signal handler in the desktop root. A Ctrl-C in a terminal that launched the GUI must keep terminating the process, as it does today.
- `App` gains two fields: `config bootstrap.Config` (zero value means the default data path) and `cleanup func() error`.
- `NewApp()` keeps its signature and still returns `&App{}`.
- `startup` replaces `app.go:44-58` with:

```go
application, cleanup, err := bootstrap.New(a.ctx, a.config)
if err != nil {
    a.initErr = err
    return
}
a.service = application.Service
a.cleanup = cleanup
```

The two goroutines at `app.go:59-84` stay unchanged and start only after a successful bootstrap.

- `shutdown` replaces `_ = a.service.Close()` (line 102) with `if a.cleanup != nil { _ = a.cleanup() }`.
- `app.go` no longer imports `internal/adapter/probe`. It still imports `storage`, `platform` and `tools` for types and direct calls (removed in H03/H04).

### 4.4 CLI root (`cmd/kali-tools/main.go`)

```go
// cliDataPath keeps today's behavior: the CLI writes nothing to disk.
// C01 decides whether the CLI uses the persistent default path.
const cliDataPath = ":memory:"

func run(ctx context.Context, args []string, getenv func(string) string, out, errout io.Writer) int

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
    cancel()
    os.Exit(code)
}
```

Inside `run`:

- The usage check (no arguments, exit 2), the flag parsing (malformed flags, exit 2) and the positional-argument check keep their current order and happen **before** bootstrap.
- In the `else` branch that holds the command `switch` (line 60), call `bootstrap.New(ctx, bootstrap.Config{DataPath: cliDataPath})` first:
  - If it fails, set `err` and skip the switch. The existing mapping produces `{"error":{"code":"failed",...}}` and exit 1.
  - If it succeeds, `defer` the cleanup, ignoring its error.
- `diagnose` uses `application.Service.Diagnostics.Diagnose(ctx, network.Request{...})` instead of constructing `network.Diagnostics{Prober: probe.ICMP{}}`.
- Every other command keeps its current code path in H01.
- `cmd/kali-tools` no longer imports `internal/adapter/probe`.
- `getenv` is unused in H01. Name the parameter `getenv` anyway; the enabled linters (`standard` set plus `.golangci.yml:6-15`) do not flag unused parameters.

### 4.5 Limits and invariants preserved

| Invariant | Where it stays enforced |
| --- | --- |
| Max 8 concurrent jobs | `internal/app/service.go:73` and `:193` (unchanged) |
| Terminal-event delivery waits at most 2 s | `internal/app/service.go:30`, `:56-63` (unchanged) |
| History summaries hold no targets or secrets | `internal/app/service.go:115`, `:223-229` (unchanged) |
| Wails event names `job`, `metrics`, `capture` | `app.go:65`, `:80`, `:240` (unchanged) |
| CLI JSON envelope with `schema_version` | `cmd/kali-tools/main.go:28-33`, `:152` (unchanged) |
| Store migration and retention behavior | `storage.Open`, `store.Retain` (called from bootstrap instead of `app.go`) |

## 5. Files

Create:

- `internal/bootstrap/bootstrap.go`: `Config` and `New`.
- `internal/bootstrap/bootstrap_test.go`: the unit tests in §8.
- `internal/bootstrap/roots_test.go`: the static checks on composition roots and on env/flag reads in §8.
- `internal/app/application.go`: the transitional `Application` with `Close`.

Modify:

- `main.go`: add `run`; `main` delegates to it; replace `log.Fatal`.
- `app.go`: add `config` and `cleanup` fields; `startup` calls `bootstrap.New`; `shutdown` calls `cleanup`; drop the `probe` import.
- `cmd/kali-tools/main.go`: new `run` signature, bootstrap call, `diagnose` through the application; drop the `probe` import.
- `cmd/kali-tools/main_test.go`: pass a `getenv` argument at every `run` call (mechanical; assertions unchanged), and add the new test in §8.
- `docs/architecture.md`: see §10.

Delete:

- `internal/bootstrap/.gitkeep`.

## 6. Implementation steps

Each step must leave `go build ./... && go vet ./... && go vet -tags desktop . && go test ./...` green.

1. Add `internal/app/application.go` (§4.2). Nothing uses it yet.
2. Add `internal/bootstrap/bootstrap.go` (§4.1) and `bootstrap_test.go`. Delete `internal/bootstrap/.gitkeep`. Run `go test ./internal/bootstrap`.
3. CLI: change `run` to the new signature, add `cliDataPath`, call `bootstrap.New` and route `diagnose` through `application.Service.Diagnostics`. In `main_test.go`, add `func noEnv(string) string { return "" }` and pass `noEnv` as the third argument of every `run` call. Add `TestRunDoesNotWriteUserConfig`. Run `go test ./cmd/kali-tools`.
4. Desktop: add `run` to `main.go`, add the `App.config`/`App.cleanup` fields, and switch `startup`/`shutdown` to bootstrap. Run `go vet -tags desktop .` and `go build -tags desktop .` after `cd frontend && bun run build`.
5. Add `internal/bootstrap/roots_test.go` (static checks). Run `go test ./internal/bootstrap`.
6. Run `node scripts/task.mjs bindings` and confirm there is no drift under `frontend/wailsjs`.
7. Update `docs/architecture.md` (§10).
8. Write `docs/verification/specs/H01.md`, following README §5.3, with the real outputs of §9 and §12.

## 7. Acceptance criteria

- **AC-1:** Given an empty temporary directory, when `bootstrap.New(ctx, Config{DataPath: <dir>/history.db})` is called, then it returns a non-nil `*app.Application`, a non-nil cleanup and a nil error, and `application.Service.Diagnostics.Prober` is a `probe.ICMP`.
- **AC-2:** Given a successful `New`, when cleanup is called twice, then both calls return nil.
- **AC-3:** Given a store with `RetentionDays = 1`, one finished job created 48 h before `Config.Now()` and one created 1 h before, when `New` runs with that `Now`, then `History` returns only the recent job.
- **AC-4:** Given a `DataPath` whose parent is a regular file, when `New` runs, then it returns a non-nil error that wraps the `storage.Open` error, a nil application and a nil cleanup.
- **AC-5:** Given `DataPath: ":memory:"`, when `New` runs, then it succeeds, and no file named `history.db` is created under the user configuration directory.
- **AC-6:** Given the CLI, when `run(ctx, []string{"version"}, noEnv, out, errout)` runs with `HOME`, `APPDATA` and `XDG_CONFIG_HOME` pointed at an empty temporary directory, then it exits 0 and that directory is still empty.
- **AC-7:** Given the existing CLI tests with only the `getenv` argument added, when they run, then they pass with unchanged assertions: same exit codes, the `schema_version: 1` envelope and the `invalid_input` codes.
- **AC-8:** Given the source of `main.go`, `app.go` and `cmd/kali-tools/main.go`, when the static check runs, then:
  - the CLI root and the Wails root (`main.go` + `app.go`) import `internal/bootstrap`;
  - neither imports `internal/adapter/probe`;
  - neither calls `storage.Open` or `storage.DefaultPath`;
  - each `main` package defines `run` with the signature `func(context.Context, []string, func(string) string, io.Writer, io.Writer) int`.
- **AC-9:** Given every non-test `.go` file under `internal/`, when the static check runs, then no file imports `flag` and none calls `os.Getenv`, `os.LookupEnv`, `os.Environ` or `os.ExpandEnv`.
- **AC-10 [native]:** Given a Windows desktop build, when the native bridge smoke test runs (`node scripts/task.mjs bridge`), then `TestNativeBridge` passes unchanged.
- **AC-11:** Given the Docker lab, when `TestCLIExecutable` runs as UID 65532 without a writable `HOME`, then `version`, `diagnose` and `discover` keep their expected exit codes. This holds because the CLI uses `:memory:`.

## 8. Tests

| Test | Package / file | Asserts | AC |
| --- | --- | --- | --- |
| `TestNewWiresStoreAndProber` | `internal/bootstrap/bootstrap_test.go` | Non-nil application and cleanup; `_, ok := application.Service.Diagnostics.Prober.(probe.ICMP)` is true | AC-1 |
| `TestCleanupIsIdempotent` | same | Two cleanup calls return nil | AC-2 |
| `TestNewAppliesRetention` | same | Pre-seed with `storage.Open` + `SaveSettings({"en",1})` + two `PutJob` (state `completed`), close, then `New` with fixed `Now`; `application.Service.Store.History` has 1 row | AC-3 |
| `TestNewReportsStorageError` | same | Parent path is a file; error non-nil and `errors.Unwrap` chain non-nil; nil app and cleanup | AC-4 |
| `TestNewInMemory` | same | `:memory:` succeeds; cleanup nil | AC-5 |
| `TestRunDoesNotWriteUserConfig` | `cmd/kali-tools/main_test.go` | `t.Setenv` of `HOME`, `APPDATA`, `XDG_CONFIG_HOME` to `t.TempDir()`; run `version`; directory has no entries | AC-6 |
| `TestCLIContract`, `TestCLIHTTPAndMalformedFlags`, `TestDiscoveryRejectsInvalidOptionsBeforeScanning`, `TestDiscoveryExports`, `TestCLICaptureConversion` | `cmd/kali-tools/main_test.go` | Unchanged assertions; only the `noEnv` argument is added | AC-7 |
| `TestCompositionRootsUseBootstrap` | `internal/bootstrap/roots_test.go` | `go/parser` on `../../main.go`, `../../app.go` and `../../cmd/kali-tools/main.go`; checks the imports and the `run` signature listed in AC-8 (parse with `parser.ParseFile`; build tags do not matter for parsing) | AC-8 |
| `TestNoEnvOrFlagReadsBelowCompositionRoots` | `internal/bootstrap/roots_test.go` | Walks `../../internal`, skips `_test.go`, fails on an import of `flag` or a selector call to `os.Getenv`, `os.LookupEnv`, `os.Environ` or `os.ExpandEnv` | AC-9 |
| `TestNativeBridge` | `bridge_smoke_test.go` (tags `windows && desktop && production && nativesmoke`) | Unchanged | AC-10 |
| `TestCLIExecutable` | `tests/integration/lab_test.go` (tag `integration`) | Unchanged | AC-11 |

These existing tests must keep passing unchanged:

- `internal/app`: `TestCancellationAndRedaction`, `TestDiscoverySummarySurvivesRestartWithoutTargets`, `TestConcurrentDiagnosticsBoundAndShutdown`, `TestCloseIsIdempotent`, `TestTerminalEventSurvivesFullEventBuffer`;
- all tests in `internal/adapter/storage`;
- `internal/network`: `TestICMPWithoutProberIsUnsupported`, `TestICMPUsesInjectedProber`.

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
go test -race ./internal/bootstrap ./cmd/kali-tools ./internal/app   # or: node scripts/task.mjs race
cd frontend && bun run build && cd .. && go build -tags desktop .
node scripts/task.mjs bindings                       # no drift under frontend/wailsjs
bash scripts/setup/setup-linux-docker.sh test        # Docker lab (or the Windows/macOS variant)
node scripts/task.mjs bridge                         # [native] Windows only; record result, do not infer
```

## 10. Documentation updates

- `docs/architecture.md`:
  - §2: change `internal/bootstrap/` from "(empty)" to "shared wiring used by every composition root".
  - §4.2: the composition roots now get the `network.Diagnostics{Prober: probe.ICMP{}}` row through `bootstrap.New`.
  - §6.1/§6.3: describe `run(ctx, args, getenv, stdout, stderr) int` in both roots and note that the CLI uses an in-memory store until C01.
- `PLAN.md`: tick the two H01 sub-items only with the test evidence above. Leave the parent `H01` box unticked until AC-10 (native) has been run by a human.

## 11. Risks and rollback

- **The desktop starts with an error state when bootstrap fails.** This is the same as today (`initErr`). The difference is that the error text is now prefixed with `bootstrap: ...`. Mitigation: AC-4 and the unchanged `readyLocked` order.
- **The CLI opens SQLite in memory on every invocation.** The cost is a few milliseconds. Mitigation: AC-6 and AC-11.
- **The Wails root loses `log.Fatal`.** The error is still printed to stderr with exit code 1.
- **Rollback:** revert the single H01 commit. No schema, binding or data format changes.

## 12. Quality and delivery assertions

### 12.1 Applicable gates

| Gate | Applies | Spec threshold |
| --- | --- | --- |
| QG-1 Formatting | yes | empty diff |
| QG-2 Lint | yes | `0 issues.` |
| QG-3 Vet on all targets | yes | exit 0, including `go vet -tags desktop .` |
| QG-4 Tests | yes | all pass; the existing CLI tests change only by the `noEnv` argument |
| QG-5 Race | yes | no race reports in `./internal/bootstrap`, `./cmd/kali-tools`, `./internal/app` |
| QG-6 Coverage | yes | `internal/bootstrap` ≥ 80 % (new); `cmd/kali-tools` and `internal/app` not below the recorded pre-spec values |
| QG-7 Vulnerabilities | yes | no new reachable vulnerability (no new dependency is added) |
| QG-8 Frontend | yes | all pass; Playwright workflow count unchanged |
| QG-9 Binding drift | yes (manual until H05) | `node scripts/task.mjs bindings` reports no drift |
| QG-10 Dependency rule | partial | DA-3 and DA-4 below (depguard arrives in H08) |
| QG-11 Secrets | n.a. | no new output path |
| QG-12 Bounds | n.a. | no new channel, buffer or parser |
| QG-13 Commit | yes | `refactor(bootstrap): share composition roots (H01)` |
| QG-14 Docs | yes | architecture §2, §4.2, §6 updated; CHANGELOG `[Unreleased]` entry |
| QG-15 Markdown | yes | `Markdown checks passed` |

### 12.2 Delivery assertions

| DA | Assertion | Check | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | Bootstrap is the only wiring path | `go test ./internal/bootstrap -run 'TestCompositionRootsUseBootstrap' -v` | `PASS` | CI |
| DA-2 | No env or flag reads below roots | `go test ./internal/bootstrap -run 'TestNoEnvOrFlagReadsBelowCompositionRoots' -v` | `PASS` | CI |
| DA-3 | The CLI root no longer imports the probe adapter | `go list -f '{{join .Imports "\n"}}' ./cmd/kali-tools \| grep -c internal/adapter/probe` | `0` (direct imports only; the transitive edge through bootstrap is expected) | reviewer |
| DA-4 | The desktop root no longer imports the probe adapter | `go list -tags desktop -f '{{join .Imports "\n"}}' . \| grep -c internal/adapter/probe` | `0` | reviewer |
| DA-5 | The CLI writes nothing to the user config dir | `go test ./cmd/kali-tools -run 'TestRunDoesNotWriteUserConfig' -v` | `PASS` | CI |
| DA-6 | Retention still runs at startup | `go test ./internal/bootstrap -run 'TestNewAppliesRetention' -v` | `PASS` | CI |
| DA-7 | The Docker lab CLI is unaffected | `bash scripts/setup/setup-linux-docker.sh test` (`TestCLIExecutable`) | exit 0 | reviewer (CI when the lab job runs) |
| DA-8 | The desktop bridge still works | `node scripts/task.mjs bridge` (`TestNativeBridge`) | `PASS` | [native] |

### 12.3 Evidence record

`docs/verification/specs/H01.md`
