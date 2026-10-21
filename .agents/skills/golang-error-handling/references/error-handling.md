# Error Handling Patterns and Logging

## Table of Contents

- [The Single Handling Rule](#the-single-handling-rule)
- [Panic and Recover](#panic-and-recover)
  - [When to panic](#when-to-panic)
  - [Recovering from panics](#recovering-from-panics)
- [Mapping errors in driving adapters](#mapping-errors-in-driving-adapters)
- [Logging Errors with `slog`](#logging-errors-with-slog)

## The Single Handling Rule

An error MUST be handled exactly once: either log it or return it, never both. Doing both causes duplicate log entries and makes debugging harder.

```go
// ✗ Bad — logs AND returns (duplicate noise)
func processOrder(id string) error {
    err := chargeCard(id)
    if err != nil {
        log.Printf("failed to charge card: %v", err)
        return fmt.Errorf("charging card: %w", err)
    }
    return nil
}

// ✓ Good — return with context, let the caller decide
func processOrder(id string) error {
    err := chargeCard(id)
    if err != nil {
        return fmt.Errorf("charging card for order %q: %w", id, err)
    }
    return nil
}

// ✓ Good — handle at the top level (HTTP handler, main, etc.)
func handleOrder(w http.ResponseWriter, r *http.Request) {
    err := processOrder(r.FormValue("id"))
    if err != nil {
        h.log.Error("order failed", "error", err)
        writeError(w, err) // single mapping function; never echoes raw causes
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

## Panic and Recover

### When to panic

Panic MUST only be used for truly unrecoverable states — programmer errors, impossible conditions, or corrupt invariants. NEVER use panic for expected failures like network timeouts or missing files.

```go
// ✓ Acceptable — programmer error in initialization
func MustCompileRegex(pattern string) *regexp.Regexp {
    re, err := regexp.Compile(pattern)
    if err != nil {
        panic(fmt.Sprintf("invalid regex %q: %v", pattern, err))
    }
    return re
}

// ✗ Bad — panic for a normal failure
func GetUser(id string) *User {
    user, err := db.Find(id)
    if err != nil {
        panic(err) // callers cannot recover gracefully
    }
    return user
}
```

### Recovering from panics

Use `recover` in deferred functions at goroutine boundaries (HTTP handlers, job goroutines, packet decoders fed untrusted input) to prevent one panic from crashing the entire process. A recovered job panic marks the job `failed`; it never leaves it `running`. The stack goes to the local log only, never to a client.

```go
func safeHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if r := recover(); r != nil {
                logger.Error("panic recovered",
                    "panic", r,
                    "stack", string(debug.Stack()),
                )
                http.Error(w, "internal error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

## Mapping errors in driving adapters

Each driving adapter has exactly one function that turns an error into its transport form (`docs/architecture-research.md` §6.5):

| Adapter | Maps an error to |
| --- | --- |
| HTTP (`internal/adapter/httpapi`) | Status code plus `{code, message}` |
| Wails | `{code, message}` |
| CLI | Exit code plus the JSON envelope |

`invalid_input` maps to a client error (HTTP 400, CLI usage or failure code); unknown errors map to a generic message. Raw subprocess stderr, SQL errors and secrets never reach a client.

## Logging Errors with `slog`

- Inject the `*slog.Logger`; inner packages never use a global logger.
- Log once, at the boundary that handles the error.
- Bound and redact: cap attribute sizes, drop secrets and payload bytes, and never log the web-server token or a revealed Wi-Fi key.
- Diagnostic history is not a log: it stores timestamps, parameters, tool version and a summary, never secrets or payloads.
