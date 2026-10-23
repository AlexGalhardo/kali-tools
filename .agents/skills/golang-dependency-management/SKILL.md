---
name: golang-dependency-management
description: "Go dependency management in kali-tools — go.mod/go.sum, exact-version go get, MVS, replace/exclude, govulncheck for default and desktop builds, pinned tool versions, and the license and docs/dependencies.md checklist for every new dependency. Use when adding, removing or upgrading a dependency, or auditing what the module pulls in. Ask the owner before adding any dependency."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.3.1"
---

# Go Dependency Management

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md`; they win over anything below.

**Persona:** You are a Go dependency steward. You treat every new dependency as a long-term maintenance commitment — you ask whether the standard library already solves the problem before reaching for an external package.

## Tools

Use the versions pinned by the project, never `@latest`: `golangci-lint` v2.15.0 and `govulncheck` v1.9.0 (see `.github/workflows/ci.yml` and `lefthook.yml`). `node scripts/task.mjs check` runs `go mod tidy -diff`, `go vet`, tests, golangci-lint and govulncheck.

## AI Agent Rule: Ask Before Adding Dependencies

**Before running `go get` to add any new dependency, AI agents MUST ask the user for confirmation.** AI agents can suggest packages that are unmaintained, low-quality, or unnecessary when the standard library already provides equivalent functionality. Using `go get -u` to upgrade an existing dependency is safe.

Before proposing a dependency, evaluate:

- Does the standard library already cover the use case?
- Is the license compatible?
- Are there well-known alternatives?
- What it does and why it's needed?

In kali-tools the standard library is the default. Favor `golang.org/x/...` or established organizations over obscure packages, and pure-Go packages over cgo (the build relies on `modernc.org/sqlite` precisely to avoid cgo).

### kali-tools dependency checklist

Every new direct dependency, Go or frontend, in the same change:

- has a license compatible with the MIT project license and with redistribution in release archives;
- gets a row in `docs/dependencies.md` (purpose and upstream license);
- is picked up by the notice generation (`scripts/`), so `THIRD_PARTY_NOTICES.txt` and `dependencies.json` stay complete;
- is pinned to an exact version, with `go.sum` committed;
- passes `govulncheck` for both the default build and the `desktop` tag;
- respects layer rules: Wails only in the desktop root and Wails adapter, `database/sql` only in `adapter/storage`, nothing but stdlib in `internal/domain`.

Optional external executables (Nmap, TShark) are not dependencies: they are neither installed nor redistributed, and they are recorded in `docs/tools-research.md`.

## Key Rules

- `go.sum` MUST be committed — it records cryptographic checksums of every dependency version, letting `go mod verify` detect supply-chain tampering. Without it, a compromised proxy could silently substitute malicious code
- `govulncheck ./...` and `govulncheck -tags desktop ./...` after every dependency change and before any release candidate — catches known CVEs in the dependency tree
- Maintenance status, license compatibility, and stdlib alternatives are important considerations before adding a dependency — every dependency increases attack surface, maintenance burden, and binary size
- `go mod tidy` before every commit that changes dependencies — removes unused modules and adds missing ones, keeping go.mod honest

## go.mod & go.sum

### Essential Commands

| Command           | Purpose                                      |
| ----------------- | -------------------------------------------- |
| `go mod tidy`     | Add missing deps, remove unused ones         |
| `go mod download` | Download modules to local cache              |
| `go mod verify`   | Verify cached modules match go.sum checksums |
| `go mod vendor`   | Copy deps into `vendor/` directory           |
| `go mod edit`     | Edit go.mod programmatically (scripts, CI)   |
| `go mod graph`    | Print the module requirement graph           |
| `go mod why`      | Explain why a module or package is needed    |

### Vendoring

kali-tools does not vendor. Do not add a `vendor/` directory without an ADR.

## Installing & Upgrading Dependencies

### Adding a Dependency

```bash
go get example.com/pkg@v1.6.0   # always an explicit version
```

Before pinning a version, inspect the module's versions, license and known vulnerabilities on pkg.go.dev.

### Upgrading

```bash
go get -u=patch ./...           # latest patch only (safer)
go get example.com/pkg@v1.5.2   # one package to an exact version
```

Avoid `go get -u ./...` across the whole module; upgrade deliberately and review each change. A Wails upgrade also requires regenerating `frontend/wailsjs` and running the native bridge smoke test on real hardware before claiming it works.

**Prefer `go get -u=patch`** for routine updates. Patch and minor updates are usually lower risk than major upgrades, but still require review. For dependency updates, run:

```bash
go get -u=patch ./...
go mod tidy
go test ./...
go vet ./... && go vet -tags desktop .
GOOS=linux go vet ./... && GOOS=darwin go vet ./...
govulncheck ./...
```

Release notes and changelogs for libraries affecting persistence, serialization, networking, authentication, authorization, cryptography, or public APIs may contain important information about breaking changes.

### Removing a Dependency

```bash
go get github.com/google/uuid@none  # Mark for removal
go mod tidy                          # Clean up go.mod and go.sum
```

### Development tools

kali-tools pins tools by exact version where they are invoked (`go install ...@v2.15.0` in CI, `go run ...@v2.15.0` in Lefthook) instead of `tool` directives in `go.mod`. Keep those pins in sync when upgrading a tool, and never introduce `@latest`. Moving to `tool` directives would change `go.mod` and needs its own change.

### Go version

The module declares `go 1.27.3`. Do not use APIs newer than the `go` directive, and do not change the directive as a side effect of another change.

## Deep Dives

- **[Versioning & MVS](./references/versioning.md)** — Semantic versioning rules (major.minor.patch), when to increment each number, pre-release versions, the Minimal Version Selection (MVS) algorithm (why you can't just pick "latest"), and major version suffix conventions (v0, v1, v2 suffixes for breaking changes).

- **[Auditing Dependencies](./references/auditing.md)** — Vulnerability scanning with `govulncheck`, tracking outdated dependencies, analyzing which dependencies make the binary large (`goweight`), and distinguishing test-only vs binary dependencies to keep `go.mod` clean.

- **[Dependency Conflicts & Resolution](./references/conflicts.md)** — Diagnosing version conflicts (what `go get` does when you request incompatible versions), resolution strategies (`replace` directives for local development, `exclude` for broken versions, `retract` for published versions that should be skipped), and workflows for conflicts across your dependency tree.

- **[Dependency Updates](./references/automated-updates.md)** — verifying an update before it lands; automated update bots are not configured.

- **[Visualizing the Dependency Graph](./references/visualization.md)** — `go mod graph` to inspect the full dependency tree, `modgraphviz` to visualize it, and interactive tools to find which dependency chains cause bloat.

## Cross-References

- → See `golang-security` skill for vulnerability scanning with govulncheck
- → See `golang-lint` skill for the pinned golangci-lint setup

## Quick Reference

```bash
go get example.com/pkg@v1.6.0          # add at an exact version (ask first)
go get -u=patch ./...                  # patch upgrades
go get example.com/pkg@none            # remove
go mod tidy                            # then check: go mod tidy -diff
go mod why -m example.com/pkg          # why is it here?
go mod verify                          # verify checksums
govulncheck ./...                      # vulnerabilities (pinned v1.9.0)
go list -m -json all                   # transitive inventory for notices
```
