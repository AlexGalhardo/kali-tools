# Development

## Prerequisites

- Go 1.27.3 on PATH.
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

Bun 1.4.3 now has a workspace `frontend/bun.lock`, ready for commit. Run `bun install --frozen-lockfile` and `bun run build` from `frontend/`; both passed on the Windows development host. The npm lockfile remains the setup/Wails fallback. Keep both lockfiles synchronized when changing dependencies.

`wails build -tags desktop` builds the Windows desktop. A direct Go desktop build must run the frontend production build first. `node scripts/task.mjs bindings` regenerates `frontend/wailsjs/go/wailsui/Bindings.{d.ts,js}` and checks drift against Git. The frontend bridge, Playwright fakes and native smoke test use `window.go.wailsui.Bindings`. The frontend build restores the dist marker used by a clean checkout.

## CLI

```text
go run ./cmd/kali-tools version
go run ./cmd/kali-tools doctor
go run ./cmd/kali-tools interfaces
go run ./cmd/kali-tools diagnose --kind tcp --target localhost --port 8080
go run ./cmd/kali-tools diagnose --kind http --target http://localhost:8080/health
go run ./cmd/kali-tools discover --target 127.0.0.1/32 --ports 8080,9090 --format csv
go run ./cmd/kali-tools import --file synthetic.pcap
```

JSON uses `schema_version: 1`. Exit 0 is success, 1 is an operation failure, 2 is invalid input. Diagnostic timeouts are 100–30000 ms. Discovery accepts IPv4 /24 or narrower, at most 32 unique ports and at most 32 workers. HTTP redirects are reported rather than followed. IPv4 ping and traceroute currently require the Windows native provider. Wi-Fi reveal is desktop-only, explicit and never written to normal history.

## Private laboratory

Current C01 checkpoint: JSON remains the default. Canonical forms include `diagnose tcp`, `wifi show`, `wifi reveal --confirm`, `capture live`, `capture import` and `capture export`. `help` lists per-command flags; unrelated flags are rejected. History/settings are currently process-local because CLI composition still uses in-memory SQLite. Human/progress output, persistent composition and SSH acceptance remain pending. See [C01 evidence](verification/specs/C01.md) for exit codes and checkpoint limits.

```text
node scripts/task.mjs lab
```

The Node lab command uses a unique Compose project, bounds build to 10 minutes and execution to 3 minutes, and always attempts cleanup with a 30-second deadline. It preserves test failures and fails if cleanup fails. The Docker-only setup scripts remain available for hosts without Node.

The test service waits for fixture health and fails on wrong HTTP status, DNS answer/NXDOMAIN or TCP inventory. The network is internal with no published ports. The CLI runtime and test service are non-root; the test image contains the toolchain required by race tests. Docker does not validate host Wi-Fi, native WebViews, capture authorization or macOS/Linux hardware.

## Native acceptance

The opt-in Windows bridge smoke command is `go test -race -tags desktop,production,nativesmoke -run TestNativeBridge -timeout 45s .` after the frontend is built. It starts a hidden actual WebView, calls Interfaces, Settings and Version through Wails, then exits. Tests use a temporary application-data directory. A denied or unavailable native environment remains a pending gate.

On an owned test Wi-Fi network, verify metadata, explicit key reveal, denied access, enterprise/open/no-saved-key states and 15-second clearing. Never attach real secrets or raw captures to verification records. For live capture, generate controlled fixture traffic, stop/cancel, and check the handle is closed. Record native OS, architecture, provider and limitations.

## Wails development tools

Reviewed 2026-10-19 against the installed Wails v2.17.0 CLI and official documentation:

- [CLI reference](https://v2.wails.io/docs/reference/cli/): `wails doctor`, `wails dev -tags desktop`, `wails build -tags desktop`, and `wails generate module -tags desktop` cover prerequisites, live reload, native packaging and generated TypeScript bindings. `node scripts/task.mjs bindings` regenerates and rejects binding drift relative to Git.
- [Runtime API](https://v2.wails.io/docs/reference/runtime/intro/): Go `pkg/runtime` and generated JavaScript wrappers provide events, dialogs and window controls. Use the lifecycle context; runtime calls that need the initialized window belong in `OnDomReady`.
- [Manual builds](https://v2.wails.io/docs/guides/manual-builds/): actual desktop execution requires `desktop,production` or the development runtime tags. The Windows native bridge check is `node scripts/task.mjs bridge`; its test thread initializes COM and uses isolated application and WebView data directories.
- [Wails v3 CLI MCP](https://v3.wails.io/guides/cli/): `wails3 mcp` manages project lifecycle with bounded jobs and a restricted project root. The separate [experimental application MCP](https://docs-preview.wails.io/guides/mcp-service/) controls a running WebView when explicitly compiled with the `mcp` tag. These are v3 capabilities; the installed v2 CLI has no MCP command. This project retains its planned v2 baseline and does not add a custom MCP server or migrate frameworks for development tooling.

## Active CI/CD and packaging

Run `node scripts/task.mjs arch` to prove each of eight dependency rules rejects a deliberate forbidden import and cleans up afterward. It requires golangci-lint 2.15.0 on PATH; the existing lint hook enforces the same rules on commits.

The owner authorized workflow activation on 2026-10-19. See [the CI/CD guide](ci/README.md). `node scripts/task.mjs workflows` checks active source/pins; `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` performs full workflow static validation.

After a native CLI/desktop build, `go mod download all` and `node scripts/package.mjs` assemble that host's release archive with notices and source metadata. Go/production frontend licenses are collected from pinned installed sources; missing notice files fail packaging. Native CI supplies Windows x64, Linux x64 and both Mac architectures.

DNS accepts `--record A|AAAA|CNAME|MX|NS|TXT|PTR`. Discovery launches at most 100 probes/second with 32 workers and a two-minute total deadline. Nmap service inspection has its own fixed two-minute deadline. Packet conversion is explicit: `go run ./cmd/kali-tools import --file input.pcap --output output.pcapng --format pcapng`.

The local Windows preview can be copied from build/bin/kali-tools-desktop.exe to the repository root; both locations are ignored. Existing user preferences/history under the previous hacker-tools directory are reused without moving files; new installations use kali-tools. No configuration outside the repository is rewritten during the rename.
