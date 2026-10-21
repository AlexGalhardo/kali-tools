# H05 Driving adapters extracted

| Field | Value |
| --- | --- |
| PLAN tasks | H05 |
| Status | Ready |
| Depends on | H01, H02, H03, H04 |
| Risk | Medium: the Wails binding namespace changes from `main.App` to `wailsui.Bindings`, which touches generated code, the frontend bridge, Playwright fakes and the native smoke test in one change |

## 1. Goal

The CLI command tree and the Wails bindings stop living in composition roots. `internal/adapter/cli` owns the CLI commands and `internal/adapter/wailsui` owns the Wails bindings and the event forwarder. Both receive the same `*app.Application` built by `internal/bootstrap`, and both only translate and delegate. `main.go` stays at the repository root and only wires. `cmd/kali-tools/main.go` only wires. The generated Wails bindings move from `frontend/wailsjs/go/main/App` to `frontend/wailsjs/go/wailsui/Bindings`, and `frontend/src/shared/api/bridge.ts` follows without changing its exported API. Users see no difference: the same CLI bytes, exit codes and JSON envelope, the same Wails event names and payloads, the same UI.

## 2. Non-goals

- Do not introduce `internal/adapter/contract` DTOs or `contract.FromError`. That is H06. In this spec the bindings and the CLI return the types that `app.Application` returns.
- Do not add, rename or remove CLI commands or flags (C01). Do not add `serve`, `history`, `settings`, `jobs` or `tools` commands.
- Do not add Wails binding methods. `JobSnapshot` and `Subscribe` stay unbound.
- Do not change the frontend exported API (`connected`, `backend`, `onEvent`, exported types) or any page in `frontend/src/app/App.tsx`.
- Do not create `frontend/embed.go` or `shared/api/transport.ts` (W01).
- Do not configure Wails `ErrorFormatter`. Rejected promises stay plain `code: message` strings.
- Do not move `main.go` out of the repository root.
- Do not change `app.Application`, `job.Manager` or `bootstrap` signatures. If something is missing there, STOP and report.

## 3. Current state

Facts at commit `8cbf25e` (re-verify; H01–H04 will have changed some of them, see the precondition table):

- `app.go:1` is `//go:build desktop || bindings`, `package main`. `App` (`app.go:22-33`) holds `ctx`, `cancel`, `*app.Service`, `initErr` and capture state. `startup` (`app.go:36-84`) opens storage, applies retention, starts the `job` forwarder (`app.go:58-67`, `wruntime.EventsEmit(a.ctx, "job", event)`) and the 2 s `metrics` ticker (`app.go:68-83`).
- Readiness errors: `domain.Invalid("application is shutting down")` and `domain.Invalid("application is starting")` (`app.go:109-120`).
- Bound methods (`app.go:121-280`): `Version`, `Capabilities`, `Interfaces`, `Inventory`, `Tools`, `Diagnose`, `StartDiagnostic`, `CancelJob`, `History`, `ClearHistory`, `Settings`, `SaveSettings`, `CurrentWiFi`, `RevealWiFi`, `Discover` (fixed 500 ms per-port timeout, `app.go:175-180`), `ImportCapture` (open dialog, `app.go:191`), `StartCapture` (emits `capture` with `map[string]any{"result": result, "error": message}`, `app.go:239`), `StopCapture`, `SaveCapture` (format check `pcap|pcapng` before the save dialog, `app.go:255-257`; dialog at `app.go:265`; 30 s save deadline), `InspectServices`.
- `main.go:18-19` builds `NewApp()` and calls `wails.Run` with `Bind: []interface{}{app}`, `OnStartup: app.startup`, `OnShutdown: app.shutdown`; `//go:embed all:frontend/dist` at `main.go:14-15`.
- `bridge_smoke_test.go:1` is `//go:build windows && desktop && production && nativesmoke`. It binds `NewApp()` and a `SmokeBridge` (`bridge_smoke_test.go:59-69`) and runs JavaScript that calls `window.go.main.App.Interfaces/Settings/Version/CancelJob("missing")/SaveCapture("invalid")/InspectServices("0.0.0.0/0",[80])` (`bridge_smoke_test.go:68`). It needs WebView2 on Windows.
- `cmd/kali-tools/main.go`: `envelope{SchemaVersion, Version, Data any (omitempty), Error *domain.Error (omitempty)}` (`main.go:27-32`); `run(ctx, args, out, errout) int` (`main.go:34`); usage line to stderr and exit 2 when `args` is empty (`main.go:35-38`); one `flag.FlagSet` with `ContinueOnError` (`main.go:41-54`), parse error exits 2; commands `version`, `doctor`, `interfaces`, `inventory`, `diagnose`, `discover`, `inspect`, `wifi`, `capture`, `import` (`main.go:61-147`); CSV discovery output (`main.go:101-116`); error mapping and exit codes (`main.go:151-172`): `*domain.Error` → itself, exit 2 if `invalid_input` else 1; `context.Canceled` → `{cancelled, "Operation cancelled"}`, exit 1; other → `{failed, err.Error()}`, exit 1; JSON encoded with two-space indent. `main` uses `signal.NotifyContext(..., os.Interrupt)` (`main.go:174-178`).
- `cmd/kali-tools/main_test.go` tests: `TestCLIContract` (`:23`), `TestCLIHTTPAndMalformedFlags` (`:46`), `TestDiscoveryRejectsInvalidOptionsBeforeScanning` (`:61`), `TestDiscoveryExports` (`:76`), `TestCLICaptureConversion` (`:109`).
- `frontend/src/shared/api/bridge.ts:1` imports `../../../wailsjs/go/main/App`. `Backend` (`bridge.ts:94-122`) declares 20 methods. `window.go?: { main: { App: Backend } }` (`bridge.ts:125`); `connected()` and `backend()` read `window.go?.main?.App` (`bridge.ts:131-137`). `backend()` returns the generated module typed as `Backend`, so `tsc` already fails when the generated signatures stop being assignable to `Backend`.
- `frontend/wailsjs/go/main/App.d.ts` and `App.js` are generated; `App.js` calls `window['go']['main']['App'][name]`.
- `frontend/tests/workflows.spec.ts` (17 tests) installs its fake as `window.go.main.App` at line 24 and overrides methods through `window.go.main.App.<Method>` at lines 37, 43, 52, 62, 99, 102, 125, 126, 141 and 151.
- `scripts/task.mjs:33-38` (`bindings` task) runs `wails generate module -tags desktop`, then `chmod 0644` on a hard-coded list that contains `go/main/App.d.ts` and `go/main/App.js`, then `git diff --exit-code -- frontend/wailsjs` (the binding drift check). `docs/development.md:33` documents the same command.
- `internal/adapter/cli/` and `internal/adapter/wailsui/` contain only `.gitkeep`.
- Wails v2.17.0 (`go.mod:7`) binds every exported method of a bound struct, except the functions passed as `OnStartup`, `OnShutdown`, `OnDomReady` and `OnBeforeClose` (`internal/app/app_production.go` builds that exemption list; method values are matched after trimming `-fm`). `github.com/wailsapp/wails/v2/pkg/runtime` compiles without build tags for windows, linux and darwin.

Preconditions from H01–H04 (verify with `go doc ./internal/app Application`, `go doc ./internal/bootstrap` and `go doc ./internal/job`; if a name differs, use the H01–H04 name and record it under "Deviations"; if a capability is missing, STOP):

| Needed here | Expected after H04 |
| --- | --- |
| Composition | `bootstrap.New(ctx, bootstrap.Config) (*app.Application, func() error, error)`; roots expose `run(ctx, args, getenv, stdout, stderr) int` |
| Application calls | `Capabilities`, `Interfaces`, `Inventory`, `Tools`, `Diagnose`, `StartDiagnostic`, `CancelJob`, `JobSnapshot`, `Subscribe`, `History`, `ClearHistory`, `Settings`, `SaveSettings`, `CurrentWiFi`, `RevealWiFi`, `Discover`, `InspectServices`, `StartCapture`, `StopCapture`, `ImportCapture(ctx, path)`, `ExportCapture(ctx, path, format)`, `Close`; every method takes `ctx` first |
| Discovery timeout | `Discover(ctx, cidr string, ports []int, timeout time.Duration)` |
| Events | `Subscribe(ctx) (<-chan job.Event, func() uint64)`; `job.Event{SchemaVersion, Seq, Topic, Job *job.Status, Data any, At}`; topics `job`, `metrics`, `capture` |
| Event data | topic `job`: `Data` is `*network.Result` or nil; topic `metrics`: `network.Metrics`; topic `capture`: the capture outcome that H04 publishes (today's shape is `{"result": capture.LiveResult, "error": string}`) |
| Metrics and capture loops | run inside `app`/`job` (D3 removed); `app.go` no longer owns a ticker or capture state |
| Import guard | `ImportCapture` returns `invalid_input` "stop capture before importing a file" while a capture runs; `ExportCapture` returns `invalid_input` "import or finish a capture before saving" when nothing is retained |

## 4. Design

### 4.1 Packages and dependencies

```text
main.go (root, desktop)  ──► bootstrap.New ──► *app.Application ──► wailsui.New(a) ──► wails.Run
cmd/kali-tools/main.go   ──► bootstrap.New ──► *app.Application ──► cli.Run(ctx, a, args, stdout, stderr)
internal/adapter/wailsui ──► app, domain, network, capture, wifi (types), wails pkg/runtime
internal/adapter/cli     ──► app, domain, network, capture, wifi (types), stdlib
```

Neither adapter imports `internal/adapter/{storage,tools,probe,platform}` or `internal/bootstrap` in non-test files. Test files may import `internal/bootstrap` to build a real application.

### 4.2 `internal/adapter/cli`

```go
package cli

// Run executes one CLI command against the shared application and returns the
// process exit code: 0 success, 1 failure, 2 invalid input.
func Run(ctx context.Context, a *app.Application, args []string, stdout, stderr io.Writer) int

type envelope struct {
    SchemaVersion int           `json:"schema_version"`
    Version       string        `json:"version"`
    Data          any           `json:"data,omitempty"`
    Error         *domain.Error `json:"error,omitempty"`
}

const (
    exitOK      = 0
    exitFailure = 1
    exitInvalid = 2
)

// captureGrace bounds how long `capture` waits for the capture event after the
// requested duration has elapsed.
const captureGrace = 10 * time.Second

// toError is the single error mapping of this adapter (moved from cmd/kali-tools/main.go:151-166).
func toError(err error) (*domain.Error, int)
```

Rules:

- `Run` keeps the exact usage text of `cmd/kali-tools/main.go:36`, written to `stderr`, exit 2, when `len(args) == 0`.
- One `flag.FlagSet` named `args[0]` with `flag.ContinueOnError` and output `stderr`, with the same flags, defaults and help strings as `main.go:41-53`. A parse error returns 2 and writes nothing to `stdout`.
- Positional arguments after flags produce `domain.Invalid("unexpected positional arguments")`.
- `toError`: `*domain.Error` → itself (exit 2 when `Code == "invalid_input"`, else 1); `context.Canceled` → `{cancelled, "Operation cancelled"}`, exit 1; anything else → `{failed, err.Error()}`, exit 1. Byte-identical to today. H06 replaces it.
- JSON is written with `json.NewEncoder(stdout)` and `SetIndent("", "  ")`. An encoding error returns 1.
- The `discover --format csv` path writes the same header `schema_version,host,port,state` and rows `1,<host>,<port>,<state>`, returning 0, and returns 1 on a CSV write error.

Command dispatch (each row is the only logic of the command; validation stays in `app`/capabilities except the transport parsing listed):

| Command | Transport parsing in `cli` | Application call | `data` |
| --- | --- | --- | --- |
| `version` | none | none | `map[string]string{"version": domain.Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH}` |
| `doctor` | none | `a.Capabilities(ctx)`, `a.Tools(ctx)` | `map[string]any{"os", "arch", "capabilities", "tools", "capture", "wifi"}` with the two literal strings from `main.go:64` |
| `interfaces` | none | `a.Interfaces(ctx)` | result |
| `inventory` | none | `a.Inventory(ctx)` | result |
| `diagnose` | `--kind`, `--target`, `--port`, `--timeout`, `--record` into `network.Request` | `a.Diagnose(ctx, req)` | result |
| `discover` | `--format` must be `json` or `csv`; `--ports` length ≤ 191 bytes and < 32 commas, every item `strconv.Atoi`; `--timeout` ms | `a.Discover(ctx, target, ports, time.Duration(timeout)*time.Millisecond)` | results or CSV |
| `inspect` | same port parsing; `--format` must be `json` | `a.InspectServices(ctx, target, ports)` | result |
| `wifi` | none | `a.CurrentWiFi(ctx)` | result |
| `import` | `--output` set requires `--format` `pcap` or `pcapng`, checked first | `a.ImportCapture(ctx, file)`; then, if `--output` is set, `a.ExportCapture(ctx, output, format)` | import result |
| `capture` | same `--output`/`--format` check first; `--target`, `--duration`, `--limit` into `capture.LiveRequest` | see the capture flow below | live result |
| other | none | none | error `domain.Invalid("unknown command %q", name)` |

Keep the error messages of `main.go` byte-identical: `"format must be json or csv"`, `"1..32 ports required"`, `"invalid port list"`, `"service inspection supports JSON only"`, `"packet export requires --format pcap or pcapng"`, `"unexpected positional arguments"`, `"unknown command %q"`.

Capture flow (`capture` is asynchronous in `app`; the CLI waits for it):

1. `events, _ := a.Subscribe(ctx)` before starting, so the outcome cannot be missed.
2. `a.StartCapture(ctx, req)`; a returned error goes to `toError`.
3. Wait with a `time.Timer` of `time.Duration(req.DurationSeconds)*time.Second + captureGrace` for the first event with `Topic == "capture"`. Ignore other topics.
4. On `ctx.Done()`: call `a.StopCapture(ctx)`, keep waiting at most 5 s for the capture event, then return `ctx.Err()` through `toError` (Ctrl-C prints `cancelled`, exit 1).
5. On timer expiry: call `a.StopCapture(ctx)` and return `&domain.Error{Code: "timeout", Message: "capture did not finish in time"}`.
6. On the capture event: if the outcome carries an `error` value, use it. If it carries the legacy string form, convert it with `parseCodedError(s string) error`: when `s` has the form `<code>: <message>` and `<code>` is one of `invalid_input`, `unsupported`, `permission_required`, `dependency_missing`, `not_found`, `timeout`, `cancelled`, `failed`, `no_connection`, return `&domain.Error{Code: code, Message: message}`; otherwise return `&domain.Error{Code: "failed", Message: s}`. An empty string means success.
7. If the capture succeeded and `--output` is set, call `a.ExportCapture(ctx, output, format)`.
8. `data` is the live result.

### 4.3 `internal/adapter/wailsui`

```go
package wailsui

// Runtime holds the Wails runtime functions the bindings use. Tests replace them.
type Runtime struct {
    Emit     func(ctx context.Context, name string, data ...any)
    OpenFile func(ctx context.Context, options wruntime.OpenDialogOptions) (string, error)
    SaveFile func(ctx context.Context, options wruntime.SaveDialogOptions) (string, error)
}

// Bindings is the struct bound to the WebView. Every exported method except
// Startup and Shutdown is a frontend call; they translate and delegate only.
type Bindings struct {
    app  *app.Application
    emit func(ctx context.Context, name string, data ...any)
    open func(ctx context.Context, options wruntime.OpenDialogOptions) (string, error)
    save func(ctx context.Context, options wruntime.SaveDialogOptions) (string, error)

    mu     sync.RWMutex
    ctx    context.Context // Wails runtime context; the only stored context in the repository
    closed bool
    stop   context.CancelFunc
    done   chan struct{}
}

func New(a *app.Application) *Bindings // Runtime{wruntime.EventsEmit, wruntime.OpenFileDialog, wruntime.SaveFileDialog}
func NewWithRuntime(a *app.Application, rt Runtime) *Bindings

func (b *Bindings) Startup(ctx context.Context)
func (b *Bindings) Shutdown(ctx context.Context)

const (
    eventJob     = "job"
    eventMetrics = "metrics"
    eventCapture = "capture"

    // desktopDiscoveryTimeout is the per-endpoint timeout the desktop has always used (app.go:179).
    desktopDiscoveryTimeout = 500 * time.Millisecond
    // shutdownWait bounds how long Shutdown waits for the forwarder goroutine.
    shutdownWait = 3 * time.Second
)
```

`New` and `NewWithRuntime` are package functions, so Wails does not bind them. `Startup` and `Shutdown` are exempted because `main.go` passes the method values `b.Startup` and `b.Shutdown` as `OnStartup` and `OnShutdown`.

Lifecycle:

- `Startup(ctx)`: under `mu`, return if `closed` or already started. Store `ctx`. Create `fctx, stop := context.WithCancel(ctx)`, `events, _ := b.app.Subscribe(fctx)`, `done = make(chan struct{})`, start `go b.forward(ctx, fctx, events)`.
- `forward`: loop on `select { case <-fctx.Done(): return; case ev, ok := <-events: ... }`; `close(done)` on exit. Emission per topic:

| Topic | Event name | Payload (JSON identical to today) |
| --- | --- | --- |
| `job` | `job` | `jobPayload{SchemaVersion int "schema_version"; Job job.Status "job"; Result *network.Result "result,omitempty"}` built from `ev.SchemaVersion`, `*ev.Job` and `ev.Data.(*network.Result)` |
| `metrics` | `metrics` | `ev.Data` unchanged (`network.Metrics`) |
| `capture` | `capture` | an object with exactly the keys `result` (live result) and `error` (string, empty on success), built from the H04 capture outcome |
| other / unexpected `Data` type | none | dropped |

- `Shutdown(ctx)`: under `mu`, return if `closed`; set `closed = true`; call `stop` if set; release `mu`; wait for `done` up to `shutdownWait`. `Shutdown` does not call `a.Close()`; the composition root owns the application lifetime through the `bootstrap` cleanup, called after `wails.Run` returns.
- `ready() (context.Context, error)`: `closed` → `domain.Invalid("application is shutting down")`; not started → `domain.Invalid("application is starting")`; else the stored context.

Bound methods. The JavaScript names and argument order must match `Backend` in `bridge.ts:94-122` exactly. Every method except `Version` calls `ready()` first and returns its error.

| Method (Go) | Delegates to | Notes |
| --- | --- | --- |
| `Version() string` | `domain.Version` | no readiness check |
| `Capabilities() []domain.Capability` | `a.Capabilities(ctx)` | on readiness error return `nil` |
| `Interfaces() (..., error)` | `a.Interfaces(ctx)` | |
| `Inventory() (..., error)` | `a.Inventory(ctx)` | |
| `Tools() (..., error)` | `a.Tools(ctx)` | |
| `Diagnose(r network.Request) (network.Result, error)` | `a.Diagnose(ctx, r)` | |
| `StartDiagnostic(r network.Request) (job.Status, error)` | `a.StartDiagnostic(ctx, r)` | |
| `CancelJob(id string) bool` | `a.CancelJob(ctx, id)` | `true` only when the job was found and cancelled; readiness error or unknown id → `false` |
| `History() ([]job.Status, error)` | `a.History(ctx)` | |
| `ClearHistory() error` | `a.ClearHistory(ctx)` | |
| `Settings() (S, error)` / `SaveSettings(v S) error` | `a.Settings` / `a.SaveSettings` | `S` is the H04 settings type, JSON `{language, retention_days}` |
| `CurrentWiFi() (wifi.Network, error)` | `a.CurrentWiFi(ctx)` | |
| `RevealWiFi() (wifi.Secret, error)` | `a.RevealWiFi(ctx)` | never logged |
| `Discover(cidr string, ports []int) ([]network.PortResult, error)` | `a.Discover(ctx, cidr, ports, desktopDiscoveryTimeout)` | |
| `InspectServices(cidr string, ports []int) (..., error)` | `a.InspectServices(ctx, cidr, ports)` | |
| `ImportCapture() (capture.ImportResult, error)` | open dialog, then `a.ImportCapture(ctx, path)` | dialog options identical to `app.go:191`; empty path → `capture.ImportResult{Packets: []capture.Packet{}}, nil` |
| `StartCapture(r capture.LiveRequest) error` | `a.StartCapture(ctx, r)` | |
| `StopCapture()` | `a.StopCapture(ctx)` | readiness error → no-op |
| `SaveCapture(format string) (bool, error)` | check format, save dialog, `a.ExportCapture(ctx, path, format)` | `format` not `pcap`/`pcapng` → `domain.Invalid("capture format must be pcap or pcapng")` before any dialog; dialog options identical to `app.go:265`; dialog error → `false, err`; empty path → `false, nil`; success → `true, nil` |

Accepted behavior difference, documented in the commit: when nothing is retained, `SaveCapture` now shows the save dialog before `ExportCapture` returns `invalid_input`. The UI disables the button in that state (`App.tsx:836`).

### 4.4 Composition roots

`main.go` (root, tags unchanged `desktop || bindings`):

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()
    os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
    // bootstrap.New(ctx, config) exactly as H01 defined it; on error write it to stderr, return 1.
    // defer cleanup(); report a cleanup error on stderr.
    b := wailsui.New(a)
    // wails.Run with the options of today's main.go:19, except:
    //   OnStartup: b.Startup, OnShutdown: b.Shutdown, Bind: []any{b}
}
```

`app.go` is deleted. `cmd/kali-tools/main.go` keeps the H01 `run` and replaces its command code with `return cli.Run(ctx, a, args, stdout, stderr)`.

### 4.5 Generated bindings and frontend

- `wails generate module -tags desktop` produces `frontend/wailsjs/go/wailsui/Bindings.d.ts` and `Bindings.js`. `Bindings.js` calls `window['go']['wailsui']['Bindings'][name]`.
- Delete `frontend/wailsjs/go/main/` with `git rm -r` before generating, so no stale `App` module remains.
- `frontend/src/shared/api/bridge.ts`:

```ts
import * as generated from "../../../wailsjs/go/wailsui/Bindings";
// ...exported types unchanged...
declare global {
  interface Window {
    go?: { wailsui?: { Bindings?: Backend } };
    runtime?: {
      EventsOn(name: string, callback: (data: unknown) => void): () => void;
    };
  }
}
export const connected = () => Boolean(window.go?.wailsui?.Bindings);
export function backend(): Backend {
  const value = window.go?.wailsui?.Bindings;
  if (!value)
    throw new Error("Native bridge unavailable. Open the desktop application.");
  return generated;
}
```

- `frontend/tests/workflows.spec.ts`: replace `go:{main:{App:api}}` with `go:{wailsui:{Bindings:api}}` and every `window.go.main.App.` with `window.go.wailsui.Bindings.`. Change nothing else in the file.
- `bridge_smoke_test.go`: build the application with `bootstrap.New` (data under the test's `APPDATA` temp dir, as today), bind `wailsui.New(a)` and `smoke`, call `b.Startup(ctx)` inside the existing `OnStartup` closure and pass `b.Shutdown` as `OnShutdown`. In the JavaScript string, replace `window.go.main.App.` with `window.go.wailsui.Bindings.`; keep `window.go.main.SmokeBridge.Complete`. Keep every assertion. Call the bootstrap cleanup after `wails.Run` returns.

## 5. Files

Create:

- `internal/adapter/cli/cli.go`: `Run`, envelope, flag parsing, dispatch, `toError`, exit codes.
- `internal/adapter/cli/capture.go`: the capture flow and `parseCodedError`.
- `internal/adapter/cli/cli_test.go`: `package cli_test`; moved CLI tests plus the new tests in §8.
- `internal/adapter/cli/testdata/golden/*.json` and `usage.txt`: byte-exact CLI output fixtures.
- `internal/adapter/wailsui/bindings.go`: `Bindings`, `Runtime`, `New`, `NewWithRuntime`, lifecycle, bound methods.
- `internal/adapter/wailsui/events.go`: `forward` and the per-topic payloads.
- `internal/adapter/wailsui/bindings_test.go`: `package wailsui_test`; tests in §8.
- `frontend/wailsjs/go/wailsui/Bindings.d.ts`, `frontend/wailsjs/go/wailsui/Bindings.js`: generated, never hand-edited.

Modify:

- `main.go`: wiring only (§4.4).
- `bridge_smoke_test.go`: new binding namespace and wiring (§4.5).
- `cmd/kali-tools/main.go`: wiring only.
- `cmd/kali-tools/main_test.go`: keep one wiring test; CLI behavior tests move to `internal/adapter/cli`.
- `frontend/src/shared/api/bridge.ts`: import path, `Window.go` shape, `connected`, `backend`.
- `frontend/tests/workflows.spec.ts`: fake namespace only.
- `frontend/wailsjs/go/models.ts`: regenerated.
- `scripts/task.mjs`: in the `bindings` task, replace `go/main/App.d.ts` and `go/main/App.js` with `go/wailsui/Bindings.d.ts` and `go/wailsui/Bindings.js`.
- `docs/architecture.md`, `docs/development.md`, `PLAN.md`: §10.

Delete:

- `app.go`.
- `frontend/wailsjs/go/main/App.d.ts`, `frontend/wailsjs/go/main/App.js`.
- `internal/adapter/cli/.gitkeep`, `internal/adapter/wailsui/.gitkeep`.

## 6. Implementation steps

1. **Pin CLI bytes.** In `cmd/kali-tools/main_test.go` add `TestCLIGolden` (§8) calling the current `run`, with an `-update` flag that rewrites `cmd/kali-tools/testdata/golden/`. Generate the goldens once with `-update`, review them, and commit them with the test. `go test ./cmd/kali-tools` passes.
2. **Create `internal/adapter/cli`.** Move the command code from `cmd/kali-tools/main.go` into `cli.Run` exactly as §4.2 describes, including the capture flow. Move the goldens to `internal/adapter/cli/testdata/golden/` and the five existing tests plus `TestCLIGolden` into `internal/adapter/cli/cli_test.go`, rewritten to call `cli.Run` with an application from `bootstrap.New` and a temp data directory. Do not edit any golden file. `cmd/kali-tools/main.go` now calls `cli.Run`. `go test ./...` passes.
3. **Create `internal/adapter/wailsui`** with §4.3 and its tests. Nothing uses it yet. `go test ./internal/adapter/wailsui` passes.
4. **Switch the desktop root.** Delete `app.go`, rewire `main.go` (§4.4) and `bridge_smoke_test.go` (§4.5). `go vet -tags desktop .` passes. `go vet -tags desktop,production,nativesmoke .` passes on Windows.
5. **Regenerate bindings.** `git rm -r frontend/wailsjs/go/main`, update `scripts/task.mjs`, run `node scripts/task.mjs bindings` once without the drift assertion by running `wails generate module -tags desktop` directly, then commit the generated files. Run `node scripts/task.mjs bindings` again: it must exit 0 (no drift).
6. **Frontend.** Update `bridge.ts` and `workflows.spec.ts` (§4.5). `bun run typecheck`, `bun run lint`, `bun run build` and `bunx playwright test` pass (17/17).
7. **Docs.** Apply §10. Run the full §9 block.
8. **Evidence.** Write `docs/verification/specs/H05.md` with the README §5.3 structure, including the baseline taken before step 1 (coverage of `cmd/kali-tools`, Go test count, Playwright workflow count 17).

Commit as one Conventional Commit, `refactor(adapters): extract cli and wailsui driving adapters (H05)`, or one commit per step with the same scope.

## 7. Acceptance criteria

- AC-1: Given the CLI goldens recorded in step 1, when `cli.Run` executes each golden case, then stdout (after normalizing the `version` runtime fields) and the exit code are byte-identical.
- AC-2: Given no arguments, when `cli.Run` runs, then it writes the usage text to stderr, nothing to stdout, and returns 2.
- AC-3: Given `diagnose --target a;whoami`, `discover --target 0.0.0.0/0` or `unknown`, when `cli.Run` runs, then stdout is a JSON envelope with `schema_version` 1, `error.code` `invalid_input`, and the exit code is 2.
- AC-4: Given `diagnose --timeout NaN`, when `cli.Run` runs, then it returns 2 and stdout is empty.
- AC-5: Given a cancelled context, when a command fails with `context.Canceled`, then the envelope error is `{"code":"cancelled","message":"Operation cancelled"}` and the exit code is 1.
- AC-6: Given a capture request on a host without a live provider (or without permission), when `capture` runs, then it returns within `duration + captureGrace` with exit 1 and `error.code` `unsupported` or `permission_required`.
- AC-7: Given `import --file x.pcapng --output y.pcap --format pcap`, when `cli.Run` runs, then `y.pcap` is readable, holds the same packet, and stdout does not contain the payload marker `synthetic-private`.
- AC-8: Given `Bindings` before `Startup`, when a bound method other than `Version` is called, then it returns `invalid_input` "application is starting"; after `Shutdown`, `invalid_input` "application is shutting down".
- AC-9: Given a started `Bindings` with a recording emitter, when a diagnostic job runs to completion, then the emitter receives `job` events whose JSON has exactly the keys `schema_version`, `job` and, on the terminal event, `result`, and the last state is `completed`.
- AC-10: Given a started `Bindings`, when `Shutdown` returns, then the forwarder goroutine has exited (within 3 s) and no further `emit` call happens.
- AC-11: Given `SaveCapture("invalid")`, when called, then it returns `invalid_input` and the save dialog function is never called.
- AC-12: Given the open dialog returns an empty path, when `ImportCapture` is called, then the result JSON is `{"packets":[],"limited":false,"format":""}` and no error.
- AC-13: Given the `*Bindings` method set, when compared with `Backend` in `bridge.ts`, then the exported methods other than `Startup` and `Shutdown` are exactly the 20 `Backend` names.
- AC-14: Given a clean checkout, when `node scripts/task.mjs bindings` runs, then `frontend/wailsjs/go/wailsui/Bindings.{d.ts,js}` exist, `frontend/wailsjs/go/main/` does not exist, and `git diff --exit-code -- frontend/wailsjs` exits 0.
- AC-15: Given the Playwright fake installed at `window.go.wailsui.Bindings`, when `bunx playwright test` runs, then 17/17 workflows pass, including "browser without Wails shows explicit limitation".
- AC-16: Given the source tree, when `go list -f '{{.Imports}}'` runs on `internal/adapter/cli` and `internal/adapter/wailsui`, then neither imports `internal/adapter/storage`, `tools`, `probe`, `platform` or `internal/bootstrap`, and `cmd/kali-tools/main.go` and `main.go` contain no command or binding logic.
- AC-17 [native]: Given Windows with WebView2, when `node scripts/task.mjs bridge` runs, then `TestNativeBridge` passes through `window.go.wailsui.Bindings`.
- AC-18 [native]: Given a desktop build, when a user runs a diagnostic, watches metrics and stops a live capture, then the UI behaves as before (job, metrics and capture events arrive).

## 8. Tests

`internal/adapter/cli` (`package cli_test`; the helper `newApp(t)` calls `bootstrap.New(t.Context(), cfg)` with the H01 config field for the data location set to `t.TempDir()` and registers the cleanup):

| Test | Asserts | AC |
| --- | --- | --- |
| `TestCLIGolden` | table of cases `version`, `unknown`, `diagnose --target a;whoami`, `discover --target 0.0.0.0/0`, `discover --target 127.0.0.1/32 --format invalid`, `discover` with 33 ports, `inspect --format csv --target 127.0.0.1/32`, `import --file <fixture>`; byte comparison with `testdata/golden/<name>.json`; `version` replaces `go`, `os`, `arch` with `<go>`, `<os>`, `<arch>` before comparing; exit code stored in the table | AC-1 |
| `TestCLIUsageWithoutArgs` | exit 2, stdout empty, stderr equals `testdata/golden/usage.txt` | AC-2 |
| `TestCLIContract` (moved) | unchanged assertions | AC-3 |
| `TestCLIHTTPAndMalformedFlags` (moved) | unchanged assertions; plus stdout empty for `--timeout NaN` | AC-4 |
| `TestCLICancelledContext` | `diagnose --kind http` against an `httptest` server that blocks on `r.Context().Done()`, context cancelled after the request arrives (signalled through a channel); envelope error `cancelled`, exit 1 | AC-5 |
| `TestCLICaptureReportsProviderState` | first non-loopback local IPv4 from `net.InterfaceAddrs` (skip when none); `capture --target <ip> --duration 1 --limit 1`; returns before 11 s; on non-Windows exit 1 with `unsupported`; on Windows exit 0 or exit 1 with `permission_required` | AC-6 |
| `TestDiscoveryRejectsInvalidOptionsBeforeScanning`, `TestDiscoveryExports`, `TestCLICaptureConversion` (moved) | unchanged assertions | AC-3, AC-7 |
| `TestParseCodedError` | known code → typed error; unknown prefix → `failed` with the full string; empty → nil | AC-6 |

`internal/adapter/wailsui` (`package wailsui_test`; `NewWithRuntime` with a recording `Emit` guarded by a mutex and channel-based waiting, never `time.Sleep`):

| Test | Asserts | AC |
| --- | --- | --- |
| `TestBindingsRejectBeforeStartupAndAfterShutdown` | `Interfaces`, `History`, `SaveCapture("pcap")` errors and `CancelJob("x") == false` before `Startup` and after `Shutdown`; `Version()` always returns `domain.Version` | AC-8 |
| `TestForwarderEmitsLegacyJobPayload` | start a diagnostic against a local `httptest` server; collect `job` events until a terminal state (deadline 3 s); marshal each payload and assert the key sets | AC-9 |
| `TestShutdownStopsForwarder` | after `Shutdown` returns, start another operation through the application and assert the emit count does not change before a 200 ms `time.After` select | AC-10 |
| `TestSaveCaptureValidatesFormatBeforeDialog` | `SaveFile` stub counts calls; `SaveCapture("invalid")` → `invalid_input`, zero calls | AC-11 |
| `TestSaveCaptureCancelledDialog` | `SaveFile` returns `""` → `false, nil` | AC-11 |
| `TestImportCaptureEmptySelection` | `OpenFile` returns `""` → JSON equals AC-12 | AC-12 |
| `TestImportCaptureReadsSelectedFile` | `OpenFile` returns a synthetic one-packet PCAPNG written with `capture.NewArtifact` → 1 packet | AC-12 |
| `TestBindingMethodSetMatchesBackend` | `reflect.TypeOf(&wailsui.Bindings{})` exported methods minus `Startup`, `Shutdown` equal the sorted list `CancelJob, Capabilities, ClearHistory, CurrentWiFi, Diagnose, Discover, History, ImportCapture, InspectServices, Interfaces, Inventory, RevealWiFi, SaveCapture, SaveSettings, Settings, StartCapture, StartDiagnostic, StopCapture, Tools, Version` | AC-13 |

`cmd/kali-tools`: `TestRunWiresCLI` runs `run(ctx, []string{"version"}, getenv, &out, &errOut)` with `getenv` pointing the data location at `t.TempDir()`; exit 0, envelope `schema_version` 1. Root: `TestNativeBridge` (existing, rewired) covers AC-17. AC-14 is the `bindings` task; AC-15 is Playwright; AC-16 is checked by `go list` in §9 (enforced mechanically by H08).

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
go test -race -timeout 120s ./internal/adapter/...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
```

Binding regeneration and drift check (requires the Wails CLI v2.17.0, `go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0`):

```bash
node scripts/task.mjs bindings            # wails generate module -tags desktop + git diff --exit-code -- frontend/wailsjs
test ! -e frontend/wailsjs/go/main
grep -rn "go/main/App\|go\.main\.App\|go:{main" frontend/src frontend/tests bridge_smoke_test.go scripts && exit 1 || true
```

Dependency check until H08 lands:

```bash
go list -f '{{join .Imports "\n"}}' ./internal/adapter/cli ./internal/adapter/wailsui | grep -E 'internal/(adapter/(storage|tools|probe|platform)|bootstrap)' && exit 1 || true
```

Native, Windows host with WebView2 only (`[native]`):

```bash
node scripts/task.mjs bridge
```

## 10. Documentation updates

- `docs/architecture.md`: §1 diagram and §2 map (`wailsui` and `cli` no longer empty; `app.go` removed; generated path `frontend/wailsjs/go/wailsui/Bindings`); §4.2 last row; §5.1 flow (`wailsui.Bindings` and the forwarder); §5.3 producers; §6.1 (namespace changed, `main.go` wires only); §6.3 (`internal/adapter/cli` exists); §8 D2 and D3 marked resolved for the driving side.
- `docs/development.md:33`: generated files are `frontend/wailsjs/go/wailsui/Bindings.*`; `node scripts/task.mjs bindings` is the regeneration plus drift command.
- `PLAN.md` H05: tick only the boxes whose evidence exists; leave the smoke-test assertion unticked unless AC-17 ran on Windows.

## 11. Risks and rollback

- Stale namespace: any leftover `window.go.main.App` silently disables the UI ("Native bridge unavailable"). The `grep` in §9 and Playwright catch it; AC-17 proves it natively.
- `Startup`/`Shutdown` become JavaScript bindings if they are not passed as method values in `main.go`. `TestBindingMethodSetMatchesBackend` and the generated `.d.ts` diff reveal it.
- Calling `wruntime.EventsEmit` with a non-Wails context calls `log.Fatal`. Tests must use `NewWithRuntime`.
- `wails generate module` runs the root `main` in bindings mode; if H01 opens the database there, generation touches the user data directory. Record it as a follow-up; do not change H01 here.
- Capture events can be dropped by a full subscriber buffer; the CLI then reports `timeout` instead of hanging. Record any observed drop.
- Rollback: `git revert` the H05 commit(s). This restores `app.go`, the `main` namespace bindings, `bridge.ts`, the fakes and `scripts/task.mjs` together. Never revert the generated files alone.

## 12. Quality and delivery assertions

Applicable global gates (README §5.1):

| Gate | Threshold for H05 |
| --- | --- |
| QG-1, QG-2, QG-3, QG-4, QG-5, QG-7 | as defined |
| QG-6 | `internal/adapter/cli` ≥ 80 % and `internal/adapter/wailsui` ≥ 80 % statements (new packages); `cmd/kali-tools` is reduced to wiring, so record before/after and require that every remaining statement except `main` is covered by `TestRunWiresCLI` |
| QG-8 | 17 Playwright workflows before and after; the count must not decrease |
| QG-9 | binding drift gate becomes active in this spec: `node scripts/task.mjs bindings` exits 0 |
| QG-10 | `go list` check of §9 prints nothing (depguard arrives in H08) |
| QG-11 | `TestCLICaptureConversion` (payload marker) passes from its new location |
| QG-12 | `captureGrace` (10 s) and `shutdownWait` (3 s) are named constants; boundary tests: `TestCLICaptureReportsProviderState` (returns before duration + grace) and `TestShutdownStopsForwarder` (returns before `shutdownWait`) |
| QG-13, QG-14, QG-15 | as defined; commit messages contain `(H05)` |

Delivery assertions:

| DA | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | CLI output is byte-compatible with the pre-refactor CLI | `go test ./internal/adapter/cli -run 'TestCLIGolden\|TestCLIUsageWithoutArgs'` against goldens committed in step 1 | pass; `git log --follow internal/adapter/cli/testdata/golden` shows no golden edited after step 1 | CI, reviewer |
| DA-2 | `app.go` is gone and roots only wire | `test ! -e app.go`; `grep -cE 'case "' cmd/kali-tools/main.go` | file absent; count `0` | reviewer |
| DA-3 | Old binding namespace is gone everywhere | `test ! -e frontend/wailsjs/go/main` and the `grep` in §9 | no match | CI, reviewer |
| DA-4 | Generated bindings are current | `node scripts/task.mjs bindings` | exit 0, empty diff | CI (native jobs) |
| DA-5 | Go binding method set equals the frontend `Backend` | `go test ./internal/adapter/wailsui -run TestBindingMethodSetMatchesBackend` | pass | CI |
| DA-6 | Event names and payload keys unchanged | `go test ./internal/adapter/wailsui -run TestForwarderEmitsLegacyJobPayload` | pass | CI |
| DA-7 | Driving adapters import no driven adapter | `go list` command in §9 | prints nothing | CI, reviewer |
| DA-8 | Desktop IPC works through the new namespace | `node scripts/task.mjs bridge` on Windows with WebView2 | `ok` for `TestNativeBridge` | [native] |

Evidence record: `docs/verification/specs/H05.md`.
