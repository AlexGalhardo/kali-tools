# Good and Bad Tests

## Good tests

**Behavior through the public API.** Test what callers observe.

```go
// GOOD: observable behavior of the use case
func TestStartCancelledJobEndsCancelled(t *testing.T) {
    svc := newTestService(t) // real app.Service with fakes for its ports
    id, err := svc.Start(t.Context(), tcpRequest("127.0.0.1", 9))
    if err != nil {
        t.Fatal(err)
    }
    if err := svc.Cancel(id); err != nil {
        t.Fatal(err)
    }
    if got := waitTerminal(t, svc, id).State; got != "cancelled" {
        t.Fatalf("state = %q, want %q", got, "cancelled")
    }
}
```

Helper and method names are illustrative; use the real API of the package under test.

Characteristics:

- Tests behavior callers care about
- Uses the package's exported API (black-box `_test` package where possible)
- Survives internal refactors
- Describes what, not how
- One logical assertion per test or subtest

## Bad tests

**Implementation-detail tests:** coupled to internal structure.

```go
// BAD: asserts on how the use case talks to its collaborator
if store.putCalls != 2 {
    t.Fatalf("PutJob called %d times", store.putCalls)
}
```

Red flags:

- Faking collaborators inside the same layer
- Testing unexported helpers of another package
- Asserting on call counts or order
- Breaks on refactor without behavior change
- Test name describes how, not what
- Verifying through a side channel instead of the interface

```go
// BAD: bypasses the store to verify
row := db.QueryRowContext(ctx, "SELECT state FROM jobs WHERE id = ?", id)

// GOOD: verifies through the interface
hist, err := store.History(ctx)
```

**Tautological tests:** the expected value restates the implementation.

```go
// BAD: expected value recomputed the way the code does it
want := prev.Bytes - cur.Bytes
if got := rate(prev, cur); got != want { ... }

// GOOD: independent, known values, including the counter-reset case
tests := []struct {
    name      string
    prev, cur uint64
    want      uint64
}{
    {name: "normal delta", prev: 1000, cur: 1500, want: 500},
    {name: "counter reset never goes negative", prev: 1500, cur: 200, want: 0},
}
```

The rate example is illustrative; the architecture requires that a counter reset never produces a negative spike.

## kali-tools specifics worth a test

- Every limit: port counts, durations, packet counts, retained bytes, output caps, subscriber caps.
- Every capability state: `unsupported`, `permission_required`, `dependency_missing`, `failed`.
- Secrets and payloads: a path that touches a Wi-Fi key or packet bytes asserts they are absent from history, logs, events and DTOs.
- Stable contract: JSON envelope with `schema_version`, error codes and job states.
