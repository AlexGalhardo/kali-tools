---
name: golang-concurrency
description: "Go concurrency in kali-tools — goroutine lifecycle and leak prevention, channels and select, mutexes and atomics, bounded queues, the job manager model (manager-scoped job contexts, bounded Subscribe with drop counting, cancellation), and shutdown tests with synctest and -race. Use when writing or reviewing goroutines, channels, worker pools, event fan-out or SSE subscribers. Not for nil or aliasing bugs (golang-safety) or debugging a hang (golang-troubleshooting)."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.2.1"
---

# Go Concurrency Best Practices

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md`; they win over anything below.

**Persona:** You are a Go concurrency engineer. You assume every goroutine is a liability until proven necessary — correctness and leak-freedom come before performance.

## Modes

- **Write mode** — implement concurrent code (goroutines, channels, sync primitives, worker pools, pipelines). Follow the sequential instructions below.
- **Review mode** — reviewing a PR's concurrent code changes. Focus on the diff: check for goroutine leaks, missing context propagation, ownership violations, and unprotected shared state. Sequential.
- **Audit mode** — auditing existing concurrent code across a codebase, using the categories in the "Concurrency audits" section.

Go's concurrency model is built on goroutines and channels. Goroutines are cheap but not free — every goroutine you spawn is a resource you must manage. The goal is structured concurrency: every goroutine has a clear owner, a predictable exit, and proper error propagation.

## Core Principles

1. **Every goroutine must have a clear exit** — without a shutdown mechanism (context, done channel, WaitGroup), they leak and accumulate until the process crashes
2. **Share memory by communicating** — channels transfer ownership explicitly; mutexes protect shared state but make ownership implicit
3. **Send copies, not pointers** on channels — sending pointers creates invisible shared memory, defeating the purpose of channels
4. **Only the sender closes a channel** — closing from the receiver side panics if the sender writes after close
5. **Specify channel direction** (`chan<-`, `<-chan`) — the compiler prevents misuse at build time
6. **Every queue is bounded** (AGENTS.md) — default to unbuffered channels; when a buffer is needed, give it an explicit size and decide what happens when it is full. Event fan-out to subscribers never blocks the producer: a slow subscriber drops events and the drop is counted
7. **Always include `ctx.Done()` in select** — without it, goroutines leak after caller cancellation
8. **Avoid repeated `time.After` in hot loops** — each call allocates a timer and creates unnecessary churn; use `time.NewTimer` + `Reset` for long-running loops
9. **Prove goroutine shutdown in tests** with the race detector, `testing/synctest` and `runtime.NumGoroutine` checks; `go.uber.org/goleak` is not a dependency and needs license review in `docs/dependencies.md` before it is added

For detailed channel/select code examples, see [Channels and Select Patterns](references/channels-and-select.md).

## Channel vs Mutex vs Atomic

| Scenario | Use | Why |
| --- | --- | --- |
| Passing data between goroutines | Channel | Communicates ownership transfer |
| Coordinating goroutine lifecycle | Channel + context | Clean shutdown with select |
| Protecting shared struct fields | `sync.Mutex` / `sync.RWMutex` | Simple critical sections |
| Simple counters, flags | `sync/atomic` | Lock-free, lower overhead |
| Many readers, few writers on a map | `sync.Map` | Optimized for read-heavy workloads. **Concurrent map read/write causes a hard crash** |
| Caching expensive computations | `sync.Once` / `singleflight` | Execute once or deduplicate |

## WaitGroup vs errgroup

`golang.org/x/sync` (errgroup, singleflight, semaphore) is not in `go.mod` today. Prefer `sync.WaitGroup`, a buffered-channel semaphore or `errors.Join`; adding `x/sync` is a dependency change recorded in `docs/dependencies.md`.

| Need | Use | Why |
| --- | --- | --- |
| Wait for goroutines, errors not needed | `sync.WaitGroup` | Fire-and-forget |
| Wait + collect first error | `errgroup.Group` | Error propagation |
| Wait + cancel siblings on first error | `errgroup.WithContext` | Context cancellation on error |
| Wait + limit concurrency | `errgroup.SetLimit(n)` | Built-in worker pool |

## Sync Primitives Quick Reference

| Primitive | Use case | Key notes |
| --- | --- | --- |
| `sync.Mutex` | Protect shared state | Keep critical sections short; never hold across I/O |
| `sync.RWMutex` | Many readers, few writers | Never upgrade RLock to Lock (deadlock) |
| `sync/atomic` | Simple counters, flags | Prefer typed atomics (Go 1.19+): `atomic.Int64`, `atomic.Bool` |
| `sync.Map` | Concurrent map, read-heavy | No explicit locking; use `RWMutex`+map when writes dominate |
| `sync.Pool` | Reuse temporary objects | Always `Reset()` before `Put()`; reduces GC pressure |
| `sync.Once` | One-time initialization | Go 1.21+: `OnceFunc`, `OnceValue`, `OnceValues` |
| `sync.WaitGroup` | Waiting for simple goroutines | Go 1.25+: prefer `wg.Go(func(){ ... })` for fire-and-wait tasks that do not panic and do not need error propagation. For Go <1.25 use `Add`/`Done`. For errors/cancellation/limits, use `errgroup` with context. |
| `x/sync/singleflight` | Deduplicate concurrent calls | Cache stampede prevention |
| `x/sync/errgroup` | Goroutine group + errors | `SetLimit(n)` replaces hand-rolled worker pools |

For detailed examples and anti-patterns, see [Sync Primitives Deep Dive](references/sync-primitives.md).

## Concurrency Checklist

Before spawning a goroutine, answer:

- [ ] **How will it exit?** — context cancellation, channel close, or explicit signal
- [ ] **Can I signal it to stop?** — pass `context.Context` or done channel
- [ ] **Can I wait for it?** — `sync.WaitGroup` or `errgroup`
- [ ] **Who owns the channels?** — creator/sender owns and closes
- [ ] **Should this be synchronous instead?** — don't add concurrency without measured need

## Pipelines and Worker Pools

For pipeline patterns (fan-out/fan-in, bounded workers, generator chains, Go 1.23+ iterators), see [Pipelines and Worker Pools](references/pipelines.md).

## Concurrency audits

When auditing concurrency across the codebase, check these independent concerns (they can be split across parallel reviewers):

1. Find all goroutine spawns (`go func`, `go method`) and verify shutdown mechanisms
2. Search for mutable globals and shared state without synchronization
3. Audit channel usage — ownership, direction, closure, buffer sizes
4. Find `time.After` in loops, missing `ctx.Done()` in select, unbounded spawning
5. Check mutex usage, `sync.Map`, atomics, and thread-safety documentation

## Common Mistakes

| Mistake | Fix |
| --- | --- |
| Fire-and-forget goroutine | Provide stop mechanism (context, done channel) |
| Closing channel from receiver | Only the sender closes |
| `time.After` in hot loop | Reuse `time.NewTimer` + `Reset` |
| Missing `ctx.Done()` in select | Always select on context to allow cancellation |
| Unbounded goroutine spawning | Use a bounded worker count or a channel semaphore |
| Sharing pointer via channel | Send copies or immutable values |
| `wg.Add` inside goroutine | Call `Add` before `go` — `Wait` may return early otherwise |
| Forgetting `-race` | Run `go test -race ./...` where the toolchain supports it (cgo is required) |
| Mutex held across I/O | Keep critical sections short |

## kali-tools job model

The job manager (`internal/app` today, `internal/job` after PLAN H02) is the main concurrency boundary. See `docs/architecture-research.md` §6.3.

- A job runs under a context derived from the manager (application lifetime), never from the Wails call or HTTP request that started it. Application shutdown cancels every job.
- Cancellation is a use case reachable from every driving adapter; it must free sockets, capture handles and child processes.
- `Subscribe(ctx, buf)` returns a bounded channel; events carry a monotonic `seq`; a slow consumer drops and the manager counts drops; the producer never blocks.
- Concurrent jobs and SSE subscribers are capped.
- Subprocess readers stop at their output cap and on context cancellation.
- Tests cover state transitions, cancellation, a slow subscriber and shutdown without goroutine leaks under `-race`.

```go
select {
case sub.ch <- ev:
default:
    sub.dropped.Add(1) // never block the producer
}
```

## Cross-References

- → See `golang-context` skill for cancellation propagation and timeout patterns
- → See `golang-safety` skill for concurrent map access and race condition prevention
- → See `golang-troubleshooting` skill for debugging goroutine leaks and deadlocks
- → See `golang-design-patterns` skill for graceful shutdown patterns

### Goroutine leak profile

The goroutine leak profile (experimental behind `GOEXPERIMENT=goroutineleakprofile` in Go 1.26) is generally available in `runtime/pprof` since Go 1.27 — no build flag required. kali-tools does not expose `net/http/pprof`; never add a debug listener to the desktop build or to `serve`. Write the profile from a test or a local debug build with `pprof.Lookup("goroutineleak").WriteTo(w, 2)` instead.

Other tools:

- tests: `testing/synctest` and `runtime.NumGoroutine()` before and after shutdown
- stack dump: `pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)` or `SIGQUIT` on Unix
- race checks: `go test -race ./...`

## References

- [Go Concurrency Patterns: Pipelines](https://go.dev/blog/pipelines)
- [Effective Go: Concurrency](https://go.dev/doc/effective_go#concurrency)
