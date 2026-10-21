# Concurrency Debugging

## Table of Contents

- [Goroutine Leaks](#goroutine-leaks)
- [Race Conditions](#race-conditions)
- [Deadlocks](#deadlocks)

## Goroutine Leaks

**Symptoms:** Memory slowly increasing, goroutine count growing, no obvious CPU spike.

### Diagnosis

Use pprof goroutine profile (see [pprof.md](./pprof.md)) with `?debug=2` for human-readable output, then look for goroutines stuck in `chan receive`.

The goroutine leak profile is available in `runtime/pprof` in Go 1.27 (`goroutineleak`). kali-tools has no pprof HTTP endpoint; write it from a test or a debug build:

```go
_ = pprof.Lookup("goroutineleak").WriteTo(os.Stderr, 2)
```

Other tools: `testing/synctest` in tests, `runtime.NumGoroutine()` before and after shutdown, `pprof.Lookup("goroutine").WriteTo(w, 2)` for stack dumps, and `go test -race ./...` for race checks.

```go
// Programmatic monitoring — log goroutine count to detect leaks
go func() {
    for {
        log.Printf("goroutines: %d", runtime.NumGoroutine())
        time.Sleep(3 * time.Second)
    }
}()

// In tests, wrap the code in synctest.Test: it fails if goroutines
// started in the bubble are still blocked when the function returns.
```

### Common causes

```go
// 1. Unclosed channel — goroutine blocks forever
// BAD
for {
    job := <-jobs
    process(job)
}
// GOOD
for {
    select {
    case job, ok := <-jobs:
        if !ok { return }
        process(job)
    case <-ctx.Done():
        return
    }
}

// 2. Forgotten response body close — leaks HTTP connection
// Always defer resp.Body.Close() after HTTP calls.
// bodyclose is enabled in .golangci.yml and catches most of these.

// 3. time.After in hot loop — allocates a new timer each iteration
// BAD
for {
    select {
    case <-time.After(time.Second):
        do()
    }
}
// GOOD — reuse a ticker for repeated intervals
ticker := time.NewTicker(time.Second)
defer ticker.Stop()
for {
    select {
    case <-ticker.C:
        do()
    case <-ctx.Done():
        return
    }
}
```

## Race Conditions

**Symptoms:** Intermittent failures, "sometimes works sometimes doesn't", different results on different machines.

**Diagnosis:** Race conditions MUST be tested with the `-race` flag:

```bash
go test -race ./...
go run -race main.go
# Race detector slows code ~10x but finds data races reliably
```

### Common patterns

- Shared map without mutex
- Shared variable without atomic
- Publishing reference before initialization
- go func() accessing outer variables without synchronization

## Deadlocks

**Symptoms:** Program hangs, goroutines stuck in "chan receive" or "mutex lock".

### Diagnosis

```bash
# A test that hangs past -timeout prints every goroutine stack
go test -timeout 30s -run TestHang ./internal/app

# Or programmatic, in a debug build
runtime.Stack(buf, true)
```

### Common patterns

1. **Circular wait** — A waits for B, B waits for A
2. **Forgotten channel send** — sender goroutine exited
3. **Wrong lock order** — always acquire locks in the same order
