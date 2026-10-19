# Changelog

Changes follow [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

- Renamed the project and CLI to `kali-tools`; existing development data remains readable.
- Updated Go to 1.27.2 and patched x/net, x/crypto and x/text dependencies.
- Added a frozen Bun lockfile alongside the npm fallback.
- Paginated packet/discovery tables and translated status/permission failures.

### Fixed

- Desktop initialization/shutdown races and late Wi-Fi secret reveal after blur/navigation.
- Unbounded subprocess output through promoted buffer methods; cancellation now stops descendants.
- Corrupt/oversized stored preferences and discovery history privacy.
- Capture save failure preserves the destination; packet timestamps retain nanosecond precision.

### Known limitations

- Native Windows Wi-Fi/key and administrator-authorized capture qualification, hot-plug acceptance and performance budgets remain open.
- Raw Windows capture is IPv4-only; kernel drop counts are unavailable. Display filters do not restrict captured/saved data.
- Linux/macOS Wi-Fi, capture and full platform adapters remain incomplete, even when their desktop binaries build.
- Nmap is not installed in the development host; automated adapter tests use executable fixtures. TShark is inventory-only.
- No qualified alpha/stable release, trusted code signature or notarization is claimed.
