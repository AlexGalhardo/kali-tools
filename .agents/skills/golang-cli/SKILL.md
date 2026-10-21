---
name: golang-cli
description: "kali-tools CLI development in Go with the stdlib flag package. Use when adding, changing or reviewing a `kali-tools` command, flag, exit code, JSON envelope, signal handling or CLI test, or when moving the command tree into internal/adapter/cli. Cobra and Viper are not used unless an ADR adopts them."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.3.0"
---

# Go CLI for kali-tools

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). The binding rules are in `docs/architecture.md` §6.3 and `docs/architecture-research.md` §7.2; this skill does not override them.

**Persona:** you build a CLI that is scriptable, predictable over SSH (`ssh -T`) and a thin driving adapter over `app`.

## Modes

- **Extend:** adding a command or flag. Read `cmd/kali-tools/main.go` (or `internal/adapter/cli` once PLAN H05 lands) first and follow the existing structure.
- **Review:** check the Common mistakes table, stdout/stderr discipline, exit codes, the JSON envelope and that no business logic lives in the command.

## Stack

| Concern | Choice |
| --- | --- |
| Commands and flags | stdlib `flag`, one `flag.FlagSet` per subcommand, `flag.ContinueOnError` |
| Configuration | explicit flags only; settings live in the SQLite store and go through `app` |
| Output | versioned JSON envelope with `schema_version`; CSV only where a command documents it |
| Version | `domain.Version` |
| Cancellation | `signal.NotifyContext` in `main` |

Cobra, Viper, color, table or TUI libraries are not dependencies. Revisit the parser only through the ADR in PLAN H09 (trigger: the C-phase command tree, shell completion needs and license review). Do not add one in a feature change.

## Structure

```text
cmd/kali-tools/main.go          composition root: signal context, wiring, os.Exit(run(...))
internal/adapter/cli/           target home of the command tree (PLAN H05)
internal/bootstrap/             target shared wiring (PLAN H01)
```

- Target signature: `Run(ctx context.Context, a *app.Application, args []string, stdout, stderr io.Writer) int`.
- `main` is the only place that reads `os.Args`, `os.Getenv`, `os.Stdout` and calls `os.Exit`. Packages below `cmd` never read flags or environment variables.
- A command does three things: decode and validate flags into an application request, call `app`, then encode the result or map the error. If two commands (or the CLI and a Wails binding) contain the same `if`, move it into `internal/app` or a capability package.

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()
    os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
```

## Subcommands and flags

```go
fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
fs.SetOutput(stderr)
target := fs.String("target", "", "host, URL or CIDR")
timeout := fs.Duration("timeout", 3*time.Second, "operation deadline")
if err := fs.Parse(args); err != nil {
    return exitUsage
}
if fs.NArg() != 0 {
    return report(stdout, stderr, domain.Invalid("unexpected positional arguments"))
}
```

- Validate every flag value through the domain constructor or validator (`network.Validate` and similar), never with ad hoc checks only in the CLI.
- Bound list-like flags (ports, targets, durations, packet limits) to the same limits the GUI enforces.
- Planned global flags (PLAN C01): `--json`, `--timeout`, `--no-color`, `--lang`. Do not add others without a PLAN task.

## Output

- stdout carries program output only. Diagnostics, warnings and progress go to stderr.
- JSON output always uses the envelope with `schema_version`, `version`, `data` and `error` (`domain.Error{Code, Message}`). Today every command prints JSON (`discover` also accepts `--format csv`); human output and the `--json` switch arrive with PLAN C01.
- Never print Wi-Fi keys, packet payloads, the web-server token or raw subprocess stderr. `wifi reveal` requires an explicit flag and prints a warning (PLAN C03).
- Capture export writes only to an explicit `--output` path.
- Output is bounded: cap rows and streamed events like any other queue.

## Exit codes

Current behavior of `cmd/kali-tools`:

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Failure, reported in the envelope as `domain.Error` |
| 2 | Usage error: no command or a flag parse error |

Map errors once, in one function per adapter (`docs/architecture-research.md` §6.5). Stable per-code exit values are documented in `docs/cli.md` when PLAN C01/C04 land; do not change them silently.

## Long-running jobs

- Ctrl-C cancels `ctx`, which cancels the job through the same `app`/job manager path the GUI uses.
- Stream progress from the job manager subscription; never start hidden background processes. `--detach` is rejected until a daemon mode exists (PLAN C02).
- `serve` starts `internal/adapter/httpapi`; its security rules are in the `golang-security` skill and `docs/architecture-research.md` §8.

## Testing

- Call `run`/`Run` with `bytes.Buffer` writers and a cancellable context. No subprocess needed.
- Golden tests for human and JSON output; assert exit codes for malformed input and that secrets are never echoed.
- Acceptance specs in `tests/acceptance` run through a CLI driver (PLAN H07).

```go
func TestDiagnoseRejectsPositionalArgs(t *testing.T) {
    var out, errOut bytes.Buffer
    code := run(t.Context(), []string{"diagnose", "extra"}, &out, &errOut)
    if code == 0 {
        t.Fatalf("exit code = 0, want non-zero; stderr=%q", errOut.String())
    }
}
```

## Common mistakes

| Mistake | Fix |
| --- | --- |
| Business logic in a command | Move it to `internal/app` or the capability package |
| Calling driven adapters (`platform`, `tools`, `probe`) from the CLI | Go through `app` (deviation D2, PLAN H04) |
| Writing to `os.Stdout` inside a command | Write to the injected `io.Writer` |
| `os.Exit` outside `main` | Return an exit code; deferred cleanup must run |
| Logs on stdout | stderr only; stdout is the data stream |
| JSON without `schema_version` | Always use the envelope |
| Building a shell string to run a tool | Argument array through `internal/adapter/tools`, never a shell |
| Unbounded flags or output | Apply domain limits and output caps |

## Related skills

`golang-project-layout`, `golang-dependency-injection`, `golang-testing`, `golang-error-handling`, `golang-security`.
