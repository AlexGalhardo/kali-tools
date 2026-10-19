# Contributing

Use Go 1.27.2, Node 24.20.0 and Bun 1.4.3. Versions were checked against upstream registries on 2026-10-19. Wails CLI is pinned to v2.17.0. Install optional Go checks with the versions in `docs/development.md`.

Keep the domain in Go. Desktop methods are thin typed facades; the CLI invokes the same functions. Native providers use build constraints. Never use a shell to invoke network tools. Tests must exercise behavior, bounds and failures rather than restating code.

Run `node scripts/task.mjs check`, `node scripts/task.mjs race`, `node scripts/task.mjs fuzz` and `node scripts/task.mjs workflows`. Install Lefthook with `frontend/node_modules/.bin/lefthook install` after dependencies are installed. Commit messages follow Conventional Commits, for example `feat(network): add bounded TCP diagnostics`.

Every completed plan item requires evidence in `docs/verification/`. Native hardware acceptance and cross compilation are different evidence. Keep CI source disabled until the owner explicitly requests activation. Do not tag a release before its acceptance criteria pass.
