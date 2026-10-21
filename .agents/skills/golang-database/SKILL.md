---
name: golang-database
description: "SQLite access in kali-tools with database/sql and modernc.org/sqlite — parameterized queries, scanning and NULLable columns, transactions, the single coordinated writer, explicit migrations, retention, and keeping secrets and packet payloads out of the database. Use when writing or reviewing code in internal/adapter/storage, adding a table or migration, or testing a store or its port."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.3.0"
---

# SQLite in kali-tools

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT; rewritten from a PostgreSQL/sqlx/pgx focus to the project's embedded SQLite). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md` §6.4 and §9.3; they win over anything below.

**Persona:** you write explicit SQL inside one adapter. Nothing above `internal/adapter/storage` sees SQL, rows or `database/sql`.

## Modes

- **Write:** adding a query, table or migration. Read `internal/adapter/storage/storage.go` first and follow its patterns.
- **Review:** check placeholders, `*Context` calls, `rows.Close`/`rows.Err`, the writer lock, migration versioning and that no secret or payload is persisted.

## Stack and boundaries

| Concern | Choice |
| --- | --- |
| Driver | `modernc.org/sqlite` (pure Go, no cgo), registered as `"sqlite"` |
| API | stdlib `database/sql`; no ORM, no sqlx/pgx/sqlc |
| Location | `internal/adapter/storage` only (`database/sql` is forbidden in `domain`, capabilities and `app`) |
| Port | `JobStore`/`SettingsStore` declared in `internal/app` (PLAN H03); the store returns concrete types |
| Contract test | `internal/porttest` suite run against the real store and an in-memory fake (PLAN H07) |

Row structs stay in the adapter and are mapped to application types at the boundary. Storage types must not appear in Wails, HTTP or CLI signatures (deviation D4, PLAN H06).

## Rules

1. Queries MUST use `?` placeholders. NEVER concatenate input into SQL. Column or ordering choices come from a fixed allowlist.
2. Every call uses a `*Context` variant: `ExecContext`, `QueryContext`, `QueryRowContext`, `BeginTx(ctx, ...)`.
3. Handle `sql.ErrNoRows` with `errors.Is` and translate it to a domain error or an `ok bool`.
4. `defer rows.Close()` right after `QueryContext`, and check `rows.Err()` after the loop.
5. Use `Exec` for statements that return no rows.
6. Bound every read: list queries carry a `LIMIT` (history uses `LIMIT 200`).
7. One coordinated writer: the store serializes writes (`SetMaxOpenConns(1)` plus a write mutex today). Do not open a second `*sql.DB` on the same file.
8. Pragmas are set once at open: `busy_timeout`, `foreign_keys=ON`, `journal_mode=WAL`.
9. Multi-statement writes run in one transaction with `defer tx.Rollback()`.
10. Times are stored as UTC RFC 3339 text and parsed back explicitly.
11. Wi-Fi secrets, the web-server token and packet payloads never reach the store. Type separation enforces this: the persisted struct has no field that could hold them (`docs/architecture-research.md` §6.6).
12. Never return raw SQL errors to a client. Wrap with `%w` inside the adapter; driving adapters map them once.

## Queries

```go
// ✗ SQL injection
q := fmt.Sprintf("SELECT summary FROM jobs WHERE id = '%s'", id)

// ✓ Parameterized
var summary string
err := s.db.QueryRowContext(ctx, "SELECT summary FROM jobs WHERE id = ?", id).Scan(&summary)
if errors.Is(err, sql.ErrNoRows) {
    return "", domain.Invalid("unknown job")
}
if err != nil {
    return "", fmt.Errorf("reading job %q: %w", id, err)
}
```

Dynamic `IN` lists are built with one `?` per value and a hard cap on the number of values:

```go
if len(ids) == 0 || len(ids) > 100 {
    return nil, domain.Invalid("1..100 ids required")
}
q := "SELECT id, state FROM jobs WHERE id IN (" + strings.Repeat("?,", len(ids)-1) + "?)"
args := make([]any, len(ids))
for i, id := range ids {
    args[i] = id
}
rows, err := s.db.QueryContext(ctx, q, args...)
```

## Iterating rows

```go
rows, err := s.db.QueryContext(ctx, "SELECT id, kind, state FROM jobs ORDER BY created DESC LIMIT 200")
if err != nil {
    return nil, fmt.Errorf("listing jobs: %w", err)
}
defer func() { _ = rows.Close() }()

jobs := []Job{} // never nil: serializes as [] for the UI
for rows.Next() {
    var j Job
    if err := rows.Scan(&j.ID, &j.Kind, &j.State); err != nil {
        return nil, fmt.Errorf("scanning job: %w", err)
    }
    jobs = append(jobs, j)
}
if err := rows.Err(); err != nil {
    return nil, fmt.Errorf("iterating jobs: %w", err)
}
return jobs, nil
```

## Migrations

kali-tools uses explicit, in-code migrations in `storage.migrate`, versioned in `schema_migrations` and applied in one transaction at open. There is no external migration tool.

- Add a new numbered step; never edit a released step.
- An unknown (newer) schema version fails open with a clear error instead of guessing.
- Schema changes are reviewed by a human. Do not invent tables or columns beyond what a PLAN task requires.
- Recovery rules (for example, marking interrupted `queued`/`running` jobs as `cancelled`) live in the migration/open path and are tested.

## Transactions

See [Transactions](./references/transactions.md). Start with the update-function pattern for single-aggregate writes; add a unit-of-work port only on the first multi-repository write (`docs/architecture-research.md` §6.4). Never put a transaction in a context.

## Errors

| Error | Detect | Action |
| --- | --- | --- |
| Not found | `errors.Is(err, sql.ErrNoRows)` | Domain error or `ok=false` |
| Busy or locked | SQLite busy error after `busy_timeout` | Return a wrapped error; do not spin |
| Cancelled | `errors.Is(err, context.Canceled)` | Stop and propagate |
| Unknown schema | Migration version check | Fail open with a clear message |

## Deep dives

- [Transactions](./references/transactions.md): `BeginTx`, rollback discipline, SQLite locking.
- [Scanning](./references/scanning.md): NULLable columns, time columns, row structs versus DTOs.
- [Testing](./references/testing.md): temp-file stores, `:memory:`, fakes and contract suites.

## Cross-references

- `golang-security` for injection and secret handling.
- `golang-context` for deadlines on database calls.
- `golang-error-handling` for wrapping and mapping.
- `golang-testing` for table tests and contract suites.

## References

- [database/sql tutorial](https://go.dev/doc/database/)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
- [SQLite WAL](https://www.sqlite.org/wal.html)
