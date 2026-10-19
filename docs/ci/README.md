# Continuous integration and release drafts

The owner explicitly authorized GitHub Actions activation on 2026-10-19. The active source is [.github/workflows/ci.yml](../../.github/workflows/ci.yml); dormant copies were removed to prevent drift.

Pushes to main, pull requests and manual dispatch run formatting, Go unit/race/vet/lint/vulnerability checks, parser fuzz smoke, frontend typecheck/lint/build, Playwright accessibility/workflows, Markdown, actionlint, npm audit and the private Docker CLI lab.

Native jobs build Windows x64, Linux x64 on Ubuntu 24.04, macOS Intel and macOS Apple Silicon with platform prerequisites. Each archive includes the desktop, CLI, MIT license, third-party notices, dependency inventory and source/version/platform metadata. These builds verify compilation and automated contracts, not Wi-Fi/capture hardware acceptance.

Only the final main-branch release job has contents-write permission. It waits for every check, downloads that run's native archives, generates a source archive and SHA-256 checksums, and assembles an unsigned development release draft. It refuses to replace a published release. Pull requests cannot create releases. Git tags and publication remain gated by PLAN.md acceptance.

Actions are pinned to immutable upstream release revisions. No PAT, signing certificate or deployment credential is stored in the repository. The release job uses the short-lived GitHub token.
