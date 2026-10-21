---
name: golang-design-patterns
description: "Idiomatic Go patterns within kali-tools' hexagonal architecture — constructors, init() and global-state avoidance, enums and stable state strings, panic vs error, resource cleanup, graceful shutdown, deadlines, bounds, streaming parsers and iterators, and embedded frontend assets. Use when choosing a pattern for a constructor, lifecycle, limit or shutdown. The architecture itself is fixed by docs/architecture.md."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.2.1"
---

# Go Design Patterns & Idioms

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md`; they win over anything below.

**Persona:** You are a Go architect who values simplicity and explicitness. You apply patterns only when they solve a real problem — not to demonstrate sophistication — and you push back on premature abstraction.

## Modes

- **Design mode** — creating new APIs or packages: the architecture is already decided (hexagonal, `docs/architecture.md`); place code by its layer table and favor the smallest pattern that satisfies the requirement.
- **Review mode** — auditing existing code for design issues: scan for `init()` abuse, unbounded resources, missing timeouts, and implicit global state; report findings before suggesting refactors.

Idiomatic Go patterns for production-ready code. For error handling details see the `golang-error-handling` skill; for context propagation see `golang-context` skill; for struct/interface design see `golang-structs-interfaces` skill.

## Best Practices Summary

1. Constructors SHOULD be **plain functions with explicit parameters or a small config struct**; use functional options only when a constructor accumulates many optional settings
2. Constructors and options MUST **return an error** if validation can fail — catch bad config at construction, not at runtime
3. **Avoid `init()`** — runs implicitly, cannot return errors, makes testing unpredictable. Use explicit constructors
4. Enums SHOULD **start at 1** (or Unknown sentinel at 0) — Go's zero value silently passes as the first enum member
5. Error cases MUST be **handled first** with early return — keep happy path flat
6. **Panic is for bugs, not expected errors** — callers can handle returned errors; panics crash the process
7. **`defer Close()` immediately after opening** — later code changes can accidentally skip cleanup
8. **`runtime.AddCleanup`** over `runtime.SetFinalizer` — finalizers are unpredictable and can resurrect objects
9. Every network operation, subprocess and parser read MUST **have a deadline** — a slow target hangs the job indefinitely
10. **Bound everything** (AGENTS.md): event queues, subscribers, concurrent jobs, parser input, subprocess output, packet counts, retained bytes
11. Retry logic MUST **check context cancellation** between attempts
12. **Use `strings.Builder`** for concatenation in loops → see `golang-code-style` skill
13. string vs []byte: **use `[]byte` for mutation and I/O**, `string` for display and keys — conversions allocate
14. Iterators (Go 1.23+): **use for lazy evaluation** — avoid loading everything into memory
15. **Stream large transfers** — loading millions of rows causes OOM; stream keeps memory constant
16. `//go:embed` for **static assets** — `frontend/dist` is embedded once and shared by Wails and `httpapi` (PLAN W01)
17. **Use `crypto/rand`** for keys/tokens — `math/rand` is predictable → see `golang-security` skill
18. Regexp MUST be **compiled once at package level** — compilation is O(n) and allocates
19. Compile-time interface checks: **`var _ Interface = (*Type)(nil)`**
20. **A little recode > a big dependency** — each dep adds attack surface, license review and maintenance burden
21. **Design for testability** — consumer-side ports at real boundaries, injected through constructors
22. **Capability limits are explicit states** (`available`, `dependency_missing`, `permission_required`, `unsupported`, `failed`) — never fake a success
23. **Never concatenate shell commands** — executables take an argument array built from validated domain values

## Constructor Patterns: Functional Options vs Builder

### Functional Options (when there are many optional settings)

```go
type Server struct {
    addr         string
    readTimeout  time.Duration
    writeTimeout time.Duration
    maxConns     int
}

type Option func(*Server)

func WithReadTimeout(d time.Duration) Option {
    return func(s *Server) { s.readTimeout = d }
}

func WithWriteTimeout(d time.Duration) Option {
    return func(s *Server) { s.writeTimeout = d }
}

func WithMaxConns(n int) Option {
    return func(s *Server) { s.maxConns = n }
}

func NewServer(addr string, opts ...Option) *Server {
    // Default options
    s := &Server{
        addr:         addr,
        readTimeout:  5 * time.Second,
        writeTimeout: 10 * time.Second,
        maxConns:     100,
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
srv := NewServer(":8080",
    WithReadTimeout(30*time.Second),
    WithMaxConns(500),
)
```

Most kali-tools constructors take a few explicit values and need nothing more. Reach for functional options only when optional settings accumulate; avoid builders.

## Constructors & Initialization

### Avoid `init()` and Mutable Globals

`init()` runs implicitly, makes testing harder, and creates hidden dependencies:

- Multiple `init()` functions run in declaration order, across files in **filename alphabetical order** — fragile
- Cannot return errors — failures must panic or `log.Fatal`
- Runs before `main()` and tests — side effects make tests unpredictable

```go
// Bad — hidden global state
var db *sql.DB

func init() {
    var err error
    db, err = sql.Open("sqlite", os.Getenv("DB_PATH"))
    if err != nil {
        log.Fatal(err)
    }
}

// Good — explicit initialization; the path comes from main
func Open(path string) (*Store, error) {
    // ...
}
```

### Enums: Start at 1

Zero values should represent invalid/unset state:

```go
type Status int

const (
    StatusUnknown Status = iota // 0 = invalid/unset
    StatusActive                // 1
    StatusInactive              // 2
    StatusSuspended             // 3
)
```

States that cross the frontend contract or the database (job states `queued`, `running`, `completed`, `failed`, `cancelled`; capability states) are stable strings. Validate them on input and never invent new values without a PLAN task.

### Compile Regexp Once

```go
// Good — compiled once at package level
var hostLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

func validLabel(s string) bool {
    return hostLabel.MatchString(s)
}

// Prefer typed parsers when they exist: netip.ParseAddr, netip.ParsePrefix, url.Parse.
```

### Use `//go:embed` for Static Assets

```go
import "embed"

//go:embed all:frontend/dist
var assets embed.FS
```

### Compile-Time Interface Checks

→ See `golang-structs-interfaces` skill for the `var _ Interface = (*Type)(nil)` pattern.

## Error Flow Patterns

Error cases MUST be handled first with early return — keep the happy path at minimal indentation. → See `golang-code-style` skill for the full pattern and examples.

### When to Panic vs Return Error

- **Return error**: network failures, file not found, invalid input — anything a caller can handle
- **Panic**: violated invariant that indicates a bug, or `Must*` constructors on constant input at package init. Never panic on network, parser, subprocess or user input: a malformed packet or tool output is an error, not a crash
- **`.Close()` / `Flush()` errors**: read-only cleanup can often use `defer f.Close()`, but write/flush resources must report close or flush errors when durability matters

## Data Handling

### string vs []byte vs []rune

| Type     | Default for | Use when                                            |
| -------- | ----------- | --------------------------------------------------- |
| `string` | Everything  | Immutable, safe, UTF-8                              |
| `[]byte` | I/O         | Writing to `io.Writer`, building strings, mutations |
| `[]rune` | Unicode ops | `len()` must mean characters, not bytes             |

Avoid repeated conversions — each one allocates. Stay in one type until you need the other.

### Iterators & Streaming for Large Data

Use iterators (Go 1.23+) and streaming decoders to process PCAP files, Nmap XML and tool output without loading everything into memory, and stop at the configured limit.

For code examples, see [Data Handling Patterns](references/data-handling.md).

## Resource Management

`defer Close()` immediately after opening — don't wait, don't forget:

```go
f, err := os.Open(path)
if err != nil {
    return err
}
defer f.Close() // right here, not 50 lines later

rows, err := db.QueryContext(ctx, query)
if err != nil {
    return err
}
defer rows.Close()
```

For graceful shutdown, resource pools, and `runtime.AddCleanup`, see [Resource Management](references/resource-management.md).

## Resilience & Limits

### Timeout Every External Call

```go
ctx, cancel := context.WithTimeout(ctx, req.Timeout) // validated, bounded by the domain
defer cancel()

cmd := exec.CommandContext(ctx, toolPath, args...) // never a shell; output capped by the runner
```

### Retry & Context Checks

Diagnostics do not retry silently: a failure is reported as a result. Where a retry is part of the operation (for example traceroute probes), it is bounded, checks `ctx.Err()` between attempts and waits via `select` on `ctx.Done()`. Long loops MUST check `ctx.Err()` periodically. → See `golang-context` skill.

## Database Patterns

→ See `golang-database` skill for modernc SQLite, transactions, nullable columns, the single writer, migrations and contract tests.

## Architecture

The architecture is decided and binding: hexagonal (ports and adapters) as described in `docs/architecture.md`, with the reasoning and anti-patterns in `docs/architecture-research.md`. Do not propose clean-architecture layers, DDD aggregates or CQRS on top of it.

Core principles that follow from it:

- **Dependencies point inward** — `domain` imports only stdlib; capability packages never import `adapter/*`, `app`, Wails or `database/sql`.
- **Fail fast** — validate at boundaries with domain constructors, trust internal code.
- **Make illegal states unrepresentable** — use types to enforce invariants and to keep secrets and payloads out of history.
- **Separate structs per layer** — domain, storage row and transport DTO; DRY applies to behavior, not data.
- **Driving adapters translate only** — decode, call `app`, encode or map the error.
- **No premature patterns** — unit of work, CQRS or generic `UseCase` interfaces only when their trigger appears.

## Code Philosophy

- **Avoid repetitive code** — but don't abstract prematurely
- **Minimize dependencies** — a little recode > a big dependency
- **Design for testability** — accept interfaces, inject dependencies, keep functions pure

## Cross-References

- → See `golang-error-handling` skill for error wrapping, sentinel errors, and the single handling rule
- → See `golang-structs-interfaces` skill for interface design and composition
- → See `golang-concurrency` skill for goroutine lifecycle and graceful shutdown
- → See `golang-context` skill for timeout and cancellation patterns
- → See `golang-project-layout` skill for architecture and directory structure
- → See `golang-dependency-injection` skill for consumer-side ports and manual wiring
