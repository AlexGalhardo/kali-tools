---
name: golang-context
description: "context.Context in kali-tools — propagation from Wails bindings, HTTP handlers and CLI commands through app to adapters, deadlines on network, subprocess and SQLite calls, manager-scoped job contexts, SSE stream cancellation, and the single Wails runtime-context exception. Use when designing cancellation or timeouts, starting a job, or reviewing context use."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.3.1"
---

# Go context.Context Best Practices

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md`; they win over anything below.

`context.Context` is Go's mechanism for propagating cancellation signals, deadlines, and request-scoped values across API boundaries and between goroutines. Think of it as the "session" of a request — it ties together every operation that belongs to the same unit of work.

## Best Practices Summary

1. Propagate the same context through the entire call chain: binding, HTTP handler or CLI command → `app` → capability → adapter (SQLite, subprocess, socket) — any link that starts a fresh context keeps working after cancellation.
2. Take `ctx` as the first parameter, named `ctx context.Context` — the fixed position is what makes context-aware APIs recognizable at a glance and what linters check.
3. Pass context through function parameters instead of storing it in a struct — the struct outlives the request that filled it. The single accepted exception in kali-tools is the Wails runtime context from `OnStartup`, confined to the Wails adapter (`docs/architecture-research.md` §3.6).
4. Pass `context.TODO()` rather than a `nil` context — `nil` panics on the first `Done()` or `Value()` call, far from the caller that passed it.
5. Call `cancel()` on all control-flow paths for `WithCancel`/`WithTimeout`/`WithDeadline`, unless ownership of the context and cancel function is explicitly returned or transferred — an uncalled `cancel()` keeps the child attached to its parent and leaks its timer until the parent finishes.
6. Create `context.Background()` only at top-level entry points (main, init, tests). Deeper in the call chain — especially mid-request — it detaches the work from the caller's deadline and cancellation, the propagation break shown below.
7. Use `context.TODO()` as a placeholder when a context is needed but none exists yet — it marks the gap for a later fix instead of hiding it behind a `Background()` that looks deliberate.
8. Declare context value keys as unexported types — with a plain `string` key, two packages using `"user"` silently overwrite each other.
9. Carry only request-scoped metadata in context values, never function parameters — values retrieved through `Value()` lose compile-time typing and disappear from the function signature.
10. Jobs outlive the call that started them, so they run under a context derived from the job manager (application lifetime), not from the Wails call, HTTP request or `WithoutCancel(r.Context())`. Application shutdown must still cancel them.
11. Never put dependencies (store, `*sql.DB`, transaction, logger configuration) in a context; pass them through constructors.
12. Every network operation, subprocess and parser read has a deadline: derive it with `context.WithTimeout` from the validated request timeout.

## Creating Contexts

| Situation | Use |
| --- | --- |
| Entry point (main, init, test) | `context.Background()` |
| Function needs context but caller doesn't provide one yet | `context.TODO()` |
| Inside an HTTP handler (`internal/adapter/httpapi`) | `r.Context()` for the request; the manager context for jobs it starts |
| CLI | `signal.NotifyContext(context.Background(), os.Interrupt)` in `main` |
| Wails binding | the stored runtime context only for `runtime.*` calls; jobs use the manager context |
| Need cancellation control | `context.WithCancel(parentCtx)` |
| Need a deadline/timeout | `context.WithTimeout(parentCtx, duration)` |

## Context Propagation: The Core Principle

The most important rule: **propagate the same context through the entire call chain**. When you propagate correctly, cancelling the parent context cancels all downstream work automatically.

```go
// ✗ Bad — creates a new context, breaking the chain
func (s *Store) PutJob(ctx context.Context, j Job) error {
    _, err := s.db.ExecContext(context.Background(), "INSERT INTO jobs (id, state) VALUES (?, ?)", j.ID, j.State)
    return err
}

// ✓ Good — propagates the caller's context
func (s *Store) PutJob(ctx context.Context, j Job) error {
    _, err := s.db.ExecContext(ctx, "INSERT INTO jobs (id, state) VALUES (?, ?)", j.ID, j.State)
    return err
}
```

## Deep Dives

- **[Cancellation, Timeouts & Deadlines](./references/cancellation.md)** — How cancellation propagates: `WithCancel` for manual cancellation, `WithTimeout` for automatic cancellation after a duration, `WithDeadline` for absolute time deadlines. Patterns for listening (`<-ctx.Done()`) in concurrent code, `AfterFunc` callbacks, and `WithoutCancel` for operations that must outlive their parent request (e.g., audit logs).

- **[Context Values](./references/values-tracing.md)** — Safe context value patterns: unexported key types to prevent namespace collisions, and what belongs in a value versus a parameter.

- **[Context in the HTTP adapter, diagnostics and storage](./references/http-services.md)** — `r.Context()` in handlers, SSE streams, starting jobs from a handler, `NewRequestWithContext` for HTTP diagnostics, `exec.CommandContext` and `*Context` SQL variants.

## Cross-References

- → See the `golang-concurrency` skill for goroutine cancellation patterns using context
- → See the `golang-database` skill for context-aware database operations (QueryContext, ExecContext)
- → See the `golang-design-patterns` skill for timeout and resilience patterns

## Enforce with Linters

Many context pitfalls are caught automatically by linters: `govet`, `staticcheck`. → See the `golang-lint` skill for configuration and usage.
