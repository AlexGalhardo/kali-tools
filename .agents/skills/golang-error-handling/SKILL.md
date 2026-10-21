---
name: golang-error-handling
description: "Go error handling in kali-tools — wrapping with %w, errors.Is/As/AsType, errors.Join, domain.Error codes, one error-mapping function per driving adapter (Wails, HTTP, CLI), the single handling rule, panic/recover at job and handler boundaries, and redacted slog logging with no secrets, tokens or payloads. Use when creating, wrapping, mapping or logging errors."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.3.1"
---

# Go Error Handling Best Practices

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md`; they win over anything below.

**Persona:** You are a Go reliability engineer. You treat every error as an event that must either be handled or propagated with context — silent failures and duplicate logs are equally unacceptable.

## Modes

- **Coding mode** — writing new error handling code. Follow the best practices sequentially; optionally launch a background sub-agent to grep for violations in adjacent code (swallowed errors, log-and-return pairs) without blocking the main implementation.
- **Review mode** — reviewing a PR's error handling changes. Focus on the diff: check for swallowed errors, missing wrapping context, log-and-return pairs, and panic misuse. Sequential.
- **Audit mode** — auditing existing error handling across a codebase. Use up to 5 parallel sub-agents, each targeting an independent category (creation, wrapping, single-handling rule, panic/recover, structured logging).

This skill guides the creation of robust, idiomatic error handling in Go applications. Follow these principles to write maintainable, debuggable, and production-ready error code.

## Best Practices Summary

1. **Returned errors MUST always be checked** — NEVER discard with `_`
2. **Errors MUST be wrapped with context** using `fmt.Errorf("{context}: %w", err)`
3. **Error strings MUST be lowercase**, without trailing punctuation
4. **Use `%w` internally, `%v` at system boundaries** to control error chain exposure
5. **MUST use `errors.Is` for sentinel matching and `errors.As`/`errors.AsType` for typed chain inspection** instead of direct comparison or bare type assertions. For Go 1.26+, prefer `errors.AsType[T](err)` when `T` implements `error`; use `errors.As(err, &target)` for Go <1.26 or for non-error interface targets.
6. **SHOULD use `errors.Join`** (Go 1.20+) to combine independent errors
7. **Errors MUST be either logged OR returned**, NEVER both (single handling rule)
8. **Use sentinel errors** for expected conditions, custom types for carrying data
9. **NEVER use `panic` for expected error conditions** — reserve for truly unrecoverable states
10. **SHOULD use `log/slog`** for structured logging, with the logger injected as a dependency — not `fmt.Println` or `log.Printf`
11. **Domain and application errors use `domain.Error{Code, Message}`** with stable codes (`invalid_input`, `unsupported`, and the capability states). Create them with the domain constructors (`domain.Invalid`, `domain.Unsupported`); add a new code only with a PLAN task, because codes are part of the frontend, HTTP and CLI contract
12. **Map errors once per driving adapter**: Wails returns `{code, message}`, HTTP returns a status plus `{code, message}`, the CLI returns an exit code plus the JSON envelope (`docs/architecture-research.md` §6.5)
13. **Use log levels** to indicate error severity
14. **Never expose technical errors to clients** — raw subprocess stderr, SQL errors, OS error text with paths, and stack traces stay out of responses. Log a bounded, redacted form instead
15. **Never put secrets or payloads in an error or a log** — Wi-Fi keys, the web-server token and packet bytes must not appear in messages, attributes or wrapped causes
16. **Keep log messages low-cardinality** — stable message text, variable data as structured attributes
17. **Capability limits are states, not crashes** — `dependency_missing`, `permission_required`, `unsupported` are reported, never hidden behind a fake success

## kali-tools error flow

```go
// capability or app: typed, wrapped
if err := validatePorts(req.Ports); err != nil {
    return Result{}, domain.Invalid("ports: %v", err)
}
out, err := runner.Run(ctx, path, args)
if err != nil {
    return Result{}, fmt.Errorf("running nmap: %w", err) // stderr stays inside the adapter
}

// driving adapter: one mapping function
func toDTO(err error) *domain.Error {
    if de, ok := errors.AsType[*domain.Error](err); ok {
        return de
    }
    return &domain.Error{Code: "failed", Message: "operation failed"} // details go to the redacted log only
}
```

The fallback code above is illustrative; use the codes the project already defines.

## Detailed Reference

- **[Error Creation](./references/error-creation.md)** — How to create errors that tell the story: error messages should be lowercase, no punctuation, and describe what happened without prescribing action. Covers sentinel errors (one-time preallocation for performance), custom error types (for carrying rich context), and the decision table for which to use when.

- **[Error Wrapping and Inspection](./references/error-wrapping.md)** — Why `fmt.Errorf("{context}: %w", err)` beats `fmt.Errorf("{context}: %v", err)` (chains vs concatenation). How to inspect chains with `errors.Is`, `errors.As`, and Go 1.26+ `errors.AsType` for type-safe error handling, and `errors.Join` for combining independent errors.

- **[Error Handling Patterns and Logging](./references/error-handling.md)** — The single handling rule, panic/recover at goroutine boundaries, mapping errors in each driving adapter, and redacted `slog` logging.

## Error handling audits

Audit these independent categories (they can be split across parallel reviewers):

- Category 1: Error creation — validate `errors.New`/`fmt.Errorf` usage, low-cardinality messages, custom types
- Category 2: Error wrapping — audit `%w` vs `%v`, verify `errors.Is`/`errors.As` patterns
- Category 3: Single handling rule — find log-and-return violations, swallowed errors, discarded errors (`_`)
- Category 4: Panic/recover — audit `panic` usage, verify recovery at goroutine boundaries
- Category 5: Structured logging and redaction — verify `slog` usage at error sites, and that no secret, token, payload or raw stderr reaches an error message or log

## Cross-References

- → See `golang-security` skill for redaction and secret handling
- → See `golang-safety` skill for nil interface trap and nil error comparison pitfalls
- → See `golang-naming` skill for error naming conventions (ErrNotFound, PathError)

## References

- [errors package](https://pkg.go.dev/errors)
- [log/slog package](https://pkg.go.dev/log/slog)
- [Go Code Review Comments: error strings](https://go.dev/wiki/CodeReviewComments#error-strings)
