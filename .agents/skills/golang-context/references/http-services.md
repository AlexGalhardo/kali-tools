# Context in the HTTP Adapter, Diagnostics and Storage

## Table of Contents

- [HTTP handlers](#http-handlers)
- [Starting a job from a handler](#starting-a-job-from-a-handler)
- [Server-Sent Events](#server-sent-events)
- [Outbound calls made by diagnostics](#outbound-calls-made-by-diagnostics)
- [Subprocesses and SQLite](#subprocesses-and-sqlite)

## HTTP handlers

`http.Request` carries a context that is cancelled when the client disconnects or the handler returns. MUST use `r.Context()` for work that belongs to the request; NEVER create `context.Background()` inside a handler.

```go
func handleJobStatus(a *app.Application) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        st, err := a.Jobs.Snapshot(r.PathValue("id"))
        if err != nil {
            writeError(w, err) // the single error-mapping function of httpapi
            return
        }
        writeJSON(w, http.StatusOK, st)
    })
}
```

## Starting a job from a handler

A diagnostic outlives the `POST` that started it. The handler validates input, asks the manager to start the job and returns the job ID. The manager runs the job under its own context, so neither the request ending nor `WithoutCancel(r.Context())` decides the job's lifetime; cancellation goes through `Cancel(id)` and application shutdown.

## Server-Sent Events

- Stop the stream on `r.Context().Done()` and unsubscribe from the manager.
- Do not set a global `WriteTimeout` (it kills SSE); use `http.NewResponseController(w).SetWriteDeadline` per write and flush after each event.
- Send a keep-alive comment every 15 s.

```go
for {
    select {
    case <-r.Context().Done():
        return
    case ev, ok := <-sub:
        if !ok {
            return
        }
        _ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
        if err := writeEvent(w, ev); err != nil {
            return
        }
        _ = rc.Flush()
    case <-keepAlive.C:
        // write a ": ping" comment line and flush
    }
}
```

## Outbound calls made by diagnostics

HTTP, TLS, DNS and TCP diagnostics are bounded by the validated request timeout. Use context-aware APIs: `http.NewRequestWithContext`, `(&net.Dialer{}).DialContext`, `(*net.Resolver).LookupHost(ctx, ...)`.

```go
// ✗ Bad — ignores cancellation and the job deadline
httpReq, _ := http.NewRequest(http.MethodGet, target, nil)

// ✓ Good
ctx, cancel := context.WithTimeout(ctx, timeout)
defer cancel()
httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
if err != nil {
    return Result{}, fmt.Errorf("building request: %w", err)
}
```

## Subprocesses and SQLite

- Executables run with `exec.CommandContext(ctx, path, argv...)` through `internal/adapter/tools`, which also cancels descendants (Job Objects on Windows, process groups on Unix).
- SQL uses `ExecContext`, `QueryContext`, `QueryRowContext` and `BeginTx(ctx, ...)` with `?` placeholders.

```go
row := s.db.QueryRowContext(ctx, "SELECT state FROM jobs WHERE id = ?", id)
```
