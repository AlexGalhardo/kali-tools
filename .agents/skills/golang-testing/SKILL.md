---
name: golang-testing
description: "Go tests for kali-tools with the standard library — table-driven tests, hand-written fakes, internal/porttest contract suites, acceptance drivers per interface, httptest and SSE tests for the web server's security rules, synctest, fuzzing of parsers, the integration-tagged Docker lab, and native evidence rules. Use when writing or reviewing Go tests or debugging flaky ones. No testify, gomock or goleak."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.4.0"
---

# Go Testing Best Practices

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md`; they win over anything below.

**Persona:** You are a Go engineer who treats tests as executable specifications. You write tests to constrain behavior, not to hit coverage targets.

## Modes

- **Write mode** — generating new tests for existing or new code. Work sequentially through the code under test: table-driven cases first, then edge cases, error paths, limits and cancellation.
- **Review mode** — reviewing a PR's test changes. Focus on the diff: check coverage of new behaviour, assertion quality, table-driven structure, and absence of flakiness patterns. Sequential.
- **Audit mode** — auditing an existing test suite for gaps, flakiness, or bad patterns (order-dependent tests, missing `t.Parallel()`, implementation-detail coupling). Concerns: (1) unit test quality and coverage gaps, (2) integration test isolation and build tags, (3) goroutine leaks and race conditions.
- **Debug mode** — a test is failing or flaky. Work sequentially: reproduce reliably, isolate the failing assertion, trace the root cause in production code or test setup.

## Project test levels

kali-tools follows the strategy in `docs/architecture-research.md` §10. Use the standard library `testing` package only: testify, gomock, mockery, goleak and gotests are not dependencies.

| Level | Target | Technique |
| --- | --- | --- |
| Domain unit | `internal/network`, `capture`, `wifi`, `domain` | Table-driven, black-box `_test` packages, no fakes |
| Use case | `internal/app`, `internal/job` | Hand-written fakes of driven ports; lifecycle, cancellation and bounded-drop tests |
| Contract | Each driven port | One suite in `internal/porttest`, run against the real adapter **and** the fake |
| Acceptance | Each driving adapter | One spec in `tests/acceptance`, drivers for direct `Application`, CLI `run` with buffers, `httptest` + SSE client |
| Process | Composition roots | `run()` in a goroutine with a cancellable context |
| Integration | Docker lab | `//go:build integration` in `tests/integration`, private Compose network |
| Native | Real hardware | Evidence recorded in `docs/verification/`; never inferred from cross-compilation |
| Browser | React | Playwright with contract-faithful fakes; does not prove Wails IPC |

After relevant changes run `go test ./...`, `go vet ./...` and the frontend check/build (AGENTS.md). Mark a PLAN task complete only with the actual output as evidence.

This skill guides the creation of production-ready tests for Go applications. Follow these principles to write maintainable, fast, and reliable tests.

## Best Practices Summary

1. Table-driven tests MUST use named subtests -- every test case needs a `name` field passed to `t.Run`
2. Integration tests MUST use build tags (`//go:build integration`) to separate from unit tests
3. Tests MUST NOT depend on execution order -- each test MUST be independently runnable
4. Independent tests SHOULD use `t.Parallel()` when possible
5. Tests MUST assert observable behavior and public API contracts, not implementation details -- a test coupled to internals turns every refactor into a test rewrite while proving nothing about the contract
6. Packages with goroutines SHOULD prove clean shutdown (`testing/synctest`, `runtime.NumGoroutine`, `-race`)
7. Use the standard library: `t.Errorf`/`t.Fatalf` with `got`/`want` messages; no assertion library
8. Fake the consumer-side port, not concrete types, and run the fake through the same contract suite as the adapter
9. Keep unit tests fast (< 1ms), use build tags for integration tests
10. Run tests with `-race` where the toolchain supports it (`node scripts/task.mjs race`)
11. Include examples as executable documentation
12. Test files MUST be named after the source file under test, not after the function or method being tested
13. Test functions SHOULD appear in the same order as the functions/methods they test in the source file
14. Test every bound: limits, timeouts, output caps and the behavior when they are hit
15. Tests MUST prove secrets and payloads stay out of history, logs, events and DTOs wherever a code path touches them
16. Fixtures are synthetic; never commit real captures, keys or tokens

## Test Structure and Organization

### File Conventions

```go
// package_test.go - tests in same package (white-box, access unexported)
package mypackage

// mypackage_test.go - tests in test package (black-box, public API only)
package mypackage_test
```

Name the test file after the source file it tests, not after the function or method under test. Go's convention is one test file per source file (`foo.go` -> `foo_test.go`), because tools (`go test`, coverage reports, IDE "jump to test" navigation, `gotests`) and reviewers all resolve tests by source file, not by symbol. A source file usually declares several functions/methods; splitting its tests by symbol name scatters them across many files and breaks that file-to-file mapping.

```text
// ✓ Good — one test file per source file
helloworld.go       -> helloworld_test.go   // contains TestHelloWorld, TestAbcd, TestXyz, ...

// ✗ Bad — test file named after the function/method instead of the source file
helloworld.go       -> abcd_test.go         // wrong: should be helloworld_test.go
```

Exception: very large source files MAY be split into multiple `_test.go` files by concern (e.g. `foo_test.go` + `foo_edgecases_test.go`), but each split file's name MUST still be derived from the source file name, never from an individual function name. Prefer keeping a single `_test.go` file per source file even when it grows large — splitting adds navigation overhead and is rarely worth it; reach for the exception only when a single file becomes genuinely unwieldy to browse or review.

Within a test file, order test functions to match the order their tested functions/methods appear in the source file. A reader (human or agent) scrolling `foo.go` alongside `foo_test.go` can then find the matching test by position instead of searching; drift between the two orderings compounds every time either file grows.

### Naming Conventions

```go
func TestAdd(t *testing.T) { ... }               // function test
func TestMyStruct_MyMethod(t *testing.T) { ... } // method test
func BenchmarkAdd(b *testing.B) { ... }          // benchmark
func ExampleAdd() { ... }                        // example
func FuzzAdd(f *testing.F) { ... }               // fuzz test
```

## Table-Driven Tests

Table-driven tests are the idiomatic Go way to test multiple scenarios. Always name each test case.

```go
func TestCalculatePrice(t *testing.T) {
    tests := []struct {
        name     string
        quantity int
        unitPrice float64
        expected  float64
    }{
        {
            name:      "single item",
            quantity:  1,
            unitPrice: 10.0,
            expected:  10.0,
        },
        {
            name:      "bulk discount - 100 items",
            quantity:  100,
            unitPrice: 10.0,
            expected:  900.0, // 10% discount
        },
        {
            name:      "zero quantity",
            quantity:  0,
            unitPrice: 10.0,
            expected:  0.0,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := CalculatePrice(tt.quantity, tt.unitPrice)
            if got != tt.expected {
                t.Errorf("CalculatePrice(%d, %.2f) = %.2f, want %.2f",
                    tt.quantity, tt.unitPrice, got, tt.expected)
            }
        })
    }
}
```

## Unit Tests

Unit tests should be fast (< 1ms), isolated (no external dependencies), and deterministic.

## Testing HTTP Handlers

Use `httptest` for `internal/adapter/httpapi` tests with table-driven patterns, including the mandatory security cases (foreign Host, cross-origin POST, missing token, oversized body). See [HTTP Testing](./references/http-testing.md).

## Goroutine Leak Detection

kali-tools does not depend on goleak. Prove shutdown with the standard library:

```go
func TestManagerCloseStopsWorkers(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        m := newTestManager(t)
        m.Start(validRequest())
        m.Close()
    }) // fails if goroutines started in the bubble are still blocked
}
```

Run concurrency tests with `-race` where cgo is available.

## testing/synctest for Deterministic Goroutine Testing

`testing/synctest` (Go 1.25+) provides deterministic tests for goroutines, timers, deadlines, and context cancellation. Time advances only when all goroutines are blocked, making ordering predictable.

When to use `synctest` instead of real time:

- Testing concurrent code with time-based operations (time.Sleep, time.After, time.Ticker)
- When race conditions need to be reproducible
- When tests are flaky due to timing issues

```go
import (
    "context"
    "testing"
    "testing/synctest"
    "time"
)

func TestContextTimeout(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        const timeout = 5 * time.Second

        ctx, cancel := context.WithTimeout(t.Context(), timeout)
        defer cancel()

        time.Sleep(timeout - time.Nanosecond)
        synctest.Wait()
        if err := ctx.Err(); err != nil {
            t.Fatalf("before timeout: %v", err)
        }

        time.Sleep(time.Nanosecond)
        synctest.Wait()
        if err := ctx.Err(); err != context.DeadlineExceeded {
            t.Fatalf("after timeout: got %v, want DeadlineExceeded", err)
        }
    })
}
```

Use `synctest.Test` in Go 1.25+ and later. Do not use the old Go 1.24 experimental `synctest.Run` API in Go 1.25+ code. If a module explicitly targets Go 1.24 and opts into `GOEXPERIMENT=synctest`, use the old API only as a compatibility fallback.

Key differences in `synctest`:

- `time.Sleep` advances synthetic time instantly when the goroutine blocks
- `time.After` fires when synthetic time reaches the duration
- All goroutines run to blocking points before time advances
- Test execution is deterministic and repeatable
- Go 1.27+ adds `synctest.Sleep(d)` as a direct helper to advance the bubble's fake clock, equivalent to `time.Sleep(d)` followed by `synctest.Wait()` but without needing a real goroutine to block on

Go 1.27+ also adds `httptest.NewTestServer()`, an in-memory fake-network variant of `httptest.NewServer` that composes with `synctest` — no real socket, so server tests can run inside a `synctest.Test` bubble instead of needing `httptest.NewServer` plus real timers.

## Test Timeouts

For tests that may hang, use a timeout helper that panics with caller location. See [Helpers](./references/helpers.md).

## Benchmarks

Write benchmarks as sub-benchmarks (`b.Run` per variant) so each variant gets its own name in the output — that name is what comparison tooling diffs. For Go 1.24+, use `b.Loop()` rather than a `b.N` loop.

→ See [Benchmarks in a Test Suite](./references/benchmarks.md) for the code shape and size-parameterized examples.

Compare runs with `benchstat` at a pinned version when a change claims a performance effect; there is no benchmark gate in kali-tools.

## Go 1.26+: test artifacts

When a test, benchmark, or fuzz target needs to persist files for inspection, use `ArtifactDir()` instead of ad-hoc paths or repo-local output.

```go
func TestRenderGoldenArtifact(t *testing.T) {
    dir := t.ArtifactDir()

    out := filepath.Join(dir, "rendered.json")
    if err := os.WriteFile(out, renderedBytes, 0o644); err != nil {
        t.Fatal(err)
    }

    t.Logf("artifact written: %s", out)
}
```

Available on `*testing.T`, `*testing.B`, and `*testing.F` in Go 1.26+.

### Go 1.27+: `stdversion` runs automatically

`go test` now invokes the `stdversion` vet check by default, flagging any use of an API newer than the module's `go` directive. A CI failure from this check means either the `go` directive needs bumping or the code needs to stop using the newer API — it is not a check to silence.

## Parallel Tests

Use `t.Parallel()` to run tests concurrently:

```go
func TestParallelOperations(t *testing.T) {
    tests := []struct {
        name string
        data []byte
    }{
        {"small data", make([]byte, 1024)},
        {"medium data", make([]byte, 1024*1024)},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            if got := Process(tt.data); got == nil {
                t.Fatal("Process returned nil")
            }
        })
    }
}
```

## Fuzzing

Use fuzzing to find edge cases and bugs:

Every parser of untrusted input (PCAP/PCAPNG, Nmap XML, tool output) has a fuzz target that asserts "no panic, bounded work, error on malformed input":

```go
func FuzzNmapXML(f *testing.F) {
    f.Add([]byte(`<nmaprun></nmaprun>`))
    f.Fuzz(func(t *testing.T, data []byte) {
        _, _ = ParseNmapXML(bytes.NewReader(data)) // must not panic or exceed limits
    })
}
```

`node scripts/task.mjs fuzz` runs the existing `FuzzCapture` and `FuzzNmap` targets for 10 s each. Add crashers as regression cases under `testdata/fuzz/`. The function name above is illustrative.

## Examples as Documentation

`ExampleXxx` functions are executable documentation: `go test` compares their stdout to the `// Output:` comment, so a drifting example fails the build instead of misleading readers.

→ See [Examples as Documentation](./references/examples.md) for naming rules, `Unordered output`, and placement.

## Code Coverage

Generate a profile with `go test -coverprofile=coverage.out ./...`, then read the uncovered lines with `go tool cover -html=coverage.out`. Coverage locates untested paths; it does not measure assertion quality, so treat a percentage as a gap finder rather than a target.

`node scripts/task.mjs check` writes `coverage.out`.

## Integration Tests

Use build tags to separate integration tests from unit tests:

```go
//go:build integration

package integration

func TestPrivateLab(t *testing.T) {
    host := os.Getenv("LAB_FIXTURE")
    if host == "" {
        t.Fatal("LAB_FIXTURE is required; use docker compose lab")
    }
    // Diagnostics against the deterministic fixture on a private network.
}
```

Run the lab through the setup scripts, for example `bash scripts/setup/setup-linux-docker.sh test`. Diagnostics in tests target only the lab fixture or loopback, never public hosts. SQLite store tests do not need the integration tag: modernc SQLite runs in-process. See [Integration Testing](./references/integration-testing.md).

## Fakes and contract suites

Fake the port declared in the consumer with a small hand-written type. Every fake passes the same `internal/porttest` suite as the real adapter, so it cannot drift. Inject time through the `Clock` port.

For fake patterns, contract suites and time control, see [Fakes](./references/mocking.md).

## Enforce with Linters

`thelper` and `paralleltest` exist but are not enabled in `.golangci.yml`; review enforces those practices. See the `golang-lint` skill.

## Cross-References

- → See `golang-database` skill (testing.md) for store tests and contract suites
- → See `golang-concurrency` skill for job-manager and shutdown tests
- → See `golang-security` skill for the web-server security test cases
- → See `golang-cli` skill for CLI golden and exit-code tests
- → See `tdd` skill for the red-green loop

## Quick Reference

```bash
go test ./...                          # all tests
go test -run TestName ./...            # specific test by exact name
go test -run TestName/subtest ./...    # subtests within a test
go test -run 'Test(Add|Sub)' ./...     # multiple tests (regexp OR)
go test -run 'Test[A-Z]' ./...         # tests starting with capital letter
go test -run 'TestUser.*' ./...        # tests matching prefix
go test -run '.*Validation.*' ./...    # tests containing substring
go test -run TestName/. ./...          # all subtests of TestName
go test -run '/(unit|integration)' ./... # filter by subtest name
go test -race ./...                    # race detection
go test -cover ./...                   # coverage summary
go test -bench=. -benchmem ./...       # benchmarks
go test -fuzz=FuzzName ./...           # fuzzing
go test -tags=integration ./tests/integration/...  # Docker lab (needs LAB_FIXTURE)
node scripts/task.mjs bridge           # native Wails bridge smoke (Windows desktop only)
node scripts/task.mjs check            # format, tidy, vet, tests, lint, govulncheck
node scripts/task.mjs race             # race detector
node scripts/task.mjs fuzz             # parser fuzz smoke
```
