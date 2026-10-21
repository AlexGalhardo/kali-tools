# HTTP Handler Testing

Use `net/http/httptest` to test `internal/adapter/httpapi` without a real listener. Go 1.27 adds `httptest.NewTestServer(t, handler)`, an in-memory server that also works inside a `synctest` bubble.

## Handler test

```go
func TestStartDiagnostic(t *testing.T) {
    tests := []struct {
        name       string
        body       string
        wantStatus int
    }{
        {name: "valid request", body: `{"kind":"tcp","target":"127.0.0.1","port":443}`, wantStatus: http.StatusAccepted},
        {name: "invalid json", body: `{`, wantStatus: http.StatusBadRequest},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            h := newTestHandler(t) // real app.Application built from fakes
            req := httptest.NewRequest(http.MethodPost, "/api/diagnostics", strings.NewReader(tt.body))
            req.Host = "127.0.0.1:8123"
            addValidSession(req)
            w := httptest.NewRecorder()
            h.ServeHTTP(w, req)
            if w.Code != tt.wantStatus {
                t.Fatalf("status = %d, want %d; body=%s", w.Code, tt.wantStatus, w.Body)
            }
        })
    }
}
```

The status codes and helper names are illustrative; follow the routes and error mapping defined by PLAN W03.

## Mandatory security cases (PLAN W04)

| Case | Expect |
| --- | --- |
| `Host: evil.example:8123` | 403 |
| Cross-origin `POST` (`Sec-Fetch-Site: cross-site` or foreign `Origin`) | Rejected by `CrossOriginProtection` |
| Missing or invalid token/cookie on any `/api` route, including `/api/events` | 401 |
| Body larger than the route limit | 413 or 400 from `MaxBytesReader` |
| Any response or log line | Never contains the token |

## Server-Sent Events

Test `/api/events` with a real client over `httptest.NewServer` (or `NewTestServer`): read events, cancel the request context, and assert that the handler returns and the subscription is released. Verify that a reconnecting client can resynchronize with `seq` plus `Snapshot` without losing a terminal state.
