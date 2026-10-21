# Spec Driven Development (SDD) for kali-tools

Status: active, created 2026-10-21. Audience: implementation agents (Codex and others) and human reviewers.

Every non-trivial change in kali-tools starts from a written specification in this folder. A spec is a contract, not a suggestion. It names:

- the exact files to touch;
- the Go and TypeScript signatures to create;
- the behavior, as Given/When/Then acceptance criteria;
- the tests that prove it;
- the commands that verify it.

An agent implements **one spec at a time**, in the order given below, and does not invent requirements that the spec does not state.

## 1. Why SDD here

kali-tools has three interfaces (Wails GUI, web server, CLI) over one Go core, plus platform adapters that can only be accepted on real hardware. Without precise specs, three things tend to go wrong:

- Logic is duplicated per interface.
- Security rules are skipped.
- "Compiles on Linux" gets mistaken for "works on Linux".

Specs make the boundaries explicit. They also make review mechanical, because each acceptance criterion maps to a test.

## 2. Mandatory reading before any spec

1. [AGENTS.md](../../AGENTS.md): project rules. It also lists the skills to load first: `karpathy-guidelines`, `ponytail` and `clean-architecture-in-practice`.
2. [docs/architecture.md](../architecture.md): the binding architecture, the dependency matrix and the known deviations D1–D8.
3. [docs/architecture-research.md](../architecture-research.md): the reasoning behind each pattern, including the mandatory web security in §8.
4. The spec itself, end to end, including **Non-goals**.

## 3. The SDD loop (follow it exactly)

```text
1. READ      spec + referenced files; confirm every "Current state" claim still holds.
             If it does not hold, STOP and report the difference — do not improvise.
2. TEST      write the failing tests listed under "Tests" first (or alongside, for refactors
             where existing tests already pin behavior). Run them; confirm they fail for the
             right reason.
3. BUILD     implement the smallest change that satisfies the acceptance criteria, touching only
             the files listed under "Files". New files only where the spec names them.
4. VERIFY    run every command under "Verification" and every gate/DA in §12. All must pass.
             Write docs/verification/specs/<ID>.md (README §5.3) with real outputs.
5. DOCUMENT  update docs/architecture.md (and any doc the spec names) in the same change;
             tick only the PLAN.md checkboxes whose assertions now have evidence.
6. COMMIT    one Conventional Commit per spec (or per numbered step for large specs),
             e.g. `refactor(app): extract job manager (H02)`.
```

Rules inside the loop:

- **No scope creep.** If you see a problem outside the spec, record it under "Follow-ups" in your report. Do not fix it.
- **No behavior change in refactor specs.** Existing tests must pass unchanged unless the spec says that a test moves or changes.
- **Never weaken a test** to make it pass. Never delete a security assertion.
- **Never claim native acceptance** (Wi-Fi, capture, WebView, Linux/macOS hardware) from unit tests or cross-compilation. Specs mark these criteria `[native]`. Leave them unchecked unless a human runs them.
- **Owner-authorized headless continuation (2026-10-21):** implement specs sequentially and proceed when the predecessor's implementation and non-native acceptance pass, while retaining its explicit `native acceptance pending` status. The owner requested uninterrupted headless implementation, commits and pushes to `main`, without releases or waiting for remote CI. This overrides the execution-order requirement for a predecessor to be fully Done; it does not approve native checks, release qualification, or shipping `serve` before W04 security is implemented and verified.
- **Formatting is automatic.** The Lefthook pre-commit runs `golangci-lint fmt` (gofmt -s + goimports), `golangci-lint run` and `go vet`. Run `lefthook install` once per clone.

## 4. Spec template

Every spec file uses these 12 sections, in this order:

```markdown
# <ID> <Title>

| Field | Value |
| --- | --- |
| PLAN tasks | H0x / C0x / W0x / A0x / B0x |
| Status | Ready / In progress / Done (date, commit) |
| Depends on | other spec IDs that must be Done first |
| Risk | Low / Medium / High + one line why |

## 1. Goal
One paragraph: the outcome, in user or architecture terms.

## 2. Non-goals
Explicit list of what NOT to do in this spec.

## 3. Current state
Facts about the code today, with file:line references. The implementer re-verifies them.

## 4. Design
Target packages, types and function signatures (Go/TS code blocks), data flow,
error codes, limits (numbers), concurrency model. No "TBD".

## 5. Files
Create / Modify / Delete lists with one-line purpose each.

## 6. Implementation steps
Numbered, small, each leaving the build green.

## 7. Acceptance criteria
AC-1 … AC-n in Given/When/Then form. Mark hardware-only items [native].

## 8. Tests
Test names, packages and what each asserts (maps to ACs).

## 9. Verification
Exact commands to run.

## 10. Documentation updates
Which docs/sections change.

## 11. Risks and rollback
What can break and how to revert.

## 12. Quality and delivery assertions
QG table (inherited global gates, list only the ones that apply + any tightened threshold)
and DA-1 … DA-n spec-specific delivery assertions, each with: the check, the exact
command or file evidence, the pass threshold, and who/what verifies it (CI, hook, reviewer, [native]).
Ends with the evidence record path: docs/verification/specs/<ID>.md.
```

## 5. Quality gates and delivery assertions (auditable)

A spec is **Done** only when every applicable gate below passes **and** its evidence record exists. "Looks good" is not evidence: each assertion names a command or an artifact a reviewer can re-run or open.

### 5.1 Global quality gates (QG), inherited by every spec

| ID | Gate | Command / artifact | Pass threshold | Enforced by |
| --- | --- | --- | --- | --- |
| QG-1 | Formatting | `golangci-lint fmt --diff ./...` | empty diff | pre-commit, pre-push, CI |
| QG-2 | Lint (incl. depguard after H08) | `golangci-lint run ./...` | `0 issues.` | pre-commit, CI |
| QG-3 | Vet on all targets | `go vet ./...`, `-tags desktop .`, `GOOS=linux`, `GOOS=darwin` | exit 0 | pre-commit (host), CI |
| QG-4 | Unit/integration tests | `go test ./...` | all pass, no skipped test added by the spec without a reason | pre-push, CI |
| QG-5 | Race detector | `go test -race ./...` (needs cgo; CI Linux) | no race reports | CI |
| QG-6 | Coverage of touched packages | `go test -coverprofile=cover.out ./<pkg>/... && go tool cover -func=cover.out` | ≥ 80 % statements for new packages; never lower than the pre-spec value for existing packages (record before/after) | reviewer from evidence |
| QG-7 | Vulnerabilities | `govulncheck ./...` | no reachable vulnerability | pre-push, CI |
| QG-8 | Frontend | `bun run typecheck && bun run lint && bun run build && bunx playwright test` | all pass; workflow count never decreases | CI |
| QG-9 | Binding drift | regenerate Wails bindings, then `git diff --exit-code frontend/wailsjs` | empty diff | CI (after H05) |
| QG-10 | Dependency rule | depguard rules (H08) / `go list -deps` assertions | no forbidden edge | CI |
| QG-11 | Secrets and payloads | redaction tests with synthetic secrets (`synthetic-psk`, synthetic token) asserting absence from history, logs, errors and DTO JSON | tests present and passing for every new output path | reviewer |
| QG-12 | Bounds | every new channel/buffer/body/parser/subprocess has a named limit constant and a test at limit and limit+1 | 100 % of new bounded resources | reviewer |
| QG-13 | Conventional commit + scope | `commitlint` (commit-msg hook); commit references the spec ID | hook passes; message contains `(Hxx)`-style ID | hook, reviewer |
| QG-14 | Docs in sync | `docs/architecture.md`, PLAN.md checkbox, CHANGELOG `[Unreleased]` entry linking the commit | present in the same PR/commit series | reviewer |
| QG-15 | Markdown | `node scripts/markdown.mjs` | `Markdown checks passed` | CI |

Native-only acceptance (`[native]`) never passes from CI; it requires a human run recorded with OS build, hardware and date.

### 5.2 Delivery assertions (DA), per spec

Each spec lists DA-1 … DA-n in its §12: concrete, binary, re-runnable statements tied to the spec's goal, for example "`go list -deps ./internal/app | grep internal/adapter` prints nothing", "`kali-tools jobs list --json` matches `tests/fixtures/cli/jobs-list.golden.json`", "server refuses `Host: evil.test` with 403 (TestForeignHostRejected)". A DA without a command, test name or artifact is invalid.

### 5.3 Evidence record (audit trail)

For every Done spec, create `docs/verification/specs/<ID>.md`:

```markdown
# Evidence <ID> — <title>
- Date (UTC) / OS + build / Go + Bun versions / host type (dev, CI run URL)
- Commits: <sha> <subject> (one per line, linked)
- Baseline before change: coverage per touched package, test count, Playwright workflow count
## Gates
| Gate | Command | Result | Output excerpt or CI link |
## Acceptance criteria
| AC | Test / procedure | Result |
## Delivery assertions
| DA | Command / artifact | Result |
## Native checks
| Check | Procedure | Result or "not run" |
## Deviations and follow-ups
```

Rules: paste real output (trimmed), never paraphrase a failure as a pass; "not run" is an allowed, honest result; a reviewer must be able to reproduce every row from the commit SHA. PLAN.md checkboxes link to this record.

## 6. Shared conventions for all specs

### 6.1 Go

- Go 1.27, single module `github.com/AlexGalhardo/kali-tools`.
- Imports come in three groups (stdlib, third-party, `github.com/AlexGalhardo/kali-tools/...`). This is enforced by goimports.
- `context.Context` is the first parameter of anything that blocks. Never store a context in a struct; the only exception is the Wails runtime context inside `internal/adapter/wailsui`.
- Interfaces are declared by the consumer. Constructors return concrete types. There is no DI framework.
- Errors:
  - Use `*domain.Error{Code, Message}` for every error that reaches a user.
  - Wrap internal errors with `%w`.
  - Stable codes: `invalid_input`, `unsupported`, `permission_required`, `dependency_missing`, `location_consent_required`, `not_found`, `timeout`, `cancelled`, `failed`, `certificate_invalid`, `no_connection`, `conflict`, `unauthorized`, `forbidden`.
  - Do not invent new codes without adding them to `internal/domain` and to both i18n catalogs.
- Limits are named constants next to the code that enforces them. Every limit has a test at the boundary value and at boundary + 1.
- Every channel, buffer, subprocess output, request body and parser input is bounded.
- Never concatenate shell commands. Use `exec.CommandContext(ctx, path, args...)` with validated arguments.
- Secrets (Wi-Fi keys, tokens) and packet payloads never reach logs, history, SQLite or error messages.

### 6.2 Tests

- Use table-driven tests. Prefer black-box `_test` packages for capability and domain code.
- Fakes are hand-written. Each fake of a driven port must pass the same `internal/porttest` contract suite as the real adapter (from H07 on).
- No `time.Sleep` for synchronization. Use channels, `t.Context()`, and deadlines of at most 3 s in unit tests.
- Gate Docker tests behind `//go:build integration`. Hardware tests are documented as manual `[native]` steps.

### 6.3 Frontend

- React and TypeScript with strict typing. Biome formats and lints.
- `frontend/src/shared/api/bridge.ts` is the only module that talks to Go.
- Every user-visible string goes in both catalogs (`en`, `pt`) in `shared/i18n/i18n.ts`.
- Playwright workflows use contract-faithful fakes.

### 6.4 Standard verification block

Each spec's §9 lists these commands plus its own:

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
```

## 7. Spec index and execution order

Implement top to bottom. A spec may start only when every spec in its **Depends on** field is Done.

| Order | Spec | PLAN | Summary |
| --- | --- | --- | --- |
| 1 | [H01 Bootstrap](H01-bootstrap.md) | H01 | Shared wiring and `run()` composition roots |
| 2 | [H02 Job manager](H02-job-manager.md) | H02 | `internal/job` lifecycle, bounded fan-out, metrics/capture streams |
| 3 | [H03 Driven ports](H03-driven-ports.md) | H03 | Consumer-side ports for storage, platform, tools, Wi-Fi, capture |
| 4 | [H04 Application facade](H04-application-facade.md) | H04 | One `app.Application` used by every interface |
| 5 | [H05 Driving adapters](H05-driving-adapters.md) | H05 | Extract `adapter/cli` and `adapter/wailsui` |
| 6 | [H06 DTOs and errors](H06-dtos-and-errors.md) | H06 | Frontend contract types, error mapping, consent state |
| 7 | [H07 Test harness](H07-test-harness.md) | H07 | `porttest` contract suites, `tests/acceptance` drivers |
| 8 | [H08 Dependency rule](H08-dependency-rule.md) | H08, H09 | depguard enforcement and the CLI parser ADR |
| 9 | [C01 Complete CLI](C01-cli.md) | C01–C04 | Command tree, jobs in terminal, SSH ergonomics, contract freeze |
| 10 | [W01 Web server](W01-web-server.md) | W01–W03, W05, W06 | Shared assets, transport-neutral bridge, `httpapi`, remote opt-in |
| 11 | [W04 Web security](W04-web-security.md) | W04 | Mandatory local-server security baseline |
| 12 | [A05 Wi-Fi consent](A05-wifi-consent.md) | A05 | Windows 24H2 location consent and reveal states |
| 13 | [A06 Capture backends](A06-capture-backends.md) | A06 | Npcap detection and Pktmon adapter spikes |
| 14 | [A01 Neighbors](A01-neighbors.md) | A01 | Passive ARP/ND neighbor inventory |
| 15 | [B08 TLS audit](B08-tls-audit.md) | B08 | Certificate chain report, protocol matrix, zlint |
| 16 | [B09 DNS inspection](B09-dns-inspection.md) | B09 | Resolver comparison, SOA/NS consistency, DNSSEC, DoH/DoT |
| 17 | [B10 Local discovery](B10-local-discovery.md) | B10 | mDNS, SSDP and SNMP read-only inventory |
| 18 | [B11 HTTP metadata](B11-http-metadata.md) | B11 | `httptrace` timings, security headers, redirect chain |

W04 is listed after W01 for reading order. **No build that contains `kali-tools serve` may ship or be committed to `main` without W04 Done.** Implement W04 together with W01 step 6.

## 8. Reporting back (what the implementing agent returns)

```text
Spec: <ID>   Commit(s): <sha…>
ACs: AC-1 ✅ (TestX) · AC-2 ✅ (TestY) · AC-5 [native] not run
Gates: QG-1..QG-15 → pass / n.a. (reason) · DAs: DA-1 ✅ …
Evidence: docs/verification/specs/<ID>.md
Verification: <each command → pass/fail>
Docs updated: <files>
Deviations from spec: <none | list with reason>
Follow-ups (not done): <list>
```
