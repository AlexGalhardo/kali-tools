# Changelog

Changes follow [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Passive-neighbor domain types and deterministic, bounded normalization with broadcast filtering and duplicate removal; A01 adapters and interface exposure are in progress ([evidence](docs/verification/specs/A01.md)).
- Bounded passive-neighbor netlink/procfs parsers with state mapping, strict framing checks and fuzz verification; OS collection follows in A01.
- Read-only Windows neighbor-cache collection and Linux netlink/procfs glue, with explicit unsupported fallback elsewhere; public neighbor interfaces follow in A01.
- Passive neighbor inventory through Wails, authenticated HTTP and CLI JSON/CSV, using one bounded family-filtered use case without history persistence (A01).
- English/PT-BR neighbor tables on Interfaces with explicit refresh, family filtering, paging and accessible horizontal scrolling; Linux lab coverage is implemented but Docker/native qualification remains pending (A01).

- Backend-aware capture requests and loss-limit result fields, with bounded validation for raw sockets, Npcap and Pktmon; optional backends are being implemented under A06 ([evidence](docs/verification/specs/A06.md)).
- Capture backend selection checks availability before starting, and capture history stores only backend, state and packet/drop counts (A06).
- Read-only Npcap detection and an opt-in Windows `pcap` capture adapter, with bounded configuration and no driver redistribution; native qualification remains pending (A06).
- Optional Pktmon backend with bounded circular capture, cancellation-safe session cleanup, transient conversion files and explicit capture/loss limits (A06; native qualification pending).
- Capture backend status in Wails and authenticated HTTP, `--backend` selection in the CLI and a fixed-address Npcap download action in desktop builds (A06).
- Packet-inspector backend selection, English/PT-BR loss disclosures and drop counts, with accessible keyboard scrolling for the packet table; ADR 005 records pending native capture measurements (A06).

- Run the unchanged diagnostic, discovery and history acceptance scenarios through the authenticated production HTTP server alongside CLI, Wails and direct application drivers (W01; [evidence](docs/verification/specs/W01.md), [commit](https://github.com/AlexGalhardo/kali-tools/commit/e0fce6f)).
- Verify web concurrency caps, HTTP header/body boundaries, deadline reset, SSE timing, forced shutdown and the browser's 64-job recovery bound (W01/W04; [evidence](docs/verification/specs/W04.md), [commit](https://github.com/AlexGalhardo/kali-tools/commit/19bf576)).
- Wi-Fi consent guidance for Windows 11 24H2+, observed capability states, a fixed-URI desktop settings action and English/PT-BR explicit-retry notices in desktop/browser modes (A05; [evidence](docs/verification/specs/A05.md), [commit](https://github.com/AlexGalhardo/kali-tools/commit/b7d6aa1)).

### Fixed

- Discard Wi-Fi metadata and saved keys when a provider returns an error, preventing their inclusion in failed CLI responses (A05; synthetic regression tests, [commit](https://github.com/AlexGalhardo/kali-tools/commit/b7d6aa1)).

## [0.5.1] - 2026-10-21

### Fixed

- Make serve tests independent of compiled frontend assets and isolate configuration paths on every platform, fixing clean-checkout quality checks and the non-root Docker laboratory.
- Match HTTP capability-test expectations to translated labels on platforms where Wi-Fi/capture are unsupported.

## [0.5.0] - 2026-10-22

### Added

- Browser mode with shared embedded assets, authenticated HTTP routes, SSE events, streamed capture imports and binary exports.
- Per-launch token exchange, HttpOnly sessions, Host and origin checks, strict CSP, bounded requests and explicit TLS-only remote opt-in.
- HTTP browser workflows and security regression tests.

### Changed

- CLI output is human-readable by default; scripts must pass `--json`. English and PT-BR output and stable JSON contracts are verified.
- CLI settings/history use the shared persistent SQLite composition; long operations report bounded progress and support cancellation.
- Frontend pages now own feature state behind a transport-neutral bridge.

### Fixed

- Invalid diagnostic requests no longer reflect credential-bearing targets in partial results.

### Changed

- W01 step 1 centralizes the frontend build in one Go embed package shared by desktop and future HTTP delivery. No web listener is added in this step. [W01 evidence](docs/verification/specs/W01.md).

- Completed the CLI human-output migration, localized result labels, bounded job progress, persistent shared settings/history and explicit JSON contract fixtures. See the [CLI contract](docs/cli.md) and [C01 evidence](docs/verification/specs/C01.md).
- Migration from 0.1.0-alpha.1-dev: scripts must add `--json`; flags are command-specific; error codes have distinct exits; help exits 0; rejected flags never echo values; diagnostics are jobs with redacted history; doctor documents confirmed CLI key reveal. Deprecated invocations retain one warning. Rejected diagnostic targets no longer reflect credentials into partial results.

- CLI diagnostics now run through the shared job manager and retain redacted history. Interrupts cancel the job, live capture cancellation waits at most three seconds and never exports a cancelled result, and the CLI handles SIGTERM while restoring default handling for a second interrupt. [C01 evidence](docs/verification/specs/C01.md).

## [0.4.0] - 2026-10-21

Unsigned development checkpoint published at the owner's request. C01 remains partial; JSON is still the default and CLI history/settings are process-local. Full release qualification remains open.

### Added

- Added CLI capabilities/tools, job and history queries, settings updates, confirmed Wi-Fi reveal and canonical capture conversion commands. Per-command help and strict flags reject unrelated options without echoing rejected values. [C01 evidence](docs/verification/specs/C01.md).

### Deprecated

- Legacy `diagnose --kind`, `wifi`, `capture`, `import` and explicit discovery JSON-format forms remain accepted with one warning; prefer the canonical subcommands and `--json`.

### Changed

- C01 step 1 adds documented exit statuses for unsupported (3), permission (4), missing dependency (5), not found (6), timeout (7), conflict (8), authentication (9), and cancellation (130). Success, generic failure and invalid input remain 0/1/2. The complete CLI migration is still in progress. [C01 evidence](docs/verification/specs/C01.md).

- Enforced the architecture dependency matrix with eight depguard rules, deliberate-violation checks and CI wiring; documented the stdlib CLI parser decision and revisit triggers. [H08/H09 evidence](docs/verification/specs/H08.md).

- Applied a shared black/neon-green terminal theme with bundled JetBrains Mono, responsive navigation and accessible SVG actions. [Theme evidence](docs/verification/specs/A08-theme.md).

- Corrected native CI assumptions for macOS config paths and unsupported inventory, and report an inactive Windows WLAN service as unsupported. [S09 evidence](docs/verification/specs/S09.md).

- Corrected H07 capture contract selection to use an active interface; Wi-Fi, ICMP, host inventory and elevated live capture now pass with real data on Windows build 26300. [Evidence](docs/verification/specs/H07.md#native-capture-correction-2026-10-21).

## [0.3.0] - 2026-10-21

Published at the owner's explicit request with unsigned development binaries and open native qualification gates. Earlier development release entries are now presented as 0.1.0 and 0.2.0; their source commits and artifact contents are preserved.

### Changed

- Resumed H07 with real ICMP contract coverage and host validation before DNS/native delegation. Reconciled PLAN implementation, native acceptance and next-spec checklists; corrected the H06/H07 spec statuses. ([bac0210](https://github.com/AlexGalhardo/kali-tools/commit/bac0210), [H07 continuation evidence](docs/verification/specs/H07.md#continuation-2026-10-21))

- Added shared port suites, contract-tested in-memory fakes and direct/CLI/headless Wails acceptance drivers. The H07 checkpoint keeps the discovered ICMP validation gap and native data-branch qualification open. ([5b15aac](https://github.com/AlexGalhardo/kali-tools/commit/5b15aac), [H07 evidence](docs/verification/specs/H07.md))

- Separated shared transport DTOs from application models, centralized bounded safe error mapping and added bilingual location-consent labels; CLI golden output remains unchanged. Native IPC acceptance remains open. ([cf5f4dc](https://github.com/AlexGalhardo/kali-tools/commit/cf5f4dc), [H06 evidence](docs/verification/specs/H06.md))

- Extracted CLI parsing/output and Wails bindings into dedicated driving adapters; roots now only compose and clean up. The bridge and headless workflows use `wailsui.Bindings`, with unchanged pre-extraction CLI fixture bytes and job sequence fields. ([5954c1a](https://github.com/AlexGalhardo/kali-tools/commit/5954c1a), [H05 evidence](docs/verification/specs/H05.md))
- Routed all desktop and CLI operations through one `app.Application` with private dependencies, context propagation and shared capture/export bounds; removed direct adapter calls from driving roots. Native acceptance remains open. ([4937c09](https://github.com/AlexGalhardo/kali-tools/commit/4937c09), [H04 evidence](docs/verification/specs/H04.md))
- Declared consumer-side storage, host, tool, Wi-Fi and capture ports; bootstrap owns adapter cleanup and the core no longer imports driven adapters. Generated model namespaces now follow application-owned types with unchanged JSON. ([18c178f](https://github.com/AlexGalhardo/kali-tools/commit/18c178f), [H03 evidence](docs/verification/specs/H03.md))
- Extracted bounded job lifecycles and ordered subscriptions into `internal/job`; application services now own metrics and capture streams, with unchanged Wails payloads plus job sequence numbers. Native desktop acceptance remains open. ([772e3d1](https://github.com/AlexGalhardo/kali-tools/commit/772e3d1), [H02 evidence](docs/verification/specs/H02.md))
- Shared desktop and CLI composition through `internal/bootstrap`, preserving CLI in-memory storage and existing bindings; added retention, cleanup and root-boundary tests. Native bridge and Docker lab acceptance remain open. ([13c0a01](https://github.com/AlexGalhardo/kali-tools/commit/13c0a0130e76585f866bf900c503a029601190ba), [H01 evidence](docs/verification/specs/H01.md))

## [0.2.0] - 2026-10-21

Development checkpoint focused on architecture, hardening and contributor guidance. Release assets are unsigned development previews; qualification gates from 0.1.0-alpha.1-dev remain open.

### Added

- Clean/Hexagonal architecture research, binding architecture reference with known deviations, and deepened tools research covering Nmap NPSL, Npcap redistribution, Windows 24H2 Wi-Fi location consent and Pktmon. ([259b414](https://github.com/AlexGalhardo/kali-tools/commit/259b414))
- `book-to-skill` and the `clean-architecture-in-practice` agent skill generated from the owner's reference book. ([259b414](https://github.com/AlexGalhardo/kali-tools/commit/259b414))
- Spec Driven Development guide with a 12-section template, global quality gates, per-spec delivery assertions and auditable evidence records; every open PLAN task links its spec. ([e465532](https://github.com/AlexGalhardo/kali-tools/commit/e465532))
- `CLAUDE.md` and mandatory start-of-task skills (karpathy-guidelines, ponytail, clean-architecture-in-practice) in `AGENTS.md`. ([e465532](https://github.com/AlexGalhardo/kali-tools/commit/e465532))
- Go agent skills adapted from AlexGalhardo/vukem for this architecture. ([28dcdcd](https://github.com/AlexGalhardo/kali-tools/commit/28dcdcd))
- Implementation specs H01–H08 (hexagonal migration), C01 (complete CLI), W01 (web server) and W04 (web security). ([d152872](https://github.com/AlexGalhardo/kali-tools/commit/d152872))
- Specs A01, A05, A06 and B08–B11 for Windows acceptance and toolkit expansion. ([550d291](https://github.com/AlexGalhardo/kali-tools/commit/550d291))
- Lefthook pre-commit that formats Go with gofmt -s and goimports, re-stages files, then runs golangci-lint and go vet; pre-push checks the formatting diff. ([eb7b060](https://github.com/AlexGalhardo/kali-tools/commit/eb7b060))

### Changed

- Reorganized the repository into a hexagonal layout: `internal/adapter/{storage,tools,probe,platform}`, frontend `app/`, `shared/`, `styles/`, `features/`, and `scripts/setup/`. ([8cbf25e](https://github.com/AlexGalhardo/kali-tools/commit/8cbf25e))
- Reformatted Go sources into stdlib, third-party and module import groups. ([eb7b060](https://github.com/AlexGalhardo/kali-tools/commit/eb7b060))
- Native ICMP is injected into network diagnostics through the consumer-side `network.Prober` port, removing the network-to-adapter dependency. ([4dd28a3](https://github.com/AlexGalhardo/kali-tools/commit/4dd28a3))
- Bumped the development version to 0.1.0-alpha.2-dev.

### Fixed

- Updated Go to 1.27.3 for GO-2026-6809 (crypto/x509 verification reachable through the TLS diagnostic). ([bbaf1e8](https://github.com/AlexGalhardo/kali-tools/commit/bbaf1e8))
- Agent rules Markdown lint. ([7af742f](https://github.com/AlexGalhardo/kali-tools/commit/7af742f))
- Terminal job events are no longer dropped when the event buffer is full, so the UI cannot remain in "running"; service shutdown is idempotent. ([4dd28a3](https://github.com/AlexGalhardo/kali-tools/commit/4dd28a3))
- 64-bit Windows ICMP option structure layout; Linux inventory no longer returns `null` gateway lists; DNS names with underscores are accepted. ([4dd28a3](https://github.com/AlexGalhardo/kali-tools/commit/4dd28a3))

Known limitations remain open:

- Web server and complete CLI are specified but not implemented.
- `go test -race` was not run locally (no cgo compiler on the host); GitHub Actions is the authoritative check for this checkpoint.
- Native Windows Wi-Fi/key and capture qualification, hot-plug acceptance and performance budgets remain open.

## [0.1.0] - 2026-10-19

Development checkpoint; qualification remains open. Release assets are unsigned development previews.

### Added

- Go services, versioned CLI, Wails desktop and English/PT-BR interface.
- Interface/route/DNS inventory, traffic counters, scoped TCP discovery and local history/preferences.
- DNS A/AAAA/CNAME/MX/NS/TXT/PTR queries, HTTP/TLS checks and Windows IPv4 ICMP diagnostics.
- Windows WLAN metadata and explicit, automatically cleared current-network key display.
- Bounded PCAP/PCAPNG import, protocol summaries and explicit raw capture export in both formats.
- Optional Nmap service inspection with version/scope validation, bounded output and process-tree cancellation.
- Private Docker fixtures, parser fuzzing, race tests, browser accessibility/workflow tests and native Wails bridge smoke.
- GitHub Actions checks and native Windows/Linux/macOS Intel/Apple Silicon packages with notices, provenance and SHA-256 checksums.
- MIT license, contribution/security guides and dependency notices.

### Changed

- Renamed the project and CLI to `kali-tools`; existing development data remains readable. ([#8435a1f](https://github.com/AlexGalhardo/kali-tools/commit/8435a1f))
- Updated Go to 1.27.2 and patched x/net, x/crypto and x/text dependencies.
- Added a frozen Bun lockfile alongside the npm fallback.
- Paginated packet/discovery tables and translated status/permission failures.

### Fixed

- Desktop initialization/shutdown races and late Wi-Fi secret reveal after blur/navigation. ([c902560](https://github.com/AlexGalhardo/kali-tools/commit/c902560))
- Unbounded subprocess output through promoted buffer methods; cancellation now stops descendants. ([b497003](https://github.com/AlexGalhardo/kali-tools/commit/b497003))
- Corrupt/oversized stored preferences and discovery history privacy. ([5a89c71](https://github.com/AlexGalhardo/kali-tools/commit/5a89c71))
- Capture save failure preserves the destination; packet timestamps retain nanosecond precision. ([c005e7e](https://github.com/AlexGalhardo/kali-tools/commit/c005e7e))

Known limitations remain open:

- Native Windows Wi-Fi/key and administrator-authorized capture qualification, hot-plug acceptance and performance budgets remain open.
- Raw Windows capture is IPv4-only; kernel drop counts are unavailable. Display filters do not restrict captured/saved data.
- Linux/macOS Wi-Fi, capture and full platform adapters remain incomplete, even when their desktop binaries build.
- Nmap is not installed in the development host; automated adapter tests use executable fixtures. TShark is inventory-only.
- No qualified alpha/stable release, trusted code signature or notarization is claimed.
