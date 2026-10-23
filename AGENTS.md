# Contributor instructions

Release policy: always follow SemVer 2.0.0 and Keep a Changelog 1.1.0, as specified in CONTRIBUTING.md. Run `node scripts/release-policy.mjs`; keep app/package/Wails versions, Git tags, dated changelog entries and release notes consistent. Never rewrite published release contents or tags.

Use English for source, documentation and commits. Keep business logic in Go services shared by the Wails GUI, the web server and the CLI. Read docs/architecture.md before adding, moving or wiring packages, and keep it current; the reasoning is in docs/architecture-research.md. Never concatenate shell commands. Bound network operations, parser input, event queues and subprocess output. Never persist Wi-Fi secrets or packet payloads in diagnostic history. GitHub Actions activation was explicitly authorized on 2026-10-19; maintain pinned workflows under .github/workflows with minimum permissions. Mark PLAN.md tasks complete only with actual verification evidence. Run `go test ./...`, `go vet ./...` and the frontend check/build after relevant changes. Do not claim native platform acceptance from cross compilation. Use the MIT project license. No release tag before qualification and owner decisions on signing. Development artifacts must disclose unsigned status and open acceptance gates.

## Mandatory start of every task

Before writing code, plans or reviews in this repository, always read and apply these three skills, in this order:

1. `.agents/skills/karpathy-guidelines/SKILL.md`: think before coding; simplicity first; surgical changes; goal-driven verification.
2. `.agents/skills/ponytail/SKILL.md`: the smallest complete solution.
3. `.agents/skills/clean-architecture-in-practice/SKILL.md`: the dependency rule, ports and adapters, and use cases. It is the reference for docs/architecture.md.

Do this for every task, including small ones. If a skill conflicts with docs/architecture.md, the architecture document wins and the conflict must be reported.

## Release naming

GitHub releases follow the pattern: `kali-tools X.Y.Z — description`. All development releases are unsigned and published with status note: "unsigned development preview; hardware qualification remains open. Not a release-qualified build."

## Implementation workflow

- Open work is specified in docs/specs/ (Spec Driven Development). Implement exactly one spec at a time, following docs/specs/README.md:
  - read the spec;
  - write the tests first;
  - build;
  - verify every quality gate (QG) and delivery assertion (DA);
  - write the evidence record in `docs/verification/specs/<ID>.md`;
  - update docs/architecture.md and the PLAN.md checkbox.
- Do not invent requirements outside the spec. Report follow-ups instead of fixing them.
- Go formatting and linting are enforced by the Lefthook pre-commit hook:
  - `golangci-lint fmt`, which runs gofmt -s and goimports with the local prefix `github.com/AlexGalhardo/kali-tools`;
  - `golangci-lint run`;
  - `go vet`.
  Run `lefthook install` once per clone. Never bypass hooks.
- Imports come in three groups: stdlib, third-party, then module packages. Interfaces are declared by their consumer. Use manual dependency injection only, with no DI frameworks.

## Project skills

All skills live under .agents/skills. Read a skill's SKILL.md before using it. Source revisions are recorded in .agents/skills/sources.json.

- **Always:** karpathy-guidelines, ponytail, clean-architecture-in-practice (see above).
- **Interface work:** impeccable.
- **Go work** (load the ones that match the task): golang-project-layout, golang-code-style, golang-naming, golang-error-handling, golang-context, golang-concurrency, golang-structs-interfaces, golang-dependency-injection, golang-design-patterns, golang-database, golang-cli, golang-security, golang-safety, golang-testing, golang-lint, golang-modernize, golang-dependency-management, golang-troubleshooting. These were adapted from AlexGalhardo/vukem to this architecture.
- **Quality:** tdd, code-review-and-quality, ponytail-review, ponytail-audit.
- **Reading books into skills:** book-to-skill.

User scope and instructions take precedence. Do not invent product requirements, and do not mark hardware acceptance complete from compilation.

## Project

kali-tools is a local network and security toolkit. One Go module, `github.com/AlexGalhardo/kali-tools`, owns the capabilities. Three driving adapters share `app.Application`: the Wails desktop (`main.go`, `internal/adapter/wailsui`), the loopback web server (`internal/adapter/httpapi`, `kali-tools serve`), and the CLI (`cmd/kali-tools`). The UI is React, TypeScript, Base UI, and Tailwind CSS v4 in `frontend/`, with English and Brazilian Portuguese catalogs. SQLite through `modernc.org/sqlite` stores settings and redacted history. There is no cloud runtime and no telemetry.

Behavior is specified in `docs/specs/`. The package map and dependency rule are in `docs/architecture.md`. A green Linux build is not hardware acceptance.

## Layout

`docs/architecture.md` section 2 is the binding map. The packages agents edit are:

- `internal/domain` — error codes, capability states, and version. Standard library only.
- `internal/network`, `internal/capture`, `internal/wifi`, `internal/tlsaudit` — capabilities. They do not import adapters, `app`, Wails, or `database/sql`.
- `internal/app` and `internal/job` — use cases and bounded job fan-out. Ports are declared by these consumers.
- `internal/adapter/storage`, `tools`, `probe`, `platform`, `pcap` — driven adapters.
- `internal/adapter/wailsui`, `httpapi`, `cli`, `contract` — driving adapters and the shared DTO contract. They call `app` and do not import driven adapters.
- `internal/bootstrap` — wiring for every composition root. Production code outside `main.go` and `cmd/` does not import it.
- `internal/porttest` — contract suites and in-memory fakes. Production code does not import it.
- `frontend/src/features/<capability>` — UI pages, composed by `frontend/src/app/App.tsx`. `frontend/src/shared/api/bridge.ts` is the only Go transport. `frontend/wailsjs/` is generated.
- `scripts/` — Node task runners. `scripts/setup/` is the per-OS setup entry point. `scripts/task.mjs` is the allowlist of task names.
- `tests/acceptance` — in-process drivers. `tests/integration` is `//go:build integration`. `tests/e2e` is `//go:build e2e`.

`docs/architecture.md` still says `frontend/src/features/` is empty. It contains the capability pages. When the map and the tree disagree, trust the tree and update the architecture doc only as part of the spec you are implementing.

## Commands

Pins in `go.mod` and `.github/workflows/ci.yml`: Go 1.27.3, Node.js 24.21.0, Bun 1.4.3, Wails v2.17.0, golangci-lint v2.15.0, govulncheck v1.9.0. `README.md` still says Go 1.27.2 in the setup section. `CONTRIBUTING.md` and `docs/development.md` still say Node.js 24.20.0, and `CONTRIBUTING.md` still says Go 1.27.2. Trust `go.mod` and CI when those sentences disagree.

Cursor cloud agents install dependencies with `bash .cursor/install.sh`, which is the `install` step in `.cursor/environment.json`. The script installs the pinned Node and Bun, downloads Go modules, installs golangci-lint and govulncheck, installs both frontend lockfiles, and installs Playwright Chromium. It does not install Wails, GTK, or WebKit, and it does not start a desktop or lab session. The default cloud image already provides Go 1.27.3; the script exits if `go env GOVERSION` is not `go1.27.3`.

From the repository root, after that install:

```bash
node scripts/task.mjs go
node scripts/task.mjs web
node scripts/task.mjs arch
node scripts/task.mjs race
node scripts/task.mjs fuzz
node scripts/task.mjs workflows
node scripts/markdown.mjs
go run ./cmd/kali-tools version
```

`go` checks formatting, `go mod tidy -diff`, `go vet`, `go test -timeout 90s ./...`, golangci-lint, and govulncheck. `web` is the frontend lint, production build, and Playwright. `arch` proves each depguard rule rejects a forbidden import. Plain `go test ./...` skips `integration`, `e2e`, and `windows` tagged tests. The private Docker lab is `node scripts/task.mjs lab`.

Native desktop build and live reload use the host script: `bash scripts/setup/setup-linux.sh build` or `dev` (the windows and darwin scripts on those hosts). Linux desktop builds pass `-tags desktop,webkit2_41` and need the GTK 3 and WebKitGTK 4.1 packages named in the README. `node scripts/task.mjs bindings` regenerates `frontend/wailsjs` and fails if Git would see a diff.

Both frontend lockfiles are live. `frontend/package-lock.json` is what `npm ci`, setup, and Wails use. `frontend/bun.lock` is what `bun install --frozen-lockfile` uses. `frontend/package.json` sets `"packageManager": "bun@1.4.3"`. CI runs npm ci and then the frozen Bun install. Update both lockfiles together.

Install Lefthook once per clone after frontend dependencies exist: `frontend/node_modules/.bin/lefthook install`. Do not bypass it. Commit messages are Conventional Commits. `.cursor/install.sh` runs that install only when `core.hooksPath` is empty. Cursor cloud agents already set that path, and the script leaves it in place.

## Conventions

- Specs in `docs/specs/` are the requirements. Implement one at a time, tests first, and record evidence in `docs/verification/specs/<ID>.md`. Do not add behavior the spec does not state.
- Go imports are stdlib, third-party, then `github.com/AlexGalhardo/kali-tools/...`. `golangci-lint fmt` runs gofmt -s and goimports with that local prefix.
- An interface lives in the package that calls it. `app.New` takes `app.Deps`. There is no DI framework.
- OS-specific code uses build tags and an unsupported fallback in the same change (`*_windows.go`, `*_linux.go`, `*_other.go`).
- Expected failures are `domain.Error` values. New codes go in `internal/domain` and in both i18n catalogs (`en` and `pt` in `frontend/src/shared/i18n/i18n.ts`).
- Feature UI calls `bridge.ts`. It does not import `frontend/wailsjs`. Pages are registered in `frontend/src/app/App.tsx` and `frontend/src/app/shell.ts`.
- New task names are added to the allowlist in `scripts/task.mjs`. Setup actions stay in `scripts/setup.mjs`. Do not add a seventh top-level shell setup script; the README table is the complete set.
- Frontend formatting is Biome (`frontend/biome.json`): two-space indent, double quotes. `npm --prefix frontend run lint` is the check.

## Adding an optional external program

Follow the TShark inventory path and `runCommand`. Do not interpolate a shell command. Do not execute Nmap or read Nmap data files (`docs/adr/006-native-service-inspection.md` supersedes ADR 003).

1. Add the smallest method on the consumer port (`app.ToolRunner` in `internal/app/ports.go`).
2. Implement it in `internal/adapter/tools` through `runCommand`: a fixed argument array, a context deadline, stdout and stderr caps, and process-group or Windows Job Object cancellation.
3. Find the binary with `exec.LookPath`. A missing binary is `dependency_missing`. A version outside the supported range is `unsupported`. Do not vendor the executable.
4. Reject oversized or out-of-scope parser output before anything is written to history.
5. Expose the operation on `app.Application`, then through Wails, HTTP, and the CLI using `internal/adapter/contract`. Add the same strings to `en` and `pt`.
6. Extend `internal/porttest` and `internal/porttest/fake`.

Service inspection is native TCP, banner, TLS and one HTTP request: an explicit IPv4 /24 or narrower, at most 32 ports, and a two-minute deadline. TShark 4.2 or newer within major 4 is inventory-only.

## Safety and other gotchas

- Never concatenate a shell command. Never persist Wi-Fi keys or packet payloads in history, logs, SQLite, or verification notes. Tests use synthetic secrets and capture bytes.
- Report `available`, `dependency_missing`, `permission_required`, `location_consent_required`, `unsupported`, or `failed`. Do not turn a missing provider into a successful empty result.
- `kali-tools serve` binds loopback unless `--allow-remote` is combined with a TLS certificate and key. Launch tokens and session cookies are credentials. Do not log or commit them.
- `main.go` stays at the repository root. Wails v2 builds the package next to `wails.json`.
- Windows leads Wi-Fi and capture development. Linux and macOS binaries do not prove those features. Do not mark a PLAN.md checkbox without the evidence record.
- Development artifacts are unsigned. Do not create a release tag.
- GitHub Actions were authorized on 2026-10-19. Actions under `.github/workflows/` stay pinned to full commit SHAs. `pull_request_target` is rejected by `node scripts/task.mjs workflows`.
- `*.pcap`, `*.pcapng`, and `*.db` are gitignored, except `tests/fixtures/*.pcap`. `frontend/dist/` keeps only `.gitkeep`. `frontend/wailsjs/` is tracked generated source.
