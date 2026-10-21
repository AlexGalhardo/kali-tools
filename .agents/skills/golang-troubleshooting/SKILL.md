---
name: golang-troubleshooting
description: "Troubleshoot kali-tools Go code systematically — reproduce, one hypothesis at a time, root cause before fix; common Go bugs, test-driven debugging, race detection, local pprof profiles (no pprof endpoint), Delve, GODEBUG, and project-specific checks (capability states, platform files, subprocess deadlines, privileges). Use when something crashes, hangs, races or behaves unexpectedly."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.3.1"
---

# Go Troubleshooting Guide

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md`; they win over anything below.

**Persona:** You are a Go systems debugger. You follow evidence, not intuition — instrument, reproduce, and trace root causes systematically.

## Modes

- **Single-issue debug** (default): Follow the sequential Golden Rules — read the error, reproduce, one hypothesis at a time. Do not launch sub-agents; focused sequential investigation is faster for a single known symptom.
- **Codebase bug hunt** (explicit audit request): cover one bug category at a time (nil/interface, resources, error handling, races, context/slice/map, unbounded input); categories can be split across parallel reviewers. Use this mode only when asked for a broad sweep.

## Tools

Delve (`dlv`) is optional; install it at a pinned version, never `@latest`. kali-tools exposes no `net/http/pprof` endpoint; profile locally from tests or a debug build (see [pprof.md](./references/pprof.md)).

**NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST.** Symptom fixes create new bugs and waste time. This process applies ESPECIALLY under time pressure — rushing leads to cascading failures that take longer to resolve.

When the user reports a bug, crash, performance problem, or unexpected behavior in Go code:

1. **Start with the Decision Tree** below to identify the symptom category and jump to the relevant section.
2. **Follow the Golden Rules** — especially: reproduce before you fix, one hypothesis at a time, find the root cause.
3. **Work through the General Debugging Methodology** step by step. Do not skip steps.
4. **Watch for Red Flags** in your own reasoning. If you catch yourself guessing at fixes without understanding the cause, stop and gather more evidence.
5. **Escalate tools incrementally.** Start with the simplest diagnostic (`fmt.Println`, test isolation) and only reach for pprof, Delve, or GODEBUG when simpler tools are insufficient.
6. **Never propose a fix you cannot explain.** If you do not understand why the bug happens, say so and investigate further.

## Quick Decision Tree

```text
WHAT ARE YOU SEEING?

"Build won't compile"
  → go build ./... 2>&1, go vet ./...
  → See [compilation.md](./references/compilation.md)

"Wrong output / logic bug"
  → Write a failing test → Check error handling, nil, off-by-one
  → See [common-go-bugs.md](./references/common-go-bugs.md), [testing-debug.md](./references/testing-debug.md)

"Random crashes / panics"
  → GOTRACEBACK=all ./app → go test -race ./...
  → See [common-go-bugs.md](./references/common-go-bugs.md), [diagnostic-tools.md](./references/diagnostic-tools.md)

"Sometimes works, sometimes fails"
  → go test -race ./...
  → See [concurrency-debug.md](./references/concurrency-debug.md), [testing-debug.md](./references/testing-debug.md)

"Program hangs / frozen"
  → go test -timeout 30s (dumps all goroutine stacks), SIGQUIT on Unix,
    or pprof.Lookup("goroutine").WriteTo(os.Stderr, 2) in a debug build
  → See [concurrency-debug.md](./references/concurrency-debug.md), [pprof.md](./references/pprof.md)

"High CPU usage"
  → pprof CPU profiling
  → See [performance-debug.md](./references/performance-debug.md), [pprof.md](./references/pprof.md)

"Memory growing over time"
  → pprof heap profiling
  → See [performance-debug.md](./references/performance-debug.md), [concurrency-debug.md](./references/concurrency-debug.md)

"Slow / high latency / p99 spikes"
  → CPU + mutex + block profiles
  → See [performance-debug.md](./references/performance-debug.md), [diagnostic-tools.md](./references/diagnostic-tools.md)

"Works on Windows, fails on Linux/macOS (or the reverse)"
  → GOOS=<os> go vet ./... ; check the _windows/_linux/_other files and
    the unsupported fallback; native behavior needs a real machine

"Job never finishes / tool hangs"
  → Check the subprocess deadline, output cap and descendant kill in
    internal/adapter/tools; check ctx propagation from the job manager

"Capture / ICMP / Wi-Fi fails"
  → Check the capability state first (permission_required, unsupported,
    dependency_missing) before debugging code

"Simple bug, easy to reproduce"
  → Write a test, add fmt.Println / log.Debug
  → See [testing-debug.md](./references/testing-debug.md)
```

**Remember:** Read the Error → Reproduce → Measure One Thing → Fix → Verify

Most Go bugs are: missing error checks, nil pointers, forgotten context cancel, unclosed resources, race conditions, or silent error swallowing.

## The Golden Rules

### 1. Read the Error Message First

Go error messages are precise. Read them fully before doing anything else:

- **File and line number** → go directly there
- **Type mismatch** → check function signatures, interface satisfaction
- **"undefined"** → check imports, exported names, build tags
- **"cannot use X as Y"** → check concrete types vs interfaces

### 2. Reproduce Before You Fix

NEVER debug by guessing — reproduce first. Always:

- Write a failing test that captures the bug
- Make it deterministic
- Isolate the minimal failing example
- Use `git bisect` to find the breaking commit

### 3. If You Don't Measure It, You're Guessing

Never rely on intuition for performance or concurrency bugs:

- **pprof over intuition**
- **race detector over reasoning**
- **benchmarks over assumptions**

### 4. One Hypothesis at a Time

Change one thing, measure, confirm. If you change three things at once, you learn nothing.

### 5. Find the Root Cause — No Workarounds

You MUST understand **why** the bug happens before writing a fix. A band-aid that masks the symptom leaves the defect in place, so it resurfaces elsewhere — usually further from its cause and harder to trace the second time.

When you don't understand the issue:

- **Trace the data flow backwards** from the symptom to its origin.
- **Question your assumptions.** The code you trust might be wrong.
- **Ask "why" five times.** Keep going until you reach the actual root cause.
- **Perform more troubleshooting checks.** More fmt.Println, more output inspection...

### 6. Research the Codebase, Not Just the Diff

Before flagging a bug or proposing a fix, trace the data flow and check for upstream handling. A function that looks broken in isolation may be correct in context — callers may validate inputs, middleware may enforce invariants, or the surrounding code may guarantee conditions the function relies on.

1. **Trace callers** — who calls this function and with what values? Use `gopls references` or code search; gopls also resolves calls through interfaces and embedding. In kali-tools, check all three driving adapters (Wails `app.go`, CLI, HTTP) and every `GOOS` variant.
2. **Check upstream validation** — input parsing, type conversions, or guard clauses earlier in the chain may make the "bug" unreachable.
3. **Read the surrounding code** — middleware, interceptors, or init functions may set up state the function depends on.

**When the context reduces severity but doesn't eliminate the issue:** still report it at reduced priority with a note explaining which upstream guarantees protect it. Add a brief inline comment (e.g., `// note: safe because caller validates via parseID() which returns uint`) so the reasoning is documented for future reviewers.

### 7. Start Simple

Sometimes `fmt.Println` IS the right tool for local debugging; remove it before committing. Escalate tools only when simpler approaches fail. Never print or log Wi-Fi keys, the web-server token or packet payloads while debugging.

## Red Flags: You're Debugging Wrong

If any of these are happening, stop and return to Step 1:

- **"Quick fix for now, investigate later"** — There is no "later". Find the root cause.
- **Multiple simultaneous changes** — One hypothesis at a time.
- **Proposing fixes without understanding the cause** — "Maybe if I add a nil check here..." is guessing, not debugging.
- **Each fix reveals a new problem** — You're treating symptoms. The real bug is elsewhere.
- **3+ fix attempts on the same issue** — You have the wrong mental model. Re-read the code, trace the data flow from scratch.
- **"It works on my machine"** — You haven't isolated the environmental difference.
- **Blaming the framework/stdlib/compiler** — It's almost never a Go bug. Verify your code first.

## Reference Files

- **[General Debugging Methodology](./references/methodology.md)** — The systematic 10-step process: define symptoms, isolate reproduction, form one hypothesis, test it, verify the root cause, and defend against regressions. Escalation guide: when to escalate from `fmt.Println` to logging to pprof to Delve, and how to avoid the trap of multiple simultaneous changes.

- **[Common Go Bugs](./references/common-go-bugs.md)** — The bugs that crash Go code: nil pointer dereferences, interface nil gotcha (typed nil ≠ nil), variable shadowing, slice/map/defer/error/context pitfalls, race conditions, JSON unmarshaling surprises, unclosed resources. Each with reproduction patterns and fixes.

- **[Test-Driven Debugging](./references/testing-debug.md)** — Why writing a failing test is the first step of debugging. Covers test isolation techniques, table-driven test organization for narrowing failures, useful `go test` flags (`-v`, `-run`, `-count=10` for flaky tests), and debugging flaky tests.

- **[Concurrency Debugging](./references/concurrency-debug.md)** — Race conditions, deadlocks, goroutine leaks. When to use the race detector (`-race`), how to read race detector output, patterns that hide races, detecting leaks with `testing/synctest` and the goroutine-leak profile, analyzing stack dumps for deadlock clues.

- **[Performance Troubleshooting](./references/performance-debug.md)** — When your code is slow: CPU profiling workflow, memory analysis (heap vs alloc_objects profiles, finding leaks), lock contention (mutex profile), and I/O blocking (goroutine profile). How to read flamegraphs, identify hot functions, and measure improvement with benchmarks.

- **[pprof Reference](./references/pprof.md)** — Local profiling from tests and debug builds, profile types (CPU, heap, goroutine, goroutineleak, mutex, block, trace) and interactive analysis (`top`, `list`, `web`).

- **[Diagnostic Tools](./references/diagnostic-tools.md)** — Auxiliary tools for specific symptoms. GODEBUG environment variables (GC tracing, scheduler tracing), Delve debugger for breakpoint debugging, escape analysis (`go build -gcflags="-m"` to find unintended heap allocations), Go's execution tracer for understanding goroutine scheduling.

- **[Compilation Issues](./references/compilation.md)** — Build failures: module version conflicts, CGO linking problems, version mismatch between `go.mod` and installed Go version, platform-specific build tags preventing cross-compilation.

- **[Code Review Red Flags](./references/code-review-flags.md)** — Patterns to watch during code review that signal potential bugs: unchecked errors, missing nil checks, concurrent map access, goroutines without clear exit, resource leaks from defer in loops.

## Cross-References

- → See `golang-concurrency`, `golang-safety`, `golang-error-handling` and `golang-testing` skills
