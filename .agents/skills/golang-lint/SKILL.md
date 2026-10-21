---
name: golang-lint
description: "golangci-lint v2 in kali-tools — the project .golangci.yml (standard linters plus bodyclose, errorlint, rowserrcheck, sqlclosecheck and others), gofmt and goimports formatters with the local prefix github.com/AlexGalhardo/kali-tools, the pinned v2.15.0 tool, nolint rules, and depguard for the dependency rule (PLAN H08). Use when running or interpreting lint, changing .golangci.yml, suppressing a warning or formatting Go code."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.3.0"
---

# Go Linting in kali-tools

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT; the upstream 48-linter sample config was dropped in favor of the project's own `.golangci.yml`). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md` §11; they win over anything below.

**Persona:** linting is part of the change, not a cleanup afterward.

## Modes

- **Run/fix:** run the pinned linter on your change, fix root causes, format.
- **Configure:** change `.golangci.yml` only as part of a PLAN task (for example H08 depguard) and keep CI, Lefthook and `scripts/task.mjs` consistent.
- **Interpret:** read the linter name in parentheses and look it up in the [linter reference](./references/linter-reference.md).

## Tool and version

golangci-lint **v2.15.0**, pinned in `.github/workflows/ci.yml` (`go install ...@v2.15.0`) and `lefthook.yml` (`go run ...@v2.15.0`). Never use `@latest`; upgrade all pins together.

```bash
golangci-lint run ./...          # all configured linters
golangci-lint fmt ./...          # gofmt -s + goimports (local prefix)
golangci-lint fmt --diff ./...   # check only, as Lefthook does
golangci-lint run --enable-only errorlint ./...
node scripts/task.mjs check      # gofmt -l, go mod tidy -diff, vet, tests, lint, govulncheck
```

## Project configuration

`.golangci.yml` is the source of truth (`version: "2"`):

- `linters.default: standard` plus `bodyclose`, `durationcheck`, `errorlint`, `makezero`, `nilerr`, `rowserrcheck`, `sqlclosecheck`, `copyloopvar`, `gocheckcompilerdirectives`, `misspell`, `predeclared`, `unconvert`.
- `errcheck.exclude-functions` lists `Close`/`Rollback`-style calls whose errors are not actionable, plus a few `golang.org/x/sys/windows` calls. Add to it only with the same justification; do not widen it to hide real errors.
- Formatters: `gofmt` with `simplify: true` and `goimports` with `local-prefixes: github.com/AlexGalhardo/kali-tools`. Imports are grouped stdlib, third-party, then project packages. `gofumpt`, `gci`, `revive`, `wsl` and `gocritic` are not configured; do not format to their rules.

Lint must also pass for build-tagged and OS-specific files. Run vet for the other targets as the architecture requires:

```bash
go vet ./... && go vet -tags desktop .
GOOS=linux go vet ./... && GOOS=darwin go vet ./...
```

## Dependency rule (PLAN H08)

The target enforcement is depguard in `.golangci.yml` (or a pinned go-arch-lint), using the matrix in `docs/architecture.md` §3:

- `internal/domain`: stdlib only.
- `internal/network`, `internal/capture`, `internal/wifi`: no `internal/adapter/*`, `internal/app`, Wails or `database/sql`.
- `internal/app`, `internal/job`: no `internal/adapter/*`, Wails, `net/http` or `os/exec`.
- Wails only in the desktop root and `internal/adapter/wailsui`.

Until H08 lands, this is checked by review. A deliberately forbidden import must fail the check locally and in CI when it is added. Known violation D1 (`network` imports `adapter/probe`) is tracked in PLAN H03.

## Suppressing warnings

Fix the root cause first. When a suppression is justified:

```go
//nolint:errcheck // best-effort cleanup after the job already failed
proc.Kill()
```

1. Always name the linter: `//nolint:errcheck`, never bare `//nolint`.
2. Always add a reason after `//`.
3. Never suppress `bodyclose`, `rowserrcheck`, `sqlclosecheck` or `errorlint` without a strong, written reason.
4. Never suppress to hide a missing bound, a missing deadline or an unchecked subprocess error.

More patterns: [nolint directives](./references/nolint-directives.md).

## Workflow

1. Run `golangci-lint run ./...` after each significant change.
2. Format with `golangci-lint fmt ./...` (Lefthook checks staged files).
3. Run `go test ./...`, `go vet ./...` and the frontend check/build when relevant (AGENTS.md).
4. Do not mark a PLAN task complete without the actual command output as evidence.

## Interpreting output

```text
internal/app/service.go:42:10: non-wrapping format verb for fmt.Errorf (errorlint)
```

## Common issues

| Problem | Solution |
| --- | --- |
| Import order rejected | Run `golangci-lint fmt`; project imports go last under the local prefix |
| OS-specific file not linted on your host | Lint and vet with the matching `GOOS` |
| `rowserrcheck` or `sqlclosecheck` | Add `defer rows.Close()` and check `rows.Err()` |
| `errorlint` | Use `%w` and `errors.Is`/`errors.As`/`errors.AsType` |
| v1 config syntax | The project is on v2; do not add v1 keys |

## Cross-references

- `golang-code-style` for rules linters cannot check.
- `golang-security` for govulncheck and secure-coding review.
- `golang-dependency-management` for pinned tool versions.
