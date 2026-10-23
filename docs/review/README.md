# Release qualification review

Date: 2026-10-23. These notes prepare R02. They do not accept it. No child of R02 is checked.

| Open check | Note | Accepted |
| --- | --- | --- |
| Regression after the final change | CI is the suite. It has not been repeated after hardware acceptance, because that acceptance has not happened. | no |
| Native install, upgrade, rollback, uninstall, offline | [Install, upgrade and rollback](install-upgrade-rollback.md) | no |
| Numeric performance budgets and unresolved high vulnerabilities | Idle memory and CPU were never measured, so no numeric budget is agreed. `govulncheck` runs in CI. A green run is not a finding recorded in this note. | no |
| PT-BR, English, keyboard and accessibility | [Accessibility](accessibility.md) | no |
| External web-server threat-model review | [Threat model](threat-model.md) | no |

The release checklist does not link a reproducible hardware result. R03 stays open. No 1.0 tag is created from these notes.
