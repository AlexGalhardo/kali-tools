# C01 Complete CLI

| Field | Value |
| --- | --- |
| PLAN tasks | C01, C02, C03, C04 |
| Status | Steps 1–4 implemented 2026-10-21; remaining CLI migration and acceptance open |
| Depends on | H01, H02, H03, H04, H05, H06, H07, H08 (H08 includes the H09 parser ADR) |
| Risk | Medium: the default output format and per-command flag strictness change for existing invocations; a documented migration is part of this spec |

## 1. Goal

`kali-tools` becomes a complete terminal interface over the same `app.Application` used by the desktop and the web server. Every desktop operation has a CLI command. Output is human-readable by default and machine-readable with `--json` (the existing versioned envelope). Exit codes are stable and documented per error code. Long-running work streams progress, and Ctrl-C cancels it through the same job-manager path as the UI. The CLI works over SSH without a TTY, never prompts, and prints a Wi-Fi key only with an explicit `--confirm` plus a warning. `docs/cli.md` freezes the contract, and compatibility fixtures fail on any unannounced breaking change.

## 2. Non-goals

- No daemon, no `--detach`, no background processes. `--detach` is rejected (see §4.8).
- No shell completion and no Cobra. This spec assumes the H09 ADR kept stdlib `flag`. If the ADR chose another parser, STOP and report.
- No neighbor inventory. `neighbors` is a stub that returns `unsupported` until A01.
- No new `app.Application` methods, capability logic or validation rules. The CLI only decodes flags, calls the application, and encodes results.
- No web server internals. `serve` is specified in [W01](W01-web-server.md) and [W04](W04-web-security.md). This spec only reserves the command name and its help entry.
- No localized help text. Help is English. `--lang` localizes result labels, state words, warnings and error labels only.
- No interactive prompts, no reading of stdin, no pagers, no spinners, no terminal-width detection.
- No cross-process job control. A CLI process can cancel only the jobs it started (see §4.7).
- No capture export of an artifact held by a different process (each invocation is a separate process).

## 3. Current state

Verify each fact before starting. Facts marked (H) describe what H01–H08 must have produced; if any is false, STOP and report.

### 3.1 CLI today (pre-H05 locations; after H05 the same code lives in `internal/adapter/cli`)

- `cmd/kali-tools/main.go:35-174` is one `run(ctx, args, out, errout) int` function with a `switch args[0]` dispatch.
- `cmd/kali-tools/main.go:42-54` registers **one** `FlagSet` with every flag for every command, so any command accepts any flag. Flags and defaults:
  - `--target` string `""` (host, URL or CIDR; also the local IPv4 address for `capture`)
  - `--record` string `""` (DNS record type)
  - `--kind` string `"tcp"` (`dns`, `tcp`, `http`, `tls`, `ping`, `traceroute`)
  - `--port` int `443`
  - `--timeout` int `3000` (milliseconds)
  - `--file` string `""` (capture input path)
  - `--output` string `""` (packet-file destination)
  - `--ports` string `"80,443"`
  - `--format` string `"json"` (`json`/`csv` for discovery; `pcap`/`pcapng` with `--output`)
  - `--duration` int `5` (capture seconds)
  - `--limit` int `1000` (capture packets)
- `cmd/kali-tools/main.go:37` prints usage listing `version|doctor|interfaces|inventory|diagnose|discover|inspect|wifi|capture|import` and exits 2 when no command is given.
- `cmd/kali-tools/main.go:55-57`: a flag parse error returns 2 and prints the `flag` package message (which echoes the bad value) to stderr, with no JSON envelope. `-h` also returns 2.
- `cmd/kali-tools/main.go:58-59`: positional arguments are rejected with `invalid_input`.
- Commands (`main.go:62-149`): `version`, `doctor` (with free-text `capture` and `wifi` strings at `:65`, the latter claiming reveal is desktop-only), `interfaces`, `inventory`, `diagnose` (synchronous `Diagnose`, no history row, `:71`), `discover`/`inspect` (`:72-118`, port-list parsing in the CLI, CSV output at `:102-117`), `wifi` (metadata only, `:119-120`), `capture`/`import` (`:121-147`).
- `cmd/kali-tools/main.go:28-33,152-173`: output is **always** the JSON envelope `{schema_version, version, data?, error?}` indented with two spaces. Exit codes: 0 success, 2 `invalid_input`, 1 everything else. Non-`domain.Error` errors become `failed` with the raw `err.Error()` message (`:165`). A failed diagnostic prints both `data` and `error` (`:71` assigns both).
- `cmd/kali-tools/main.go:175-179`: `signal.NotifyContext(ctx, os.Interrupt)` only; no SIGTERM; no job cancellation beyond context propagation.
- Tests: `cmd/kali-tools/main_test.go:23` `TestCLIContract`, `:46` `TestCLIHTTPAndMalformedFlags`, `:61` `TestDiscoveryRejectsInvalidOptionsBeforeScanning`, `:76` `TestDiscoveryExports`, `:109` `TestCLICaptureConversion`. All parse stdout as JSON without passing any output flag.
- `tests/integration/lab_test.go:65-100` (`TestCLIExecutable`) runs the built binary in Docker and parses `schema_version` from stdout without an output flag.
- `README.md:198-212` and `docs/development.md:38-44,80` document the CLI as JSON-only with exit codes 0/1/2. `docs/cli.md` does not exist.

### 3.2 Application surface the CLI uses (H)

`internal/adapter/cli.Run(ctx, a, args, stdout, stderr) int` exists (H05), `cmd/kali-tools/main.go` only wires through `bootstrap.New(ctx, Config)` (H01). The CLI uses these `app.Application` methods. For readability the result types below are written as the H06 contract DTOs. In the code, the application returns the H04 types and the CLI maps them with the H06 `contract.From*` functions (`FromStatus`, `FromDiagnosticResult`, `FromWiFiSecret`, ...) before encoding, and maps events with `contract.EventPayload`. If a method is missing, STOP.

```go
func (a *Application) Capabilities(ctx context.Context) ([]contract.Capability, error)
func (a *Application) Interfaces(ctx context.Context) ([]contract.Interface, error)
func (a *Application) Inventory(ctx context.Context) (contract.Inventory, error)
func (a *Application) Tools(ctx context.Context) ([]contract.Tool, error)
func (a *Application) Diagnose(ctx context.Context, r contract.DiagnosticRequest) (contract.DiagnosticResult, error)
func (a *Application) StartDiagnostic(ctx context.Context, r contract.DiagnosticRequest) (contract.Job, error)
func (a *Application) CancelJob(ctx context.Context, id string) (bool, error)
func (a *Application) JobSnapshot(ctx context.Context, id string) (contract.Job, error)
func (a *Application) Subscribe(ctx context.Context) (<-chan job.Event, func() uint64)
func (a *Application) History(ctx context.Context) ([]contract.Job, error)
func (a *Application) ClearHistory(ctx context.Context) error
func (a *Application) Settings(ctx context.Context) (contract.Settings, error)
func (a *Application) SaveSettings(ctx context.Context, s contract.Settings) error
func (a *Application) CurrentWiFi(ctx context.Context) (contract.WiFi, error)
func (a *Application) RevealWiFi(ctx context.Context) (contract.WiFiSecret, error)
func (a *Application) Discover(ctx context.Context, cidr string, ports []int, timeout time.Duration) ([]contract.PortResult, error)
func (a *Application) InspectServices(ctx context.Context, cidr string, ports []int) (contract.ServiceInspection, error)
func (a *Application) StartCapture(ctx context.Context, r contract.LiveCaptureRequest) error
func (a *Application) StopCapture(ctx context.Context) error
func (a *Application) ImportCapture(ctx context.Context, path string) (contract.Capture, error)
func (a *Application) ExportCapture(ctx context.Context, path, format string) error
func (a *Application) Close() error
```

- `job.Event{SchemaVersion, Seq, Topic, Job *Status, Data, At}` with `Topic` in `job`, `metrics`, `capture` (H02). A terminal diagnostic `job` event carries `*network.Result` in `Data`, which `contract.EventPayload` maps to `contract.JobEvent.Result` (H06 §4.3; the UI depends on it at `frontend/src/app/App.tsx:277-281`). A `capture` event maps to `contract.CaptureEvent{Result, Error string}`; H05 §4.2 `parseCodedError` converts the `Error` string to a typed error.
- H05/H06 created `internal/adapter/cli` with a single legacy `FlagSet`, `exitCode(contract.Error)` returning only 0/1/2, synchronous `diagnose` through `a.Diagnose`, and the capture wait flow (`captureGrace = 10 * time.Second`). This spec replaces `exitCode` with the exported `ExitCode` of §4.6 and keeps the capture wait flow, adding the Ctrl-C rules of §4.7.
- `contract.FromError(err) contract.Error` maps any error to a stable `{code, message}` without raw subprocess, SQL or secret text, and caps the message at 512 bytes (H06 §4.4).
- `internal/adapter/storage.Open` uses WAL and `busy_timeout=5000` (`internal/adapter/storage/storage.go:67`), so the CLI and the desktop may open the same database concurrently.

## 4. Design

Checkout corrections (2026-10-21): the H07 CLI acceptance driver must prepend `--json` and use `cli.ExitCode` as soon as stricter parsing arrives, rather than waiting for the human-default step. Update `tests/acceptance/cli_driver_test.go` in this spec and retain its history skip until history commands are implemented. Application signatures use domain values and `job.ID`/boolean status (as delivered by H04); map through the existing H06 functions, without adding application methods or DTO dependencies. Interface inventory and generated job IDs remain native/random; normalize those fields in golden fixtures rather than adding production test seams.

Step 4 also updates `tests/acceptance/history_test.go`, `diagnostics_test.go` and `driver_test.go` to remove the obsolete CLI-history skip/sentinel and require redacted empty history after invalid input through every driver. CLI adapter tests and `tests/acceptance/app_test.go` use bootstrap's real in-memory SQLite mode: repeated Windows cleanup failures were isolated to temporary SQLite directories after Close, while these tests assert command/interface behavior rather than file persistence. Storage/bootstrap tests retain their real temporary databases and persistence assertions. This is a test-fixture scope correction, not a production storage change or a diagnosis of the OS/driver cleanup race.

### 4.1 Package layout

All code is in `internal/adapter/cli` except the `serve` wiring (cmd root, see §4.11).

```text
internal/adapter/cli/
├── cli.go          Run, NeedsApp, root parsing, dispatch, envelope, exit mapping
├── commands.go     the command table (one entry per leaf command)
├── flags.go        global flags, --timeout parsing, port-list parsing, flag error text
├── human.go        human renderers (text/tabwriter), color
├── messages.go     en and pt-BR catalogs for human output
├── progress.go     job-event progress printer and Ctrl-C cancellation helper
├── cli_test.go     behavior tests (exit codes, cancellation, secrets, legacy forms)
├── golden_test.go  golden runner with -update
├── contract_test.go JSON contract fixtures and docs/cli.md coverage
└── testdata/
    ├── golden/*.golden
    ├── contract/*.json
    └── legacy_invocations.txt
```

### 4.2 Signatures

```go
package cli

// Run executes one invocation. a may be nil only when NeedsApp(args) is false.
// Run never reads stdin and never writes outside stdout and stderr, except
// explicit --output paths.
func Run(ctx context.Context, a *app.Application, args []string, stdout, stderr io.Writer) int

// NeedsApp reports whether args select a command that needs the application graph.
// False for: empty args, "help", "version", any -h/--help, unknown commands, "serve".
func NeedsApp(args []string) bool

type globals struct {
    JSON    bool
    Timeout time.Duration // 0 means "not set"
    NoColor bool
    Lang    string // "en" (default) or "pt-BR"
}

type command struct {
    path     string // "diagnose", "jobs list", ...
    synopsis string // one line, English
    help     string // paragraph, English
    needsApp bool
    positional string // "", "kind" or "id"
    flags    func(fs *flag.FlagSet) *cmdFlags
    run      func(ctx context.Context, e *env) (any, error)
    human    func(w io.Writer, e *env, data any) error
}

type env struct {
    app     *app.Application
    g       globals
    f       *cmdFlags
    arg     string // positional value, if any
    stdout  io.Writer
    stderr  io.Writer
    msg     messages
    color   bool
}
```

Invocation grammar:

```text
kali-tools [global flags] <command> [<subcommand>] [<positional>] [flags]
```

- Global flags are registered on the root `FlagSet` (parsed before the command name) and on every leaf `FlagSet` (so they also work after the command).
- A positional (`diagnose <kind>`, `jobs show <id>`, `jobs cancel <id>`, `help <command>`) must come immediately after the command path. After flag parsing, `fs.NArg() != 0` is `invalid_input` "unexpected positional arguments".
- Every `FlagSet` uses `flag.ContinueOnError` and `fs.SetOutput(io.Discard)`. The CLI prints its own parse error: `invalid value for --<name>` or `unknown flag --<name>`. It **never** echoes the rejected value (C01 "without echoing secrets").
- `-h`/`--help` on any command prints that command's help to stdout and exits 0. `kali-tools help [<command path>]` does the same. Plain `kali-tools` prints the command list to stderr and exits 2.
- JSON mode is detected by a pre-scan before parsing: any token equal to `--json`, `-json`, `--json=true` or `-json=true` before a `--` terminator turns JSON mode on, so even parse errors produce an envelope.

### 4.3 Command tree

`App call` names `app.Application` methods. "Data" is the `data` field of the JSON envelope.

| Command | Positional / flags | App call | Data |
| --- | --- | --- | --- |
| `version` | none | none | `{"version","go","os","arch"}` (unchanged keys) |
| `doctor` | none | `Capabilities`, `Tools` | `{"os","arch","capabilities","tools","capture","wifi"}` (unchanged keys; `wifi` text updated, see §4.9) |
| `capabilities` | none | `Capabilities` | `[]Capability` |
| `interfaces` | none | `Interfaces` | `[]Interface` |
| `inventory` | none | `Inventory` | `Inventory` |
| `neighbors` | none | none | error `unsupported` "neighbor inventory is not implemented yet" |
| `diagnose <kind>` | kind ∈ `dns tcp http tls ping trace traceroute`; `--target` (required), `--port` int default 443, `--record` string, `--kind` (deprecated) | `StartDiagnostic` + `Subscribe` (§4.7) | `DiagnosticResult` |
| `discover` | `--target` CIDR, `--ports` default `80,443`, `--format json\|csv` default `json` | `Discover(ctx, cidr, ports, timeout)` | `[]PortResult` |
| `inspect` | `--target` CIDR, `--ports` default `80,443`, `--format json` (deprecated) | `InspectServices` | `ServiceInspection` |
| `wifi show` | none | `CurrentWiFi` | `WiFi` |
| `wifi reveal` | `--confirm` bool (required) | `RevealWiFi` | `{"state","value"}` |
| `capture live` | `--target` local IPv4, `--duration` int 1..60 default 5, `--limit` int 1..10000 default 1000, `--output`, `--format pcap\|pcapng` | `StartCapture` + `Subscribe`, then `ExportCapture` if `--output` | capture result |
| `capture import` | `--file` (required), `--output`, `--format pcap\|pcapng` | `ImportCapture`, then `ExportCapture` if `--output` | `Capture` |
| `capture export` | `--file`, `--output`, `--format` (all required) | `ImportCapture` then `ExportCapture` | `{"path","format"}` |
| `jobs list` | `--all` bool | `History` (filtered) | `[]Job` |
| `jobs show <id>` | id = 32 lowercase hex | `JobSnapshot`, fallback `History` | `Job` |
| `jobs cancel <id>` | id = 32 lowercase hex | `CancelJob` | `{"id","cancelled":true}` |
| `history list` | none | `History` | `[]Job` |
| `history clear` | none | `ClearHistory` | `{"cleared":true}` |
| `settings get` | none | `Settings` | `Settings` |
| `settings set` | `--language en\|pt-BR`, `--retention-days` int | `Settings` then `SaveSettings` | `Settings` (after save) |
| `tools` | none | `Tools` | `[]Tool` |
| `serve` | see W01 §4.6 | handled by cmd root | see W01 |
| `help [<path>]` | command path | none | help text (stdout, not an envelope) |

Rules per command:

- `diagnose`: `trace` maps to the domain kind `"traceroute"` (`internal/network/network.go:111`). If both the positional and `--kind` are given and differ after mapping, `invalid_input` "kind given twice". With neither, the kind is `"tcp"` (legacy default). `--record` is passed through; validation stays in the application.
- `discover`/`inspect`: the port list is split on `,`; more than 191 bytes or 32 commas is `invalid_input` "1..32 ports required" before any network call (keeps `main.go:77-79`); a non-integer element is `invalid_input` "invalid port list". Range and uniqueness are validated by the application.
- `discover --format csv` writes the existing CSV (header `schema_version,host,port,state`, then rows; `main.go:103-111`) and no envelope. `--format csv` together with `--json` is `invalid_input`.
- `jobs list` returns `History` rows whose state is `queued` or `running`; `--all` returns every row (the store caps history at 200, `internal/adapter/storage/storage.go:119`).
- `jobs show <id>`: call `JobSnapshot`; if its error code is `not_found`, search `History` for the ID; if absent, `not_found` "job not found".
- `jobs cancel <id>`: `CancelJob`; `false` maps to `not_found` "job is not running in this process; cancel it from the interface that started it".
- `settings set` with neither flag is `invalid_input` "nothing to set". Unset flags keep the current value.
- `capture live`: `--timeout` is rejected with `invalid_input` "capture live uses --duration".
- `--output` given without `--format pcap|pcapng` is `invalid_input` "packet export requires --format pcap or pcapng" and is checked **before** any capture starts or file is created (keeps `main.go:122-124`, pinned by `main_test.go:109`).
- `capture live` completes when the `capture` event arrives. If `--output` is set and the capture completed without error, call `ExportCapture(ctx, output, format)`. Export writes only to the explicit path (C03).

### 4.4 Global flags

| Flag | Type | Default | Meaning |
| --- | --- | --- | --- |
| `--json` | bool | false | Print the JSON envelope on stdout instead of human output. |
| `--timeout` | duration | not set | Bare integer = milliseconds (legacy form, e.g. `3000`); otherwise Go duration (`5s`). Valid range 100ms..10m, else `invalid_input`. |
| `--no-color` | bool | false | Disable ANSI color. The cmd root prepends `--no-color` when the environment variable `NO_COLOR` is non-empty (packages below `cmd` never read the environment). |
| `--lang` | string | `en` | `en` or `pt-BR`; anything else is `invalid_input`. Affects human output only. |

`--timeout` semantics:

- `diagnose`: becomes `DiagnosticRequest.TimeoutMS`; default 3000 ms (legacy). The application enforces 100..30000 ms.
- `discover`: the per-probe timeout passed to `Discover`; default 3000 ms (legacy). The application enforces 100..5000 ms.
- `capture live`: rejected (see §4.3).
- `version`, `help`: ignored.
- every other command: when set, the command runs under `context.WithTimeout(ctx, timeout)`; expiry maps to `timeout`.

### 4.5 Output modes

Streams:

- stdout carries the result only: the envelope (JSON mode), the CSV (`discover --format csv`), human output, or help.
- stderr carries warnings, deprecation notices, progress lines (human mode only) and, in human mode, the error line.
- Nothing else is written. No ANSI sequence is written unless color is enabled.

JSON mode: the envelope is unchanged from `main.go:28-33`, except that `error` becomes `*contract.Error` (same JSON keys `code`, `message`). It is encoded with `json.Encoder` and `SetIndent("", "  ")`. On error, `data` is still present when the command produced a partial result (failed diagnostic, cancelled capture).

```go
type envelope struct {
    SchemaVersion int             `json:"schema_version"`
    Version       string          `json:"version"`
    Data          any             `json:"data,omitempty"`
    Error         *contract.Error `json:"error,omitempty"`
}
```

Human mode: tables use `text/tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)`. Headers and state words come from `messages.go`. Times are RFC 3339 UTC. Formats:

| Command | Human output |
| --- | --- |
| `version` | `kali-tools <version> (<go> <os>/<arch>)` |
| `doctor` | `System: <os>/<arch>`, blank line, capabilities table, blank line, tools table |
| `capabilities` | table `CAPABILITY STATE PROVIDER DETAIL` |
| `interfaces` | table `NAME INDEX UP MTU MAC ADDRESSES` (addresses joined by a comma and one space) |
| `inventory` | table `ADAPTER GATEWAYS DNS`, blank line, table `COUNTER RECEIVED SENT` |
| `diagnose` | `<kind> <target>: <status> (<elapsed_ms> ms, <provider>)`, then each value on its own line indented by two spaces |
| `discover` | table `HOST PORT STATE` |
| `inspect` | `<scanner> <version>`, then table `HOST PORT STATE SERVICE METHOD` |
| `wifi show` | `key: value` lines: SSID, BSSID, profile, signal (`%`), authentication, channel (omitted when absent), state |
| `wifi reveal` | `state: <state>` and, when the value is non-empty, `key: <value>` |
| `capture live` / `capture import` | `<n> packets, format <format>[, limited]`, then table `NO TIME LENGTH SOURCE DESTINATION PROTOCOL INFO`; if exported, a final line `saved <path> (<format>)` |
| `capture export` | `saved <path> (<format>)` |
| `jobs list`, `history list` | table `ID CREATED STATE KIND SUMMARY` |
| `jobs show` | `key: value` lines: id, kind, state, created, finished (omitted when absent), summary |
| `jobs cancel` | `cancellation requested: <id>` |
| `history clear` | `history cleared (queued and running jobs are kept)` |
| `settings get`, `settings set` | `language: <lang>` and `retention_days: <n>` |
| `tools` | table `NAME STATE VERSION PATH` |

The localized labels in the table above are the `en` catalog values; `pt-BR` has a translation for every key. Human error line on stderr: `error: <localized label for code>: <message>`.

Color: enabled only when `--no-color` is absent **and** stdout is an `*os.File` whose `Stat().Mode()` has `os.ModeCharDevice`. Only state words are colored: green (`\x1b[32m`) for `available`, `completed`, `open`, `connected`; red (`\x1b[31m`) for `failed`, `closed`; yellow (`\x1b[33m`) for every other state; reset `\x1b[0m`.

### 4.6 Exit codes

```go
const (
    ExitOK          = 0
    ExitFailed      = 1   // failed, no_connection, certificate_invalid, any unknown code
    ExitUsage       = 2   // invalid_input, flag/usage errors
    ExitUnsupported = 3   // unsupported
    ExitPermission  = 4   // permission_required, location_consent_required
    ExitDependency  = 5   // dependency_missing
    ExitNotFound    = 6   // not_found
    ExitTimeout     = 7   // timeout
    ExitConflict    = 8   // conflict
    ExitAuth        = 9   // unauthorized, forbidden
    ExitCancelled   = 130 // cancelled (128 + SIGINT)
)

// ExitCode maps a stable error code to the process exit code.
func ExitCode(code string) int
```

| Error code | Exit | Error code | Exit |
| --- | --- | --- | --- |
| (success) | 0 | `not_found` | 6 |
| `failed` | 1 | `timeout` | 7 |
| `no_connection` | 1 | `conflict` | 8 |
| `certificate_invalid` | 1 | `unauthorized` | 9 |
| `invalid_input` | 2 | `forbidden` | 9 |
| `unsupported` | 3 | `cancelled` | 130 |
| `permission_required` | 4 | help requested (`-h`, `help`) | 0 |
| `location_consent_required` | 4 | no command | 2 |
| `dependency_missing` | 5 | stdout write failure | 1 |

For `diagnose`, the error code is `DiagnosticResult.ErrorCode` when the result status is `failed` or `cancelled`; otherwise `contract.FromError(err).Code`.

### 4.7 Long-running jobs and Ctrl-C (C02)

- The cmd root uses `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`. When the context is done, the cmd root calls the returned `stop()` immediately, so a second Ctrl-C terminates the process with the default signal behavior.
- `diagnose` flow:
    1. `ch, dropped := a.Subscribe(subCtx)` **before** `StartDiagnostic`, so no event is missed.
    2. `j, err := a.StartDiagnostic(ctx, req)`.
    3. Loop on `ch` until an event with `Topic == "job"`, `Job.ID == j.ID` and a terminal state (`completed`, `failed`, `cancelled`).
    4. If `ctx` is done first: call `a.CancelJob(context.Background(), j.ID)`, then keep reading `ch` for at most `cancelGrace` (3 s) for the terminal event. If none arrives, use `a.JobSnapshot`. Return `cancelled` (exit 130).
    5. If `dropped()` grew and the terminal event was missed (channel drained without it), call `a.JobSnapshot` to obtain the final state.
- `discover`, `inspect`, `capture import/export`: these are synchronous application calls whose jobs derive from the passed context, so Ctrl-C cancels them by context. In addition, the progress printer tracks non-terminal job IDs seen on `Subscribe` and calls `CancelJob` for each when `ctx` is done.
- `capture live` flow: subscribe, `StartCapture`, wait for the `capture` event. On Ctrl-C call `a.StopCapture(context.Background())`, wait at most `cancelGrace` for the `capture` event, print the partial result, return `cancelled` (exit 130), and **do not** export.
- After `Run` returns, the cmd root calls the bootstrap `cleanup()`, which calls `Application.Close()`, which cancels remaining jobs and waits for them, so no child process (Nmap) or capture handle outlives the command.
- Progress (human mode only): every `job` event for a job this process started prints one stderr line `job <first 8 hex of id> <kind>: <localized state>`. At most `maxProgressLines = 1000` lines are printed per invocation; after that, progress is silent. JSON mode prints no progress.
- `--detach` is a known flag on every command that is rejected with `invalid_input` "--detach is not supported: kali-tools has no daemon mode" (C02: "rejected until a daemon mode exists").

```go
const (
    cancelGrace      = 3 * time.Second
    maxProgressLines = 1000
)
```

### 4.8 Headless and SSH behavior (C03)

- `Run` has no stdin parameter; the CLI never prompts and never waits for input.
- No TTY assumptions: no cursor movement, no carriage-return redraws, no width detection; color only per §4.5.
- `wifi reveal` without `--confirm` returns `invalid_input` "wifi reveal prints the saved network key; re-run with --confirm" and **does not call** `RevealWiFi`.
- `wifi reveal --confirm` first writes this warning to stderr (localized), then calls `RevealWiFi`:

```text
warning: the saved Wi-Fi key will be printed to standard output. Terminal scrollback, shell logs and SSH session recordings may retain it.
```

- The key is written only to stdout. It is never passed to a logger, never included in an error message, and the CLI creates no history row for a reveal.
- Capture export writes only to the explicit `--output` path; there is no default file name.

### 4.9 Backward compatibility and migration

Kept unchanged:

- Command names `version`, `doctor`, `interfaces`, `inventory`, `diagnose`, `discover`, `inspect`.
- Flags `--target`, `--record`, `--port`, `--timeout` (bare integer milliseconds), `--file`, `--output`, `--ports`, `--format`, `--duration`, `--limit` on the commands that use them.
- Envelope keys, `schema_version: 1`, `discover --format csv` bytes, exit codes 0 (success), 1 (`failed`), 2 (`invalid_input`).

Deprecated aliases (still accepted; each prints one stderr line `warning: "<old>" is deprecated; use "<new>"`, in both modes):

| Old form | New form |
| --- | --- |
| `diagnose --kind <k>` | `diagnose <k>` |
| `wifi` (no subcommand) | `wifi show` |
| `capture [flags]` (no subcommand) | `capture live [flags]` |
| `import [flags]` | `capture import [flags]` |
| `discover --format json`, `inspect --format json` given explicitly | `--json` |

Breaking changes (documented in `docs/cli.md` "Migration from 0.1.0-alpha.1-dev" and `CHANGELOG.md`):

1. Default output is human-readable. Scripts must add `--json`. Every in-repo caller is updated in the same commit (`main_test.go`, `tests/integration/lab_test.go`, `README.md`, `docs/development.md`).
2. Flags are per command. A flag that a command does not use (for example `version --port 1`) is now `invalid_input` (exit 2) instead of being ignored.
3. Error codes other than `invalid_input` and `failed` now have their own exit codes (§4.6). Previously they all exited 1.
4. `-h`/`--help` exits 0 instead of 2.
5. Flag parse errors print an envelope in JSON mode and never echo the rejected value.
6. `diagnose` now runs as a job and records a redacted history row (state and summary only, as the desktop already does).
7. `doctor` `wifi` text changes to `"Windows WLAN API; key reveal requires wifi reveal --confirm or the desktop reveal action"`.

### 4.10 Limits

| Constant | Value | Where |
| --- | --- | --- |
| `maxPortsArg` | 191 bytes | `flags.go` (existing `main.go:77`) |
| `maxPortsCommas` | 31 | `flags.go` |
| `minTimeout`, `maxTimeout` | 100ms, 10m | `flags.go` |
| `cancelGrace` | 3s | `progress.go` |
| `maxProgressLines` | 1000 | `progress.go` |
| job ID | exactly 32 chars `[0-9a-f]` | `flags.go` |

### 4.11 `serve` wiring

`cmd/kali-tools/main.go` exposes `run(ctx, args, getenv, stdout, stderr) int` (H01). It resolves the command name after leading global flags. For `serve` it calls `runServe` in `cmd/kali-tools/serve.go` (specified in W01 §4.6) and never calls `cli.Run`. For every other command it calls `bootstrap.New` only when `cli.NeedsApp(args)` is true, then `cli.Run`, then `cleanup()`. `cli.Run` given `serve` returns `invalid_input` "serve is handled by the kali-tools binary" (reachable only in tests). `help serve` prints the serve help text, which `cli` owns so that `help` stays complete.

### 4.12 Contract freeze (C04)

- `docs/cli.md` is the frozen contract. Required sections: Synopsis; Global flags; one `## <command path>` section per row of §4.3 (synopsis, flags with defaults, JSON `data` schema, human example); Exit codes; Output streams; Environment (`NO_COLOR`); Security (`wifi reveal`, history redaction); Compatibility policy; Migration from 0.1.0-alpha.1-dev.
- Compatibility policy text: within one `schema_version`, JSON changes are additive only (new keys, new commands). Removing or retyping a key, renaming a command or flag, or changing an exit code requires a `schema_version` bump, an entry in `docs/cli.md` and `CHANGELOG.md`, and a deprecation period of at least one minor release with a stderr warning.
- `testdata/contract/<command-path>.json` fixtures record the JSON shape of each command's `data`:

```json
{
  "schema_version": 1,
  "command": "interfaces",
  "paths": {
    "data[].index": "number",
    "data[].name": "string",
    "data[].mac": "string",
    "data[].mtu": "number",
    "data[].up": "boolean",
    "data[].loopback": "boolean",
    "data[].addresses[]": "string"
  }
}
```

- `TestJSONContractFixtures` derives the path→type map from each JSON golden output. Every fixture path must exist with the same type. Extra paths are allowed. A missing or retyped path fails unless `domain.SchemaVersion` is greater than the fixture's `schema_version`.
- `testdata/legacy_invocations.txt` lists the legacy forms that must keep working, one per line: `<exit code> <error code or -> <args...>`.

## 5. Files

Create:

- `internal/adapter/cli/cli.go`: `Run`, `NeedsApp`, root parse, dispatch, envelope, `ExitCode`.
- `internal/adapter/cli/commands.go`: the command table from §4.3.
- `internal/adapter/cli/flags.go`: globals, `parseTimeout`, `parsePorts`, `parseJobID`, flag error text.
- `internal/adapter/cli/human.go`: renderers and color.
- `internal/adapter/cli/messages.go`: `en` and `pt-BR` catalogs (`type messages map[string]string`).
- `internal/adapter/cli/progress.go`: event loop, progress printer, cancellation helper.
- `internal/adapter/cli/cli_test.go`, `golden_test.go`, `contract_test.go`, `testapp_test.go` (`newTestApp(t)`).
- `internal/adapter/cli/testdata/golden/*.golden`, `testdata/contract/*.json`, `testdata/legacy_invocations.txt`.
- `docs/cli.md`: frozen contract.

Modify:

- `cmd/kali-tools/main.go`: SIGTERM, `stop()` on first signal, `NO_COLOR` → `--no-color`, `NeedsApp` gating, `serve` dispatch hook (implemented by W01).
- `cmd/kali-tools/main_test.go`: add `--json` to every invocation; keep all assertions; move behavior tests that now belong in `internal/adapter/cli` only if H05 has not already.
- `tests/integration/lab_test.go`: add `--json`; add a `wifi reveal` without `--confirm` case expecting exit 2.
- `README.md` §6, `docs/development.md` CLI examples, `docs/architecture.md` §6.3, `CHANGELOG.md`.

Delete: none.

## 6. Implementation steps

Each step ends with `go test ./...` green.

1. **Exit codes and envelope.** Add `ExitCode` and its table test. Switch the envelope error to `*contract.Error` via `contract.FromError`. Exit codes for `invalid_input`/`failed` are unchanged, so existing tests pass.
2. **Per-command FlagSets and root parsing.** Introduce the command table with the existing commands only (`version`, `doctor`, `interfaces`, `inventory`, `diagnose`, `discover`, `inspect`, `wifi`, `capture`, `import`) and the global flags. Keep JSON as the only output in this step (`--json` accepted and implied). Add the JSON pre-scan, non-echoing flag errors and `-h` exit 0. Add `legacy_invocations.txt` and `TestLegacyInvocationsStillParse`.
3. **New read-only commands.** `capabilities`, `tools`, `neighbors` stub, `wifi show`, `jobs list|show`, `history list`, `settings get`, `help`.
4. **Mutating commands.** `jobs cancel`, `history clear`, `settings set`, `wifi reveal --confirm`, `capture live|import|export`, deprecated aliases with warnings.
5. **Job-backed diagnose and cancellation (C02).** `diagnose` through `StartDiagnostic` + `Subscribe`; progress printer; Ctrl-C helper; `--detach` rejection; cmd root SIGTERM and `stop()`.
6. **Human output (default switch).** Add `human.go`, `messages.go`, color, `--lang`. Make human the default. In the same commit, add `--json` to `cmd/kali-tools/main_test.go` and `tests/integration/lab_test.go`.
7. **Golden tests.** Add `golden_test.go` with `-update`, generate goldens, review them by hand against §4.5, commit.
8. **Contract freeze (C04).** Add `contract/*.json` fixtures, `TestJSONContractFixtures`, write `docs/cli.md`, `TestCLIDocCoversCommands`.
9. **Docs.** README, development guide, architecture §6.3, CHANGELOG migration entry.
10. **Evidence.** Write `docs/verification/specs/C01.md` following the README §5.3 template, with the before/after coverage of `internal/adapter/cli` and `cmd/kali-tools`.

## 7. Acceptance criteria

- **AC-1** Given the command tree in §4.3, when `kali-tools help` runs, then every command path is listed, and `kali-tools <path> --help` prints that command's help to stdout and exits 0.
- **AC-2** Given any command in §4.3 except `serve` and `help`, when it runs with `--json`, then stdout is exactly one envelope with `schema_version: 1` and stderr contains no JSON.
- **AC-3** Given the golden fixtures, when each golden case runs, then stdout, stderr and the exit code match the golden file byte for byte in both human and JSON mode, and in `--lang pt-BR` for the localized cases.
- **AC-4** Given each error code in §4.6, when a command fails with that code, then the process exits with the mapped exit code.
- **AC-5** Given a malformed flag value containing `synthetic-secret` (for example `--port synthetic-secret`, `--timeout synthetic-secret`), when the CLI rejects it, then the exit code is 2 and neither stdout nor stderr contains `synthetic-secret`.
- **AC-6** Given a running diagnostic, when the context is cancelled (Ctrl-C), then `Run` returns 130 within 3 s, the envelope error code is `cancelled`, `JobSnapshot` reports `cancelled`, and the history row is `cancelled`.
- **AC-7** Given a running Nmap inspection or live capture, when the context is cancelled, then the fake runner/provider observes cancellation, `Run` returns 130, no export file is created, and after `Close` no goroutine started by the command remains (checked under `-race` by comparing `runtime.NumGoroutine()` before and after, polling for at most 3 s; no new dependency).
- **AC-8** Given `--detach` on any command, then the exit code is 2 with `invalid_input`.
- **AC-9** Given stdout is not a terminal and `--no-color` is absent, then no output contains the byte `0x1b`.
- **AC-10** Given `wifi reveal` without `--confirm`, then the exit code is 2 and the fake Wi-Fi provider's reveal counter is 0.
- **AC-11** Given `wifi reveal --confirm`, then stderr contains the warning, stdout contains the synthetic key, and `History` contains no row and no summary containing the key.
- **AC-12** Given each line of `legacy_invocations.txt`, when it runs, then the exit code and error code match, and deprecated forms print exactly one deprecation warning on stderr.
- **AC-13** Given the contract fixtures, when a fixture path is removed from a command's JSON without a `schema_version` bump, then `TestJSONContractFixtures` fails.
- **AC-14** Given `docs/cli.md`, then it has a `## <command path>` heading for every command path in the command table.
- **AC-15** Given `--output` without `--format pcap|pcapng` on any capture command, then the exit is 2 before any capture starts and the destination file does not exist.
- **AC-16** [native] Given a Windows 11 host reachable by SSH, when `ssh -T host kali-tools doctor` and `ssh -T host kali-tools --json interfaces` run, then both complete without a TTY, without prompts and without ANSI sequences. Record the output in `docs/verification/`.
- **AC-17** Given the Docker lab, when `TestCLIExecutable` runs the built binary with `--json`, then all cases pass, including `wifi reveal` without `--confirm` exiting 2.

## 8. Tests

Package `internal/adapter/cli` (black-box `cli_test` package where possible):

| Test | Asserts | AC |
| --- | --- | --- |
| `TestHelpListsEveryCommand` | `help` output contains every `command.path`; `<path> --help` exits 0 | AC-1 |
| `TestJSONModeAlwaysEnvelope` | table over all commands with fakes: one envelope, `schema_version == 1`, including parse errors | AC-2 |
| `TestGolden` | runs `testdata/golden/*.golden` cases; `-update` rewrites; `go version` normalized to `goX` | AC-3 |
| `TestExitCodeTable` | every code in §4.6 maps as documented; unknown code → 1 | AC-4 |
| `TestExitCodePerErrorCode` | fakes returning each code produce the mapped process exit | AC-4 |
| `TestFlagErrorsDoNotEchoValues` | `--port`, `--timeout`, `--ports`, `--target` (HTTP URL with credentials) with `synthetic-secret` | AC-5 |
| `TestInterruptCancelsDiagnostic` | blocking fake diagnostic; cancel after `running` event; 130, `cancelled` everywhere | AC-6 |
| `TestInterruptCancelsInspection` | fake runner records `ctx.Done()`; 130 | AC-7 |
| `TestInterruptStopsCaptureWithoutExport` | fake capture provider; `StopCapture` called; no file at `--output` | AC-7 |
| `TestDetachRejected` | `diagnose tcp --detach --target x` → 2 | AC-8 |
| `TestNoColorWithoutTerminal` | buffers and an `os.Pipe` writer produce no `0x1b` | AC-9 |
| `TestWiFiRevealRequiresConfirm` | reveal counter stays 0; exit 2 | AC-10 |
| `TestWiFiRevealWarnsAndNeverPersists` | warning on stderr; key only on stdout; history unchanged | AC-11 |
| `TestLegacyInvocationsStillParse` | each legacy line; one deprecation warning for deprecated forms | AC-12 |
| `TestJSONContractFixtures` | fixture paths present with same types | AC-13 |
| `TestCLIDocCoversCommands` | `docs/cli.md` headings cover the command table | AC-14 |
| `TestCaptureOutputRequiresFormatFirst` | all three capture commands; no file created | AC-15 |
| `TestTimeoutParsing` | `100` and `100ms` accepted; `99`, `10m1s`, `NaN` rejected; boundary values | AC-5 |
| `TestPortListLimits` | 32 ports accepted by the CLI; 33 rejected before the app is called | AC-15 |
| `TestJobIDValidation` | 32 hex accepted; 31, 33, uppercase rejected | AC-4 |

`newTestApp(t)` builds `*app.Application` from the H07 in-memory fakes with a fixed clock `2026-10-19T12:00:00Z`, deterministic job IDs, and fixture data identical to `frontend/tests/workflows.spec.ts` (interface "Fixture Ethernet", Wi-Fi "Fixture Network", synthetic key `synthetic-test-key`). If H07 provides no way to build an Application from fakes, STOP and report.

Updated: `cmd/kali-tools/main_test.go` (all five tests gain `--json`; assertions unchanged), `tests/integration/lab_test.go` `TestCLIExecutable` (AC-17).

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
go test -race -count=1 ./internal/adapter/cli/... ./cmd/kali-tools/...
go test ./internal/adapter/cli -run 'TestGolden|TestJSONContractFixtures|TestCLIDocCoversCommands' -count=1
go run ./cmd/kali-tools --json version
go run ./cmd/kali-tools wifi reveal; echo "exit=$?"        # expect exit=2
go run ./cmd/kali-tools diagnose tcp --target 127.0.0.1 --port 9 --timeout 200ms; echo "exit=$?"
bash scripts/setup/setup-linux-docker.sh test               # or the Windows/macOS variant
node scripts/markdown.mjs
```

## 10. Documentation updates

- `docs/cli.md`: new, the frozen contract (§4.12).
- `README.md` §6: examples use the new forms and `--json`; exit-code paragraph links to `docs/cli.md`.
- `docs/development.md`: CLI examples and the conversion example use `capture import`/`capture export`.
- `docs/architecture.md` §6.3: command list, `cli.Run`, exit-code policy, `serve` dispatch in the cmd root.
- `CHANGELOG.md`: "Migration from 0.1.0-alpha.1-dev" entry listing §4.9 breaking changes.
- `PLAN.md`: tick C01–C04 sub-items only with evidence; AC-16 stays unchecked until a human runs it.

## 11. Risks and rollback

- **Scripts break on the human default.** Mitigation: the migration entry, `--json` added to every in-repo caller, and deprecation warnings for old forms. Rollback: revert step 6 only; steps 1–5 keep JSON as the default.
- **Concurrent SQLite access from CLI and desktop.** WAL and `busy_timeout=5000` exist; a `history clear` during desktop writes may wait up to 5 s. If it fails, the error maps to `failed` (exit 1).
- **Golden churn.** Goldens depend on catalogs and fixtures. Update with `-update` only together with an intentional catalog or contract change, and review the diff.
- **Cancellation races.** The terminal event may be dropped by a slow subscriber; the `JobSnapshot` fallback covers it. If AC-6 is flaky, do not raise the timeouts; check that the command subscribes before it starts the job.
- Rollback is per commit: each numbered step is one Conventional Commit, e.g. `feat(cli): per-command flags and exit codes (C01)`.

## 12. Quality and delivery assertions

### 12.1 Applicable quality gates

| Gate | Applies | Threshold for this spec |
| --- | --- | --- |
| QG-1, QG-2, QG-3, QG-4 | yes | as in README §5.1 |
| QG-5 | yes | `go test -race ./internal/adapter/cli/... ./cmd/kali-tools/...` has no race report |
| QG-6 | yes | `internal/adapter/cli` ≥ 85 % statements (tightened from 80 %, because the package is pure dispatch and fully testable with fakes); `cmd/kali-tools` not lower than its pre-spec value (record both) |
| QG-7 | yes | as in README §5.1 |
| QG-8 | yes | Playwright workflow count unchanged (this spec adds no UI) |
| QG-9 | no | no binding change |
| QG-10 | yes | `internal/adapter/cli` imports no `internal/adapter/{storage,tools,probe,platform,httpapi,wailsui}` |
| QG-11 | yes | AC-5 and AC-11 tests with `synthetic-secret` and `synthetic-test-key` |
| QG-12 | yes | every constant in §4.10 has a limit and limit+1 test |
| QG-13, QG-14, QG-15 | yes | as in README §5.1 |

### 12.2 Delivery assertions

| DA | Assertion | Command / test / artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | Every command path in §4.3 is reachable and has help | `go test ./internal/adapter/cli -run TestHelpListsEveryCommand -count=1` | pass | CI |
| DA-2 | Human and JSON outputs match the goldens | `go test ./internal/adapter/cli -run TestGolden -count=1` | pass; `git status --porcelain internal/adapter/cli/testdata` empty afterwards | CI |
| DA-3 | Exit codes follow §4.6 | `go test ./internal/adapter/cli -run 'TestExitCodeTable\|TestExitCodePerErrorCode' -count=1` | pass; table covers all 14 error codes | CI |
| DA-4 | Rejected flag values are never echoed | `go test ./internal/adapter/cli -run TestFlagErrorsDoNotEchoValues -count=1` | pass | CI |
| DA-5 | Ctrl-C cancels through the job manager | `go test -race ./internal/adapter/cli -run 'TestInterrupt' -count=1` | 3 tests pass, each under 3 s | CI |
| DA-6 | Wi-Fi reveal needs `--confirm` and never persists | `go test ./internal/adapter/cli -run 'TestWiFiReveal' -count=1` | 2 tests pass | CI |
| DA-7 | Legacy invocations keep working | `go test ./internal/adapter/cli -run TestLegacyInvocationsStillParse -count=1` | every line of `testdata/legacy_invocations.txt` passes | CI |
| DA-8 | The JSON contract is frozen | `go test ./internal/adapter/cli -run 'TestJSONContractFixtures\|TestCLIDocCoversCommands' -count=1` | pass; one fixture per JSON-producing command path | CI |
| DA-9 | `docs/cli.md` exists with the required sections | reviewer opens `docs/cli.md` and checks the §4.12 section list | all sections present | reviewer |
| DA-10 | Docker lab runs the built CLI with `--json` | `bash scripts/setup/setup-linux-docker.sh test` | exit 0; `TestCLIExecutable` passes | CI |
| DA-11 | SSH without a TTY | `ssh -T <host> kali-tools doctor` and `ssh -T <host> kali-tools --json interfaces` on Windows 11 | both exit 0, no `0x1b` byte in output | [native] |
| DA-12 | Migration is documented | `CHANGELOG.md` `[Unreleased]` and `docs/cli.md` "Migration from 0.1.0-alpha.1-dev" list the 7 breaking changes of §4.9 | 7 of 7 listed | reviewer |

Evidence record: `docs/verification/specs/C01.md`.
