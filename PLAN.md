# Hacker Tools — delivery plan

Planning baseline: 2026-10-19. Product owner decisions are recorded in [brainstorming](docs/brainstorming.md); technical sources are in [research](docs/tools-research.md).

## 1. Product contract

Build a local desktop toolkit with Go, Wails, React, TypeScript, Base UI and Tailwind CSS v4. Windows 11 leads development; Linux and macOS adapters must remain separate from the domain. Implement as much as practical in Go. Integrate optional Nmap/TShark executables where a native implementation would be incomplete or unreasonable. SQLite stores local preferences and diagnostic history. Interface languages: English and PT-BR. Code, documentation and commits: English.

The alpha includes Wi-Fi password display and live packet capture, as well as addresses, interfaces, passive traffic, diagnostics and discovery. These are real implementation requirements, not placeholder panels. Alpha acceptance is Windows-first; beta and 1.0 broaden native verification to Linux and macOS. Capability limitations must be visible to users.

The project has no commercial purpose. Select dependencies with source available under suitable open-source licenses and no mandatory paid runtime service. Do not infer that every open-source license forbids commercial use. Project license remains an owner decision: adding a noncommercial restriction would change whether the result qualifies as open source. Keep Nmap/TShark optional and separately installed until redistribution is reviewed. Windows capture-driver selection is a feasibility gate, because a free driver is not automatically an eligible open-source dependency.

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
cmd/hacker-tools/                CLI composition root
internal/app/                   shared use cases and job lifecycle
internal/network/               interfaces, diagnostics, metrics
internal/wifi/                  current-network metadata and explicit secret retrieval
internal/capture/               capture sessions, files, decoding
internal/platform/              windows/linux/darwin adapters
internal/tools/                 Nmap and TShark discovery, execution, parsing
internal/storage/               SQLite, migrations, retention
frontend/src/                   React features, shared components, translations
docs/adr/                       architecture decisions
docs/ci/                        inactive workflow source
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
- GitHub Actions must stay inactive until the owner explicitly requests activation. Store source as `.yml.disabled` outside `.github/workflows/`; no scheduled, push, PR or dispatch workflow is registered yet.

## 4. Structure phase

- [x] S01 Research and brainstorming baseline.
  - [x] Read shared conversation and inspect reference repository snapshot.
  - [x] Record sources, architecture tradeoffs and owner answers.
  - Assert: both documents distinguish confirmed decisions from proposals. Verify: document review.
- [x] S02 Initialize local Git and configure the requested origin.
  - Assert: origin resolves to the owner's hacker-tools URL. Verify: `git remote -v`.
- [ ] S03 Establish GitHub access and push the planning baseline.
  - [ ] Reauthenticate GitHub CLI using the owner's browser authorization.
  - [ ] Configure Git credential helper; inspect remote state before pushing.
  - [ ] Reconcile any existing branch without force push; push and verify remote SHA.
  - Assert: authenticated identity has repository write access and remote HEAD contains this plan.
- [ ] S04 Reproducible Go and frontend toolchain.
  - [ ] Install/verify Go and Wails v2 stable; record exact versions and `wails doctor` output.
  - [ ] Create Go module and actual Wails React/TypeScript shell; pin versions after registry verification.
  - [ ] Configure Bun, committed lockfile, Vite, Base UI, Tailwind v4 and translation catalogs.
  - [ ] Add Biome, golangci-lint, govulncheck, commitlint, Lefthook and Markdown checks.
  - [ ] Supply dev/check/build commands for Windows and Unix hosts.
  - Assert: clean checkout installs frozen dependencies and opens native shell; no placeholder build reports success.
- [ ] S05 Repository conventions and developer guides.
  - [ ] EditorConfig and LF policy, contribution guide, security policy and agent instructions.
  - [ ] Commit-message hook and release/changelog generation configuration.
  - [ ] Document capability boundaries and dependency/license inventory.
  - Assert: formatting is consistent and invalid commit messages fail locally once hooks are installed.
- [ ] S06 CLI and core contracts.
  - [ ] Add version, doctor and interfaces commands; JSON mode with schema version.
  - [ ] Define errors, job states, cancellation, clock abstraction and bounded event stream.
  - [ ] Share services with Wails; return a clear unsupported response for absent providers.
  - Assert: CLI and UI get equivalent interface data; malformed input produces a stable nonzero exit code.
- [ ] S07 SQLite foundation.
  - [ ] Compare eligible drivers, document decision and pin dependency.
  - [ ] Create migration runner, schema and per-user application data path.
  - [ ] Test empty install, upgrade, failed migration rollback, concurrent reads and retention.
  - Assert: restart preserves preferences/history; failed writes do not leave partial jobs; secrets never enter DB.
- [ ] S08 Docker CLI laboratory.
  - [ ] Multi-stage CLI build and non-root runtime; separate test stage with required test toolchain.
  - [ ] Private Compose lab with health-checked HTTP/DNS/TCP fixture services.
  - [ ] Deterministic integration assertions; cleanup and bounded startup time.
  - [ ] Document that host Wi-Fi, elevation, capture drivers and WebViews require native tests.
  - Assert: one documented command fails on wrong results and returns zero only after assertions pass.
- [ ] S09 Dormant CI/CD implementation.
  - [ ] Prepare CI source for formatting/lint/typecheck/build, race tests, vulnerabilities and CLI lab.
  - [ ] Prepare native Wails build matrix with per-OS dependencies and capability tests.
  - [ ] Prepare release source for draft artifacts, checksums and source/dependency notices.
  - [ ] Pin actions to reviewed immutable revisions; minimize permissions and avoid release credentials in CI.
  - Assert: static workflow checks pass; `.github/workflows/` has no executable YAML; no workflow is activated.

Structure exit: S01–S09 pass; a clean checkout runs the native shell and CLI lab; GitHub push is verified. Documentation alone does not complete this phase.

## 5. Alpha — Windows-first integrated toolkit

- [ ] A01 Interface and route inventory.
  - [ ] IPv4/IPv6, prefix, MAC, interface identity/status, gateway and configured DNS.
  - [ ] Native Windows adapter; contracts and explicit unavailable states for other OSes.
  - [ ] Handle disconnected devices, VPNs, loopback and multiple default routes.
  - Assert: compare against OS data; hot-plug/reconnect does not duplicate stale interfaces.
- [ ] A02 Passive traffic monitoring.
  - [ ] Sample native interface counters with monotonic elapsed time.
  - [ ] Handle counter reset, suspend/resume and interface replacement.
  - [ ] Bounded history and batched UI events; graph units and sample timestamp visible.
  - Assert: controlled fixture traffic yields expected positive rates; reset never produces a negative spike.
- [ ] A03 Go diagnostics.
  - [ ] DNS record queries, ping latency/loss and bounded TCP connect checks.
  - [ ] HTTP response/timing and TLS certificate summary; redirects remain within selected scope.
  - [ ] Traceroute with platform-specific transport and permission reporting.
  - [ ] Rate limits, target validation, IPv6 scope handling and cancellable worker pool.
  - Assert: local fixtures distinguish success, refusal, timeout, NXDOMAIN and cancellation without goroutine leaks.
- [ ] A04 LAN discovery and basic service inventory.
  - [ ] Explicit CIDR/host scope, host/port limits and deterministic result ordering.
  - [ ] Native TCP discovery first; label inference and unknown states honestly.
  - [ ] Store summaries and export versioned JSON/CSV.
  - Assert: lab service inventory matches fixture ground truth; no scan expands beyond requested scope.
- [ ] A05 Wi-Fi inspector and current-network password.
  - [ ] Windows WLAN metadata: current profile, SSID/BSSID, signal/channel when available.
  - [ ] Explicit reveal action for the current saved network; native authorization as required.
  - [ ] Distinguish open network, enterprise authentication, no saved key, denied and unsupported.
  - [ ] Mask by default; automatically clear displayed secret; no logs, database or default clipboard writes.
  - [ ] Add Linux/macOS provider contracts and document implementation milestones in beta.
  - Assert: native Windows acceptance verifies reveal and denial with owner's test network; automated tests use synthetic secrets and check redaction.
- [ ] A06 Live packet capture and offline import.
  - [ ] Spike an eligible open-source Windows capture backend; document driver/license/performance tradeoffs.
  - [ ] Prefer Go capture/decoding; optional separate TShark process only for justified gaps.
  - [ ] Start/stop capture, interface selection, limits, dropped-packet counters and bounded buffers.
  - [ ] PCAP/PCAPNG import/export with file-size limits and malformed-file handling.
  - [ ] Ethernet/IPv4/IPv6/TCP/UDP/ICMP/DNS and plaintext HTTP summaries in Go where feasible.
  - [ ] Distinguish capture filters from display filters and encrypted metadata from readable content.
  - Assert: synthetic captures decode expected fields; native Windows test captures controlled traffic, stops promptly and leaves no live handle.
  - Gate: resolve capture-driver eligibility before declaring alpha complete; an incompatible dependency is not silently accepted.
- [ ] A07 Optional advanced adapters.
  - [ ] Record why a Go implementation is unsuitable before adding each Nmap/TShark path.
  - [ ] Tool discovery, supported versions, structured parsers and dependency diagnostics.
  - [ ] Argument arrays, output limits, deadlines and cancellation of descendant processes.
  - [ ] Parser fixtures for valid, partial, malformed and version-incompatible output.
  - Assert: core works without tools installed; optional results preserve provider/version and cannot execute arbitrary shell input.
- [ ] A08 Integrated UI.
  - [ ] Dashboard, interfaces, diagnostics, Wi-Fi, packet inspector, toolkit, history and settings.
  - [ ] PT-BR/English catalogs, keyboard access, focus states and Base UI primitives.
  - [ ] Virtualized packet/result tables, loading/error/permission states and cancel controls.
  - [ ] SQLite preferences and history; opt-in capture save and clear-history action.
  - Assert: UI has real service connections; Playwright tests with contract-faithful providers pass; native smoke verifies the actual Wails bridge.
- [ ] A09 Alpha validation and release candidate.
  - [ ] Go unit/integration/race checks; fuzz packet and tool parser boundaries.
  - [ ] React typecheck/lint/build, accessibility smoke and critical workflow tests.
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
  - [ ] Finalize license and dependency redistribution decisions; exclude ineligible bundled components.
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
  - [ ] Obtain explicit instruction to activate workflows before moving source into `.github/workflows/`.
  - [ ] Configure protected release environment and minimum repository permissions.
  - Assert: dormant source remains dormant until instructed; 1.0 can be released manually if automation stays disabled.

## 8. Verification strategy

Use behavioral tests for packet parsing, rate calculations, state transitions, timeouts, process cancellation, persistence and scoped execution. Avoid tests that merely restate implementation. Native hardware/API checks and container tests are separate evidence categories. Frontend browser tests validate UI flows but do not prove native Wails IPC works.

Required check interface after S04: Go format/lint/vet/test/race/vulnerability checks; Bun lint/typecheck/test/build; parser fuzz smoke; Docker integration; native Wails build/smoke. Before Go/frontend scaffolding exists, these checks must not be reported as passed.

Threat boundaries to verify: untrusted packet bytes, external tool output, user-provided targets/files, Wails calls, local helper IPC and export paths. Test malformed input and resource limits at each boundary. Capture only in an explicitly started session. No cloud account or telemetry required.

## 9. Deferred beyond 1.0

Third-party downloaded plugins, firewall modification, traffic shaping, Wi-Fi monitor mode, credential attacks, exploitation frameworks, cloud synchronization and automatic TLS interception are separate proposals. The current plan focuses on the integrated monitoring/diagnostic/capture scope and does not claim feature parity with every Kali package.

## 10. Current evidence and next task

- Research baseline committed as `c4bf7fe`.
- Go executable is not currently on PATH; Bun, Node, Docker CLI and GitHub CLI are present. Docker daemon and toolchain functionality still need verification.
- GitHub CLI reports an invalid saved token for AlexGalhardo; authentication is pending.
- Next: establish GitHub access, push this plan, then execute S04 onward. App implementation tasks remain unchecked.
