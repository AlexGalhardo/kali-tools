# kali-tools

kali-tools is a local network and security toolkit for personal study and learning, for use on your own systems and lab. It is not a Kali Linux tool orchestrator and not a penetration-testing product; the name only nods to the Kali ecosystem it learns from ([ADR 014](docs/adr/014-product-identity.md)). One Go core serves three interfaces: a native desktop app, a local web server with the same UI, and a CLI for headless or SSH hosts. The interface supports English and Brazilian Portuguese.

**Development preview:** core workflows are implemented and tested; hardware qualification is still open. Windows 11 leads development. Linux/macOS binaries do not imply Wi-Fi or capture parity. Native SSH acceptance is still open. The frozen surface is the [CLI contract](docs/cli.md), the [HTTP contract](docs/http.md), schema version 1, settings `language` and `retention_days`, and the [packaging matrix](docs/packaging.md). See [PLAN.md](PLAN.md) and the [verification evidence](docs/verification/0.1.0-alpha.2-dev.md).

<!-- TODO: add screenshots of the desktop app once captured; the repository has none yet. -->

## What it does

- Lists interfaces, addresses, gateways and DNS; monitors native traffic counters.
- Runs DNS record queries, bounded TCP/HTTP/TLS checks and Windows IPv4 ping/traceroute. An HTTP check reports phase timings and security headers. It traces at most five redirects only when asked, and never follows one on its own.
- Discovers explicitly selected IPv4 scopes and identifies services with built-in TCP, banner and TLS checks.
- Audits a TLS endpoint: certificate chain, protocols and lint findings.
- Inspects Windows Wi-Fi metadata and reveals the current saved key only on explicit request; the display clears automatically.
- Imports PCAP/PCAPNG, summarizes packets and explicitly saves retained packet data in either format.
- Keeps preferences and redacted operation history locally in SQLite. No cloud runtime or telemetry is required.

## Quick start

1. Open the [GitHub releases page](https://github.com/AlexGalhardo/kali-tools/releases) and download the archive for your system, plus `SHA256SUMS`:
   - Windows 11 x64: `windows-x64.zip` (needs the Edge WebView2 Runtime).
   - Ubuntu 24.04 x64: `linux-x64.tar.gz` (needs GTK 3 and WebKitGTK 4.1).
   - macOS 13 or later, Apple Silicon: `macos-arm64.zip`. There is no Intel build.
2. Verify the download against the checksum, as described in [ADR 009](docs/adr/009-signing-and-provenance.md):
   - Windows (PowerShell): `Get-FileHash .\<archive> -Algorithm SHA256`, then compare with the matching line in `SHA256SUMS`.
   - Linux: `sha256sum --check --ignore-missing SHA256SUMS`.
   - macOS: `shasum -a 256 --check --ignore-missing SHA256SUMS`.
   - Optional, with the GitHub CLI: `gh attestation verify <archive> --repo AlexGalhardo/kali-tools` checks the Sigstore build attestation.
3. Extract the archive and run `kali-tools-desktop` for the desktop app, or `kali-tools` for the CLI (`kali-tools --help`). `kali-tools serve` starts the local web server.

Releases are vendor-unsigned: no Apple Developer ID, notarization or Windows Authenticode certificate, so your OS warns on first launch:

- **Windows:** SmartScreen may say the app is unrecognized. Choose **More info**, then **Run anyway**.
- **macOS:** Gatekeeper may block the app. Open **System Settings > Privacy & Security** and choose **Open Anyway**. If it stays blocked, clear the quarantine flag with `xattr -d com.apple.quarantine` on the `.app`. An Apple Silicon build keeps only the ad-hoc signature the Go linker applies.
- **Linux:** no extra first-launch prompt.

Read the release notes before testing; a successful native build does not qualify hardware features. Combinations not in the [packaging matrix](docs/packaging.md) are not supported.

## First five minutes

Use only a network you own, such as a home lab or a VM network. The examples use `192.168.56.0/28`; replace it with your own range.

1. Open the desktop app, or run `kali-tools doctor` to see which capabilities are available on this host and why any are not.
2. Inventory your machine: `kali-tools inventory` shows adapters, gateways, DNS and traffic counters; `kali-tools interfaces` lists the interfaces.
3. Discover your lab: `kali-tools discover --authorized --target 192.168.56.0/28 --ports 22,80,443`. A scope is IPv4 /24 or narrower with at most 32 ports. `--authorized` is your statement that you own or are allowed to scan the target.
4. Audit TLS on a service you run: `kali-tools tls-audit --target <your-host>`. Add `--json` for scripts.
5. Export the results: `kali-tools discover --authorized --target 192.168.56.0/28 --ports 22,80,443 --format csv > lab.csv`, or add `--json` to any command and redirect it. Saved packet data is exported with `kali-tools capture export --file in.pcap --output out.pcapng --format pcapng`.

The full command list, JSON envelope and exit codes are in the [CLI contract](docs/cli.md).

## Responsible use

Use kali-tools only on systems you own or have written authorization to test. Active scans and checks send real traffic to the target, so run them on your own lab or with permission.

## What it does not do

- No exploitation frameworks or exploit probes.
- No credential attacks, password guessing or brute forcing.
- No Wi-Fi injection, monitor-mode attacks or evasion. Wi-Fi support reads metadata of the network you are connected to and reveals only a key your own computer has saved.
- No Nmap: service inspection is native TCP, banner, TLS and one HTTP request ([ADR 006](docs/adr/006-native-service-inspection.md)). TShark is optional and separately installed.

See the scope boundary in [PLAN.md](PLAN.md).

## Build from source

Contributors build with one of exactly six shell scripts. Run them in Git Bash on Windows, or Terminal on Linux/macOS, from the repository root. Prerequisites, test actions, Docker lab, pinned tool versions and troubleshooting are in [docs/development.md](docs/development.md).

| System | Without Docker: native desktop + CLI | With Docker: isolated CLI lab |
| --- | --- | --- |
| Windows 11 | `scripts/setup/setup-windows.sh` | `scripts/setup/setup-windows-docker.sh` |
| Linux | `scripts/setup/setup-linux.sh` | `scripts/setup/setup-linux-docker.sh` |
| macOS | `scripts/setup/setup-darwin.sh` | `scripts/setup/setup-darwin-docker.sh` |

Docker tests the headless CLI/core in a private laboratory. It does not verify host Wi-Fi, Keychain, WebViews or capture authorization.

## Guides

- [Delivery plan](PLAN.md)
- [Architecture (hexagonal, binding reference)](docs/architecture.md) and [architecture research](docs/architecture-research.md)
- [Spec Driven Development specs](docs/specs/README.md): what to implement next, with auditable quality gates
- [Tools, licenses and platform research](docs/tools-research.md)
- [Development and native acceptance](docs/development.md)
- [Dependency inventory](docs/dependencies.md)
- [Packaging matrix](docs/packaging.md)
- [Release qualification review](docs/review/README.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)
- [Verification evidence](docs/verification/0.1.0-alpha.2-dev.md) ([previous](docs/verification/0.1.0-alpha.1-dev.md))

## License and contribution

Project code is licensed under [MIT](LICENSE.md). Bundled dependencies and vendored development skills retain their own licenses; see [the inventory](docs/dependencies.md). Nmap is not used. TShark is an optional separate installation and is not redistributed.

Contributions follow [CONTRIBUTING.md](CONTRIBUTING.md). Report bugs through [GitHub issues](https://github.com/AlexGalhardo/kali-tools/issues) and security concerns through [SECURITY.md](SECURITY.md). Do not attach passwords, tokens or private packet contents.

CI/CD is active under `.github/workflows/ci.yml`. It checks code, vulnerabilities, browser accessibility/workflows, Docker integration and native packaging. Only main-branch jobs can prepare release drafts. See [CI/CD details](docs/ci/README.md).

New installations use the per-user `kali-tools/history.db` configuration path. Existing development data under the legacy `hacker-tools` path remains readable without moving an open database. Source and CLI paths changed to `cmd/kali-tools`; the working directory itself need not be renamed.

Impeccable, Karpathy guidelines and Ponytail remain under `.agents/skills` with pinned source revisions. Signing, hardware qualification and stable distribution remain separate release gates.

## Use it in a browser

Run `kali-tools serve` and open the printed URL. The default listener is `127.0.0.1` on a free port. The URL fragment contains a per-launch token; the browser exchanges it for an HttpOnly session cookie and removes the fragment. Never share that URL. Stop the server with Ctrl-C.

Remote access requires a specific address, `--allow-remote`, `--tls-cert` and `--tls-key`. Anyone holding the token can run diagnostics, discovery and capture and reveal the saved Wi-Fi key as the server user. Capabilities reflect that user's permissions and consent. See [web security](SECURITY.md) and [release limits](docs/releases/0.5.0.md).
