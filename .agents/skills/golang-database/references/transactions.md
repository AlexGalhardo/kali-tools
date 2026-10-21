# Transactions in SQLite

## Basic transaction pattern

```go
tx, err := s.db.BeginTx(ctx, nil)
if err != nil {
    return fmt.Errorf("beginning transaction: %w", err)
}
defer func() { _ = tx.Rollback() }() // no-op after Commit

// ... tx.ExecContext / tx.QueryRowContext ...

if err := tx.Commit(); err != nil {
    return fmt.Errorf("committing transaction: %w", err)
}
```

## Locking model

SQLite has database-level write locking, not row locks. There is no `SELECT ... FOR UPDATE`, and isolation levels other than the default serializable behavior do not apply.

- Writes are serialized by the store (one connection, one write mutex). Read-modify-write sequences run inside one transaction under that writer.
- `busy_timeout` makes a blocked writer wait briefly instead of failing at once. Do not add retry loops on top.
- Keep transactions short. Never hold one across a network diagnostic, a subprocess or a user prompt.

## Update-function pattern

For a single aggregate, keep the transaction in the adapter and the rule in a closure (`docs/architecture-research.md` §6.4):

```go
func (s *Store) UpdateJob(ctx context.Context, id string, fn func(*Job) error) error {
    s.write.Lock()
    defer s.write.Unlock()
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("beginning transaction: %w", err)
    }
    defer func() { _ = tx.Rollback() }()
    j, err := loadJob(ctx, tx, id)
    if err != nil {
        return err
    }
    if err := fn(&j); err != nil {
        return err
    }
    if err := saveJob(ctx, tx, j); err != nil {
        return err
    }
    return tx.Commit()
}
```

Add a unit-of-work port (`RunInTx(ctx, func(Stores) error) error` with a small `dbtx` interface) only when the first write across several stores appears.
