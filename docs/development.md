# Development

## Prerequisites

- Go 1.27.3 on PATH.
- Node 24.21.0 and Bun 1.4.3.
- Wails v2.17.0: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0`.
- golangci-lint v2.15.0: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0`.
- govulncheck v1.9.0: `go install golang.org/x/vuln/cmd/govulncheck@v1.9.0`.
- A native C compiler for `-race`; Windows verification uses w64devkit GCC 16.2.0.
- Docker Desktop with its Linux engine running for the private lab.

Run `wails doctor`. Windows needs WebView2; Linux needs GTK3 and WebKitGTK 4.1 development packages and the `webkit2_41` tag; macOS 13 or later on Apple Silicon needs its native development toolchain. Intel macOS is not a 1.0 target. Cross compilation does not prove runtime support. Releases stay vendor-unsigned ([ADR 009](adr/009-signing-and-provenance.md)).

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
go run ./cmd/kali-tools diagnose tcp --target localhost --port 8080
go run ./cmd/kali-tools diagnose http --target http://localhost:8080/health
go run ./cmd/kali-tools discover --authorized --target 127.0.0.1/32 --ports 8080,9090 --format csv
go run ./cmd/kali-tools inspect --authorized --target 127.0.0.1/32 --ports 8080
go run ./cmd/kali-tools --json neighbors --family all
go run ./cmd/kali-tools tls-audit --target example.com --lint=true --json
go run ./cmd/kali-tools dns-inspect --target example.com --json
go run ./cmd/kali-tools local-discover --interface eth0 --scope 192.0.2.0/24 --json
go run ./cmd/kali-tools snmp --interface eth0 --scope 192.0.2.0/24 --target 192.0.2.1 --owned --secret-file ./secret.txt --json
go run ./cmd/kali-tools capture import --file synthetic.pcap
```

Start a local listener for the TCP and HTTP examples; a refused connection is an expected diagnostic failure. Diagnostics stream bounded progress on stderr and record redacted job history. `kali-tools serve` is the local web server; its routes, errors and capability rules are in the [HTTP contract](http.md).

Human output is the default; scripts use `--json` with `schema_version: 1`. The [CLI contract](cli.md) defines all stable exit codes. Diagnostic timeouts are 100–30000 ms. Discovery accepts IPv4 /24 or narrower, at most 32 unique ports and at most 32 workers. HTTP redirects are reported rather than followed. IPv4 ping and traceroute currently require the Windows native provider. Wi-Fi reveal requires explicit desktop consent or `wifi reveal --confirm`; keys never enter normal history.

## Private laboratory

The CLI uses canonical command paths, human or explicit JSON output, localized result labels and bounded job progress. Settings/history persist in the shared per-user SQLite database; jobs and capture artifacts remain process-local. The CLI never prompts or reads stdin. `NO_COLOR` disables state colors. Native SSH acceptance remains pending; see [C01 evidence](verification/specs/C01.md).

```text
node scripts/task.mjs lab
```

The Node lab command uses a unique Compose project, bounds build to 10 minutes and execution to 3 minutes, and always attempts cleanup with a 30-second deadline. It preserves test failures and fails if cleanup fails. The Docker-only setup scripts remain available for hosts without Node.

The test service waits for fixture health and fails on wrong HTTP status, DNS answer/NXDOMAIN or TCP inventory. The network is internal with no published ports. The CLI runtime and test service are non-root; the test image contains the toolchain required by race tests. Docker does not validate host Wi-Fi, native WebViews, capture authorization or macOS/Linux hardware.

## Native acceptance

The opt-in Windows bridge smoke command is `go test -race -tags desktop,production,nativesmoke -run TestNativeBridge -timeout 45s .` after the frontend is built. It starts a hidden actual WebView, calls Interfaces, Settings and Version through Wails, then exits. Tests use a temporary application-data directory. A denied or unavailable native environment remains a pending gate.

On an owned test Wi-Fi network, verify metadata, explicit key reveal, denied access, enterprise/open/no-saved-key states and 15-second clearing. Never attach real secrets or raw captures to verification records. For live capture, generate controlled fixture traffic, stop/cancel, and check the handle is closed. Record native OS, architecture, provider and limitations.

## Windows: slow tests under Microsoft Defender

On Windows, Defender real-time scanning can make Go builds and the SQLite/storage tests slow, because every test binary under `%TEMP%\go-build*` and every database file is scanned. This is optional and affects only your machine. If those tests are noticeably slow, you may exclude the Go build cache directory and your clone from real-time scanning. It needs an elevated PowerShell and lowers protection for those paths, so exclude only directories you control:

```powershell
# Optional, run as Administrator. Adjust the repository path.
Add-MpPreference -ExclusionPath "$env:TEMP\go-build*"
Add-MpPreference -ExclusionPath "C:\dev\kali-tools"
```

Remove them later with `Remove-MpPreference -ExclusionPath <path>`. If the wildcard is not accepted on your Defender version, exclude `$env:TEMP` instead only if you accept that wider scope, or skip this step. Never exclude paths that hold downloaded artifacts or captures.

## Wails development tools

Reviewed 2026-10-19 against the installed Wails v2.17.0 CLI and official documentation:

- [CLI reference](https://v2.wails.io/docs/reference/cli/): `wails doctor`, `wails dev -tags desktop`, `wails build -tags desktop`, and `wails generate module -tags desktop` cover prerequisites, live reload, native packaging and generated TypeScript bindings. `node scripts/task.mjs bindings` regenerates and rejects binding drift relative to Git.
- [Runtime API](https://v2.wails.io/docs/reference/runtime/intro/): Go `pkg/runtime` and generated JavaScript wrappers provide events, dialogs and window controls. Use the lifecycle context; runtime calls that need the initialized window belong in `OnDomReady`.
- [Manual builds](https://v2.wails.io/docs/guides/manual-builds/): actual desktop execution requires `desktop,production` or the development runtime tags. The Windows native bridge check is `node scripts/task.mjs bridge`; its test thread initializes COM and uses isolated application and WebView data directories.
- [Wails v3 CLI MCP](https://v3.wails.io/guides/cli/): `wails3 mcp` manages project lifecycle with bounded jobs and a restricted project root. The separate [experimental application MCP](https://docs-preview.wails.io/guides/mcp-service/) controls a running WebView when explicitly compiled with the `mcp` tag. These are v3 capabilities; the installed v2 CLI has no MCP command. This project retains its planned v2 baseline and does not add a custom MCP server or migrate frameworks for development tooling.

## Active CI/CD and packaging

Run `node scripts/task.mjs arch` to prove each of eight dependency rules rejects a deliberate forbidden import and cleans up afterward. It requires golangci-lint 2.15.0 on PATH; the existing lint hook enforces the same rules on commits.

The owner authorized workflow activation on 2026-10-19. See [the CI/CD guide](ci/README.md). `node scripts/task.mjs workflows` checks active source/pins; `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` performs full workflow static validation.

After a native CLI/desktop build, `go mod download all` and `node scripts/package.mjs` assemble that host's release archive with notices and source metadata. Go/production frontend licenses are collected from pinned installed sources; missing notice files fail packaging. Native CI supplies Windows x64, Ubuntu 24.04 x64 and macOS Apple Silicon. The matrix, uninstall and writable-data path are in [packaging](packaging.md).

DNS accepts `--record A|AAAA|CNAME|MX|NS|TXT|PTR`. Discovery and native service inspection each use at most 100 probes/second, 32 workers and a two-minute deadline. Service inspection does not run Nmap. Packet conversion is explicit: `go run ./cmd/kali-tools capture import --file input.pcap --output output.pcapng --format pcapng`.

The local Windows preview can be copied from build/bin/kali-tools-desktop.exe to the repository root; both locations are ignored. Existing user preferences/history under the previous hacker-tools directory are reused without moving files; new installations use kali-tools. No configuration outside the repository is rewritten during the rename.

## HTTP browser verification

Build the frontend before running Playwright: `cd frontend`, `bun run build`, then `bunx playwright test`. The `wails-fake` project retains 19 desktop workflows. The `http` project runs five shared workflows and six HTTP-only checks against `go run -tags e2e ./tests/e2e/httpserver`. The fixture is not part of production builds. It uses synthetic Wi-Fi data, a local TCP target and a synthetic PCAP.

`PW_PORT` defaults to 4173 and `PW_HTTP_PORT` to 4174; both accept integer ports 1024?65535. Tests use one worker to isolate shared settings/history. Global teardown terminates the fixture process tree. The production server composes the same application services through `bootstrap.New`; tests do not add production injection hooks.

## Build from source

The end-user quick start is in the [README](../README.md). This section is for contributors who build kali-tools themselves. The six setup scripts are listed in the README table.

### 1. Get the source

```bash
git clone https://github.com/AlexGalhardo/kali-tools.git
cd kali-tools
```

If GitHub requests authentication, use an account with access to the repository. Never save access tokens in project files.

### 2. Prepare your computer without Docker

The development baseline is Go **1.27.3**, Node.js **24.21.0**, Bun **1.4.3** and Wails **2.17.0**. Install Go, Node.js and Git first. Reopen your terminal after installation to load PATH changes. macOS development for 1.0 is Apple Silicon on macOS 13 or later ([ADR 007](docs/adr/007-macos-baseline.md)).

Setup installs pinned Wails, golangci-lint and govulncheck into the repository's ignored `.tools/bin` directory. It installs exact frontend dependencies from the npm lockfile and downloads Playwright Chromium. It does not change your global Go binary directory. GitHub Actions is enabled separately in the repository.

#### Windows 11

1. Install [Git for Windows](https://git-scm.com/downloads/win), [Go](https://go.dev/dl/) and [Node.js](https://nodejs.org/en/download).
2. Ensure [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) is installed; Wails doctor checks this.
3. Install a native GCC compiler for Go race tests, such as [w64devkit](https://github.com/skeeto/w64devkit/releases). Extract it and add its `bin` directory to PATH.
4. Open a new Git Bash at the repository root and run:

```bash
go version
node --version
npm --version
gcc --version
bash scripts/setup/setup-windows.sh setup
```

#### Linux

Ubuntu 24.04 LTS is the documented Linux baseline. Debian 13 and Kali are best-effort and come with no guarantee. Other distributions need equivalent GTK3 and WebKitGTK development packages. See [ADR 010](docs/adr/010-linux-baseline.md).

1. Install Go and Node.js from their official distributions.
2. Install native build dependencies:

```bash
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
```

Continue by preparing the project:

```bash
go version
node --version
bash scripts/setup/setup-linux.sh setup
```

The Linux script supplies the `webkit2_41` Wails build tag. If Playwright reports missing browser system packages, follow its diagnostic instructions. Native Linux feature acceptance remains pending.

#### macOS

Install Go and Node.js for your Mac architecture. Install Xcode Command Line Tools if they are not already installed:

```bash
xcode-select --install
go version
node --version
bash scripts/setup/setup-darwin.sh setup
```

Native macOS Wi-Fi/Keychain and capture support remain pending. Successful compilation does not imply feature parity.

### 3. Test without Docker

Choose the script for your computer:

```bash
# Windows / Git Bash
bash scripts/setup/setup-windows.sh test

# Linux
bash scripts/setup/setup-linux.sh test

# macOS
bash scripts/setup/setup-darwin.sh test
```

The `test` action stops on failure and runs Go formatting/module checks, vet, unit tests with coverage, golangci-lint, govulncheck, race tests, parser fuzz smoke, frontend lint/typecheck/build, Playwright workflows/accessibility, workflow pin/static checks and Markdown lint.

Tests use synthetic secrets and capture bytes. A skipped native check means hardware or permission acceptance is still pending, not that the capability was verified.

To run one group after setup:

```bash
export PATH="$PWD/.tools/bin:$PATH"
node scripts/task.mjs go
node scripts/task.mjs race
node scripts/task.mjs fuzz
node scripts/task.mjs web
node scripts/task.mjs workflows
node scripts/markdown.mjs
```

### 4. Compile and open without Docker

Choose **one** build command:

```bash
bash scripts/setup/setup-windows.sh build
bash scripts/setup/setup-linux.sh build
bash scripts/setup/setup-darwin.sh build
```

Outputs are in `build/bin/`. The Windows desktop executable is `kali-tools-desktop.exe`; the CLI is `kali-tools.exe`. These are unsigned development artifacts.

For development with automatic frontend/backend rebuilds, use your script with `dev`:

```bash
bash scripts/setup/setup-windows.sh dev
# Linux: bash scripts/setup/setup-linux.sh dev
# macOS: bash scripts/setup/setup-darwin.sh dev
```

Use `doctor` to inspect prerequisites or `help` to see the action list. `doctor --bundle PATH` writes a redacted diagnostic zip on this computer and does not upload it.

### 5. Build and test with Docker

Install and start an engine first:

- **Windows:** Docker Desktop with its WSL2 backend and **Linux containers** selected.
- **Linux:** Docker Engine plus the Compose plugin, following [Docker's official instructions](https://docs.docker.com/engine/install/). Your account must be able to access the daemon.
- **macOS:** Docker Desktop for your Mac's Intel or Apple Silicon architecture.

Verify engine and Compose access:

```bash
docker version
docker compose version
```

Choose **one** command for your host. No Go or Node.js installation is needed on the host for these Docker scripts:

```bash
# Windows / Git Bash
bash scripts/setup/setup-windows-docker.sh test

# Linux
bash scripts/setup/setup-linux-docker.sh test

# macOS
bash scripts/setup/setup-darwin-docker.sh test
```

The first run downloads base images and builds the CLI, fixture and test images. A private network contains health-checked HTTP/TCP/DNS services. Tests assert HTTP status, TCP inventory, DNS answers and NXDOMAIN, and execute the built CLI to check JSON schemas, successful diagnostics and invalid-input exit codes. Runtime and test services use a non-root account; no host ports are published.

The test script returns a nonzero exit code on failure and cleans up its lab containers/network on exit. It uses the Compose project name `kali-tools-lab`.

All three Docker scripts accept the same actions:

```bash
# Replace windows with linux or darwin for those hosts.
bash scripts/setup/setup-windows-docker.sh build    # Build images only
bash scripts/setup/setup-windows-docker.sh doctor   # Check daemon/Compose/configuration
bash scripts/setup/setup-windows-docker.sh down     # Manual lab cleanup
bash scripts/setup/setup-windows-docker.sh help
```

Docker builds Linux CLI images. It does not create native Windows/macOS desktop installers or replace native acceptance tests.

## Troubleshooting

| Symptom | Action |
| --- | --- |
| Go, Node, npm or GCC not found | Install the prerequisite, add it to PATH and reopen the terminal. |
| Missing WebView/GTK/WebKit | Install the OS prerequisites above, then run `doctor`. |
| `-race` needs CGO/C compiler | Install GCC on Windows/Linux or Xcode tools on macOS. |
| Docker daemon unavailable/denied | Start the engine; check context, Linux-container mode and daemon access. |
| Bun reports `EBADF` in a sandbox | Use the committed npm lockfile and `npm ci`; the new Bun lockfile is now also verified with `bun install --frozen-lockfile`. |
| Browser test port is occupied | Stop the previous development/test server and retry. |
| Windows reports `EPERM` on a native npm module during build | Close running Vite/Playwright development processes before `npm ci` or a Wails rebuild; the DLL may still be loaded. |
| Wi-Fi/capture denied or unsupported | Read the capability response; permissions and hardware need separate native acceptance. |

Windows raw capture is limited to IPv4, requires administrator authorization, reports no reliable drop count, and is bounded to 60 seconds/10000 packets. Revealed Wi-Fi keys are never written to ordinary history. See [capture boundaries](docs/adr/002-capture-backend.md).

Capture backends are selectable in the packet inspector and with `capture live --backend raw_socket|pktmon|npcap|libpcap`. Pktmon requires an elevated Windows process, captures all NICs with 512-byte truncation and reports circular-buffer/duplicate limits. Npcap must be [installed separately by the user](https://npcap.com/#download) and requires a build with `-tags pcap`. The Npcap free license forbids redistribution without a commercial license, so kali-tools never bundles, installs or redistributes it. Npcap permits local IPv4/IPv6 selection; Pktmon requires no target address. On Linux, `libpcap` is included in the Ubuntu native CI artifact (`-tags pcap`). Opening an interface needs `CAP_NET_RAW` on the capture process. The UI and the web server are not run as root. An untagged build reports `unsupported`. Optional backends have automated implementation evidence but native qualification remains open; see [ADR 005](docs/adr/005-windows-capture-backend.md) and [ADR 010](docs/adr/010-linux-baseline.md).

kali-tools 1.0 has no privileged helper and no helper IPC ([ADR 011](docs/adr/011-no-privileged-helper.md)). The UI and the web server stay unprivileged. On Linux, grant capture once with `sudo setcap cap_net_raw+eip` on the capture binary. Do not run the UI or the web server as root. On macOS, `doctor` reports whether this user can open `/dev/bpf`. The fix is membership of the `access_bpf` group from the ChmodBPF package, then a new login. kali-tools never changes device permissions. On Windows, install Npcap from the link above or start an elevated terminal for Pktmon and raw IPv4. When permission is missing, the UI, CLI and `doctor` show the capability state and that fix.

On Windows 11 24H2+, a Wi-Fi location-consent notice points to Settings > Privacy & security > Location (`ms-settings:privacy-location`); allow location for desktop apps and select **Check again**. Browser mode displays the URI for the server account and cannot open that host's settings.

## Repository layout

```text
main.go, app.go          Wails desktop composition root (must stay at the root for Wails v2)
cmd/kali-tools/          CLI
internal/domain, app     shared kernel and application services
internal/network|capture|wifi   capability packages
internal/adapter/        SQLite, external tools, ICMP, OS platform (+ future cli, wailsui, httpapi)
frontend/src/            app/, features/, shared/{api,i18n,ui}/, styles/
scripts/, scripts/setup/ task runners and per-OS setup entry points
docs/                    architecture, research, specs, ADRs, releases, verification
```

Formatting and linting are enforced: run `lefthook install` once; each commit runs `golangci-lint fmt` (gofmt -s + goimports), `golangci-lint run` and `go vet`.
