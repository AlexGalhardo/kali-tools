# Tooling Modernization

kali-tools pins its developer tools: golangci-lint v2.15.0 and govulncheck v1.9.0 in `.github/workflows/ci.yml` and `lefthook.yml`, with pinned GitHub Actions and minimum permissions (AGENTS.md). Suggest tool upgrades as separate changes that update every pin together; never introduce `@latest`. New tools, bots or workflows need the owner's decision.

## Update to the latest Go version

Compare the project's `go` directive in `go.mod` against the latest stable release and suggest updating it and the `toolchain` directive if present. Each Go release brings performance improvements, security fixes, and new features.

```bash
# Check current version
go version

# Propose a directive bump only as its own change, to an exact version
go mod edit -go=1.27.N
```

## golangci-lint v2 _(golangci-lint v2.0.0+, March 2025)_

kali-tools is already on golangci-lint v2 (`version: "2"`). See the `golang-lint` skill for the project configuration.

## govulncheck _(works best with Go 1.22+)_

`govulncheck` scans Go code for known vulnerabilities using the Go vulnerability database at `vuln.go.dev`. It analyzes call graphs to report only **reachable** vulnerabilities.

**Note**: The vulnerability database tracks Go standard library issues starting from Go 1.18. Third-party module vulnerabilities are tracked regardless of Go version. For best results (better call graph analysis), use Go 1.22+.

```bash
# Pinned version used by the project (v1.9.0)
govulncheck ./...
govulncheck -tags desktop ./...

# Scan a compiled binary
govulncheck -mode=binary ./kali-tools
```

## Profile-Guided Optimization (PGO)

Not used in kali-tools. There is no performance requirement that justifies it; propose it only with a measured profile and a PLAN task.

## `go fix` and `go doc` changes _(Go 1.27+)_

`go fix` now applies more of the `modernize` fixer suite automatically. Coverage shifted between Go 1.26 and 1.27:

- **Added**: `atomictypes`, `embedlit`, `slicesbackward`, `unsafefuncs`
- **Removed**: `fmtappendf` (superseded by other fixers)
- **Renamed**: `waitgroup` → `waitgroupgo`

```bash
go fix ./...          # applies the enabled safe transformations
go tool fix help      # check exact fixer coverage for the installed toolchain
```

`go doc` accepts `package@version` to look up a specific module version without changing `go.mod`, and `-ex` lists executable examples:

```bash
go doc golang.org/x/tools/cmd/stringer@v0.30.0
go doc -ex net/http.Client
```

`go mod tidy` for modules with `go 1.27` or newer auto-merges duplicate `require` blocks and enforces a two-block layout (direct dependencies, then indirect), preserving existing comments.

`go test -json` output gained an optional `OutputType` field (`"error"`, `"error-continue"`, `"frame"`) for consumers that need to distinguish test-output kinds programmatically.

`go tool trace -http` now binds to localhost by default; pass `-http=0.0.0.0:6060` explicitly to expose the trace viewer beyond the local machine (relevant when tracing inside a container or remote dev environment).
