# Fakes, Contract Suites and Time

kali-tools uses hand-written fakes, not a mocking framework. A fake is a small in-memory implementation of a port declared by the consumer.

## Fake a consumer-side port

```go
// internal/app declares the port.
type JobStore interface {
    PutJob(ctx context.Context, j Job) error
    History(ctx context.Context) ([]Job, error)
}

// In the test package (or internal/porttest when shared):
type fakeJobStore struct {
    mu   sync.Mutex
    jobs []app.Job
    err  error // inject failures
}

func (f *fakeJobStore) PutJob(_ context.Context, j app.Job) error {
    f.mu.Lock()
    defer f.mu.Unlock()
    if f.err != nil {
        return f.err
    }
    f.jobs = append(f.jobs, j)
    return nil
}

func (f *fakeJobStore) History(context.Context) ([]app.Job, error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    return slices.Clone(f.jobs), f.err
}
```

Assert on observable state (what the fake holds after the call), not on call sequences.

## Keep the fake honest: contract suites

```go
// internal/porttest/jobstore.go
func JobStoreContract(t *testing.T, newStore func(t *testing.T) app.JobStore) {
    t.Run("put then list", func(t *testing.T) { /* ... */ })
    t.Run("history is bounded", func(t *testing.T) { /* ... */ })
    t.Run("never stores secrets or payloads", func(t *testing.T) { /* ... */ })
}
```

Run the suite against the fake and against the real adapter. If they disagree, the fake is wrong.

## Fakes for OS and process boundaries

- **Tool runner:** a fake that returns canned stdout from `testdata/` (synthetic Nmap XML), respects the context and enforces the same output cap.
- **Platform/probe:** a fake host network or prober returning fixed routes, counters or echo results; include `unsupported` and `permission_required` cases.
- **Wails events:** the binding's emitter is a `func(ctx context.Context, name string, data ...any)` field; record calls in a slice.

## Time

Inject the `Clock` port (`app.SystemClock` in production) and use a fixed clock in tests. For timers, tickers and deadlines inside goroutines, use `testing/synctest` so time advances deterministically:

```go
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }
```

## Fixtures

- Store synthetic fixtures under the package `testdata/` or `tests/fixtures`.
- Never commit real packet captures, Wi-Fi keys or tokens.
- Golden files for CLI output include `schema_version`; update them only for a documented contract change.
