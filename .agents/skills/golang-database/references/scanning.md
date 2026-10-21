# Scanning and NULLable Columns

## Row structs versus DTOs

Row structs and SQL live in `internal/adapter/storage`. Map them to application types at the boundary. A separate struct per layer (domain, storage row, transport DTO) is deliberate: sharing one struct is how a Wi-Fi key or packet bytes could reach history or a client (`docs/architecture-research.md` §6.6).

## Manual scanning

kali-tools uses `database/sql` without a scanning library. Scan columns in the order the query lists them, and never use `SELECT *`.

```go
var j Job
var created string
var finished sql.NullString
err := row.Scan(&j.ID, &j.Kind, &j.State, &created, &finished, &j.Summary)
```

## NULLable columns

Two approaches, in order of preference.

First, `sql.NullString`, `sql.NullInt64` or the generic `sql.Null[T]` in the adapter, converted to a pointer or zero value when mapping out:

```go
if finished.Valid {
    t, err := time.Parse(time.RFC3339Nano, finished.String)
    if err != nil {
        return Job{}, fmt.Errorf("parsing finished time: %w", err)
    }
    j.Finished = &t
}
```

Second, `COALESCE` in SQL when NULL and the zero value mean the same thing:

```sql
SELECT id, COALESCE(summary, '') FROM jobs WHERE id = ?
```

## Time columns

Store times as UTC text with `time.RFC3339Nano` and parse them explicitly on read. A parse failure is a wrapped error, not a silent zero time.

## JSON columns

When a result is stored as JSON text (for example `diagnostic_results.result`), marshal from a type that has no secret or payload fields, and bound the size you read back before `json.Unmarshal`.
