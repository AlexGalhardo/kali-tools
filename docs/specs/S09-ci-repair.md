# S09 Repair hosted native checks

| Field | Value |
| --- | --- |
| PLAN tasks | S09 |
| Status | In progress |
| Depends on | H07 implementation |
| Risk | Low; platform-aware assertions and native WLAN error mapping |

## 1. Goal

Repair the failures from run 39655757089 without skipping tests or weakening gates. The owner explicitly requested fixing all CI errors on 2026-10-21.

## 2. Non-goals

No new platform provider, dependency, release, or native hardware qualification from hosted builds.

## 3. Current state

The Windows runner returns WLAN service-not-active code 1062, incorrectly mapped to `failed`. Both macOS jobs fail because bootstrap and binding tests assume a host provider exists, and because the config-path test assumes all operating systems use the same config subdirectory. Linux and Docker passed. Logs: [run 39655757089](https://github.com/AlexGalhardo/kali-tools/actions/runs/39655757089).

## 4. Design

Map Windows WLAN service-not-active to typed `unsupported`, with no machine-specific data. Test that mapping directly. Bootstrap tests must require typed `unsupported` on platforms whose provider is unsupported, and success on Windows/Linux. The binding delegation assertion compares its result and mapped error to the application's result. Derive the expected config directory through `os.UserConfigDir` after isolating HOME/APPDATA/XDG_CONFIG_HOME. No cancellation or redaction assertion changes.

## 5. Files

Modify `internal/wifi/wifi_windows.go`, `internal/bootstrap/bootstrap_test.go`, `internal/adapter/wailsui/bindings_test.go`, PLAN.md and CHANGELOG.md. Create `internal/wifi/wifi_windows_test.go` and `docs/verification/specs/S09.md`.

## 6. Implementation steps

1. Preserve hosted failing logs; add a failing pure WLAN mapping test.
2. Correct WLAN mapping and platform/path assumptions.
3. Run host tests, race tests, vet, lint and workflow validation.
4. Push with hooks; inspect the hosted run and fix any remaining failures.

## 7. Acceptance criteria

- AC-1: WLAN error 1062 maps to unsupported; code 5 stays permission_required; unknown errors stay failed.
- AC-2: Native provider and binding delegation tests accept only the implemented result or specified unsupported result.
- AC-3: Default storage creation is verified at the actual platform config path.
- AC-4: Hosted quality, Docker and all native jobs pass; no test is removed or skipped.

## 8. Tests

`TestWLANErrorStates`, `TestNativeProviderAdapters`, `TestBindingsDelegateToApplication`, `TestNewDefaultPath` and the existing full suite.

## 9. Verification

Run `go test ./...`, `go test -race ./...`, `go vet ./...`, desktop/Linux/Darwin vet, `golangci-lint fmt --diff ./...`, `golangci-lint run ./...`, `node scripts/task.mjs workflows`, `node scripts/markdown.mjs`, `node scripts/release-policy.mjs`; inspect the new GitHub run.

## 10. Documentation updates

Record the actual logs and hosted results in `docs/verification/specs/S09.md`; update PLAN and Unreleased.

## 11. Risks and rollback

A stopped WLAN service is unavailable, not evidence of a connected network. Revert the repair commit to roll back; no storage or public signature changes.

## 12. Quality and delivery assertions

Global QG-1 through QG-15 apply as relevant; reuse the unchanged frontend/binding checks from H07, record touched-package coverage before/after, and record hosted race results. DA-1: the WLAN regression fails before and passes after. DA-2: no skip or continue-on-error is added. DA-3: the new hosted run URL shows all required jobs pass. Release draft creation does not establish signing or hardware acceptance.

Evidence record: `docs/verification/specs/S09.md`.
