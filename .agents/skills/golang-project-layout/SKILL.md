---
name: golang-project-layout
description: "Where Go code goes in kali-tools — the single module github.com/AlexGalhardo/kali-tools, the hexagonal layout (internal/domain, internal/app, internal/job, capability packages network/capture/wifi, internal/adapter/<technology>, internal/bootstrap, internal/porttest, cmd/kali-tools, root main.go for Wails v2), build-tagged platform files and test placement. Use when adding a package or file, moving code, placing a test or fixture, or reviewing an import that crosses layers."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.4.2"
---

# Project Layout in kali-tools

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT; the generic layout catalog, `go.work`, `pkg/`, Makefile and Viper material were removed). The binding source is `docs/architecture.md`; read it before adding, moving or wiring packages, and keep it current in the same change. This skill only summarizes it.

**Persona:** you place code by its layer, keep the import graph pointing inward and do not create structure ahead of the PLAN task that needs it.

## Module

- One module: `github.com/AlexGalhardo/kali-tools`, `go 1.27.2`. No `go.work`, no nested modules, no `pkg/`, no `vendor/`.
- All Go logic lives under `internal/`.

## Where things go

| Path | Layer | Contains |
| --- | --- | --- |
| `main.go`, `app.go`, `doc.go` | Wails composition root and desktop facade | Must stay at the root: Wails v2 builds the root package, `//go:embed` cannot reach parent directories, and the binding namespace follows the package name |
| `cmd/kali-tools` | CLI composition root | `run` and wiring; `serve` will start the web server |
| `cmd/lab-fixture` | Docker lab fixture | Deterministic HTTP/DNS/TCP services |
| `internal/domain` | Shared kernel | `Error{Code, Message}`, `Capability`, versions; stdlib only |
| `internal/network`, `internal/capture`, `internal/wifi` | Capabilities (inner hexagon) | Domain types, validating constructors, limits, pure logic; may declare ports |
| `internal/app` | Application | Use cases, job lifecycle, history, capability registry; declares driven ports |
| `internal/job` | Application | Target home of the job manager and bounded event fan-out (PLAN H02) |
| `internal/adapter/storage`, `tools`, `probe`, `platform` | Driven adapters | SQLite, bounded process runner and Nmap parser, ICMP, routes/counters |
| `internal/adapter/wailsui`, `httpapi`, `cli` | Driving adapters | Target homes of the Wails bindings, the web server (REST + SSE + security middleware) and the CLI command tree |
| `internal/bootstrap` | Composition | Target shared wiring for every root (PLAN H01) |
| `internal/porttest` | Tests | Target contract suites for driven ports (PLAN H07) |
| `tests/integration` | Tests | Docker lab tests, `//go:build integration` |
| `tests/acceptance` | Tests | Interface-agnostic acceptance specifications |
| `tests/fixtures`, `testdata/` | Tests | Synthetic PCAP and tool-output fixtures; never real captures |

Folders that contain only `.gitkeep` are target locations. Do not add placeholder Go files; add code only with the PLAN task that fills them.

## Import rules

| Package | May import | Must not import |
| --- | --- | --- |
| `domain` | stdlib | anything in the module |
| capabilities | `domain`, stdlib, pure libraries (gopacket) | `adapter/*`, `app`, Wails, `database/sql` |
| `app`, `job` | `domain`, capabilities, `job` | `adapter/*`, Wails, `net/http`, `os/exec` |
| driven adapters | `domain`, capabilities, OS/driver libraries | driving adapters, `cmd`, `app` (except types) |
| driving adapters | `app`, `domain`, DTO types | driven adapters directly |
| composition roots | everything | nothing imports them |

Known deviations D1 to D8 are listed in `docs/architecture.md` §8. Do not add new ones; when you touch affected code, move toward the target described by the PLAN task.

## Platform files

- `<name>_windows.go`, `<name>_linux.go` and an `_other.go` fallback with the same constructor and method set. The fallback returns an explicit unsupported error, never a fake success.
- `//go:build` goes before the package clause, followed by a blank line.
- Verify with `GOOS=windows|linux|darwin go vet ./...` and `go vet -tags desktop .`. Cross-compilation proves compilation only, never native acceptance.

## Tests

- Unit tests sit beside the code (`*_test.go`). Prefer black-box `package x_test` for capability packages; same-package tests where internals must be reached.
- Use-case tests in `internal/app` use hand-written fakes of the ports.
- Port contract suites live in `internal/porttest` and run against the adapter and its fake.
- Integration tests need the `integration` tag and the Docker lab; native tests need real hardware evidence recorded in `docs/verification/`.
- `bridge_smoke_test.go` at the root is the native Wails bridge smoke test (`windows && desktop && production && nativesmoke` tags, run by `node scripts/task.mjs bridge`).

## Adding a capability

Follow `docs/architecture.md` §7.1: capability package first, a port in the consumer plus an adapter under `internal/adapter/<technology>` with build-tagged files, a use case in `app`, then thin exposure through all three interfaces (Wails binding, HTTP route, CLI command), UI under `frontend/src/features/<capability>/`, tests at every level, and documentation updates.

## Moving packages

Use `git mv`, rewrite import paths, run `gofmt`, vet all three `GOOS` targets plus the `desktop` tag, then `go test ./...`. Update `.golangci.yml`, `scripts/task.mjs`, `docker/Dockerfile` and the docs that mention the old path. Historical evidence in `docs/verification/` keeps its original paths.

## Anti-patterns

- Packages named by kind: `controllers/`, `services/`, `repositories/`, `models/`, `utils/`, `common/`, `types/`, `ports/`.
- `pkg/` or the full golang-standards tree.
- A new module or `go.work` for a subpackage.
- Moving `main.go` out of the root.
- Business logic in `app.go`, handlers or CLI commands.

## Cross-references

- `golang-dependency-injection` for composition roots and ports.
- `golang-naming` for package and identifier names.
- `golang-testing` for test placement and contract suites.
