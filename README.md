# Hacker Tools

A local desktop network toolkit built with Go, Wails, React, TypeScript, Base UI, Tailwind CSS v4 and SQLite. The interface supports English and Brazilian Portuguese.

**Development alpha:** this is an implementation checkpoint, not a qualified release. Native hardware, Linux/macOS acceptance and packaging have open gates. See [PLAN.md](PLAN.md) and [verification evidence](docs/verification/0.1.0-alpha.1-dev.md).

## Choose your setup

There are exactly six shell scripts:

| System | Without Docker: native desktop + CLI | With Docker: isolated CLI lab |
| --- | --- | --- |
| Windows 11 | `setups/setup-windows.sh` | `setups/setup-windows-docker.sh` |
| Linux | `setups/setup-linux.sh` | `setups/setup-linux-docker.sh` |
| macOS | `setups/setup-darwin.sh` | `setups/setup-darwin-docker.sh` |

Run these in **Git Bash on Windows**, or Terminal on Linux/macOS. Start at the repository root. Using `bash script.sh` does not require changing executable permissions.

Docker tests the headless CLI/core in a private laboratory. To open the native desktop application, use the **without Docker** instructions. A container does not verify host Wi-Fi, Keychain, WebViews or capture authorization.

## 1. Get the source

```bash
git clone https://github.com/AlexGalhardo/hacker-tools.git
cd hacker-tools
```

If GitHub requests authentication, use an account with access to the repository. Never save access tokens in project files.

## 2. Prepare your computer without Docker

The development baseline is Go **1.27.2**, Node.js **24.20.0**, Bun **1.4.3** and Wails **2.17.0**. Install Go, Node.js and Git first. Reopen your terminal after installation to load PATH changes.

Setup installs pinned Wails, golangci-lint and govulncheck into the repository's ignored `.tools/bin` directory. It installs exact frontend dependencies from the npm lockfile and downloads Playwright Chromium. It does not change your global Go binary directory or activate GitHub Actions.

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
bash setups/setup-windows.sh setup
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
bash setups/setup-linux.sh setup
```

The Linux script supplies the `webkit2_41` Wails build tag. If Playwright reports missing browser system packages, follow its diagnostic instructions. Native Linux feature acceptance remains pending.

### macOS

Install Go and Node.js for your Mac architecture. Install Xcode Command Line Tools if they are not already installed:

```bash
xcode-select --install
go version
node --version
bash setups/setup-darwin.sh setup
```

Native macOS Wi-Fi/Keychain and capture support remain pending. Successful compilation does not imply feature parity.

## 3. Test without Docker

Choose the script for your computer:

```bash
# Windows / Git Bash
bash setups/setup-windows.sh test

# Linux
bash setups/setup-linux.sh test

# macOS
bash setups/setup-darwin.sh test
```

The `test` action stops on failure and runs Go formatting/module checks, vet, unit tests with coverage, golangci-lint, govulncheck, race tests, parser fuzz smoke, frontend lint/typecheck/build, Playwright workflows/accessibility, dormant workflow checks and Markdown lint.

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
bash setups/setup-windows.sh build
bash setups/setup-linux.sh build
bash setups/setup-darwin.sh build
```

Outputs are in `build/bin/`. The Windows desktop executable is `hacker-tools-desktop.exe`; the CLI is `hacker-tools.exe`. These are unsigned development artifacts.

For development with automatic frontend/backend rebuilds, use your script with `dev`:

```bash
bash setups/setup-windows.sh dev
# Linux: bash setups/setup-linux.sh dev
# macOS: bash setups/setup-darwin.sh dev
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
bash setups/setup-windows-docker.sh test

# Linux
bash setups/setup-linux-docker.sh test

# macOS
bash setups/setup-darwin-docker.sh test
```

The first run downloads base images and builds the CLI, fixture and test images. A private network contains health-checked HTTP/TCP/DNS services. Tests assert HTTP status, TCP inventory, DNS answers and NXDOMAIN. Runtime and test services use a non-root account; no host ports are published.

The test script returns a nonzero exit code on failure and cleans up its lab containers/network on exit. It uses the Compose project name `hacker-tools-lab`.

All three Docker scripts accept the same actions:

```bash
# Replace windows with linux or darwin for those hosts.
bash setups/setup-windows-docker.sh build    # Build images only
bash setups/setup-windows-docker.sh doctor   # Check daemon/Compose/configuration
bash setups/setup-windows-docker.sh down     # Manual lab cleanup
bash setups/setup-windows-docker.sh help
```

Docker builds Linux CLI images. It does not create native Windows/macOS desktop installers or replace native acceptance tests.

## 6. Try the CLI

```bash
go run ./cmd/hacker-tools version
go run ./cmd/hacker-tools doctor
go run ./cmd/hacker-tools interfaces
go run ./cmd/hacker-tools diagnose --kind dns --target example.com
go run ./cmd/hacker-tools diagnose --kind tcp --target localhost --port 8080
go run ./cmd/hacker-tools diagnose --kind http --target http://localhost:8080/health
go run ./cmd/hacker-tools discover --target 127.0.0.1/32 --ports 8080,9090
```

Start a listening local service for the TCP/HTTP examples; otherwise a refused connection is an expected diagnostic failure. JSON includes `schema_version: 1`. Exit codes: 0 success, 1 operation failure, 2 invalid input. Discovery accepts IPv4 /24 or narrower and up to 32 unique ports.

## Troubleshooting

| Symptom | Action |
| --- | --- |
| Go, Node, npm or GCC not found | Install the prerequisite, add it to PATH and reopen the terminal. |
| Missing WebView/GTK/WebKit | Install the OS prerequisites above, then run `doctor`. |
| `-race` needs CGO/C compiler | Install GCC on Windows/Linux or Xcode tools on macOS. |
| Docker daemon unavailable/denied | Start the engine; check context, Linux-container mode and daemon access. |
| Bun reports `EBADF` in a sandbox | Use the committed npm lockfile and `npm ci`; frozen Bun parity remains pending. |
| Browser test port is occupied | Stop the previous development/test server and retry. |
| Wi-Fi/capture denied or unsupported | Read the capability response; permissions and hardware need separate native acceptance. |

Windows raw capture is limited to IPv4, requires administrator authorization, reports no reliable drop count, and is bounded to 60 seconds/10000 packets. Revealed Wi-Fi keys are never written to ordinary history. See [capture boundaries](docs/adr/002-capture-backend.md).

## Guides

- [Delivery plan](PLAN.md)
- [Development and native acceptance](docs/development.md)
- [Dependency inventory](docs/dependencies.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)
- [Verification evidence](docs/verification/0.1.0-alpha.1-dev.md)

Impeccable, Karpathy guidelines and Ponytail are installed in `.agents/skills`, with pinned upstream revisions. CI source remains inactive under `docs/ci/*.yml.disabled`. Project license, signing and stable distribution decisions remain open.
