# kali-tools — delivery plan

Planning baseline: 2026-10-19. Architecture revision: 2026-10-21.

Owner release instruction (2026-10-21): publish v0.3.0 and rename the earlier release records to v0.1.0/v0.2.0. This explicitly authorizes publication with disclosed unsigned development status; it does not complete any native or release-qualification checkbox below.

Owner checkpoint instruction (2026-10-21): expedite the current work with limited credits, synchronize PLAN/docs/changelog, commit, push and publish the next development release (0.5.0). Earlier 0.1.0/0.2.0 drafts were published without replacing their contents. This publication remains unsigned and does not mark the full PLAN or native acceptance complete. C01 non-native implementation is verified; W01/W04 implementation is present with the explicit open gates below.

Related documents:

- Product owner decisions: [brainstorming](docs/brainstorming.md).
- Tool and platform sources: [tools research](docs/tools-research.md).
- Architecture reasoning: [architecture research](docs/architecture-research.md).
- **Binding architecture:** [architecture](docs/architecture.md). Read it before any structural change.
- **Implementation specs:** [Spec Driven Development](docs/specs/README.md). Every open task below links its spec. Implement from the spec, satisfy its quality gates (QG) and delivery assertions (DA), and record evidence in `docs/verification/specs/<ID>.md`.

## 1. Product contract

Build a local security and network toolkit inspired by the Kali Linux ecosystem.

**Stack.** Go, Wails, React, TypeScript, Base UI and Tailwind CSS v4.

**Interfaces.** Three interfaces share **one Go core**:

1. **Native desktop GUI (Wails v2)**: the primary interface.
2. **Web server**: serves the same React UI to a browser (`kali-tools serve`).
3. **Complete CLI**: for headless and SSH hosts, with human and versioned JSON output.

Every capability is implemented once in Go services and exposed through all three interfaces with equivalent results, errors and capability states. No interface may contain business logic.

**Platforms.** Windows 11 leads development. Linux and macOS adapters stay separate from the domain.

**Implementation choices.**

- Implement as much as practical in Go.
- Integrate optional TShark, iperf3 or Pktmon executables only where a native implementation would be incomplete or unreasonable. Service inspection is native and does not execute Nmap (ADR 006).
- SQLite stores local preferences and diagnostic history.

**Language.** Interface languages are English and PT-BR. Code, documentation and commits are in English.

**Alpha scope.** The alpha includes:

- Wi-Fi password display;
- live packet capture;
- addresses and interfaces;
- passive traffic;
- diagnostics;
- discovery.

These are real implementation requirements, not placeholder panels. Alpha acceptance is Windows-first. Beta and 1.0 broaden native verification to Linux and macOS. Capability limitations must be visible to users.

**Scope boundary.** Defensive, diagnostic and authorized-testing capabilities only. Credential attacks, exploitation frameworks, Wi-Fi injection/monitor-mode attacks and evasion are out of scope (§11).

**Licensing.**

- The project has no commercial purpose.
- Select dependencies whose source is available under suitable open-source licenses, with no mandatory paid runtime service. Do not infer that every open-source license forbids commercial use.
- The owner delegated open-source housekeeping on 2026-10-19. Project code uses MIT in LICENSE.md, and no noncommercial restriction is added.
- Keep TShark and Npcap optional and separately installed. Do not bundle Npcap. Do not execute or bundle Nmap (NPSL; ADR 006). See tools research §3.
- Windows capture-driver selection is a feasibility gate, because a free driver is not automatically an eligible open-source dependency.

## 2. Architecture

Hexagonal (ports and adapters). Dependencies point inward. Full rules, the repository map, current deviations and extension recipes are in [docs/architecture.md](docs/architecture.md).

```text
Wails desktop ─┐                                     ┌─ adapter/storage   (SQLite)
Web (httpapi) ─┼─► internal/app ─► capabilities ◄────┼─ adapter/tools     (native inspect, TShark inventory)
CLI           ─┘   (use cases,     network/capture/  ├─ adapter/probe     (ICMP)
                    jobs, events)  wifi + domain     └─ adapter/platform  (routes/counters)
```

```text
main.go, app.go                 Wails composition root and current desktop facade (root required by Wails v2)
cmd/kali-tools/                 CLI composition root; `serve` starts the web server
cmd/lab-fixture/                Docker lab fixture services
internal/domain/                shared kernel: typed errors, capability, versions
internal/app/                   use cases, job lifecycle, history; declares driven ports
internal/job/                   (target) job manager and bounded event fan-out
internal/network|capture|wifi/  capability packages (domain rules and limits)
internal/adapter/storage|tools|probe|platform/   driven adapters
internal/adapter/wailsui|httpapi|cli/            (target) driving adapters
internal/bootstrap/             (target) shared wiring for all composition roots
internal/porttest/              (target) contract suites for driven ports
frontend/src/{app,features,shared,styles}/       React UI; shared/api/bridge.ts is the only transport seam
tests/integration|acceptance|fixtures/           Docker lab, interface-agnostic specs, synthetic fixtures
scripts/, scripts/setup/        task runners and per-OS setup entry points
docs/, docs/adr/                architecture, research, decisions, evidence
```

**Rules:**

- No UI or transport business logic. No shell command concatenation.
- Bounded queues and explicit deadlines. Cancellation frees every resource.
- Typed domain errors, stable job IDs and a stable event schema.
- Interfaces are declared by their consumer, and only at real boundaries.
- Generated Wails bindings are regenerated and checked for drift.
- Neither the UI nor HTTP routes ever invoke arbitrary executables, SQL or file paths.
- Native platform files use Go build constraints with explicit unsupported fallbacks.

**States:**

- Capability states: available, dependency missing, permission required, location consent required, unsupported, failed. H06 defines the consent state; A05 adds native detection.
- Job states: queued, running, completed, failed, cancelled.
- History includes timestamps, operation parameters, tool/provider version and a summary. Secrets and payloads are excluded from ordinary logs and history.

**Storage:**

- SQLite holds settings, jobs, diagnostic results, tool inventory and schema migrations.
- Use explicit migrations, transactions, parameterized SQL and a single coordinated writer.
- Packet files are separately managed opt-in artifacts. There is no automatic payload database.
- Apply retention and export controls, and redact secrets.
- Do not claim guaranteed zeroization of Go strings.

## 3. Completion and version rules

- **Checkboxes.** `[x]` means implemented and verified with recorded evidence; `[ ]` means pending. A parent completes only after all its children and assertions pass.
- **Evidence.** Each task contains an assertion and a verification method. Store evidence in `docs/verification/<version>.md` with OS, commands, date and limitations.
- **Versions.** The SemVer sequence is: structure before release; `0.1.0-alpha.N`; `0.1.0-beta.N`; `1.0.0-rc.N`; `1.0.0`. No tag is created merely because a checklist exists.
- **Public contracts.** Define the public CLI, the HTTP API, the result/export schemas and the supported configuration contract before 1.0. After 1.0, breaking changes require a major version. Before 1.0, they need explicit migration notes.
- **Changelog.** Conventional Commits feed git-cliff and Keep a Changelog. Review the generated changelog and version consistency before a release. Never include tokens, saved Wi-Fi keys or real capture data in commits.
- **CI.** GitHub Actions activation was explicitly authorized on 2026-10-19. The active pinned source is under `.github/workflows/`. Pull requests receive read-only permissions, and only the gated main-branch draft-release job can write releases.

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
  - Evidence:
    - Authenticated as AlexGalhardo with repository push permission; `main` tracks `origin/main`.
    - The GitHub API confirmed planning commit `223c4b06b7dcb764456a179ab349b373f9e54870` after push.
    - Credentials are outside the repository; global Git configuration was not required.
- [x] S04 Reproducible Go and frontend toolchain.
  - [x] Install/verify Go and Wails v2 stable; record exact versions and `wails doctor` output.
  - [x] Create Go module and actual Wails React/TypeScript shell; pin versions after registry verification.
  - [x] Configure Bun, committed lockfile, Vite, Base UI, Tailwind v4 and translation catalogs. Committed in c902560. A clean source archive installed frozen Bun dependencies, built the frontend and passed the actual native race/bridge test.
  - [x] Add Biome, golangci-lint, govulncheck, commitlint, Lefthook and Markdown checks.
  - [x] Supply dev/check/build commands for Windows and Unix hosts.
  - Assert: clean checkout installs frozen dependencies and opens native shell; no placeholder build reports success.
- [x] S05 Repository conventions and developer guides.
  - [x] EditorConfig and LF policy, contribution guide, security policy and agent instructions.
  - [x] Commit-message hook and release/changelog generation configuration.
  - [x] Document capability boundaries and dependency/license inventory.
  - Assert: formatting is consistent and invalid commit messages fail locally once hooks are installed.
  - Evidence: installed pre-commit/pre-push/commit-msg hooks on 2026-10-19. The actual commit-msg hook rejects invalid input and accepts a Conventional Commit message.
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
- [x] S09 CI/CD implementation (activation authorized 2026-10-19).
  - Hosted repair verified: Windows inactive-WLAN state and macOS inventory/config-path assertions corrected; [run 39668017937](https://github.com/AlexGalhardo/kali-tools/actions/runs/39668017937) passes every job, including draft assembly. [S09 evidence](docs/verification/specs/S09.md).
  - [x] Prepare CI source for formatting/lint/typecheck/build, race tests, vulnerabilities and CLI lab. Active source passes actionlint and immutable-pin checks; execution acceptance is recorded separately.
  - [x] Prepare native Wails build matrix with per-OS dependencies and capability tests. The `native` job builds on windows-2025, ubuntu-24.04 (GTK and WebKitGTK) and macos-15, and runs `go test -race` on each. [Run 39981075045](https://github.com/AlexGalhardo/kali-tools/actions/runs/39981075045) passed that matrix. Hosted builds are not hardware acceptance.
  - [x] Prepare release source for draft artifacts, checksums and source/dependency notices. `scripts/release.mjs` writes `SHA256SUMS`, copies the notice files and archives the source. The release job runs only for a non-pull-request push to `main`. Draft assembly passed on [run 39668017937](https://github.com/AlexGalhardo/kali-tools/actions/runs/39668017937).
  - [x] Pin official Actions release revisions; minimize permissions and use the short-lived GitHub token only in the main-branch release job.
  - [x] Add `GOOS=linux|darwin|windows go vet ./...` and the dependency-rule check (H08) to CI. The quality job runs `CGO_ENABLED=0` vet for those three targets and `node scripts/task.mjs arch`. `node scripts/task.mjs workflows` fails if either check is removed. Cross-compilation is not native acceptance. [S09 evidence](docs/verification/specs/S09.md).
  - Assert: static workflow checks and hosted jobs pass; activation follows explicit owner authorization; only successful main-branch runs assemble development release drafts.
- [x] S10 Architecture research and repository reorganization (2026-10-21).
  - [x] Research Clean/Hexagonal architecture in Go and record decisions in `docs/architecture-research.md`.
  - [x] Install `book-to-skill` (revision recorded in `.agents/skills/sources.json`). Convert the owner's Clean Architecture reference book into `.agents/skills/clean-architecture-in-practice`. The PDF stays untracked (`/docs/*.pdf` ignored).
  - [x] Deepen `docs/tools-research.md`: licensing (NPSL, Npcap), Windows 24H2 location consent, Pktmon, Go libraries per category, structured formats.
  - [x] Move driven adapters to `internal/adapter/{storage,tools,probe,platform}`; reorganize `frontend/src` into `app/`, `shared/{api,i18n,ui}/`, `styles/`, `features/`; move `setups/` to `scripts/setup/`; add `.gitkeep` target folders. Only import paths, configuration paths and documentation references changed; there were no logic changes.
  - [x] Write `docs/architecture.md` for agents, including the known deviations D1–D8.
  - Assert: the reorganization changes no behavior.
  - Evidence (Windows 10, 2026-10-21):
    - Go: `go vet ./...`, `go vet -tags desktop .`, `GOOS=linux go vet ./...` and `GOOS=darwin go vet ./...` passed. `go test ./...` passed for all packages.
    - Frontend: Bun frozen install, `typecheck`, `lint` and `build` passed, plus 17/17 Playwright workflows.
    - Scripts: `bash -n` passed on the moved setup scripts.
    - Not run: race, the Docker lab and the native Wails build.

- [x] S11 Code review, hardening and formatting enforcement (2026-10-21).
  - [x] Review the Go core and adapters. Fix the following:
    - dropped terminal job events under a full buffer;
    - non-idempotent `Service.Close`;
    - the 64-bit `IP_OPTION_INFORMATION` layout;
    - Linux `null` gateway lists;
    - rejected underscore DNS labels.
    - Commit 4dd28a3.
  - [x] Enforce gofmt -s and goimports (local prefix) through golangci-lint formatters. Add a sequential Lefthook pre-commit (format and re-stage, then lint, then vet) and a pre-push format diff. Commit eb7b060.
  - [x] Add the Spec Driven Development specs with global quality gates, per-spec delivery assertions and auditable evidence records (`docs/specs/`).
  - [x] Add the project agent skills: Go skills adapted from AlexGalhardo/vukem, and `clean-architecture-in-practice`. AGENTS.md requires `karpathy-guidelines`, `ponytail` and `clean-architecture-in-practice` at the start of every task.
  - Assert: no behavior regression; new behavior covered by tests.
  - Evidence: [0.1.0-alpha.2-dev](docs/verification/0.1.0-alpha.2-dev.md). Lint reported 0 issues. `go vet` passed for every target. `go test ./...` passed. `-race` was not run locally because the host has no cgo compiler; it is delegated to CI.

Structure exit: S01–S11 pass; a clean checkout runs the native shell and CLI lab; GitHub push is verified. Documentation alone does not complete this phase.

## 5. Hexagonal migration (H) — prerequisite for web and CLI parity

These tasks remove the deviations listed in [architecture §8](docs/architecture.md). They change structure, not behavior.

- Each task keeps all existing tests green.
- Each task adds the tests named in it.
- Each task updates `docs/architecture.md` in the same change.
- Work in small vertical steps. Do not rewrite packages wholesale.

Checkpoint reconciliation (2026-10-21, history through `c630f7d`): H01–H06 have verified implementation records; their parent boxes remain open because native acceptance is pending. H07 is the active spec. H08/H09 follow only after H07's non-native verification passes, under the authorized headless continuation. No release qualification is implied.

SDD delivery checklist, applied to each spec before its parent checkbox can close:

- [ ] Confirm the spec's current-state claims against the checkout and record deviations.
- [ ] Run the specified regression tests before implementation (or preserve existing refactor tests).
- [ ] Implement only the active spec and its recorded scope corrections.
- [ ] Record every applicable QG-1–QG-15 result, including coverage before/after, in `docs/verification/specs/<ID>.md`; explain every not-applicable or not-run result.
- [ ] Record every AC and DA with its test, command or artifact; retain each pending native check explicitly.
- [ ] Synchronize the spec status, architecture, PLAN and Unreleased changelog with the evidence.
- [ ] Pass Conventional Commit and Lefthook checks; never bypass hooks.
- [ ] Close the parent checkbox only when all applicable gates and native acceptance have evidence.

These are reusable delivery requirements, not a separate completed milestone. Detailed command thresholds and the ordered list of all 18 specs remain in [SDD](docs/specs/README.md).

- [ ] H01 Shared composition: `internal/bootstrap`.
  - Spec: [H01-bootstrap](docs/specs/H01-bootstrap.md)
  - [x] `bootstrap.New(ctx, Config) (*app.Application, cleanup, error)` opens storage, applies retention and injects the existing ICMP adapter, as scoped by H01. Tool discovery and remaining platform wiring stay with H03/H04. [Evidence](docs/verification/specs/H01.md).
  - [x] Wails `main.go`/`app.go` and `cmd/kali-tools` both use it; composition roots expose `run(ctx, args, getenv, stdout, stderr)`. [Evidence](docs/verification/specs/H01.md).
  - Acceptance pending: native Wails bridge (AC-10); non-root Docker lab (AC-11) passed. H01 remains open for native acceptance; owner authorized sequential headless implementation with native gates pending.
  - [ ] H01 native bridge AC-10, with OS/hardware/date evidence.
  - Assert: the desktop and the CLI build the same application graph; no package below `cmd`/root reads flags or environment variables.
- [ ] H02 Job manager and event fan-out: `internal/job`.
  - Spec: [H02-job-manager](docs/specs/H02-job-manager.md)
  - [x] Extract the job lifecycle from `app.Service`. Jobs run under a manager context. Events carry a monotonic `seq`. Provide `Subscribe(ctx)` with `Limits.SubscriberBuffer` and drop counting, plus `Cancel`, `Snapshot` and `Wait`. [Evidence](docs/verification/specs/H02.md).
  - [x] Move the metrics ticker and the capture session state from `app.go` into application use cases that publish through the manager (D3). [Evidence](docs/verification/specs/H02.md).
  - Acceptance pending: native desktop rates, job states and capture (AC-16); sequential headless implementation is owner-authorized.
  - [ ] H02 native desktop AC-16/DA-8.
  - Assert: tests cover state transitions, cancellation, a slow subscriber that drops without blocking, shutdown without goroutine leaks (race detector), and stable job IDs.
- [ ] H03 Consumer-side driven ports.
  - Spec: [H03-driven-ports](docs/specs/H03-driven-ports.md)
  - [x] Declare a `Prober` port in `network` and inject `probe.ICMP` from the composition roots (D1). Evidence: commit 4dd28a3; `go list -deps ./internal/network` lists only `internal/domain`; tests `TestICMPWithoutProberIsUnsupported` and `TestICMPUsesInjectedProber`.
  - [x] Declare `JobStore`/`SettingsStore` in `app` (D5). [Evidence](docs/verification/specs/H03.md).
  - [x] Declare `HostNetwork`, `ToolRunner` (including service inspection), `WiFiProvider` and `CaptureProvider` in their consumers. [Evidence](docs/verification/specs/H03.md).
  - [x] Add compile-time assertions in bootstrap or tests only. [Evidence](docs/verification/specs/H03.md).
  - Acceptance pending: native desktop on moved types (AC-11); sequential headless implementation is owner-authorized.
  - [ ] H03 native desktop AC-11.
  - Assert: `internal/network`, `internal/capture`, `internal/wifi` and `internal/app` import no `internal/adapter/*` package.
- [ ] H04 Single application facade.
  - Spec: [H04-application-facade](docs/specs/H04-application-facade.md)
  - [x] `app.Application` exposes every operation currently called directly by `app.go` or the CLI: interfaces, inventory, tools, diagnostics, discovery, inspection, Wi-Fi, capture, import/export, history and settings (D2). [Evidence](docs/verification/specs/H04.md).
  - Acceptance pending: complete native desktop workflow (AC-12); sequential headless implementation is owner-authorized.
  - [ ] H04 native desktop AC-12.
  - Assert: `app.go` and the CLI import only `app`, `domain` and DTO packages.
- [ ] H05 Driving adapters extracted.
  - Spec: [H05-driving-adapters](docs/specs/H05-driving-adapters.md)
  - [x] Move the CLI command tree to `internal/adapter/cli` (`Run(ctx, app, args, stdout, stderr) int`); `cmd/kali-tools/main.go` only wires. [Evidence](docs/verification/specs/H05.md).
  - [x] Move the Wails bindings and event forwarder to `internal/adapter/wailsui`. Regenerate `frontend/wailsjs`, update `shared/api/bridge.ts` and verify there is no binding drift. Keep `main.go` at the root. [Evidence](docs/verification/specs/H05.md).
  - Acceptance pending: native bridge and desktop workflows (AC-17/18); sequential headless implementation is owner-authorized.
  - [ ] H05 native AC-17/18 and bridge DA-8.
  - Assert: the native bridge smoke test and all Playwright workflows pass; the CLI JSON output is byte-compatible except for documented changes.
- [ ] H06 Transport DTOs and error mapping.
  - Spec: [H06-dtos-and-errors](docs/specs/H06-dtos-and-errors.md)
  - [x] Define one frontend contract (DTOs shared by `wailsui` and `httpapi`), separate from storage rows (D4).
  - [x] Map errors once per adapter.
  - [x] Add the `location_consent_required` capability state.
  - Implementation evidence: [H06](docs/verification/specs/H06.md). Native bridge AC-17/DA-8 remains pending under the authorized headless workflow.
  - [ ] H06 native bridge AC-17/DA-8.
  - Assert: no `storage.*` type appears in a binding, HTTP or CLI signature; secret-redaction tests cover every DTO.
- [x] H07 Contract and acceptance test harness.
  - Spec: [H07-test-harness](docs/specs/H07-test-harness.md)
  - [x] `internal/porttest` suites for each driven port, run against the real adapter and an in-memory fake. [Continuation evidence](docs/verification/specs/H07.md#continuation-2026-10-21).
  - [x] `tests/acceptance` diagnostics/discovery for direct application, CLI and headless Wails; history for direct/Wails. [Evidence](docs/verification/specs/H07.md).
  - [x] Verify the real ICMP regression after rejecting invalid targets before DNS/native delegation. `TestProberContract` passes; probe coverage 75.8% → 78.1%. [Evidence](docs/verification/specs/H07.md#continuation-2026-10-21).
  - [x] Rerun H07 race checks after installing GCC 16.2.0: `go test -race -timeout 120s ./internal/porttest/... ./internal/adapter/... ./tests/acceptance/...` passes. [Evidence](docs/verification/specs/H07.md#continuation-2026-10-21).
  - [x] H07 native OS-port data branches AC-10/DA-7: Wi-Fi, ICMP, host inventory and elevated capture pass on Windows build 26300 after the suite selects an active interface. [Native evidence](docs/verification/specs/H07.md#native-capture-correction-2026-10-21).
  - [x] C01 step 4: CLI history list/clear implemented; the acceptance skip is removed and history runs through all three drivers.
  - [x] W01/W03: add the HTTP driver and run the unchanged shared acceptance specifications. `tests/acceptance` runs the shared specifications through `newHTTP` as well as direct, CLI and Wails. [W01 evidence](docs/verification/specs/W01.md).
  - Progress: [H07 evidence](docs/verification/specs/H07.md). Native OS-port data branches now pass; C01 step 4 removes the CLI-history skip. The HTTP driver is in the shared acceptance loop. Other specs' native gates are independent.
  - Assert: each fake passes the same suite as its adapter; at least the diagnostics, discovery and history specifications run through two drivers.
- [x] H08 Enforce the dependency rule.
  - Eight depguard cases, wrong-version guard, cleanup and actual commit-hook rejection pass. Positive hosted run 39669810016 passes; negative PR #1 fails specifically on the application depguard rule, then was closed and deleted without merging. [Evidence](docs/verification/specs/H08.md).
  - Spec: [H08-dependency-rule](docs/specs/H08-dependency-rule.md)
  - [x] Configure depguard in `.golangci.yml` (or pinned go-arch-lint) with the matrix in architecture §3. Run it in CI and in Lefthook.
  - [x] Use the spec's pinned depguard configuration, including test-only porttest and composition-root restrictions.
  - [x] Add `scripts/depguard-check.mjs` and `node scripts/task.mjs arch`; prove each of the eight rules rejects a deliberate violation and cleans up.
  - [x] Wire the architecture check into the existing pinned CI job; record QG/AC/DA evidence in `docs/verification/specs/H08.md`.
  - Assert: a deliberately forbidden import fails the check locally and in CI.
- [x] H09 Decide the CLI parser.
  - [x] ADR 004 keeps stdlib `flag`. The owner accepted it on 2026-10-23. Cobra is not adopted. [ADR](docs/adr/004-cli-parser.md).
  - Spec: [H08-dependency-rule](docs/specs/H08-dependency-rule.md)
  - [x] Record an ADR: stdlib `flag` versus Cobra, based on the C-phase command tree, completion needs and license review.
  - [x] Write ADR 004 using H08 §4.5's decision rule and revisit triggers. Accepted by the owner on 2026-10-23.
  - Assert: the ADR names the trigger for revisiting the decision.

## 6. Alpha — Windows-first integrated toolkit

- [ ] A01 Interface and route inventory.
  - Spec: [A01-neighbors](docs/specs/A01-neighbors.md)
  - [ ] IPv4/IPv6, prefix, MAC, interface identity/status, gateway and configured DNS.
  - [ ] Native Windows adapter; contracts and explicit unavailable states for other OSes.
  - [ ] Handle disconnected devices, VPNs, loopback and multiple default routes.
  - [ ] Passive neighbor inventory from the OS ARP/ND cache (`GetIpNetTable2`), with no raw packets.
    - [x] A01 step 1: pure family validation and normalized, sorted, deduplicated neighbor tables, verified at 4,096 and 4,097 rows. [Evidence](docs/verification/specs/A01.md).
    - [x] A01 step 2: bounded netlink/procfs parsers, malformed-input tests and 30-second fuzz smoke (791,337 executions). [Evidence](docs/verification/specs/A01.md).
    - [x] A01 steps 3–4: Windows read-only shim/layout/conversion and Linux/fallback glue; Windows smoke and cross-vet pass. Windows hardware comparison gates remain open; Linux Docker lab passes. [Evidence](docs/verification/specs/A01.md).
    - [x] A01 steps 5–6: shared passive neighbor use case, expanded real/fake port contracts and Wails/HTTP/CLI JSON/CSV exposure; cancellation, bounds and no-history tests pass. [Evidence](docs/verification/specs/A01.md).
    - [x] A01 step 7: translated neighbor table, family filter, explicit refresh and pagination; both transports pass, 38 browser workflows total. Windows native acceptance stays open; Linux Docker lab passes. [Evidence](docs/verification/specs/A01.md).
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
  - Spec: [A05-wifi-consent](docs/specs/A05-wifi-consent.md)
  - [ ] Windows WLAN metadata: current profile, SSID/BSSID, signal/channel when available.
    - [x] Channel query and bounded native-structure decoding implemented; synthetic Windows metadata/layout tests passed.
    - [ ] Controlled native Wi-Fi network acceptance.
  - [ ] Windows 11 24H2 location consent: implementation maps opcode-7 `ERROR_ACCESS_DENIED` on build >= 26100, reports `location_consent_required` and offers the fixed settings URI; no automatic retry. `AppCapability.CheckAccess` remains a WinRT follow-up. Linux-host pure-function coverage now meets the spec gate (`consent.go` 100%, `profile.go` 90%). Native consent acceptance remains open. [Evidence](docs/verification/specs/A05.md).
    - [x] Synthetic consent/reveal mapping, observed capability state, fixed-URI desktop opener, HTTP/CLI error hints and translated UI recovery; key/metadata discarded on provider errors.
  - [ ] Explicit reveal action for the current saved network; native authorization as required.
  - [ ] Distinguish open network, enterprise authentication, no saved key, denied and unsupported.
  - [x] Mask by default; automatically clear displayed secret; no logs, database or default clipboard writes.
  - [x] Shared Linux/macOS provider contracts return explicit unsupported states; native implementation milestones remain B01/B02.
  - Assert: native Windows acceptance verifies reveal, denial and consent-denied states with the owner's test network; automated tests use synthetic secrets and check redaction.
- [ ] A06 Live packet capture and offline import.
  - Spec: [A06-capture-backends](docs/specs/A06-capture-backends.md)
  - [x] Implement shared backend selection, optional Npcap/Pktmon adapters, safe count-only history and translated selector/loss UI; automated adapter, interface and browser acceptance passed. Native evaluation remains open. [Evidence](docs/verification/specs/A06.md).
  - Headless continuation: A06 A06 implementation and automated acceptance cover contracts, history, Npcap/Pktmon adapters, all interfaces and the translated selector; native spike measurements and qualification remain open. [Evidence](docs/verification/specs/A06.md).
  - [ ] Spike an eligible open-source Windows capture backend; document driver/license/performance tradeoffs.
    - [ ] Evaluate user-installed Npcap through gopacket/pcap (detection plus download link only; never bundled or silently installed).
    - [ ] Evaluate built-in Pktmon → pcapng (`etl2pcap`) as an elevated optional adapter; record the privilege and data-loss limits.
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
  - [x] ADR 003 recorded optional Nmap service inspection; native TCP remained the default and no TShark execution was introduced.
  - [x] Owner decision (Alex Galhardo, 2026-10-23): cancel Nmap service inspection because of the NPSL 0.95 execute-and-parse clause. [ADR 006](docs/adr/006-native-service-inspection.md) supersedes ADR 003. The evaluation is in [tools research](docs/tools-research.md) §3.1.
  - [x] Native TCP/banner/TLS inspection replaces the Nmap adapter behind `ToolRunner`. Linux loopback tests identify HTTP, TLS and SSH and reject an out-of-scope prefix before any dial. Windows, macOS and non-loopback qualification remain open.
  - [ ] Tool discovery, supported versions, structured parsers and dependency diagnostics.
    - [x] Bounded optional-executable version inventory and explicit missing-dependency states; core remains usable without installed tools.
    - [x] TShark 4.2+ within major 4 remains inventory-only. The Nmap executable, XML parser and version gate are removed.
  - [x] Fixed argument arrays, stdout/stderr limits and deadlines remain for optional executables. Windows Job Object and Unix process-group tests still cover that runner; they no longer wrap Nmap.
  - [ ] Qualify separately installed optional executables on native hosts. TShark execution is not implemented. Nmap qualification was withdrawn with the feature.
  - Assert: core works without tools installed; service inspection does not execute Nmap or accept shell input; history stores the provider and host count only.
- [ ] A08 Integrated UI.
  - [x] Owner-requested black/neon-green terminal theme, bundled JetBrains Mono, mobile layout and Impeccable review. [Theme spec](docs/specs/A08-terminal-theme.md); [evidence](docs/verification/specs/A08-theme.md). Shared styling applies to desktop and browser; HTTP implementation and security are present; open acceptance is recorded in W01/W04.
  - [x] Dashboard, interfaces, diagnostics, Wi-Fi, packet inspector, toolkit, history and settings.
  - [ ] PT-BR/English catalogs, keyboard access, focus states and Base UI primitives.
  - [ ] Paginated packet/result tables, loading/error/permission states and cancel controls.
    - [x] Packet and discovery tables render at most 100 rows per page; discovery pagination verified with 205 synthetic endpoints. Owner decision 2026-10-23: this pagination satisfies the 1.0 virtualization requirement. Do not add a virtualization library. [ADR 008](docs/adr/008-paginated-tables.md).
    - [x] Diagnostics cancellation, capture stop and discovery cancellation controls; browser discovery test uses the emitted service job ID.
    - [x] Actual provider capability states, bounded-import notice and metric sample timestamps.
    - [x] Translated states, permission/diagnostic reasons, capture start failure recovery and optional-adapter dependency flows.
    - [x] Pagination satisfies the virtualization requirement for 1.0. [ADR 008](docs/adr/008-paginated-tables.md).
    - [ ] Complete all permission/error acceptance.
  - [ ] SQLite preferences and history; opt-in capture save and clear-history action.
    - [x] SQLite-backed preferences/history and explicit clear-history action; restart/retention and settings validation tests pass.
    - [x] Native save-dialog integration and explicit PCAP/PCAPNG format selection; browser success/cancellation/failure and Go filesystem tests passed.
  - Assert: UI has real service connections; Playwright tests with contract-faithful providers pass; native smoke verifies the actual Wails bridge.
- [ ] A09 Alpha validation and release candidate.
  - [x] Go unit/integration/race checks; fuzz packet and tool parser boundaries. Windows unit/race, Linux Docker integration and parser fuzz smoke passed; native hardware and release qualification remain open.
  - [x] React typecheck/lint/build, accessibility smoke and critical workflow tests.
  - [ ] Record Windows native evidence for Wi-Fi and capture, with dependency/setup instructions.
  - [ ] Measure idle memory/CPU and capture workload; establish numeric beta regression budgets.
  - Assert: all alpha tasks pass; release notes disclose platform and protocol limits.

Alpha exit: on a supported Windows setup, the desktop GUI provides:

- addresses;
- passive monitoring;
- diagnostics;
- discovery;
- Wi-Fi key display;
- live capture.

H01–H06 have verified implementations; native acceptance remains open in their evidence records and checklists above. Web and CLI parity reuse the same core. Missing Linux/macOS implementations are explicit. Unrelated future capabilities do not block alpha.

## 7. Interfaces — complete CLI (C) and web server (W)

These start after H04 and can run in parallel with late alpha work. Parity is defined against the desktop capability list: every desktop operation has a CLI command and an HTTP route backed by the same application call.

### 7.1 Complete CLI

- [x] C01 Command tree and conventions. [Evidence](docs/verification/specs/C01.md).
  - Spec: [C01-cli](docs/specs/C01-cli.md)
  - [x] Commands (neighbors and serve are explicit reserved stubs until their specs):
    - `version`
    - `doctor`
    - `capabilities`
    - `interfaces`
    - `inventory`
    - `neighbors`
    - `diagnose <dns|tcp|http|tls|ping|trace>`
    - `discover`
    - `inspect`
    - `wifi <show|reveal>`
    - `capture <live|import|export>`
    - `jobs <list|show|cancel>`
    - `history <list|clear>`
    - `settings <get|set>`
    - `tools`
    - `serve`
  - [x] Global flags `--json`, `--timeout`, `--no-color` and `--lang`. Stable exit codes documented per error code. Help text for every command.
  - Assert: golden tests for human and JSON output; malformed input returns documented exit codes without echoing secrets.
- [x] C02 Long-running jobs in a terminal. [Evidence](docs/verification/specs/C01.md).

  - Spec: [C01-cli](docs/specs/C01-cli.md)
  - [x] Progress streaming from the job manager; Ctrl-C cancels through `Cancel`; `--detach` is rejected until a daemon mode exists (no hidden background processes).
  - Assert: cancellation leaves no child process or capture handle; output stays bounded.
- [ ] C03 Headless/SSH ergonomics. Implementation and Docker pass; native SSH acceptance pending. [Evidence](docs/verification/specs/C01.md).
  - Spec: [C01-cli](docs/specs/C01-cli.md)
  - [x] Non-interactive defaults; no TTY assumptions; `wifi reveal` requires an explicit flag and prints a warning; capture export writes only to an explicit path.
  - Assert: commands run under `ssh -T` and in the Docker lab; secrets are never written to history.
- [x] C04 CLI contract freeze for 1.0. [Evidence](docs/verification/specs/C01.md).
  - Spec: [C01-cli](docs/specs/C01-cli.md)
  - [x] Document commands, flags, JSON schemas and exit codes in `docs/cli.md`; add compatibility fixtures.
  - Assert: schema fixtures fail on an unannounced breaking change.

### 7.2 Web server: same UI in a browser

- [x] W01 Shared frontend assets. [Implementation evidence](docs/verification/specs/W01.md).
  - Spec: [W01-web-server](docs/specs/W01-web-server.md)
  - [x] Expose the built `frontend/dist` through one embed package used by both Wails and `httpapi`.
  - Assert: desktop and server serve byte-identical assets from one build.
- [x] W02 Transport-neutral frontend. [31 browser tests](docs/verification/specs/W01.md).
  - Spec: [W01-web-server](docs/specs/W01-web-server.md)
  - [x] `shared/api/bridge.ts` selects Wails or HTTP/SSE at runtime.
  - [x] Feature code never imports `wailsjs` directly.
  - [x] Split `app/App.tsx` into `features/<capability>/` modules (D7).
  - Assert: Playwright runs the same workflows against the Wails-fake transport and against a real `httpapi` test server.
- [x] W03 HTTP adapter (`internal/adapter/httpapi`). Shared HTTP acceptance driver and non-native boundary verification pass. [Evidence](docs/verification/specs/W01.md).
  - Spec: [W01-web-server](docs/specs/W01-web-server.md)
  - [x] REST routes mirroring every application operation. `GET /api/events` Server-Sent Events carry the job, metrics and capture streams. Also `/healthz`.
  - [x] Go 1.22+ `ServeMux` patterns in one `routes.go`. Per-route request limits. One error-mapping function.
  - Assert: an acceptance driver over `httptest` passes the H07 specifications; SSE reconnect with `seq`/`Snapshot` loses no terminal state.
- [ ] W04 Security baseline: implementation, automated security checks and non-native boundary verification pass; native Edge/Chrome acceptance remains open. [Evidence](docs/verification/specs/W04.md).
  - Spec: [W04-web-security](docs/specs/W04-web-security.md)
  - [x] Loopback bind by default.
  - [x] Host allowlist against DNS rebinding.
  - [x] Per-launch `crypto/rand` token, delivered in a URL fragment and exchanged for an `HttpOnly; SameSite=Strict` cookie, required on every `/api` route including SSE.
  - [x] `http.NewCrossOriginProtection`.
  - [x] No CORS.
  - [x] CSP and `nosniff`.
  - [x] `ReadHeaderTimeout` and `MaxBytesReader`.
  - [x] Caps on concurrent jobs and SSE subscribers.
  - Assert: tests reject a foreign Host, a cross-origin POST, a missing or invalid token and oversized bodies; the token never appears in logs, history or responses.
- [x] W05 Remote exposure (explicit opt-in). TLS validation and warning verified. [Evidence](docs/verification/specs/W01.md).
  - Spec: [W01-web-server](docs/specs/W01-web-server.md)
  - [x] `--listen` on a non-loopback address requires TLS (user certificate with fingerprint display, per W01) and an explicit acknowledgement flag; print a warning.
  - Assert: non-loopback without TLS fails to start; the documented threat model is reviewed.
- [ ] W06 Capability honesty in server mode. Browser/API parity verified; alternate account/consent acceptance remains open. [Evidence](docs/verification/specs/W01.md).
  - Spec: [W01-web-server](docs/specs/W01-web-server.md)
  - [ ] Report permission, consent and session differences (for example, Wi-Fi location consent or capture permission under a service account) as capability states.
  - Assert: the UI shows the server's actual capability states, not the desktop's.

Interfaces exit: a parity matrix (capability × desktop/CLI/web) is recorded in `docs/verification/` with evidence; every gap is an explicit, documented capability state.

## 8. Beta — portability, hardening and toolkit expansion

- [ ] B01 Linux native adapters. Native acceptance remains open.
  - [x] Select documented distro baseline; implement routes/counters/neighbors (netlink) and NetworkManager integration. Ubuntu 24.04 LTS is the baseline; Debian 13 and Kali are best-effort ([ADR 010](docs/adr/010-linux-baseline.md)). Routes and counters read netlink, with a procfs fallback. Neighbors already use netlink. NetworkManager GetSecrets is the only key path and reports `dependency_missing` when NetworkManager is absent. Native acceptance is not closed by CI.
  - [x] Wi-Fi metadata via nl80211 (`mdlayher/wifi`) and key retrieval through authorized NetworkManager secret APIs; handle systems without NetworkManager. Signal is a 0–100 value. Native acceptance remains open.
  - [x] Capture with libpcap and `CAP_NET_RAW`; avoid elevating the full UI or server. The `libpcap` backend opens one interface only in a Linux build tagged `pcap`. A missing capability is `permission_required` and names `CAP_NET_RAW`. The UI and the web server are not elevated. Native acceptance remains open.
  - Assert: native evidence covers real adapter, denied permission, restart and no saved PSK.
- [ ] B02 macOS native adapters.
  - [x] Select supported macOS/architecture baseline and native bridge approach. macOS 13 Ventura or later, Apple Silicon (`darwin/arm64`) only. The bridge is cgo with a thin Objective-C layer. No Intel, universal, or Swift helper for 1.0. [ADR 007](docs/adr/007-macos-baseline.md).
  - [ ] Implement interfaces/routes/counters and current Wi-Fi metadata. The routing sysctl reader and CoreWLAN call are implemented ([ADR 013](docs/adr/013-macos-wifi.md)). On real Apple Silicon hardware, verify whether an ad-hoc-signed `.app` can read CoreWLAN SSID/BSSID and whether Location Services prompts. Show an explicit capability state when that access is unavailable. Not accepted from documentation or cross-compilation. [ADR 007](docs/adr/007-macos-baseline.md), [ADR 009](docs/adr/009-signing-and-provenance.md).
  - [ ] Keychain feasibility spike. The lookup uses the AirPort generic-password item and does not prompt. Which status an ad-hoc app receives is still a hardware step. [ADR 013](docs/adr/013-macos-wifi.md).
  - [x] BPF capture permissions: detect ChmodBPF group access and never change device permissions. `capture_live` on macOS reports whether this user can open `/dev/bpf` and names the `access_bpf` group when access is missing. The probe never changes device permissions. Live capture is still not implemented. Native acceptance remains open. [ADR 011](docs/adr/011-no-privileged-helper.md).
  - [x] Key-reveal authorization, with an explicit unsupported state if OS policy prevents access. A missing entitlement is `unsupported`. A lookup that would prompt is `permission_required`. The call does not prompt. Native acceptance remains open. [ADR 013](docs/adr/013-macos-wifi.md).
  - Assert: native evidence records actual supported behaviors rather than inferring parity from compilation.
- [ ] B03 Privilege boundary. No privileged helper and no helper IPC for 1.0 ([ADR 011](docs/adr/011-no-privileged-helper.md)). Install and recovery acceptance remains open.
  - [x] Use existing OS authorization where possible; implement helper only for operations that require it. Resolved by ADR 011: a helper is out of scope for 1.0. Linux documents `sudo setcap cap_net_raw+eip`. macOS detects `access_bpf` and never changes device permissions. Windows detects a user-installed Npcap and never redistributes it. A missing permission is a capability state plus the fix. The UI and web server stay unprivileged.
  - [x] Authenticated local IPC, restricted operations, caller verification and bounded requests. Resolved by ADR 011: helper IPC is out of scope for 1.0. There is no helper to call.
  - [ ] Install/update/uninstall and recovery behavior per OS.
  - Assert: unauthorized caller and malformed request fail; ordinary UI and web server remain unprivileged.
- [ ] B04 Connection/process inventory and throughput. Linux `/proc` mapping, loopback throughput and the iperf3 `-J` parser are in [B04 evidence](docs/verification/specs/B04.md). Live Windows and macOS socket mapping are `unsupported`. Hardware confirmation remains open.
  - [x] Native per-OS socket-to-process mapping with permission and race handling. Linux maps `/proc` socket inodes to a process. A missing inode is omitted. An unreadable socket table is `permission_required`. Windows and macOS return `unsupported`. Native acceptance remains open.
  - [x] Separate per-interface counters from per-process attribution; no invented bandwidth estimates. The connection table has neither interface counters nor a rate.
  - [x] Bounded Go throughput client/server for owned endpoints (not iperf3-wire-compatible); optional iperf3 `-J` adapter. The exchange is `127.0.0.1` only, 1ms..10s, cancellable, and reports endpoint, duration and byte count. The `-J` parser does not execute iperf3 and does not copy `bits_per_second`.
  - Assert: fixture processes match reported connections; active throughput has explicit endpoint/duration and cancellation.
- [x] B05 Capture and adapter resilience. Automated bounds and loss reporting are in [B05 evidence](docs/verification/specs/B05.md). Pulling a cable is not covered.
  - [x] Reassembly limits, IPv6 extensions, truncated packets and sustained capture backpressure. Decode reports truncation, follows a complete IPv6 hop-by-hop header, and does not reassemble TCP or IPv6 fragments. Artifact growth stops at 10000 packets or the reserved 64 MiB budget.
  - [x] TShark broad decoding (`-T ek`/`-T fields`, streaming) where Go support is incomplete; document supported protocol levels. TShark stays inventory-only ([ADR 006](docs/adr/006-native-service-inspection.md)). Supported levels are in [capture decoding](docs/capture-decoding.md).
  - [x] Test interface disappearance, tool termination, disk full and corrupted input. A provider loss is a failed job whose history omits packet text. Pktmon cancel always stops. Temporary-directory and export writer failures do not echo private paths. Corrupted captures are rejected. Pulling a cable stays on the Windows runbook.
  - Assert: bounded workload stays within alpha-established budgets and reports data loss accurately.
- [ ] B06 Persistence, privacy and compatibility.
  - [x] SQLite upgrade/backup/recovery, retention, export and migration fixtures. Schema version is 1. The committed SQL reopens without loss, and a copy of the closed database file keeps settings and a finished job. Recovery, retention and a rejected future schema were already tested. [Evidence](docs/verification/specs/B06.md).
  - [x] Validate settings and versioned result formats; redact diagnostic bundles.
    - [x] Stored preferences reject malformed, oversized or unsupported values without echoing contents.
    - [x] Versioned CLI JSON and discovery JSON/CSV contracts tested; discovery history stores counts without targets.
    - [x] Redacted diagnostic bundles and compatibility fixtures for all exported formats. The on-demand zip is implemented ([ADR 012](docs/adr/012-diagnostic-bundle.md), [evidence](docs/verification/specs/B06.md)). pcap, pcapng, discovery CSV and neighbors CSV fixtures import.
  - [x] Test translation key parity and all permission/error UI flows. English and PT-BR catalog keys match. Every domain error code and capability state has a UI label. Operating-system permission dialogs stay on the runbooks. [Evidence](docs/verification/specs/B06.md).
  - Assert: prior beta data upgrades without silent loss; secret fixtures appear in no ordinary artifact.
- [ ] B07 Native packaging matrix. Archive names, notices, uninstall and the writable database path are in [packaging](docs/packaging.md). Clean-machine WebView checks remain open. [Evidence](docs/verification/specs/B07.md).
  - [ ] Build Windows, Linux and macOS artifacts on native hosts; verify actual WebView prerequisites. CI builds the three archives. A clean machine has not confirmed the WebView prerequisite. Native acceptance remains open.
  - [ ] Dependency doctor and install instructions; clean-machine smoke checks. README and `doctor` already name the Ubuntu 24.04 baseline and the WebView packages. A clean-machine smoke check was not run.
  - [x] Dependency/source notices, installer uninstall behavior and writable-data locations. Each archive includes `THIRD_PARTY_NOTICES.txt` and `dependencies.json`. There is no installer. Uninstall is deleting the unpacked directory. `history.db` stays in the per-user config directory. See [packaging](docs/packaging.md).
  - Assert: verified matrix names OS/version/architecture/provider; untested combinations are not advertised.
- [ ] B08 TLS and certificate audit module (`internal/tlsaudit` + adapters).
  - Spec: [B08-tls-audit](docs/specs/B08-tls-audit.md)
  - Implementation and non-native checks completed; Windows platform-verifier and native GUI acceptance remain pending. [Evidence](docs/verification/specs/B08.md).
  - [x] Chain report (validity, SAN coverage, key/signature algorithms, AIA/OCSP/CRL, stapling, SCT count), protocol matrix via `MinVersion`/`MaxVersion`, zlint findings. Local fixture tests and both browser transports verified; native checks remain open.
  - [x] Optional zcrypto legacy-protocol observation behind a labelled switch, isolated from kali-tools' own connections. Deferred beyond 1.0 (§11). zcrypto has no releases and is research-grade. The B08 spec already excludes a handshake client. zlint still uses it only to parse certificates.
  - Assert: controlled local certificates cover expired, self-signed, mismatch, weak key and legacy-only servers; no exploit checks.
- [x] B09 DNS inspection module.
  - Spec: [B09-dns-inspection](docs/specs/B09-dns-inspection.md)
  - [x] System vs explicit resolver comparison, SOA/NS consistency, read-only DNSSEC chain and DoH/DoT reachability. Unit checks, adapters, the Diagnostics panel and Docker AC-10 passed. [Evidence](docs/verification/specs/B09.md). GitHub run [39970855209](https://github.com/AlexGalhardo/kali-tools/actions/runs/39970855209).
  - [x] Decide `miekg/dns`: `github.com/miekg/dns` v1 (BSD-3-Clause), latest v1.1.x at decision time v1.1.73, not `codeberg.org/miekg/dns`. `go.mod` requires v1.1.73 for DS parsing. [dependencies](docs/dependencies.md).
  - Assert: Docker lab DNS fixtures cover each check; blocked port-53 egress is a distinct result.
- [x] B10 Local service discovery (read-only).
  - Spec: [B10-local-discovery](docs/specs/B10-local-discovery.md)
  - [x] mDNS/DNS-SD (hashicorp/mdns), SSDP M-SEARCH listener and SNMP read-only queries against user-declared owned devices. Docker AC-9 passed on GitHub run [39981075045](https://github.com/AlexGalhardo/kali-tools/actions/runs/39981075045). Native AC-11 and AC-12 were not run. [Evidence](docs/verification/specs/B10.md).
  - Assert: lab fixtures announce known services; scope rules match A04.
- [x] B11 HTTP metadata.
  - Spec: [B11-http-metadata](docs/specs/B11-http-metadata.md)
  - [x] `httptrace` phase timings, security-header presence report and redirect chain without following. [Evidence](docs/verification/specs/B11.md).
  - Assert: fixtures cover missing/present headers and redirect loops within limits.

Beta exit: the agreed capability matrix is verified on all three OSes and all three interfaces. Limitations are approved and documented. No data loss, privilege-boundary, web-server-security or critical parser defects remain unresolved.

## 9. Stable 1.0

- [x] R01 Freeze the supported CLI commands, HTTP API, schemas, settings and platform matrix. The freeze is the [CLI contract](docs/cli.md), the [HTTP contract](docs/http.md), schema version 1, settings `language` and `retention_days`, and the [packaging matrix](docs/packaging.md). Hardware acceptance is not part of this freeze. [Evidence](docs/verification/specs/R01.md).
  - [x] Document API compatibility, errors, permission requirements and unavailable capabilities. [HTTP contract](docs/http.md) and [CLI contract](docs/cli.md). `TestHTTPContractMatchesRoutes` fails if a route, error status or body limit drifts. The platform matrix is [packaging](docs/packaging.md). An operating system that is not in that table is not supported.
  - [x] Adopt MIT for project code, retain third-party/vendored licenses and collect notices for pinned Go/production frontend dependencies; Nmap/TShark remain excluded.
  - Assert: README and doctor agree with tested artifacts; no promise of all Kali tools or all protocol payloads.
- [ ] R02 Release qualification.
  - [ ] Re-run meaningful regression suites after final changes and review evidence.
  - [ ] Test native install, upgrade, rollback/data recovery, uninstall and offline operation.
  - [ ] Validate agreed numeric performance budgets and absence of critical/high unresolved vulnerabilities.
  - [ ] Confirm PT-BR/English and keyboard/accessibility acceptance.
  - [ ] External review of the web-server threat model.
  - Assert: release checklist links reproducible results; no unfinished blocker is marked complete.
- [ ] R03 Distribution and provenance.
  - [x] Signing decision (2026-10-23): no Apple Developer Program, Developer ID, notarization, or Windows Authenticode, EV, or Azure Trusted Signing, now or for 1.0. Releases are vendor-unsigned. macOS arm64 keeps the ad-hoc linker signature. Every artifact gets a SHA-256 checksum. GitHub artifact attestations are the provenance mechanism. [ADR 009](docs/adr/009-signing-and-provenance.md).
  - [ ] Produce checksums, dependency inventory, source archive and release notes for the qualified 1.0. Development drafts already publish SHA-256 checksums.
  - [x] Vendor signatures are not configured. Development artifacts stay labeled unsigned. An ad-hoc macOS signature is not a vendor signature.
  - [ ] Create RC, test it, then tag 1.0.0 only after acceptance.
  - Assert: version matches CLI, desktop metadata, tag and changelog; artifacts correspond to tagged source.
- [ ] R04 Optional automation activation.
  - [x] Owner explicitly requested full GitHub Actions activation on 2026-10-19; active source replaced dormant copies.
  - [ ] Configure protected release environment and minimum repository permissions.
  - Assert: activation follows the recorded owner instruction; pull requests cannot create releases and qualified releases retain acceptance gates.

## 10. Verification strategy

**Test types.** Use behavioral tests for:

- packet parsing;
- rate calculations;
- state transitions;
- timeouts;
- process cancellation;
- persistence;
- scoped execution.

Avoid tests that merely restate the implementation.

**Architecture tests.** These cover:

- contract suites for every driven port (fakes cannot drift);
- interface-agnostic acceptance specifications run through each driving adapter;
- the mechanical dependency-rule check.

**Evidence categories.** Native hardware/API checks, container tests and browser tests are separate categories:

- Browser tests validate UI flows, but do not prove native Wails IPC works.
- `httptest` checks do not prove browser security behavior. Keep explicit Host, Origin and token tests.

**Required checks.**

- Go: format, lint (with depguard), vet for three GOOS targets, test, race and vulnerability checks.
- Frontend (Bun): lint, typecheck, test and build.
- Parser fuzz smoke.
- Docker integration.
- Native Wails build and smoke.

None of these may be reported as passed unless they actually ran.

**Threat boundaries to verify.** Test malformed input and resource limits at each boundary:

- untrusted packet bytes;
- external tool output;
- user-provided targets and files;
- Wails calls;
- HTTP requests (Host, Origin, token, body size);
- export paths.

There is no privileged helper and no helper IPC in 1.0 ([ADR 011](docs/adr/011-no-privileged-helper.md)). Capture happens only in an explicitly started session. No cloud account or telemetry is required.

## 11. Deferred or out of scope

**Deferred beyond 1.0.** Each of these is a separate proposal:

- third-party downloaded plugins;
- firewall modification;
- traffic shaping;
- cloud synchronization;
- automatic TLS interception;
- JA4+ fingerprints under the FoxIO License;
- OSINT/CT-log queries for third-party domains;
- optional zcrypto legacy-protocol observation. It is research-grade, has no releases, and [B08](docs/specs/B08-tls-audit.md) already excludes a handshake client. zlint may keep the parser. It must not carry kali-tools' own TLS connections.

**Out of scope.**

- a privileged helper and local helper IPC for 1.0 ([ADR 011](docs/adr/011-no-privileged-helper.md));
- credential attacks;
- exploitation frameworks and exploit checks;
- Wi-Fi monitor mode, injection, deauthentication and handshake capture;
- spoofing/MITM;
- SYN/stealth scanning and OS fingerprinting;
- brute-force enumeration;
- evasion.

The plan does not claim feature parity with every Kali package.

## 12. Current evidence and next task

**Checklist reviews.**

- 2026-10-19: Go tests/vet, capture race tests and 11 frontend workflow tests passed. See [development evidence](docs/verification/0.1.0-alpha.1-dev.md).
- 2026-10-21: architecture review and S10 reorganization, verified as recorded in S10.
- 2026-10-21: S11 code review and formatting enforcement. Version bumped to `0.1.0-alpha.2-dev`. See [evidence](docs/verification/0.1.0-alpha.2-dev.md). Owner instruction: CI/CD runs on GitHub, not locally.

**History.**

- Research baseline committed as `c4bf7fe`.
- Toolchain:
  - The module requires Go 1.27.2. Wails 2.17.0 builds the native Windows shell.
  - Bun frozen installation and build pass with the workspace `bun.lock`.
  - npm remains the setup/Wails fallback.
- GitHub authorization completed and the planning baseline was pushed to `main`. Use the task-local GitHub CLI configuration outside the repository for subsequent pushes.
- Development implementation checkpoint: these exist, with exact evidence and limitations in docs/verification/0.1.0-alpha.1-dev.md:
  - real CLI/core;
  - SQLite;
  - Windows providers;
  - React/Wails shell;
  - bounded capture/import prototype;
  - tests;
  - authorized CI/CD source;
  - the private Docker laboratory.
- Six setup scripts, now under `scripts/setup/`, and a step-by-step README cover Windows, Linux and macOS, each with and without Docker. Script syntax is verified; cross-platform native acceptance is still pending.

**Session scope update (2026-10-19).**

- The owner:
  - renamed the final project/repository to kali-tools;
  - requested commits, push, a changelog and a GitHub release with native binaries;
  - delegated open-source housekeeping;
  - explicitly activated CI/CD.
- Origin is https://github.com/AlexGalhardo/kali-tools.
- MIT is adopted. Unsigned development artifacts and release drafts disclose the remaining qualification gates.

**Session scope update (2026-10-21).** The owner confirmed three interfaces (Wails GUI primary, a web server with the same UI, a complete CLI) and requested:

- a Clean/Hexagonal architecture;
- a repository reorganization;
- this plan revision.

**Next tasks, in order** (each is a spec in [docs/specs](docs/specs/README.md)):

1. A05 implementation is verified on Windows. Linux-host pure-function coverage meets the spec gate; manual consent/reveal acceptance remains open. W01/W04 implementation and non-native acceptance are verified; native account/browser/desktop gates remain open.
2. H09 is accepted: ADR 004 keeps stdlib `flag` (owner, 2026-10-23). H08 mechanical enforcement and hosted positive/negative proofs pass.
3. Retain the W01/W04 native gates; the server's shared acceptance driver, bounds and shutdown verification are recorded.
4. B11 HTTP metadata is implemented and its non-native checks pass. A05, A06, A01 and B08 remain open on native acceptance. B09 is done, including Docker AC-10 on GitHub run 39970855209. B10 is done, including Docker AC-9 on GitHub run 39981075045. Its native AC-11 and AC-12 stay open. B02's macOS baseline, A08's pagination decision and R03's vendor-unsigned provenance are recorded. The A07 NPSL decision is closed by ADR 006. The service-inspection result type is `ServiceInspection` (nested `Inspection*`); JSON field names are unchanged, so R01 does not freeze an Nmap type name. Keep H01–H07 native checks visible; do not treat them as headless passes. The checklists are [Windows](docs/verification/native-acceptance-windows.md), [Linux](docs/verification/native-acceptance-linux.md) and [macOS](docs/verification/native-acceptance-macos.md). None of them closes a box until the pasted evidence is recorded.

**Constraints.**

- Hardware acceptance, privilege architecture and Linux/macOS parity are not inferred from compilation. Signing is decided: vendor-unsigned releases, SHA-256 checksums and GitHub artifact attestations ([ADR 009](docs/adr/009-signing-and-provenance.md)). Do not add Apple or Microsoft certificates.
- Do not mark the plan complete. The owner's current explicit 0.4.0 checkpoint-publication request supersedes the earlier no-tag instruction only for that disclosed development release after its checks pass.

## 0.5.0 owner checkpoint (2026-10-22)

The owner requested a 20-minute completion/publication checkpoint. C01 is implemented; W01/W04 now provide HTTP/SSE browser mode and automated security verification. This is an unsigned development preview, with open native acceptance and remaining SDD gates explicitly retained. See [release verification](docs/verification/0.5.0.md). No published tag or release content was rewritten.

CI regression follow-up: [serve test isolation](docs/verification/ci-serve-tests.md) fixes clean-checkout and non-root laboratory assumptions. This changes test setup only and does not close native acceptance gates.
