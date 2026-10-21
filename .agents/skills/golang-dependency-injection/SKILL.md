---
name: golang-dependency-injection
description: "Manual dependency injection in kali-tools — constructor injection, consumer-side ports, the shared internal/bootstrap composition used by the Wails root, the CLI and `serve`, the run() pattern, cleanup order and fakes in tests. Use when wiring a new adapter or use case, adding a port, removing a global or init() side effect, or reviewing a composition root. DI frameworks (Wire, dig, fx, samber/do) are not used."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.3.0"
---

# Dependency Injection in kali-tools

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT; library sections removed because the project wires manually). Binding project rules are in `AGENTS.md`, `docs/architecture.md` §4 and `docs/architecture-research.md` §3.2, §3.4 and §6.7; they win over anything below.

**Persona:** you choose the simplest wiring that keeps dependencies explicit, compile-checked and greppable.

## Modes

- **Wire:** adding an adapter or use case. Add a constructor, declare the port in the consumer, wire it in the composition root (or `internal/bootstrap` after PLAN H01).
- **Refactor:** removing coupling. Look for package-level state, `init()` side effects, inner packages reading flags or environment variables, and capability or app packages importing `internal/adapter/*` (deviations D1, D2, D5).

## Rules

1. Dependencies are passed through constructors or struct fields. No globals, no `init()` service setup, no service locator.
2. Wire manually. Google Wire is archived; dig and fx resolve at runtime with reflection; samber/do is a container. None are dependencies, and adding one would need an ADR.
3. A port is a small interface declared **in the package that consumes it** (`app`, `network`, `capture`). Adapters return concrete types and never import the port's package just to implement it.
4. Add an interface only at a real boundary: a driven dependency (OS, process, SQLite, socket) or a test stand-in for one. Inside a layer, call functions directly.
5. Only composition roots (`main.go`, `cmd/*`, `internal/bootstrap`) know concrete adapters. Nothing imports a composition root.
6. Only `main` reads flags, `os.Args` and environment variables. Inner packages receive values (`docs/architecture-research.md` §3.4). Loggers are dependencies too.
7. Put compile-time assertions in bootstrap or tests, not in the adapter: `var _ app.JobStore = (*storage.Store)(nil)`.
8. Executable paths are resolved once at bootstrap and injected; nothing calls `exec.LookPath` per request.
9. Never put a dependency (store, transaction, logger) in a `context.Context`.

## Ports in practice

```go
package app

// Declared here because app consumes it. The adapter returns *storage.Store.
type JobStore interface {
    PutJob(ctx context.Context, j Job) error
    History(ctx context.Context) ([]Job, error)
}

type Service struct {
    store JobStore
    clock Clock
}

func NewService(store JobStore, clock Clock) *Service {
    return &Service{store: store, clock: clock}
}
```

```go
package network

// Prober is the port that removes the network -> adapter/probe import (D1, PLAN H03).
type Prober interface {
    Echo(ctx context.Context, target netip.Addr, timeout time.Duration) (EchoResult, error)
}
```

## Composition

Target shape (PLAN H01): one shared builder used by every composition root, so the desktop, the CLI and `serve` construct the same application graph.

```go
package bootstrap

func New(ctx context.Context, cfg Config) (*app.Application, func(), error) {
    store, err := storage.Open(cfg.DBPath)
    if err != nil {
        return nil, nil, fmt.Errorf("opening store: %w", err)
    }
    inventory := tools.Discover(ctx)            // resolve executables once
    host := platform.New()                      // build-tagged implementation
    a := app.New(store, inventory, host, app.SystemClock{})
    cleanup := func() {
        a.Close()     // cancel jobs first
        store.Close() // then close driven adapters
    }
    return a, cleanup, nil
}
```

Names above are illustrative; follow the actual constructors in the code and the H01 task.

Each root keeps the `run` pattern so it can be tested:

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()
    os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}
```

Initialize in dependency order (driven adapters, then application, then driving adapter) and clean up in reverse. Cleanup cancels jobs and child processes before closing the store.

## Testing with DI

- Use-case tests pass hand-written fakes of the ports declared in the consumer. No mocking framework.
- Every port gets a contract suite in `internal/porttest`, run against both the real adapter and its fake (PLAN H07).
- Driving adapters are tested with a real `app.Application` built from fakes: CLI `run` with buffers, `httptest` for `httpapi`, and a Wails binding whose `EventsEmit` is a `func(ctx, string, ...any)` field.

```go
func TestStartRecordsQueuedJob(t *testing.T) {
    store := &fakeJobStore{jobs: map[string]app.Job{}}
    svc := app.NewService(store, fixedClock{})
    // ...
}
```

## Common mistakes

| Mistake | Fix |
| --- | --- |
| Interface declared next to its only implementation "for mocking" | Declare it in the consumer, or call the concrete type |
| Capability or app package importing `internal/adapter/*` | Declare a port in the consumer and inject the adapter (H03) |
| `os.Getenv` or a flag read below `cmd`/root | Read it in `main`, pass the value |
| Wiring duplicated between `main.go` and `cmd/kali-tools` | Move it into `internal/bootstrap` (H01) |
| Global logger or `http.DefaultClient` in inner packages | Inject them |
| Adding Wire, fx, dig or samber/do | Not used; manual wiring is the decision (architecture research §13) |
| Container or `*app.Application` passed into capability packages | Pass only the specific dependency |

## Cross-references

- `golang-structs-interfaces` for small, consumer-side interfaces.
- `golang-project-layout` for where composition roots and adapters live.
- `golang-testing` for fakes and contract suites.

## References

- [Go Code Review Comments: interfaces](https://go.dev/wiki/CodeReviewComments#interfaces)
- [How I write HTTP services in Go after 13 years (Mat Ryer)](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/)
