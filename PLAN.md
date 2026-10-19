# kali-tools — delivery plan

Planning baseline: 2026-10-19. Product owner decisions are recorded in [brainstorming](docs/brainstorming.md); technical sources are in [research](docs/tools-research.md).

## 1. Product contract

Build a local desktop toolkit with Go, Wails, React, TypeScript, Base UI and Tailwind CSS v4. Windows 11 leads development; Linux and macOS adapters must remain separate from the domain. Implement as much as practical in Go. Integrate optional Nmap/TShark executables where a native implementation would be incomplete or unreasonable. SQLite stores local preferences and diagnostic history. Interface languages: English and PT-BR. Code, documentation and commits: English.

The alpha includes Wi-Fi password display and live packet capture, as well as addresses, interfaces, passive traffic, diagnostics and discovery. These are real implementation requirements, not placeholder panels. Alpha acceptance is Windows-first; beta and 1.0 broaden native verification to Linux and macOS. Capability limitations must be visible to users.

The project has no commercial purpose. Select dependencies with source available under suitable open-source licenses and no mandatory paid runtime service. Do not infer that every open-source license forbids commercial use. The owner delegated open-source housekeeping on 2026-10-19; project code uses MIT in LICENSE.md. No noncommercial restriction is added. Keep Nmap/TShark optional and separately installed until redistribution is reviewed. Windows capture-driver selection is a feasibility gate, because a free driver is not automatically an eligible open-source dependency.

## 2. Scope and architecture

```text
React UI -> thin Wails bindings -> application services <- headless CLI
                                  | jobs, events, capability registry
                                  | network diagnostics and packet domain
                                  | platform adapters / SQLite / tool adapters
                                  v
                           native OS APIs and optional executables
```

```text
main.go                         Wails composition root
app.go                          narrow desktop facade
cmd/kali-tools/                CLI composition root
internal/app/                   shared use cases and job lifecycle
internal/network/               interfaces, diagnostics, metrics
internal/wifi/                  current-network metadata and explicit secret retrieval
internal/capture/               capture sessions, files, decoding
internal/platform/              windows/linux/darwin adapters
internal/tools/                 Nmap and TShark discovery, execution, parsing
internal/storage/               SQLite, migrations, retention
frontend/src/                   React features, shared components, translations
docs/adr/                       architecture decisions
.github/workflows/              authorized active CI/CD
docs/ci/                        CI/CD operating guide
tests/fixtures/                 synthetic PCAP and tool outputs
tests/integration/              CLI integration tests
docker/                         deterministic lab services
```

Rules: no UI business logic; no shell command concatenation; bounded queues; explicit deadlines; cancellation frees every resource; typed domain errors; stable job IDs and event schema. Generated Wails bindings are regenerated and checked for drift. UI never invokes arbitrary executables or SQL. Native platform files use Go build constraints.

Capability states: available, dependency missing, permission required, unsupported, failed. Jobs: queued, running, completed, failed, cancelled. Include timestamps, operation parameters, tool/provider version and summary; exclude secrets and payloads from ordinary logs/history.

SQLite proposed schema: settings, jobs, diagnostic_results, tool_inventory and schema_migrations. Use explicit migrations, transactions, parameterized SQL and a single coordinated writer. Choose the driver after checking maintenance, license and native-build compatibility. Packet files are separately managed opt-in artifacts; no automatic payload database. Apply retention and export controls; redact secrets. Do not claim guaranteed zeroization of Go strings.

## 3. Completion and version rules

- `[x]` means implemented and verified with recorded evidence; `[ ]` means pending. A parent completes only after all its children and assertions pass.
- Each task below contains an assertion and verification method. Store evidence in `docs/verification/<version>.md`, with OS, commands, date and limitations.
- SemVer sequence: structure before release; `0.1.0-alpha.N`; `0.1.0-beta.N`; `1.0.0-rc.N`; `1.0.0`. No tag is created merely because a checklist exists.
- Define the public CLI, result/export schemas and supported configuration contract before 1.0. Stable breaking changes require a major version; pre-1.0 breaking changes need explicit migration notes.
- Conventional Commits feed git-cliff and Keep a Changelog. Review generated changelog and version consistency before a release. Never include tokens, saved Wi-Fi keys or real capture data in commits.
- GitHub Actions activation was explicitly authorized on 2026-10-19. Active pinned source is under `.github/workflows/`; pull requests receive read-only permissions and only the gated main-branch draft-release job can write releases.

## 4. Structure phase

- [x] S01 Research and brainstorming baseline.
  - [x] Read shared conversation and inspect reference repository snapshot.
  - [x] Record sources, architecture tradeoffs and owner answers.
  - Assert: both documents distinguish confirmed decisions from proposals. Verify: document review.
- [x] S02 Initialize local Git and configure the requested origin.
  - Assert: origin resolves to the owner's kali-tools URL. Verify: `git remote -v`.
- [x] S03 Establish GitHub access and push the planning baseline.
  - [x] Reauthenticate GitHub CLI using the owner's browser authorization.
  - [x] Configure scoped Git authentication; inspect remote state before pushing.
  - [x] Confirm remote is empty; push without force and verify remote SHA.
  - Assert: authenticated identity has repository write access and remote HEAD contains this plan.
  - Evidence: authenticated as AlexGalhardo with repository push permission; `main` tracks `origin/main`. GitHub API confirmed planning commit `223c4b06b7dcb764456a179ab349b373f9e54870` after push. Credentials are outside the repository; global Git configuration was not required.
- [x] S04 Reproducible Go and frontend toolchain.
  - [x] Install/verify Go and Wails v2 stable; record exact versions and `wails doctor` output.
  - [x] Create Go module and actual Wails React/TypeScript shell; pin versions after registry verification.
  - [x] Configure Bun, committed lockfile, Vite, Base UI, Tailwind v4 and translation catalogs. Committed in c902560; a clean source archive installed frozen Bun dependencies, built the frontend and passed the actual native race/bridge test.
  - [x] Add Biome, golangci-lint, govulncheck, commitlint, Lefthook and Markdown checks.
  - [x] Supply dev/check/build commands for Windows and Unix hosts.
  - Assert: clean checkout installs frozen dependencies and opens native shell; no placeholder build reports success.
- [x] S05 Repository conventions and developer guides.
  - [x] EditorConfig and LF policy, contribution guide, security policy and agent instructions.
  - [x] Commit-message hook and release/changelog generation configuration.
  - [x] Document capability boundaries and dependency/license inventory.
  - Assert: formatting is consistent and invalid commit messages fail locally once hooks are installed. Evidence: installed pre-commit/pre-push/commit-msg hooks; actual commit-msg rejects invalid input and accepts a Conventional Commit message on 2026-10-19.
- [x] S06 CLI and core contracts.
  - [x] Add version, doctor and interfaces commands; JSON mode with schema version.
  - [x] Define errors, job states, cancellation, clock abstraction and bounded event stream.
  - [x] Share services with Wails; return a clear unsupported response for absent providers.
  - Assert: CLI and UI get equivalent interface data; malformed input produces a stable nonzero exit code.
- [x] S07 SQLite foundation.
  - [x] Compare eligible drivers, document decision and pin dependency.
  - [x] Create migration runner, schema and per-user application data path.
  - [x] Test empty install, upgrade, failed migration rollback, concurrent reads and retention.
  - Assert: restart preserves preferences/history; failed writes do not leave partial jobs; secrets never enter DB.
- [x] S08 Docker CLI laboratory.
  - [x] Multi-stage CLI build and non-root runtime; separate test stage with required test toolchain.
  - [x] Private Compose lab with health-checked HTTP/DNS/TCP fixture services.
  - [x] Deterministic integration assertions; cleanup and bounded startup time.
  - [x] Document that host Wi-Fi, elevation, capture drivers and WebViews require native tests.
  - Assert: one documented command fails on wrong results and returns zero only after assertions pass.
- [ ] S09 CI/CD implementation (activation authorized 2026-10-19).
  - [x] Prepare CI source for formatting/lint/typecheck/build, race tests, vulnerabilities and CLI lab. Active source passes actionlint and immutable-pin checks; execution acceptance is recorded separately.
  - [ ] Prepare native Wails build matrix with per-OS dependencies and capability tests.
  - [ ] Prepare release source for draft artifacts, checksums and source/dependency notices.
  - [x] Pin official Actions release revisions; minimize permissions and use the short-lived GitHub token only in the main-branch release job.
  - Assert: static workflow checks and hosted jobs pass; activation follows explicit owner authorization; only successful main-branch runs assemble development release drafts.

Structure exit: S01–S09 pass; a clean checkout runs the native shell and CLI lab; GitHub push is verified. Documentation alone does not complete this phase.

## 5. Alpha — Windows-first integrated toolkit

- [ ] A01 Interface and route inventory.
  - [ ] IPv4/IPv6, prefix, MAC, interface identity/status, gateway and configured DNS.
  - [ ] Native Windows adapter; contracts and explicit unavailable states for other OSes.
  - [ ] Handle disconnected devices, VPNs, loopback and multiple default routes.
  - Assert: compare against OS data; hot-plug/reconnect does not duplicate stale interfaces.
- [ ] A02 Passive traffic monitoring.
  - [x] Sample native interface counters with monotonic elapsed time.
  - [x] Handle counter reset, suspend/resume and interface replacement.
  - [x] Bounded history and batched UI events; graph units and sample timestamp visible.
  - Assert: controlled fixture traffic yields expected positive rates; reset never produces a negative spike.
- [x] A03 Go diagnostics.
  - [x] DNS A/AAAA/CNAME/MX/NS/TXT/PTR, native Windows IPv4 ping latency/loss and bounded TCP checks; local fixtures and native loopback passed.
  - [x] HTTP response/timing and TLS certificate summary; redirects are reported without being followed.
    - [x] HTTP response/timing, timeout and refusal to follow redirects verified with local fixtures.
    - [x] Trusted TLS summary, untrusted/expired certificate rejection and handshake deadline/cancellation verified with controlled local certificates.
  - [x] TCP success/refusal, HTTP timeout and pre-cancelled diagnostics tested against local fixtures.
  - [x] DNS record coverage and native Windows loopback ping/first-hop traceroute acceptance; IPv6 ICMP is explicitly unsupported.
  - [x] Windows IPv4 ICMP traceroute with typed permission reporting, hop/deadline limits and native loopback first-hop evidence.
  - [x] Discovery paced to 100 probes/second with 32 workers, ICMP paced to 10 probes/second and application jobs capped at eight; shutdown/cancellation and IPv6 TCP fixtures passed. Scoped IPv6 hosts validate; native ICMP explicitly requires IPv4.
  - Assert: local fixtures distinguish success, refusal, timeout, NXDOMAIN and cancellation without goroutine leaks.
- [x] A04 LAN discovery and basic service inventory.
  - [x] Explicit CIDR/host scope, host/port limits and deterministic result ordering.
  - [x] Native TCP discovery first; label inference and unknown states honestly.
  - [x] Store summaries and export versioned JSON/CSV.
  - Assert: lab service inventory matches fixture ground truth; no scan expands beyond requested scope.
- [ ] A05 Wi-Fi inspector and current-network password.
  - [ ] Windows WLAN metadata: current profile, SSID/BSSID, signal/channel when available.
    - [x] Channel query and bounded native-structure decoding implemented; synthetic Windows metadata/layout tests passed.
    - [ ] Controlled native Wi-Fi network acceptance.
  - [ ] Explicit reveal action for the current saved network; native authorization as required.
  - [ ] Distinguish open network, enterprise authentication, no saved key, denied and unsupported.
  - [x] Mask by default; automatically clear displayed secret; no logs, database or default clipboard writes.
  - [x] Shared Linux/macOS provider contracts return explicit unsupported states; native implementation milestones remain B01/B02.
  - Assert: native Windows acceptance verifies reveal and denial with owner's test network; automated tests use synthetic secrets and check redaction.
- [ ] A06 Live packet capture and offline import.
  - [ ] Spike an eligible open-source Windows capture backend; document driver/license/performance tradeoffs.
  - [ ] Prefer Go capture/decoding; optional separate TShark process only for justified gaps.
  - [ ] Start/stop capture, interface selection, limits, dropped-packet counters and bounded buffers.
  - [ ] PCAP/PCAPNG import/export with file-size limits and malformed-file handling.
    - [x] PCAP and PCAPNG import with 64 MiB file, 1 MiB packet and 10,000-packet limits; malformed declared lengths rejected and parser fuzz smoke passed.
    - [x] Explicit raw PCAP/PCAPNG export in desktop/CLI, with bounded retained bytes, nanosecond timestamps and failure-safe replacement; round-trip, writer failure and destination-preservation tests passed.
  - [x] Ethernet/IPv4/IPv6/TCP/UDP/ICMP/DNS and plaintext HTTP summaries in Go where feasible. Synthetic PCAP/PCAPNG, Ethernet/ARP, ICMPv4/v6 and HTTP tests pass. Port 443 alone is explicitly not proof of encryption.
  - [x] UI explicitly identifies display-only filtering and complete retained-data exports; encrypted content remains encrypted and port 443 is not treated as proof of encryption.
  - Assert: synthetic captures decode expected fields; native Windows test captures controlled traffic, stops promptly and leaves no live handle.
  - Gate: resolve capture-driver eligibility before declaring alpha complete; an incompatible dependency is not silently accepted.
- [ ] A07 Optional advanced adapters.
  - [x] ADR 003 justifies Nmap service signatures; native TCP remains default and no TShark execution is introduced.
  - [ ] Tool discovery, supported versions, structured parsers and dependency diagnostics.
    - [x] Bounded optional-executable version inventory and explicit missing-dependency states; core remains usable without installed tools.
    - [x] Bounded Nmap XML parser and fuzz smoke.
    - [x] Nmap 7.80+ within major 7 version validation and explicit CLI/desktop service inspection; TShark 4.2+ within major 4 is inventory-only.
  - [x] Fixed argument arrays, stdout/stderr limits and deadlines; Windows suspended-start Job Object descendant cancellation and Unix process-group execution tested.
  - [x] Executable fixtures cover valid, partial, malformed, wrong-version and out-of-scope/unrequested-port Nmap output.
  - [ ] Qualify the actual separately installed Nmap executable; it is absent from this host.
  - Assert: core works without tools installed; optional results preserve provider/version and cannot execute arbitrary shell input.
- [ ] A08 Integrated UI.
  - [x] Dashboard, interfaces, diagnostics, Wi-Fi, packet inspector, toolkit, history and settings.
  - [ ] PT-BR/English catalogs, keyboard access, focus states and Base UI primitives.
  - [ ] Virtualized packet/result tables, loading/error/permission states and cancel controls.
    - [x] Packet and discovery tables render at most 100 rows per page; discovery pagination verified with 205 synthetic endpoints. Pagination is implemented; virtualization remains pending.
    - [x] Diagnostics cancellation, capture stop and discovery cancellation controls; browser discovery test uses the emitted service job ID.
    - [x] Actual provider capability states, bounded-import notice and metric sample timestamps.
    - [x] Translated states, permission/diagnostic reasons, capture start failure recovery and optional-adapter dependency flows.
    - [ ] Complete all permission/error acceptance and decide whether pagination satisfies the virtualization requirement.
  - [ ] SQLite preferences and history; opt-in capture save and clear-history action.
    - [x] SQLite-backed preferences/history and explicit clear-history action; restart/retention and settings validation tests pass.
    - [x] Native save-dialog integration and explicit PCAP/PCAPNG format selection; browser success/cancellation/failure and Go filesystem tests passed.
  - Assert: UI has real service connections; Playwright tests with contract-faithful providers pass; native smoke verifies the actual Wails bridge.
- [ ] A09 Alpha validation and release candidate.
  - [x] Go unit/integration/race checks; fuzz packet and tool parser boundaries. Windows unit/race, Linux Docker integration and parser fuzz smoke passed; native hardware and release qualification remain open.
  - [x] React typecheck/lint/build, accessibility smoke and critical workflow tests.
  - [ ] Record Windows native evidence for Wi-Fi and capture, with dependency/setup instructions.
  - [ ] Measure idle memory/CPU and capture workload; establish numeric beta regression budgets.
  - Assert: all alpha tasks pass; release notes disclose platform and protocol limits; no enabled CI/CD is introduced.

Alpha exit: addresses, passive monitoring, diagnostics, discovery, Wi-Fi key display and live capture work on supported Windows setup. Missing Linux/macOS implementations are explicit; unrelated future capabilities do not block alpha.

## 6. Beta — portability and hardening

- [ ] B01 Linux native adapters.
  - [ ] Select documented distro baseline; implement routes/counters and NetworkManager integration.
  - [ ] Wi-Fi metadata/key retrieval through authorized secret APIs; handle systems without NetworkManager.
  - [ ] Capture and permission provisioning; avoid elevating the full UI.
  - Assert: native evidence covers real adapter, denied permission, restart and no saved PSK.
- [ ] B02 macOS native adapters.
  - [ ] Select supported macOS/architecture baseline and native bridge approach.
  - [ ] Implement interfaces/routes/counters, current Wi-Fi metadata and Keychain feasibility spike.
  - [ ] Capture permissions and key-reveal authorization; explicit unsupported state if OS policy prevents access.
  - Assert: native evidence records actual supported behaviors rather than inferring parity from compilation.
- [ ] B03 Privilege boundary.
  - [ ] Use existing OS authorization where possible; implement helper only for operations that require it.
  - [ ] Authenticated local IPC, restricted operations, caller verification and bounded requests.
  - [ ] Install/update/uninstall and recovery behavior per OS.
  - Assert: unauthorized caller and malformed request fail; ordinary UI remains unprivileged.
- [ ] B04 Connection/process inventory and throughput.
  - [ ] Native per-OS socket-to-process mapping with permission and race handling.
  - [ ] Separate per-interface counters from per-process attribution; no invented bandwidth estimates.
  - [ ] Implement bounded Go throughput client/server for owned endpoints if practical.
  - Assert: fixture processes match reported connections; active throughput has explicit endpoint/duration and cancellation.
- [ ] B05 Capture and adapter resilience.
  - [ ] Reassembly limits, IPv6 extensions, truncated packets and sustained capture backpressure.
  - [ ] TShark broad decoding where Go support is incomplete; document supported protocol levels.
  - [ ] Test interface disappearance, tool termination, disk full and corrupted input.
  - Assert: bounded workload stays within alpha-established budgets and reports data loss accurately.
- [ ] B06 Persistence, privacy and compatibility.
  - [ ] SQLite upgrade/backup/recovery, retention, export and migration fixtures.
  - [ ] Validate settings and versioned result formats; redact diagnostic bundles.
    - [x] Stored preferences reject malformed, oversized or unsupported values without echoing contents.
    - [x] Versioned CLI JSON and discovery JSON/CSV contracts tested; discovery history stores counts without targets.
    - [ ] Redacted diagnostic bundles and compatibility fixtures for all exported formats.
  - [ ] Test translation key parity and all permission/error UI flows.
  - Assert: prior beta data upgrades without silent loss; secret fixtures appear in no ordinary artifact.
- [ ] B07 Native packaging matrix.
  - [ ] Build Windows, Linux and macOS artifacts on native hosts; verify actual WebView prerequisites.
  - [ ] Dependency doctor and install instructions; clean-machine smoke checks.
  - [ ] Dependency/source notices, installer uninstall behavior and writable-data locations.
  - Assert: verified matrix names OS/version/architecture/provider; untested combinations are not advertised.

Beta exit: agreed capability matrix verified on all three OSes; limitations approved and documented; no unresolved data loss, privilege-boundary or critical parser defects.

## 7. Stable 1.0

- [ ] R01 Freeze supported CLI commands, schemas, settings and platform matrix.
  - [ ] Document API compatibility, errors, permission requirements and unavailable capabilities.
  - [x] Adopt MIT for project code, retain third-party/vendored licenses and collect notices for pinned Go/production frontend dependencies; Nmap/TShark remain excluded.
  - Assert: README and doctor agree with tested artifacts; no promise of all Kali tools or all protocol payloads.
- [ ] R02 Release qualification.
  - [ ] Re-run meaningful regression suites after final changes and review evidence.
  - [ ] Test native install, upgrade, rollback/data recovery, uninstall and offline operation.
  - [ ] Validate agreed numeric performance budgets and absence of critical/high unresolved vulnerabilities.
  - [ ] Confirm PT-BR/English and keyboard/accessibility acceptance.
  - Assert: release checklist links reproducible results; no unfinished blocker is marked complete.
- [ ] R03 Distribution and provenance.
  - [ ] Decide signing/notarization credentials and provisioning with the owner.
  - [ ] Produce checksums, dependency inventory, source archive and release notes.
  - [ ] Verify signatures where configured; clearly label unsigned development artifacts.
  - [ ] Create RC, test it, then tag 1.0.0 only after acceptance.
  - Assert: version matches CLI, desktop metadata, tag and changelog; artifacts correspond to tagged source.
- [ ] R04 Optional automation activation.
  - [x] Owner explicitly requested full GitHub Actions activation on 2026-10-19; active source replaced dormant copies.
  - [ ] Configure protected release environment and minimum repository permissions.
  - Assert: activation follows the recorded owner instruction; pull requests cannot create releases and qualified releases retain acceptance gates.

## 8. Verification strategy

Use behavioral tests for packet parsing, rate calculations, state transitions, timeouts, process cancellation, persistence and scoped execution. Avoid tests that merely restate implementation. Native hardware/API checks and container tests are separate evidence categories. Frontend browser tests validate UI flows but do not prove native Wails IPC works.

Required check interface after S04: Go format/lint/vet/test/race/vulnerability checks; Bun lint/typecheck/test/build; parser fuzz smoke; Docker integration; native Wails build/smoke. Before Go/frontend scaffolding exists, these checks must not be reported as passed.

Threat boundaries to verify: untrusted packet bytes, external tool output, user-provided targets/files, Wails calls, local helper IPC and export paths. Test malformed input and resource limits at each boundary. Capture only in an explicitly started session. No cloud account or telemetry required.

## 9. Deferred beyond 1.0

Third-party downloaded plugins, firewall modification, traffic shaping, Wi-Fi monitor mode, credential attacks, exploitation frameworks, cloud synchronization and automatic TLS interception are separate proposals. The current plan focuses on the integrated monitoring/diagnostic/capture scope and does not claim feature parity with every Kali package.

## 10. Current evidence and next task

Checklist review: 2026-10-19. Completed substeps below are separated from remaining acceptance gates. Latest verification: Go tests/vet, capture race tests and 11 frontend workflow tests passed. See [development evidence](docs/verification/0.1.0-alpha.1-dev.md).

- Research baseline committed as `c4bf7fe`.
- The module now requires Go 1.27.2; Wails 2.17.0 builds the native Windows shell. Bun 1.4.3 frozen installation and build now pass with the new workspace bun.lock; npm remains the setup/Wails fallback. See the continuation evidence for the current host and dependency security updates.
- GitHub authorization completed and planning baseline pushed to `main`; use the task-local GitHub CLI configuration outside the repository for subsequent pushes.
- Development implementation checkpoint: real CLI/core, SQLite, Windows providers, React/Wails shell, bounded capture/import prototype, tests, authorized CI/CD source and the private Docker laboratory exist. See docs/verification/0.1.0-alpha.1-dev.md for exact evidence and limitations.
- Six setup scripts and a step-by-step README cover Windows/Linux/macOS, separately with and without Docker. Script syntax is verified; cross-platform native acceptance is still pending.
- Continuation evidence closes S06, S07, S08 and A04 and verifies the runnable S04 toolchain: native WebView2 bridge, desktop build, frozen Bun install, redacted discovery history, JSON/CSV exports and the private Docker CLI lab passed. Details are in docs/verification/0.1.0-alpha.1-dev.md. S04 lockfile commit/clean-checkout acceptance, S05 hook acceptance and S09 full static/release-workflow qualification remain open.
- Next: finish the remaining diagnostics, raw capture export, advanced adapters and full UI error/permission/large-table acceptance; obtain native Windows Wi-Fi/capture acceptance; then complete beta/stable implementation and native qualification. Linux/macOS hardware, privilege architecture, license/signing decisions and workflow activation are not inferred from this checkpoint. Do not mark the plan complete or tag a release.

### Session scope update (2026-10-19)

The owner renamed the final project/repository to kali-tools, requested commits/push/changelog and a GitHub release with native binaries, delegated open-source housekeeping, and explicitly activated CI/CD. Origin is https://github.com/AlexGalhardo/kali-tools. The workspace directory remains unchanged. MIT is adopted; unsigned development artifacts and release drafts disclose remaining qualification gates.

Current implementation includes DNS records/TLS fixtures, native Windows loopback ping/trace, raw PCAP/PCAPNG export, bounded optional Nmap execution, channel metadata, translated error states and 17 browser workflows. Hosted native-build/release acceptance is pending the first pushed workflow run; no hosted success is claimed in advance. Windows Wi-Fi/key/live-capture acceptance, real Nmap qualification, hot-plug/performance budgets, Linux/macOS feature parity and stable qualification remain open.
