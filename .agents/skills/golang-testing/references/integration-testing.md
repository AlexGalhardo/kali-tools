# Integration Testing

## Docker lab

`tests/integration` holds tests behind `//go:build integration`. They run against `cmd/lab-fixture`, a deterministic HTTP/DNS/TCP fixture on a private Docker Compose network (`docker/`), located through the `LAB_FIXTURE` environment variable.

```go
//go:build integration

package integration

func TestPrivateLab(t *testing.T) {
    host := os.Getenv("LAB_FIXTURE")
    if host == "" {
        t.Fatal("LAB_FIXTURE is required; use docker compose lab")
    }
    ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
    defer cancel()
    // run diagnostics against the fixture only
}
```

Run it through the setup entry points rather than by hand, for example:

```bash
bash scripts/setup/setup-linux-docker.sh test
```

Rules:

- Target only the lab fixture or loopback. Never scan or probe public or third-party hosts from tests.
- Every operation has a deadline; the suite must not hang when the lab is down.
- The CLI is exercised as a process in the lab image; assert exit codes and the JSON envelope.

## What is not integration

- SQLite store tests run in-process with a temp file; they are ordinary unit tests.
- Contract suites in `internal/porttest` run without Docker.

## Native acceptance

Windows raw capture, ICMP, Wi-Fi and the Wails bridge need real hardware. Record the evidence (commands, OS build, output) in `docs/verification/`. Cross-compilation and Docker results never count as native acceptance (AGENTS.md).
