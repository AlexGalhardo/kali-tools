# pprof Reference

kali-tools never exposes `net/http/pprof`: the desktop build must not open a TCP listener, and `serve` only exposes its documented, token-protected API. Profile locally instead.

## Table of Contents

- [Profiling from tests](#profiling-from-tests)
- [Profiling a debug build](#profiling-a-debug-build)
- [Profile Types](#profile-types)
- [Analysis](#analysis)

## Profiling from tests

```bash
go test -run TestDiagnose -cpuprofile cpu.prof -memprofile mem.prof ./internal/network
go test -run TestManager -blockprofile block.prof -mutexprofile mutex.prof ./internal/app
go tool pprof cpu.prof
```

Use `t.ArtifactDir()` (Go 1.26+) when a test should keep profile files for inspection. Do not commit profiles.

## Profiling a debug build

For a reproduction that needs the full application, write profiles to files from a local, uncommitted debug change:

```go
f, err := os.Create(filepath.Join(os.TempDir(), "kali-cpu.prof"))
if err != nil {
    return err
}
defer f.Close()
if err := pprof.StartCPUProfile(f); err != nil {
    return err
}
defer pprof.StopCPUProfile()

// later, for goroutines and leaks
_ = pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
_ = pprof.Lookup("goroutineleak").WriteTo(os.Stderr, 2) // Go 1.27
```

Profiles can contain argument values and strings from memory. Do not share profiles captured while a Wi-Fi key was revealed or while packets were captured.

## Profile Types

| Profile | Source | What it shows |
| --- | --- | --- |
| CPU | `-cpuprofile`, `pprof.StartCPUProfile` | Where CPU time is spent |
| Heap | `-memprofile`, `pprof.Lookup("heap")` | Live objects and allocations |
| Goroutine | `pprof.Lookup("goroutine")` | Stacks of all goroutines |
| Goroutine leak | `pprof.Lookup("goroutineleak")` | Stacks of leaked goroutines (Go 1.27) |
| Mutex | `-mutexprofile` | Lock contention |
| Block | `-blockprofile` | Blocking on channels and sync |
| Trace | `-trace trace.out`, `go tool trace` | Scheduling and latency over time |

`go tool trace -http` binds to localhost by default; keep it that way.

## Analysis

```bash
go tool pprof -top cpu.prof
go tool pprof -list 'network.Diagnose' cpu.prof
go tool pprof -http=127.0.0.1:0 cpu.prof   # local web UI
```

- `flat` is time in the function itself; `cum` includes callees.
- High GC time in a CPU profile points at allocation churn; confirm with the heap profile.
- Growing goroutine counts usually mean a missing `ctx.Done()` case, an unclosed channel or a subprocess reader that never hits its cap.
