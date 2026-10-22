# kali-tools

A local network and security toolkit inspired by the Kali Linux ecosystem, built with Go, Wails, React, TypeScript, Base UI, Tailwind CSS v4 and SQLite. One Go core serves three interfaces: the native desktop GUI (primary), a web server with the same UI (planned) and a complete CLI for headless/SSH hosts (in progress). The interface supports English and Brazilian Portuguese.

**Development preview:** core workflows are implemented and tested; hardware qualification is still open. Windows 11 leads development. Linux/macOS binaries do not imply Wi-Fi or capture parity. See [PLAN.md](PLAN.md) and [verification evidence](docs/verification/0.1.0-alpha.2-dev.md).

## What it does

- Lists interfaces, addresses, gateways and DNS; monitors native traffic counters.
- Runs DNS record queries, bounded TCP/HTTP/TLS checks and Windows IPv4 ping/traceroute.
- Discovers explicitly selected IPv4 scopes and optionally inspects service signatures with separately installed Nmap.
- Inspects Windows Wi-Fi metadata and reveals the current saved key only on explicit request; the display clears automatically.
- Imports PCAP/PCAPNG, summarizes packets and explicitly saves retained packet data in either format.
- Keeps preferences and redacted operation history locally in SQLite. No cloud runtime or telemetry is required.

## Development downloads

[GitHub releases](https://github.com/AlexGalhardo/kali-tools/releases) contain development release drafts assembled only after CI passes. Archives target Windows x64, Ubuntu 24.04 x64, macOS Intel and macOS Apple Silicon. Each includes the desktop, CLI, license/notices, dependency inventory and build metadata. Verify downloads against `SHA256SUMS`.

Windows uses Edge WebView2. Linux requires GTK3 and WebKitGTK 4.1. macOS previews are not notarized. Development builds have no trusted signing identity. Read the release notes before testing; a successful native build does not qualify hardware features.

## Choose your setup

There are exactly six shell scripts:

| System | Without Docker: native desktop + CLI | With Docker: isolated CLI lab |
| --- | --- | --- |
| Windows 11 | `scripts/setup/setup-windows.sh` | `scripts/setup/setup-windows-docker.sh` |
| Linux | `scripts/setup/setup-linux.sh` | `scripts/setup/setup-linux-docker.sh` |
| macOS | `scripts/setup/setup-darwin.sh` | `scripts/setup/setup-darwin-docker.sh` |

Run these in **Git Bash on Windows**, or Terminal on Linux/macOS. Start at the repository root. Using `bash script.sh` does not require changing executable permissions.

Docker tests the headless CLI/core in a private laboratory. To open the native desktop application, use the **without Docker** instructions. A container does not verify host Wi-Fi, Keychain, WebViews or capture authorization.

## 1. Get the source

```bash
git clone https://github.com/AlexGalhardo/kali-tools.git
cd kali-tools
```

If GitHub requests authentication, use an account with access to the repository. Never save access tokens in project files.

## 2. Prepare your computer without Docker

The development baseline is Go **1.27.2**, Node.js **24.21.0**, Bun **1.4.3** and Wails **2.17.0**. Install Go, Node.js and Git first. Reopen your terminal after installation to load PATH changes.

Setup installs pinned Wails, golangci-lint and govulncheck into the repository's ignored `.tools/bin` directory. It installs exact frontend dependencies from the npm lockfile and downloads Playwright Chromium. It does not change your global Go binary directory. GitHub Actions is enabled separately in the repository.

### Windows 11

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

### Linux

The initial desktop build instructions target Ubuntu 24.04 or compatible systems. Other distributions need equivalent GTK3 and WebKitGTK development packages.

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

### macOS

Install Go and Node.js for your Mac architecture. Install Xcode Command Line Tools if they are not already installed:

```bash
xcode-select --install
go version
node --version
bash scripts/setup/setup-darwin.sh setup
```

Native macOS Wi-Fi/Keychain and capture support remain pending. Successful compilation does not imply feature parity.

## 3. Test without Docker

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

## 4. Compile and open without Docker

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

Use `doctor` to inspect prerequisites or `help` to see the action list.

## 5. Build and test with Docker

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

## 6. Try the CLI

```bash
go run ./cmd/kali-tools version
go run ./cmd/kali-tools doctor
go run ./cmd/kali-tools --json interfaces
go run ./cmd/kali-tools diagnose dns --target example.com --record MX
go run ./cmd/kali-tools diagnose tcp --target localhost --port 8080
go run ./cmd/kali-tools diagnose http --target http://localhost:8080/health
go run ./cmd/kali-tools discover --target 127.0.0.1/32 --ports 8080,9090
go run ./cmd/kali-tools inspect --target 127.0.0.1/32 --ports 8080
go run ./cmd/kali-tools capture import --file synthetic.pcap --output converted.pcapng --format pcapng
```

Start a listening local service for the TCP/HTTP examples; refusal is an expected diagnostic failure. Human output is the default; scripts must add `--json`, whose envelope includes `schema_version: 1`. Diagnostics stream bounded progress on stderr and record redacted job history. Settings and history persist in the shared per-user SQLite database. `wifi reveal --confirm` explicitly prints the saved key and a warning. Jobs remain owned by their starting process. See the [CLI contract](docs/cli.md) for commands, exit codes, SSH behavior and migration. Discovery accepts IPv4 /24 or narrower and up to 32 unique ports. Native SSH acceptance remains pending. The web server remains planned.

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

Capture backends are selectable in the packet inspector and with `capture live --backend raw_socket|pktmon|npcap`. Pktmon requires an elevated Windows process, captures all NICs with 512-byte truncation and reports circular-buffer/duplicate limits. Npcap must be [installed separately by the user](https://npcap.com/#download) and requires a build with `-tags pcap`; kali-tools never bundles or installs the driver. Npcap permits local IPv4/IPv6 selection; Pktmon requires no target address. Optional backends have automated implementation evidence but native qualification remains open; see [ADR 005](docs/adr/005-windows-capture-backend.md).

On Windows 11 24H2+, a Wi-Fi location-consent notice points to Settings > Privacy & security > Location (`ms-settings:privacy-location`); allow location for desktop apps and select **Check again**. Browser mode displays the URI for the server account and cannot open that host's settings.

## Guides

- [Delivery plan](PLAN.md)
- [Architecture (hexagonal, binding reference)](docs/architecture.md) and [architecture research](docs/architecture-research.md)
- [Spec Driven Development specs](docs/specs/README.md): what to implement next, with auditable quality gates
- [Tools, licenses and platform research](docs/tools-research.md)
- [Development and native acceptance](docs/development.md)
- [Dependency inventory](docs/dependencies.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)
- [Verification evidence](docs/verification/0.1.0-alpha.2-dev.md) ([previous](docs/verification/0.1.0-alpha.1-dev.md))

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

## License and contribution

Project code is licensed under [MIT](LICENSE.md). Bundled dependencies and vendored development skills retain their own licenses; see [the inventory](docs/dependencies.md). Nmap/TShark are optional separate installations and are not redistributed.

Contributions follow [CONTRIBUTING.md](CONTRIBUTING.md). Report bugs through [GitHub issues](https://github.com/AlexGalhardo/kali-tools/issues) and security concerns through [SECURITY.md](SECURITY.md). Do not attach passwords, tokens or private packet contents.

CI/CD is active under `.github/workflows/ci.yml`. It checks code, vulnerabilities, browser accessibility/workflows, Docker integration and native packaging. Only main-branch jobs can prepare release drafts. See [CI/CD details](docs/ci/README.md).

New installations use the per-user `kali-tools/history.db` configuration path. Existing development data under the legacy `hacker-tools` path remains readable without moving an open database. Source and CLI paths changed to `cmd/kali-tools`; the working directory itself need not be renamed.

Impeccable, Karpathy guidelines and Ponytail remain under `.agents/skills` with pinned source revisions. Signing, hardware qualification and stable distribution remain separate release gates.

## Use it in a browser

Run `kali-tools serve` and open the printed URL. The default listener is `127.0.0.1` on a free port. The URL fragment contains a per-launch token; the browser exchanges it for an HttpOnly session cookie and removes the fragment. Never share that URL. Stop the server with Ctrl-C.

Remote access requires a specific address, `--allow-remote`, `--tls-cert` and `--tls-key`. Anyone holding the token can run diagnostics, discovery and capture and reveal the saved Wi-Fi key as the server user. Capabilities reflect that user's permissions and consent. See [web security](SECURITY.md) and [release limits](docs/releases/0.5.0.md).
