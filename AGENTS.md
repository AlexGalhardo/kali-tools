# Contributor instructions

Use English for source, documentation and commits. Keep business logic in Go services shared by the Wails GUI, the web server and the CLI. Read docs/architecture.md before adding, moving or wiring packages, and keep it current; the reasoning is in docs/architecture-research.md. Never concatenate shell commands. Bound network operations, parser input, event queues and subprocess output. Never persist Wi-Fi secrets or packet payloads in diagnostic history. GitHub Actions activation was explicitly authorized on 2026-10-19; maintain pinned workflows under .github/workflows with minimum permissions. Mark PLAN.md tasks complete only with actual verification evidence. Run `go test ./...`, `go vet ./...` and the frontend check/build after relevant changes. Do not claim native platform acceptance from cross compilation. Use the MIT project license. No release tag before qualification and owner decisions on signing. Development artifacts must disclose unsigned status and open acceptance gates.

## Mandatory start of every task

Before writing code, plans or reviews in this repository, always read and apply these three skills, in this order:

1. `.agents/skills/karpathy-guidelines/SKILL.md`: think before coding; simplicity first; surgical changes; goal-driven verification.
2. `.agents/skills/ponytail/SKILL.md`: the smallest complete solution.
3. `.agents/skills/clean-architecture-in-practice/SKILL.md`: the dependency rule, ports and adapters, and use cases. It is the reference for docs/architecture.md.

Do this for every task, including small ones. If a skill conflicts with docs/architecture.md, the architecture document wins and the conflict must be reported.

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
