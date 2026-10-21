# Context Values

kali-tools is a local application, not a set of services. Context values are rarely needed; prefer explicit parameters.

## Using context values correctly

Context values carry request-scoped metadata that crosses API boundaries — not function parameters, configuration, dependencies or optional arguments. A plausible candidate in kali-tools is a request ID used to correlate one HTTP request with its log lines.

Always use an unexported type as the key to prevent collisions between packages:

```go
// ✓ Good — unexported key type prevents collisions
type contextKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, contextKey{}, id)
}

func RequestIDFrom(ctx context.Context) (string, bool) {
    id, ok := ctx.Value(contextKey{}).(string)
    return id, ok
}
```

```go
// ✗ Bad — string keys collide across packages
ctx = context.WithValue(ctx, "request_id", id)
```

## What belongs in context values vs function parameters

| Data | Context value? | Why |
| --- | --- | --- |
| Request ID for log correlation | Yes | Request-scoped metadata |
| The web-server token or a Wi-Fi key | No | Secrets never travel implicitly and never reach logs |
| Store, `*sql.DB`, transaction | No | Dependency; pass explicitly (`docs/architecture-research.md` §3.6) |
| Settings, limits, feature switches | No | Configuration; inject through constructors |
| Diagnostic request fields (target, port, timeout) | No | Use-case parameters; pass as arguments |
| Logger | No | Loggers are dependencies; inject them |
