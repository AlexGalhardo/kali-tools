---
name: golang-modernize
description: "Modernize kali-tools Go code within its go 1.27.3 target — newer language features, standard library replacements, testing helpers (t.Context, b.Loop, synctest), deprecated APIs and Go 1.27 GODEBUG checks. Use when reviewing old-style Go, handling a deprecation, or planning a toolchain or tool-pin upgrade. Suggest changes outside the current task; do not sweep unrelated code."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.5.1"
---

# Go Code Modernization Guide

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md`; they win over anything below.

**Persona:** You are a Go modernization engineer. You keep codebases current with the latest Go idioms and standard library improvements — you prioritize safety and correctness fixes first, then readability, then gradual improvements.

## Modes

- **Inline mode** (developer is actively coding): suggest only modernizations relevant to the current file or feature. A broad rewrite started during someone else's task buries their change under unrelated churn and makes the diff unreviewable — so record the other opportunities as a note, with the quality gain each would bring, and let the developer schedule them.
- **Full-scan mode** (explicit request): scan deprecated APIs, language features (range-over-int, min/max, any, iterators), standard library upgrades (slices, maps, cmp, slog), testing patterns (t.Context, b.Loop, synctest) and tooling, then prioritize with the migration priority guide. The scan is read-only. Apply a sweep only as small, reviewed, behavior-preserving changes that keep `go test ./...`, `go vet ./...` and lint green.

**Questions:** In Inline mode, this skill triggers contextually while the developer is working on something else — ask via the environment's question tool, once, whether to suggest the modernization opportunities noticed or skip for now. If the user skips, stop immediately and do not raise modernization again for the rest of the session.

This skill helps you continuously modernize Go codebases by replacing outdated patterns with their modern equivalents.

**Scope**: This skill covers roughly the last 3 years of Go releases — from the oldest to the newest row in the Go Version Changelogs table below, updated each Go release. Projects targeting an older `go.mod` than the table's oldest row still get modernization suggestions, but with narrower coverage; for best results, upgrade the Go version first. Some older modernizations (e.g., `any` instead of `interface{}`, `errors.Is`/`errors.As`, `strings.Cut`) are included because they are still commonly missed, but many pre-1.21 improvements are intentionally omitted because they should have been adopted long ago and are considered baseline Go practices by now.

You MUST NEVER conduct large refactoring while the developer is working on a different task. Mention the opportunity and its benefit once; the owner decides (karpathy-guidelines: surgical changes).

## Workflow

When invoked:

1. **Check `go.mod`**: kali-tools is a single module on `go 1.27.3`. Use any API available in Go 1.27; do not use newer APIs, and do not bump the directive as a side effect
2. **Check the latest Go release** against the changelog table and mention an upgrade only as a separate proposal
3. **Respect earlier decisions** recorded in `docs/adr/` or PLAN.md; do not re-suggest something the owner declined
4. **Scan the codebase** for modernization opportunities based on the target Go version
5. **Run `golangci-lint run ./...`** (the `modernize` linter is not enabled in `.golangci.yml`; `go fix` and gopls report similar fixes) and `go test ./...` — Go 1.27+ runs the `stdversion` vet check by default, flagging APIs newer than the module's `go` directive; bump the directive or revert the suggestion, don't ignore the hit
6. **Suggest improvements contextually**:
   - If the developer is actively coding, **only suggest improvements related to the code they are currently working on**. Do not refactor unrelated files. Instead, mention opportunities you noticed and explain why the change would be beneficial — but let the developer decide.
   - If invoked explicitly via `/golang-modernize` or in CI, scan and suggest across the entire codebase.
7. **For a full scan**, split by category (deprecated packages, language features, standard library, testing, tooling) and report findings before editing.
8. **Before suggesting a dependency update**, run `go mod tidy` and the test suite to verify compatibility. Ask the developer to review the dependency's changelog and release notes for breaking changes before proceeding.
9. **If the developer declines a suggestion**, drop it for the session. Do not create a `.modernize` file or any other untracked file in the repository.

When applying a modernization that renames an identifier or replaces a deprecated API, prefer `gopls rename` over grep/sed, then run `go vet` for windows, linux and darwin plus `go vet -tags desktop .`, because platform files are only compiled for their own `GOOS`.

## Go Version Changelogs

Reference the relevant changelog when suggesting a modernization:

| Version | Release       | Changelog                   |
| ------- | ------------- | --------------------------- |
| Go 1.21 | August 2023   | <https://go.dev/doc/go1.21> |
| Go 1.22 | February 2024 | <https://go.dev/doc/go1.22> |
| Go 1.23 | August 2024   | <https://go.dev/doc/go1.23> |
| Go 1.24 | February 2025 | <https://go.dev/doc/go1.24> |
| Go 1.25 | August 2025   | <https://go.dev/doc/go1.25> |
| Go 1.26 | February 2026 | <https://go.dev/doc/go1.26> |
| Go 1.27 | August 2026   | <https://go.dev/doc/go1.27> |

For versions newer than Go 1.27, consult the official Go release notes.

When the project's `go.mod` targets an older version, suggest upgrading and explain the benefits they'd unlock.

## Using the modernize linter

The `modernize` linter (available since **golangci-lint v2.6.0**) automatically detects code that can be rewritten using newer Go features. It originates from `golang.org/x/tools/go/analysis/passes/modernize`; `gopls` and `go fix` (rewritten onto the `go/analysis` framework in Go 1.26, with fixer coverage still growing in Go 1.27 — see [Tooling modernization](./references/tooling.md) for the exact fixer list) cover overlapping modernization checks, but exact coverage differs by tool version. See the `golang-lint` skill for configuration.

## Version-specific modernizations

For detailed before/after examples for each Go version (1.21–1.27) and general modernizations, see [Go version modernizations](./references/versions.md).

## Tooling modernization

For the Go toolchain, govulncheck, golangci-lint v2 and `go fix`, see [Tooling modernization](./references/tooling.md).

## Deprecated Packages Migration

| Deprecated | Replacement | Since |
| --- | --- | --- |
| `math/rand` | `math/rand/v2` | Go 1.22 |
| `crypto/elliptic` (most functions) | `crypto/ecdh` | Go 1.21 |
| `reflect.SliceHeader`, `StringHeader` | `unsafe.Slice`, `unsafe.String` | Go 1.21 |
| `reflect.PtrTo` | `reflect.PointerTo` | Go 1.22 |
| `runtime.GOROOT()` | `go env GOROOT` | Go 1.24 |
| `runtime.SetFinalizer` | `runtime.AddCleanup` | Go 1.24 |
| `crypto/cipher.NewOFB`, `NewCFB*` | AEAD modes or `NewCTR` | Go 1.24 |
| `golang.org/x/crypto/sha3` | `crypto/sha3` | Go 1.24 |
| `golang.org/x/crypto/hkdf` | `crypto/hkdf` | Go 1.24 |
| `golang.org/x/crypto/pbkdf2` | `crypto/pbkdf2` | Go 1.24 |
| `testing/synctest.Run` | `testing/synctest.Test` | Go 1.25 |
| `crypto/rsa.EncryptPKCS1v15` for new encryption use | RSA-OAEP (`rsa.EncryptOAEP` / `rsa.EncryptOAEPWithOptions`) or HPKE/KEM design | Go 1.26 |
| `net/http/httputil.ReverseProxy.Director` | `ReverseProxy.Rewrite` | Go 1.26 |
| `crypto/tls.Config.Rand` | `testing/cryptotest.SetGlobalRandom()` | Go 1.27 |
| `github.com/google/uuid` (simple cases) | `uuid` (stdlib) | Go 1.27 |

## Go 1.27 notes for kali-tools

kali-tools already targets Go 1.27.3. The checklist below matters when reviewing `godebug` settings or older code paths. Go 1.27 items in this skill come from the upstream skill; confirm each one against <https://go.dev/doc/go1.27> or `go doc` before relying on it.

Several Go 1.27 changes need **verification, not a rewrite**. Most notably: a `godebug` line in `go.mod` (or `//go:debug` comment) still pinning `asynctimerchan`, `tlsunsafeekm`, `tlsrsakex`, `tls3des`, `tls10server`, `x509keypairleaf`, or `gotypesalias` to its **old** value now fails the build. Full checklist in [Go version modernizations](./references/versions.md#go-127-version-bump-risk-checklist-verify-dont-rewrite).

## Migration Priority Guide

When modernizing a codebase, prioritize changes by impact:

### High priority (safety and correctness)

1. Remove loop variable shadow copies _(Go 1.22+)_ — prevents subtle bugs
2. Replace `math/rand` with `math/rand/v2` _(Go 1.22+)_ — remove `rand.Seed` calls
3. Use `os.Root` for user-supplied file paths _(Go 1.24+)_ — prevents path traversal
4. Run `govulncheck` _(Go 1.22+)_ — catch known vulnerabilities
5. Use `errors.Is`/`errors.As` instead of direct comparison _(Go 1.13+)_
6. Migrate deprecated crypto packages _(Go 1.24+)_ — security critical
7. Resolve removed `GODEBUG` keys and `crypto/tls.Config.Rand` callers _(Go 1.27+)_ — see the notes above; a stale `GODEBUG` value now fails the build

### Medium priority (readability and maintainability)

1. Replace `interface{}` with `any` _(Go 1.18+)_
2. Use `min`/`max` builtins _(Go 1.21+)_
3. Use `range` over int _(Go 1.22+)_
4. Use `slices` and `maps` packages _(Go 1.21+)_
5. Use `cmp.Or` for default values _(Go 1.22+)_
6. Use `sync.OnceValue`/`sync.OnceFunc` _(Go 1.21+)_
7. Use `sync.WaitGroup.Go` _(Go 1.25+)_
8. Use `t.Context()` in tests _(Go 1.24+)_
9. Use `b.Loop()` in benchmarks _(Go 1.24+)_
10. Use generic methods for helpers scoped to one type, and `strings.CutLast`/`bytes.CutLast` instead of `LastIndex` slicing _(Go 1.27+)_
11. Evaluate the `encoding/json/v2` API _(Go 1.27+)_ — its duplicate-key and invalid-UTF-8 strictness can change the Wails, HTTP and CLI JSON contract; any change needs golden tests for `schema_version` output and a PLAN task

### Lower priority (gradual improvement)

1. Use `log/slog` for structured logs _(Go 1.21+)_
2. Adopt iterators where they simplify code _(Go 1.23+)_
3. Replace `sort.Slice` with `slices.SortFunc` _(Go 1.21+)_
4. Use `strings.SplitSeq` and iterator variants _(Go 1.24+)_
5. Prefer the stdlib `uuid` package over adding a UUID dependency _(Go 1.27+)_
6. Run `go fix ./...` after a toolchain upgrade, then review the diff _(Go 1.27+)_

Tooling changes (golangci-lint pins, govulncheck, CI workflows, `tool` directives, PGO) are separate, owner-approved changes; see [Tooling modernization](./references/tooling.md).

## Related Skills

See `golang-concurrency`, `golang-testing`, `golang-error-handling`, `golang-lint` and `golang-dependency-management` skills.
