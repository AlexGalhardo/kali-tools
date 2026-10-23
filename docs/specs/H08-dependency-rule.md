# H08 Enforce the dependency rule (and H09 CLI parser ADR)

| Field | Value |
| --- | --- |
| PLAN tasks | H08, H09 |
| Status | Verified 2026-10-21; positive run 39669810016 and negative quality proof 39669870071. H09 ADR accepted by the owner on 2026-10-23 |
| Depends on | H03, H05, H06, H07 |
| Risk | Low for builds (lint configuration only); Medium for correctness: a wrong glob silently matches no file, so a deliberate-violation check is mandatory |

## 1. Goal

The dependency matrix in `docs/architecture.md` §3 becomes a mechanical check (D8). depguard rules in `.golangci.yml` reject every forbidden import edge in `golangci-lint run`, which already runs in Lefthook pre-commit and in CI. A script plants one forbidden import per rule, proves that each rule fires, and cleans up; CI runs it. In the same spec, the CLI parser decision (H09) is recorded as `docs/adr/004-cli-parser.md`, whose required content this spec defines.

## 2. Non-goals

- Do not fix architecture violations found by the new rules inside this spec beyond what §6 step 2 allows. If `golangci-lint run` reports a real violation in production code, STOP and report it with the file and import.
- Do not add go-arch-lint or a new Go dependency. depguard ships inside the pinned golangci-lint v2.15.0.
- Do not change linters other than enabling `depguard`.
- Do not implement the CLI parser change; the ADR only decides. C01 implements.
- The implementing agent does not choose the ADR outcome freely: §4.5 defines the decision rule.

## 3. Current state

Facts at commit `8cbf25e` (re-verify):

- `.golangci.yml`: `version: "2"`, `linters.default: standard` plus 12 enabled linters, no `depguard`; `errcheck.exclude-functions`; formatters `gofmt` (simplify) and `goimports` (local prefix `github.com/AlexGalhardo/kali-tools`).
- `lefthook.yml`: pre-commit runs `golangci-lint@v2.15.0 fmt` on staged Go files, then `golangci-lint@v2.15.0 run ./...`, then `go vet ./...`; pre-push runs `fmt --diff`, `go test`, typecheck and `govulncheck`.
- `.github/workflows/ci.yml` job `quality` installs `golangci-lint@v2.15.0` and runs `node scripts/task.mjs check`, which runs `golangci-lint run ./...` (`scripts/task.mjs:11`), then `race`, `fuzz`, `workflows`, `actionlint` and `node scripts/markdown.mjs`. Actions are pinned by SHA; workflow permissions are `contents: read`.
- `scripts/task.mjs:9` whitelists the task names `check, go, web, race, fuzz, build, dev, lab, workflows, bridge, bindings, scripts`.
- `docs/architecture.md` §3 matrix: domain → stdlib only; capabilities (`network`, `capture`, `wifi`) must not import `adapter/*`, `app`, Wails, `database/sql`; application (`app`, `job`) must not import `adapter/*`, Wails, `net/http`, `os/exec`; driven adapters must not import driving adapters, `cmd` or `app` (except for types); driving adapters must not import driven adapters; composition roots (`main.go`, `cmd/*`, `internal/bootstrap`) are imported by nothing.
- Known edge at `8cbf25e`: `internal/network/network.go:18` imports `internal/adapter/probe` (D1). `docs/architecture.md` §8 already marks D1 resolved; the code does not match. H03 removes the edge; this spec depends on H03 and must find no such import.
- `.agents/skills/golang-lint/SKILL.md` "Dependency rule (PLAN H08)" says the rule is checked by review until H08 lands.
- golangci-lint lints default build tags only; the root files `main.go`, `app.go` (removed by H05) and `bridge_smoke_test.go` carry `desktop`/`bindings`/`nativesmoke` tags and are not linted. They are composition roots and may import everything.
- `docs/adr/` holds ADR 001–003; format: `# ADR 00N: <title>`, a `Status:` line, prose paragraphs, a `Sources:` line (`docs/adr/003-optional-nmap.md`).
- CLI parser today: one stdlib `flag.FlagSet` per invocation, dispatch by `args[0]` (`cmd/kali-tools/main.go:41-60`, moved to `internal/adapter/cli` by H05). PLAN C01 lists 16 top-level commands, subcommands under `diagnose` (6), `wifi` (2), `capture` (3), `jobs` (3), `history` (2), `settings` (2), and global flags `--json`, `--timeout`, `--no-color`, `--lang`. No PLAN item requires shell completion. `docs/architecture-research.md` §7.2 says to re-evaluate "when the command tree grows beyond about six subcommands or shell completion is required".

## 4. Design

Verified checkout correction (2026-10-21): the external `domain_test` package imports its own domain package. The strict `domain` rule must allow that exact import (`internal/domain$`) in addition to stdlib. This cannot introduce an outward production edge: importing the package itself would be a Go import cycle. No production violation was found. The eight deliberate negative cases remain unchanged.

### 4.1 depguard semantics used

- golangci-lint v2 schema: `linters.enable: [depguard]` and `linters.settings.depguard.rules.<name>`.
- `files`: globs matched against the absolute, slash-separated file path. `$all` and `$test` are built-ins; a leading `!` negates. Every rule lists at least one positive glob.
- Use `**/internal/<pkg>/**` (matches every file below). Never `**/internal/<pkg>/**/*.go`: it requires a further `/` and misses files directly in the package directory.
- `list-mode: strict` denies everything not in `allow`; `list-mode: lax` allows everything not in `deny`.
- `deny[].pkg` and `allow` entries match by import-path prefix (`net/http` also denies `net/http/httptest`). `$gostd` means the standard library.

### 4.2 Rules (append to `.golangci.yml`)

```yaml
linters:
  enable:
    # existing entries stay; add:
    - depguard
  settings:
    depguard:
      rules:
        domain:
          list-mode: strict
          files:
            - "**/internal/domain/**"
          allow:
            - $gostd
        capabilities:
          list-mode: lax
          files:
            - "**/internal/network/**"
            - "**/internal/capture/**"
            - "**/internal/wifi/**"
            - "!$test"
          deny:
            - pkg: github.com/AlexGalhardo/kali-tools/internal/adapter
              desc: "capabilities declare ports; adapters are injected (architecture §3)"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/app
              desc: "capabilities sit below the application layer"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/job
              desc: "capabilities sit below the application layer"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/bootstrap
              desc: "only composition roots wire"
            - pkg: github.com/wailsapp/wails
              desc: "Wails is confined to adapter/wailsui and the desktop root"
            - pkg: database/sql
              desc: "SQL belongs to adapter/storage"
        application:
          list-mode: lax
          files:
            - "**/internal/app/**"
            - "**/internal/job/**"
            - "!$test"
          deny:
            - pkg: github.com/AlexGalhardo/kali-tools/internal/adapter
              desc: "the application declares ports; adapters implement them (architecture §3)"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/bootstrap
              desc: "only composition roots wire"
            - pkg: github.com/wailsapp/wails
              desc: "Wails is confined to adapter/wailsui and the desktop root"
            - pkg: net/http
              desc: "HTTP belongs to adapter/httpapi or a capability"
            - pkg: os/exec
              desc: "processes belong to adapter/tools"
            - pkg: database/sql
              desc: "SQL belongs to adapter/storage"
        driven-adapters:
          list-mode: lax
          files:
            - "**/internal/adapter/storage/**"
            - "**/internal/adapter/tools/**"
            - "**/internal/adapter/probe/**"
            - "**/internal/adapter/platform/**"
            - "!$test"
          deny:
            - pkg: github.com/AlexGalhardo/kali-tools/internal/adapter/wailsui
              desc: "driven adapters never call driving adapters"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/adapter/httpapi
              desc: "driven adapters never call driving adapters"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/adapter/cli
              desc: "driven adapters never call driving adapters"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/adapter/contract
              desc: "the frontend contract belongs to driving adapters"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/bootstrap
              desc: "only composition roots wire"
            - pkg: github.com/wailsapp/wails
              desc: "Wails is confined to adapter/wailsui and the desktop root"
        driving-adapters:
          list-mode: lax
          files:
            - "**/internal/adapter/wailsui/**"
            - "**/internal/adapter/httpapi/**"
            - "**/internal/adapter/cli/**"
            - "**/internal/adapter/contract/**"
            - "!$test"
          deny:
            - pkg: github.com/AlexGalhardo/kali-tools/internal/adapter/storage
              desc: "driving adapters go through app.Application (architecture §3)"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/adapter/tools
              desc: "driving adapters go through app.Application"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/adapter/probe
              desc: "driving adapters go through app.Application"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/adapter/platform
              desc: "driving adapters go through app.Application"
            - pkg: github.com/AlexGalhardo/kali-tools/internal/bootstrap
              desc: "only composition roots wire"
            - pkg: database/sql
              desc: "SQL belongs to adapter/storage"
            - pkg: os/exec
              desc: "processes belong to adapter/tools"
        wails:
          list-mode: lax
          files:
            - "**/internal/**"
            - "**/cmd/**"
            - "**/tests/**"
            - "!**/internal/adapter/wailsui/**"
          deny:
            - pkg: github.com/wailsapp/wails
              desc: "Wails is confined to adapter/wailsui and the desktop root (main.go)"
        bootstrap:
          list-mode: lax
          files:
            - "**/internal/**"
            - "!**/internal/bootstrap/**"
            - "!$test"
          deny:
            - pkg: github.com/AlexGalhardo/kali-tools/internal/bootstrap
              desc: "bootstrap is imported only by composition roots (main.go, cmd/*) and tests"
        porttest:
          list-mode: lax
          files:
            - "$all"
            - "!$test"
            - "!**/internal/porttest/**"
          deny:
            - pkg: github.com/AlexGalhardo/kali-tools/internal/porttest
              desc: "contract suites and fakes are test-only"
```

Notes:

- Test files are excluded from the layer rules because tests legitimately use `net/http/httptest`, `bootstrap` and `porttest`. The `domain` and `wails` rules keep applying to tests.
- `contract` may import `app`, `job` and capability packages; it is listed with the driving adapters because it is part of their side of the boundary.
- Driven adapters may import `app` and `job` for port types; "types only" stays a review rule.
- `cmd/*` and the repository root are composition roots; only the `wails` rule (for `cmd/*`) and the `porttest` rule apply to them.
- If H03 placed a driven adapter in another directory (for example a Wi-Fi provider under `internal/adapter/wifi`), add its glob to `driven-adapters` and its import path to the `driving-adapters` deny list.

### 4.3 Deliberate-violation check (`scripts/depguard-check.mjs`)

A Node script, like the other task runners, with no dependencies beyond `node:` modules.

```text
for each case:
    write <dir>/zz_depguard_violation.go:
        // Code generated by scripts/depguard-check.mjs; DO NOT COMMIT.
        package <pkg>

        import _ "<forbidden import>"
    run: golangci-lint run --enable-only depguard ./<dir>/
    expect: exit status != 0
            output contains "zz_depguard_violation.go"
            output contains "list '<rule>'" (depguard's message names the rule)
            output does not contain "typecheck" or "import cycle"
    finally: delete the file (also on failure, and on SIGINT)
after all cases:
    run: golangci-lint run --enable-only depguard ./...   (expect exit 0: the clean tree passes)
    assert: no zz_depguard_violation.go remains (git status --porcelain)
print: "depguard check passed: <n> rules fired"
exit 1 on the first unmet expectation, printing the case and the linter output
```

| Case | Directory | Package | Forbidden import | Rule expected |
| --- | --- | --- | --- | --- |
| 1 | `internal/domain` | `domain` | `github.com/gopacket/gopacket` | `domain` |
| 2 | `internal/wifi` | `wifi` | `database/sql` | `capabilities` |
| 3 | `internal/app` | `app` | `os/exec` | `application` |
| 4 | `internal/adapter/tools` | `tools` | `github.com/AlexGalhardo/kali-tools/internal/adapter/cli` | `driven-adapters` |
| 5 | `internal/adapter/cli` | `cli` | `github.com/AlexGalhardo/kali-tools/internal/adapter/storage` | `driving-adapters` |
| 6 | `cmd/lab-fixture` | `main` | `github.com/wailsapp/wails/v2/pkg/runtime` | `wails` |
| 7 | `internal/adapter/cli` | `cli` | `github.com/AlexGalhardo/kali-tools/internal/bootstrap` | `bootstrap` |
| 8 | `internal/adapter/storage` | `storage` | `github.com/AlexGalhardo/kali-tools/internal/porttest` | `porttest` |

The imports were chosen to avoid import cycles (a cycle produces a `typecheck` error that hides depguard). If a case reports `typecheck` or `import cycle` because H03–H07 introduced a new edge, choose another forbidden import from the same rule's deny list, update the table in the evidence record and keep one case per rule.

The binary is `golangci-lint` from `PATH` (CI installs v2.15.0; developers use the same version). The script prints `golangci-lint --version` first and fails unless it contains `2.15.0`.

Wiring:

- `scripts/task.mjs`: add `"arch"` to the task whitelist and `if(task==="arch")run("node",["scripts/depguard-check.mjs"]);`.
- `.github/workflows/ci.yml`, job `quality`: add `- run: node scripts/task.mjs arch` directly after `- run: node scripts/task.mjs check`. No new action, no permission change.
- Lefthook: no change. depguard runs inside the existing pre-commit `golangci-lint run ./...`.

### 4.4 Documentation of the rule

`docs/architecture.md` §3 gains a sentence after the table: "The matrix is enforced by depguard rules in `.golangci.yml` (rule names `domain`, `capabilities`, `application`, `driven-adapters`, `driving-adapters`, `wails`, `bootstrap`, `porttest`); `node scripts/task.mjs arch` proves each rule fires." Add `database/sql` to the application row's "Must not import" column and add the `contract` row if H06 has not.

### 4.5 ADR 004: CLI parser (H09)

File `docs/adr/004-cli-parser.md`, written by the implementing agent, in the ADR 003 format. Required content, in this order:

1. Title line: `# ADR 004: CLI argument parser`.
2. `Status:` `accepted` if the repository owner approves in the PR; otherwise `proposed`. Do not write `accepted` without that approval.
3. Context paragraph with these facts, each verified: the H05 parser (stdlib `flag`, one `FlagSet` per command, file and line); the C01 tree from PLAN §7.1 (16 top-level commands, the subcommand counts listed in §3, maximum depth 2, the four global flags); whether any PLAN or brainstorming item requires shell completion (cite the search command and result); the research trigger "beyond about six subcommands or shell completion" (`docs/architecture-research.md` §7.2).
4. Options paragraph: (a) stdlib `flag` with a command table in `internal/adapter/cli` (one `FlagSet` per leaf command, global flags registered on every `FlagSet` by one helper, help generated from the table); (b) Cobra on pflag. For (b) state the module paths and the versions that would be pinned, their licenses (Cobra Apache-2.0, pflag BSD-3-Clause, and any transitive module such as `github.com/inconshreveable/mousetrap` on Windows, Apache-2.0), verified from the module's LICENSE file, and the impact on `docs/dependencies.md` and `scripts/notices.mjs`.
5. Decision paragraph, applying this rule: choose Cobra if and only if shell completion is a PLAN requirement, or the C01 tree needs depth greater than 2, or the flag syntax required by C01 cannot be expressed with stdlib `flag` (for example interleaved flags after positional arguments that C01 explicitly requires). Otherwise choose stdlib `flag`. With the facts in §3 the rule yields stdlib `flag`; if a verified fact differs, apply the rule to the verified facts and say so.
6. Consequences paragraph: what C01 must implement (command table, per-command help, documented exit codes, `--json` on every command) and what is given up (no generated completion, Go-style single-dash flags accepted).
7. Revisit trigger paragraph, mandatory (PLAN H09 assertion): revisit when any of these becomes true: shell completion is added to PLAN; a command needs depth 3; the CLI has more than 40 leaf commands; or users require POSIX flag interleaving. Name who decides (repository owner) and where the change is recorded (a superseding ADR).
8. `Sources:` line linking `docs/architecture-research.md` §7.2, PLAN §7.1, the Cobra and pflag repositories and their license files.

## 5. Files

Create:

- `scripts/depguard-check.mjs`: deliberate-violation check (§4.3).
- `docs/adr/004-cli-parser.md`: ADR (§4.5).

Modify:

- `.golangci.yml`: enable `depguard`, add the rules (§4.2). Keep all existing settings.
- `scripts/task.mjs`: `arch` task.
- `.github/workflows/ci.yml`: one `run` step in `quality`.
- `docs/architecture.md`: §3 (§4.4), §8 D8 resolved, §9 command list (`node scripts/task.mjs arch`).
- `docs/architecture-research.md` §11: name the chosen tool (depguard) and the script; §13 "CLI parser" row links ADR 004.
- `.agents/skills/golang-lint/SKILL.md`: replace "Until H08 lands, this is checked by review" with the rule names and `node scripts/task.mjs arch`; remove the stale D1 sentence.
- `docs/development.md`: add `node scripts/task.mjs arch` to the task list.
- `PLAN.md`: H08 and H09 checkboxes with evidence links.

Delete: nothing.

## 6. Implementation steps

1. **Baseline.** Run `golangci-lint run ./...` and record `0 issues.`.
2. **Rules.** Add §4.2 to `.golangci.yml`. Run `golangci-lint run ./...`. If it reports issues in test helpers that are not `_test.go` files (for example a `testdata` generator), narrow the rule with an extra `!` glob and record why. If it reports a production violation, STOP and report.
3. **Violation script.** Write `scripts/depguard-check.mjs` and the `arch` task. Run `node scripts/task.mjs arch` on Windows and confirm all 8 cases fire and the tree is clean afterwards.
4. **CI.** Add the `arch` step. Run `node scripts/task.mjs workflows` and `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`. Push to a branch and confirm the `quality` job runs the step and passes; record the run URL.
5. **Negative CI proof.** On a throwaway branch (never merged), commit one violation from the table (case 3) and push; confirm the `quality` job fails at `golangci-lint run` with the `application` rule. Record the run URL, then delete the branch.
6. **ADR.** Write `docs/adr/004-cli-parser.md` per §4.5. Run `node scripts/markdown.mjs`.
7. **Docs.** Apply the doc changes in §5. Run the full §9 block.
8. **Evidence.** Write `docs/verification/specs/H08.md` with the README §5.3 structure, including both CI run URLs and the script output.

Commits: `build(lint): enforce dependency rule with depguard (H08)` and `docs(adr): record CLI parser decision (H09)`.

## 7. Acceptance criteria

- AC-1: Given the tree after H07, when `golangci-lint run ./...` runs with the new rules, then it reports `0 issues.`.
- AC-2: Given each of the 8 cases in §4.3, when the script plants the file and runs depguard, then depguard reports the planted file under the expected rule name, without `typecheck` or `import cycle` errors.
- AC-3: Given the script finished (pass or fail), when `git status --porcelain` runs, then no `zz_depguard_violation.go` file exists.
- AC-4: Given `golangci-lint` other than v2.15.0 on `PATH`, when the script starts, then it exits 1 with a version message.
- AC-5: Given a pushed commit with a forbidden import in `internal/app`, when CI runs, then the `quality` job fails at the lint step.
- AC-6: Given a staged Go change with a forbidden import, when `git commit` runs with Lefthook installed, then the pre-commit `go-lint` command fails.
- AC-7: Given `docs/adr/004-cli-parser.md`, when reviewed, then it contains the eight required parts of §4.5, a decision consistent with the decision rule and the verified facts, and a revisit-trigger paragraph naming concrete conditions.
- AC-8: Given test files that import `net/http/httptest`, `internal/bootstrap` or `internal/porttest`, when linted, then they produce no depguard issue.

## 8. Tests

| Test or check | Asserts | AC |
| --- | --- | --- |
| `golangci-lint run ./...` | clean tree passes | AC-1, AC-8 |
| `node scripts/task.mjs arch` (script cases 1–8 and the final clean run) | every rule fires on its planted file; cleanup | AC-2, AC-3 |
| script started with a fake `golangci-lint` on `PATH` printing `golangci-lint has version 2.14.0` (manual, recorded in evidence) | version guard | AC-4 |
| throwaway-branch CI run (step 5) | CI fails on a violation | AC-5 |
| manual commit with a planted violation and Lefthook installed, then `git restore --staged` and delete (recorded in evidence) | hook fails | AC-6 |
| reviewer checklist of §4.5 | ADR completeness | AC-7 |

There are no new Go tests; the rule is configuration and the proof is the script.

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
```

Spec-specific checks:

```bash
golangci-lint --version                 # must report 2.15.0
node scripts/task.mjs arch              # prints "depguard check passed: 8 rules fired"
git status --porcelain | grep zz_depguard_violation && exit 1 || true
node scripts/task.mjs workflows
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
node scripts/markdown.mjs
```

Binding regeneration is not affected; `node scripts/task.mjs bindings` must still exit 0 on native CI.

## 10. Documentation updates

- `docs/architecture.md`: §3 enforcement sentence and application row; §8 D8 resolved with the commit; §9 adds `node scripts/task.mjs arch`.
- `docs/architecture-research.md`: §11 states the chosen tool and script; §13 "CLI parser" row links ADR 004.
- `.agents/skills/golang-lint/SKILL.md`: dependency-rule section updated.
- `docs/development.md`: `arch` task.
- `docs/adr/004-cli-parser.md`: new.
- `PLAN.md`: H08, H09.

## 11. Risks and rollback

- Glob mismatch (for example Windows paths or a `**/*.go` suffix) makes a rule match nothing and pass silently. The violation script fails in that case; it is the guard.
- Prefix matching can over-deny (`net/http` also denies `net/http/httptest` in production code). Production code in `app`/`job` has no legitimate need for either.
- A future package outside the listed globs is unconstrained. Mitigation: the "How to extend" recipe in `docs/architecture.md` §7 must add the new package to a rule; reviewers check it.
- The ADR could be written as `accepted` without owner approval. §4.5 forbids it; the reviewer checks the PR approval.
- Rollback: `git revert` the H08 commit. The ADR commit is independent and can stay.

## 12. Quality and delivery assertions

Applicable global gates (README §5.1):

| Gate | Threshold for H08 |
| --- | --- |
| QG-1, QG-3, QG-4, QG-13, QG-14 | as defined; commit messages contain `(H08)` or `(H09)` |
| QG-2 | `0 issues.` with depguard enabled |
| QG-10 | becomes mechanical in this spec: depguard plus `node scripts/task.mjs arch` |
| QG-15 | `Markdown checks passed` including the new ADR |
| QG-6, QG-8, QG-9, QG-11, QG-12 | not applicable (no Go, frontend, binding or bounded-resource change); state "not applicable" in the evidence record |

Delivery assertions:

| DA | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | depguard is enabled with the 8 named rules | `grep -cE '^        (domain\|capabilities\|application\|driven-adapters\|driving-adapters\|wails\|bootstrap\|porttest):$' .golangci.yml` | `8` | reviewer |
| DA-2 | Every rule fires on a planted violation | `node scripts/task.mjs arch` | `depguard check passed: 8 rules fired`, exit 0 | CI |
| DA-3 | The clean tree passes | `golangci-lint run ./...` | `0 issues.` | hook, CI |
| DA-4 | CI fails on a forbidden import | throwaway-branch run URL in the evidence record | `quality` job failed at the lint step | reviewer |
| DA-5 | Lefthook blocks a forbidden import | recorded pre-commit output in the evidence record | `go-lint` failed | reviewer |
| DA-6 | ADR 004 names its revisit trigger | `grep -n -i 'revisit' docs/adr/004-cli-parser.md` | at least one match in a paragraph listing the §4.5 conditions | reviewer |
| DA-7 | CI step present and pinned workflow rules intact | `grep -n 'task.mjs arch' .github/workflows/ci.yml` and `node scripts/task.mjs workflows` | one match; workflows check passes | CI, reviewer |

Evidence record: `docs/verification/specs/H08.md` (covers H09; the ADR path is listed there).
