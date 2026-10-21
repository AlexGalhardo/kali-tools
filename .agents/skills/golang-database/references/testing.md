# Testing Database Code

## Real SQLite, no mocks of database/sql

modernc SQLite is pure Go and fast, so store tests use a real database. Do not use `sqlmock` or mocks of `database/sql`.

```go
func newTempStore(t *testing.T) *Store {
    t.Helper()
    s, err := Open(filepath.Join(t.TempDir(), "history.db"))
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { _ = s.Close() })
    return s
}
```

Use a temp file when the test covers reopen, recovery or retention; `:memory:` is fine for a single-connection unit test.

## What to test in the store

- Migrations apply on an empty file and are idempotent on reopen.
- An unknown schema version fails open.
- Interrupted `queued`/`running` jobs become `cancelled` after reopen.
- Retention deletes only what is older than the cutoff.
- Settings validation rejects out-of-range values.
- Concurrent writers do not corrupt or lose rows (run with `-race` where available).
- Nothing that looks like a secret or payload is persisted: write a job whose input contained one and assert the stored row does not.

## Use-case tests: hand-written fakes

`internal/app` tests use a hand-written in-memory fake of the port declared in `app` (`JobStore`, `SettingsStore`). No mocking framework is used.

```go
type fakeJobStore struct {
    mu   sync.Mutex
    jobs map[string]app.Job
}

func (f *fakeJobStore) PutJob(_ context.Context, j app.Job) error {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.jobs[j.ID] = j
    return nil
}
```

## Contract suites

The fake cannot drift from the adapter because both run the same suite from `internal/porttest` (PLAN H07):

```go
// internal/porttest/jobstore.go
func JobStoreContract(t *testing.T, newStore func(t *testing.T) app.JobStore) {
    t.Run("put then list", func(t *testing.T) { /* ... */ })
    t.Run("never stores secrets or payloads", func(t *testing.T) { /* ... */ })
}

// internal/adapter/storage/storage_test.go
func TestStoreContract(t *testing.T) {
    porttest.JobStoreContract(t, func(t *testing.T) app.JobStore { return newTempStore(t) })
}
```
