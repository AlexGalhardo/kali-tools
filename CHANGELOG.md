# Changelog

Changes follow [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Routed all desktop and CLI operations through one `app.Application` with private dependencies, context propagation and shared capture/export bounds; removed direct adapter calls from driving roots. Native acceptance remains open. ([4937c09](https://github.com/AlexGalhardo/kali-tools/commit/4937c09), [H04 evidence](docs/verification/specs/H04.md))
- Declared consumer-side storage, host, tool, Wi-Fi and capture ports; bootstrap owns adapter cleanup and the core no longer imports driven adapters. Generated model namespaces now follow application-owned types with unchanged JSON. ([18c178f](https://github.com/AlexGalhardo/kali-tools/commit/18c178f), [H03 evidence](docs/verification/specs/H03.md))
- Extracted bounded job lifecycles and ordered subscriptions into `internal/job`; application services now own metrics and capture streams, with unchanged Wails payloads plus job sequence numbers. Native desktop acceptance remains open. ([772e3d1](https://github.com/AlexGalhardo/kali-tools/commit/772e3d1), [H02 evidence](docs/verification/specs/H02.md))
- Shared desktop and CLI composition through `internal/bootstrap`, preserving CLI in-memory storage and existing bindings; added retention, cleanup and root-boundary tests. Native bridge and Docker lab acceptance remain open. ([13c0a01](https://github.com/AlexGalhardo/kali-tools/commit/13c0a0130e76585f866bf900c503a029601190ba), [H01 evidence](docs/verification/specs/H01.md))

## [0.1.0-alpha.2-dev] - 2026-10-21

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

### Known limitations

- Web server and complete CLI are specified but not implemented.
- `go test -race` was not run locally (no cgo compiler on the host); GitHub Actions is the authoritative check for this checkpoint.
- Native Windows Wi-Fi/key and capture qualification, hot-plug acceptance and performance budgets remain open.

## [0.1.0-alpha.1-dev] - 2026-10-19

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

### Known limitations

- Native Windows Wi-Fi/key and administrator-authorized capture qualification, hot-plug acceptance and performance budgets remain open.
- Raw Windows capture is IPv4-only; kernel drop counts are unavailable. Display filters do not restrict captured/saved data.
- Linux/macOS Wi-Fi, capture and full platform adapters remain incomplete, even when their desktop binaries build.
- Nmap is not installed in the development host; automated adapter tests use executable fixtures. TShark is inventory-only.
- No qualified alpha/stable release, trusted code signature or notarization is claimed.
