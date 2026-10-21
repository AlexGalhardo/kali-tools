# Dependency Updates

kali-tools has no Dependabot or Renovate configuration. Adding one changes `.github/` and needs the owner's decision; GitHub Actions workflows must stay pinned with minimum permissions (AGENTS.md).

## Update verification

Before committing a dependency update:

1. Read the changelog, especially for libraries that affect persistence (modernc SQLite), packet parsing (gopacket), the desktop bridge (Wails) or networking (`x/net`, `x/sys`).
2. Run `go mod tidy`, then `go test ./...`, `go vet ./...`, `go vet -tags desktop .` and the cross-OS vet commands.
3. Run `govulncheck ./...` for the default and `desktop` builds.
4. Update `docs/dependencies.md` when a direct dependency, its version note or its license changes.
5. For Wails, regenerate `frontend/wailsjs`, check for binding drift and run the frontend check/build. Native acceptance needs real hardware evidence, not cross-compilation.
6. Major version upgrades need a PLAN task or an ADR.
