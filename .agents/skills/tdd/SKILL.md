---
name: tdd
description: Test-driven development for kali-tools Go code. Use when the user wants to build features or fix bugs test-first, mentions "red-green-refactor", or wants use-case, contract, acceptance or integration tests.
license: MIT
---

# Test-Driven Development

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (vukem `.claude/skills` copy, upstream not recorded in vukem, used under the vukem repository MIT license; TypeScript examples rewritten in Go and seams mapped to the hexagonal architecture). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md` §10; they win over anything below.

TDD is the red → green loop. This skill is the reference that makes that loop produce tests worth keeping: what a good test is, where tests go, the anti-patterns and the rules of the loop. Consult every section before and during the loop, not after.

Use the vocabulary of `docs/architecture.md` (capability, port, adapter, use case, job, capability state) in test names, and respect ADRs in `docs/adr/` for the area you touch.

## What a good test is

Tests verify behavior through public interfaces, not implementation details. Code can change entirely; tests shouldn't. A good test reads like a specification: "diagnose rejects a port outside 1..65535" tells you exactly what capability exists, and it survives refactors because it does not care about internal structure.

See [tests.md](tests.md) for examples and [mocking.md](mocking.md) for fakes.

## Seams: where tests go

A **seam** is the public boundary you test at. In kali-tools the seams are fixed by the architecture:

| Seam | Test level |
| --- | --- |
| Capability package API (`network`, `capture`, `wifi`, `domain`) | Domain unit tests, table-driven, no fakes |
| `app` use cases and the job manager | Use-case tests with hand-written fakes of driven ports |
| Driven port (`JobStore`, `Prober`, `ToolRunner`, ...) | Contract suite in `internal/porttest`, run against adapter and fake |
| Driving adapter (Wails binding, CLI `run`, HTTP handler) | Acceptance spec in `tests/acceptance`, one driver per adapter |
| Parser of untrusted input | Fuzz target plus regression fixtures |
| Docker lab, real hardware | Integration tests (`integration` tag) and recorded native evidence |

**Test only at agreed seams.** Before writing a test, state which seam it targets and confirm it with the user when it is not obvious from the PLAN task. Give each proposed seam a one-line note on what it catches and what it misses.

When the shape of the seam itself is in question (where a port belongs, what a use case should expose), read `docs/architecture.md` §4 and the `clean-architecture-in-practice` skill; do not invent new layers.

## Anti-patterns

- **Implementation-coupled:** fakes internal collaborators, tests unexported helpers of another layer, or verifies through a side channel (querying SQLite directly instead of using the store or use case). The tell: the test breaks on refactor while behavior is unchanged.
- **Tautological:** the expected value is recomputed the way the code computes it, so it passes by construction. Expected values come from an independent source: a known literal, a worked example, the spec or a synthetic fixture.
- **Horizontal slicing:** writing all tests first, then all implementation. Work in vertical slices instead: one test → one implementation → repeat.
- **Unbounded or flaky:** real time, real network beyond loopback or the lab, or order-dependent state. Use the `Clock` port, `testing/synctest` and temp directories.

## Rules of the loop

- **Red before green.** Write the failing test first and watch it fail for the right reason, then write only enough code to pass it. No speculative features.
- **One slice at a time.** One seam, one test, one minimal implementation per cycle.
- **Keep the suite green.** Run `go test ./...` (and `go vet ./...`) after each green step; run the frontend check/build when the change crosses the bridge.
- **Refactoring is a separate step.** Do it after green, behavior-preserving, and reviewed with the `code-review-and-quality` skill.
- **Evidence, not claims.** A PLAN task is complete only with actual command output; cross-compilation never proves native behavior.
