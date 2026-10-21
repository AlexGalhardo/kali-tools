# Contributing

Releases always follow [Semantic Versioning 2.0.0](https://semver.org/) and [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/). Use `vMAJOR.MINOR.PATCH` Git tags and the same version without `v` in application/package metadata. Keep `[Unreleased]` first, dated `## [X.Y.Z] - YYYY-MM-DD` entries newest first, and only Added, Changed, Deprecated, Removed, Fixed or Security change groups. Release notes summarize user-visible changes and link evidence; commit lists supplement the changelog. Published version contents and tags are immutable; historical draft renaming requires explicit owner instruction. Run `node --test scripts/release-policy.test.mjs` and `node scripts/release-policy.mjs` before packaging or publishing. Both packaging and release scripts enforce this check.

Use Go 1.27.2, Node 24.20.0 and Bun 1.4.3. Versions were checked against upstream registries on 2026-10-19. Wails CLI is pinned to v2.17.0. Install optional Go checks with the versions in `docs/development.md`.

Keep the domain in Go. Desktop methods are thin typed facades; the CLI invokes the same functions. Native providers use build constraints. Never use a shell to invoke network tools. Tests must exercise behavior, bounds and failures rather than restating code.

Run `node scripts/task.mjs check`, `node scripts/task.mjs race`, `node scripts/task.mjs fuzz` and `node scripts/task.mjs workflows`. Install Lefthook with `frontend/node_modules/.bin/lefthook install` after dependencies are installed. Commit messages follow Conventional Commits, for example `feat(network): add bounded TCP diagnostics`.

The pre-commit hook formats staged Go files with the pinned `golangci-lint fmt` (gofmt -s and goimports, local prefix `github.com/AlexGalhardo/kali-tools`), re-stages them, then runs `golangci-lint run` and `go vet`. Do not bypass hooks. New work starts from a spec in [docs/specs](docs/specs/README.md) and must satisfy its quality gates and delivery assertions, with an evidence record under `docs/verification/specs/`. Read [docs/architecture.md](docs/architecture.md) before structural changes.

Every completed plan item requires evidence in `docs/verification/`. Native hardware acceptance and cross compilation are different evidence. CI/CD was activated by explicit owner instruction on 2026-10-19. Pull requests run the same checks as main; development releases remain clearly labeled. Do not tag a release before its acceptance criteria pass or represent a development preview as qualified. Contributions are licensed under [MIT](LICENSE.md).
