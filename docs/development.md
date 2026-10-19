# Development

## Prerequisites

- Go 1.27.2 on PATH.
- Node 24.20.0 and Bun 1.4.3.
- Wails v2.17.0: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0`.
- golangci-lint v2.15.0: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0`.
- govulncheck v1.9.0: `go install golang.org/x/vuln/cmd/govulncheck@v1.9.0`.
- A native C compiler for `-race`; Windows verification uses w64devkit GCC 16.2.0.
- Docker Desktop with its Linux engine running for the private lab.

Run `wails doctor`. Windows needs WebView2; Linux needs GTK3 and WebKitGTK 4.1 development packages and the `webkit2_41` tag; macOS needs its native development toolchain. Cross compilation does not prove runtime support.

## Commands

Run these from the repository root in PowerShell, cmd, or a Unix shell:

```text
npm --prefix frontend ci
npm --prefix frontend exec playwright install chromium
node scripts/task.mjs go
node scripts/task.mjs web
node scripts/task.mjs race
node scripts/task.mjs fuzz
node scripts/task.mjs workflows
node scripts/task.mjs build
node scripts/task.mjs dev
```

The npm lockfile is the verified fallback because Bun 1.4.3 cannot create/replace its temporary lockfile under this execution sandbox (EBADF). Bun parity and a frozen Bun install remain an open S04 gate. Do not delete the npm lockfile until a replacement has been verified.

`wails build -tags desktop` builds the Windows desktop. A direct Go desktop build must run the frontend production build first. `wails generate module -tags desktop` regenerates committed bridge files; compare `git diff -- frontend/wailsjs` after generation. The frontend build restores the dist marker used by a clean checkout.

## CLI

```text
go run ./cmd/hacker-tools version
go run ./cmd/hacker-tools doctor
go run ./cmd/hacker-tools interfaces
go run ./cmd/hacker-tools diagnose --kind tcp --target localhost --port 8080
go run ./cmd/hacker-tools diagnose --kind http --target http://localhost:8080/health
go run ./cmd/hacker-tools discover --target 127.0.0.1/32 --ports 8080,9090 --format csv
go run ./cmd/hacker-tools import --file synthetic.pcap
```

JSON uses `schema_version: 1`. Exit 0 is success, 1 is an operation failure, 2 is invalid input. Diagnostic timeouts are 100–30000 ms. Discovery accepts IPv4 /24 or narrower, at most 32 unique ports and at most 32 workers. HTTP redirects are reported rather than followed. IPv4 ping and traceroute currently require the Windows native provider. Wi-Fi reveal is desktop-only, explicit and never written to normal history.

## Private laboratory

```text
docker compose -f docker/compose.yml up --build --abort-on-container-exit --exit-code-from test
docker compose -f docker/compose.yml down --volumes --remove-orphans
```

The test service waits for fixture health and fails on wrong HTTP status, DNS answer/NXDOMAIN or TCP inventory. The network is internal with no published ports. The CLI runtime and test service are non-root; the test image contains the toolchain required by race tests. Docker does not validate host Wi-Fi, native WebViews, capture authorization or macOS/Linux hardware.

## Native acceptance

The opt-in Windows bridge smoke command is `go test -tags desktop,nativesmoke -run TestNativeBridge -timeout 45s .` after the frontend is built. It starts a hidden actual WebView, calls Interfaces, Settings and Version through Wails, then exits. Tests use a temporary application-data directory. A denied or unavailable native environment remains a pending gate.

On an owned test Wi-Fi network, verify metadata, explicit key reveal, denied access, enterprise/open/no-saved-key states and 15-second clearing. Never attach real secrets or raw captures to verification records. For live capture, generate controlled fixture traffic, stop/cancel, and check the handle is closed. Record native OS, architecture, provider and limitations.
