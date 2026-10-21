# Manual Constructor Injection in kali-tools

Manual DI is the project's decision: pass dependencies through constructors. No library, no reflection.

## Order of construction

1. **Values from `main`:** parsed flags, environment lookups, data directory, writers.
2. **Driven adapters:** `storage.Open`, tool discovery (`internal/adapter/tools`), `internal/adapter/platform`, `internal/adapter/probe`, capture and Wi-Fi providers. Each OS-specific constructor has an explicit unsupported fallback.
3. **Application:** `app` use cases and the job manager, receiving the adapters through consumer-side ports.
4. **Driving adapter:** the Wails facade, the CLI command tree or `internal/adapter/httpapi`, receiving only the application value.

Cleanup runs in reverse: stop accepting work, cancel jobs and child processes, then close the store.

## Three roots, one graph

| Root | Builds | Then |
| --- | --- | --- |
| `main.go` (Wails, `desktop`/`bindings` tags) | `bootstrap.New` | `wails.Run` with the facade bound |
| `cmd/kali-tools` | `bootstrap.New` | `cli.Run(ctx, app, args, stdout, stderr)` |
| `kali-tools serve` | `bootstrap.New` | `httpapi` server with the mandatory security middleware |

Until PLAN H01 lands, `main.go`/`app.go` and `cmd/kali-tools` wire directly. When you touch that wiring, move it toward `internal/bootstrap` rather than duplicating it.

## When manual DI stops being enough

At kali-tools' size it does not. Signals that would justify revisiting the decision in an ADR: a graph so large that wiring order is regularly wrong, or a need for lazy construction that cannot be expressed with a constructor. Neither exists today.
