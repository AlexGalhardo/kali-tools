# CI serve test isolation regression

Date: 2026-10-22. Scope: repair the failures in [workflow 39701217447](https://github.com/AlexGalhardo/kali-tools/actions/runs/39701217447) without weakening the workflow or changing production behavior.

## Reproduction and cause

The quality job runs Go tests before compiling the frontend. TestServeFlagsAndShutdown and TestServeOutputFailuresAndTLSWarning expected successful startup but passed the unbuilt embedded filesystem to httpapi.New, which correctly returned dependency_missing (exit 5).

The private Docker laboratory runs as UID 65532 without a writable default home. New serve tests isolated only Windows APPDATA; bootstrap.New on Linux tried the default user configuration path and returned failed (exit 1), before testing address validation or shutdown. The four hosted native builds succeeded because their frontend was already built and their runner accounts had writable homes.

## Correction

Only serve_test.go changes: use a synthetic index through the existing listen function seam; isolate APPDATA, XDG_CONFIG_HOME and HOME in each test that bootstraps the application. Add TestServeMissingAssets to explicitly retain the production missing-asset guard and ensure it produces exit 5 with no startup output. The real HTTP server, authentication, validation, TLS and shutdown are still exercised. No production test hook, dependency, workflow skip, permission expansion or release modification is introduced.

## Verification

- Windows: targeted serve tests under -race and go test ./... pass; go vet ./... passes.
- Frontend typecheck, lint and production build pass; no frontend source changed.
- `node scripts/lab.mjs` passes the complete private Docker laboratory with race detection, UID 65532, no compiled frontend and no writable default home. Compose exits 0 and removes the containers/network. The hosted rerun is checked during delivery; the failed hosted logs above are the before-fix evidence.
- Published 0.5.0 contents and tag remain immutable; this fix is recorded under Unreleased.

## Hosted follow-up: capability labels

[Run 39702097166](https://github.com/AlexGalhardo/kali-tools/actions/runs/39702097166) confirms the private Docker laboratory and clean-checkout Go tests now pass (root coverage remains 96.3%). Quality progressed into Playwright and exposed a previously hidden Linux-only assertion: unsupported Wi-Fi renders the catalog text "Not available", while the HTTP parity test incorrectly expected "Unsupported". The test now checks the English catalog for each supported state, including location consent. Production rendering is unchanged; the existing Wails unsupported/consent tests and the HTTP API-to-row/state comparison remain intact.
