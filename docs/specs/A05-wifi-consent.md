# A05 Wi-Fi location consent and reveal states

| Field | Value |
| --- | --- |
| PLAN tasks | A05 (location consent, reveal states, native acceptance) |
| Status | Ready |
| Depends on | H03 (`app.WiFiProvider`), H04 (`app.Application`), H05 (driving adapters), H06 (`location_consent_required` state, contract DTOs, error mapping), W01 + W04 (HTTP route) |
| Risk | Medium: the consent behavior can only be proven on a real Windows 11 24H2 host; a wrong mapping would hide a real permission denial |

## 1. Goal

On Windows 11 24H2 and later, reading the current Wi-Fi connection fails with `ERROR_ACCESS_DENIED` when the user has not granted location access to the app. Today kali-tools reports that failure as a generic `permission_required`. After this spec:

- the Wi-Fi provider reports the stable error code `location_consent_required` for that case;
- the application exposes it as the `wifi` capability state;
- every interface tells the user how to fix it. The desktop opens `ms-settings:privacy-location` through Wails `BrowserOpenURL`. The CLI and the web UI print the URI and the settings path.

The explicit key-reveal action keeps its existing masking and auto-clear behavior. Its result states (`open_network`, `enterprise`, `no_saved_key`, `permission_required`, `available`) are kept stable and covered by tests.

## 2. Non-goals

- No WinRT. Do not call `AppCapability.CheckAccess("wiFiControl")` or `WlanConnectionProfileDetails.GetConnectedSsid` in this spec. §4.7 records that work as a follow-up.
- No automatic retry, polling or background re-query after a denial. PLAN A05 says "never retry silently".
- No change to the Wi-Fi metadata fields, the visible-BSS list or scanning (`WlanScan` is not used).
- No Linux/macOS provider (B01/B02). The non-Windows provider keeps returning `unsupported`.
- No key reveal over HTTP or the CLI. The current CLI usage text (`cmd/kali-tools/main.go:37`) states that reveal is desktop-only. This spec keeps that rule. Changing it needs an owner decision.
- No change to the masking timer (15 s), the blur/visibility clearing or the reveal epoch logic in the frontend.
- No elevation, no registry writes and no change to Windows privacy settings by the app.

## 3. Current state

Re-verify each fact before starting. The working tree was being reorganized when this spec was written, so line numbers can drift.

1. `internal/wifi/wifi_windows.go:15-21` lazily loads `wlanapi.dll` procs: `WlanOpenHandle`, `WlanCloseHandle`, `WlanEnumInterfaces`, `WlanQueryInterface`, `WlanGetProfile` and `WlanFreeMemory`.
2. `internal/wifi/wifi_windows.go:51-56` has `wlanError(code)`. It maps 5 (`ERROR_ACCESS_DENIED`) to `domain.Error{Code: "permission_required"}` and maps every other non-zero code to `failed`.
3. `internal/wifi/wifi_windows.go:82-86`: `connected()` calls `WlanQueryInterface` with opcode `7` (`wlan_intf_opcode_current_connection`). Any non-zero return goes through `wlanError`, so the 24H2 location denial surfaces today as `permission_required`.
4. `internal/wifi/wifi_windows.go:108-117`: `Current()` queries opcode `8` (`wlan_intf_opcode_channel_number`). Failure leaves `Channel` nil and is not reported.
5. `internal/wifi/wifi_windows.go:120-153`: `Reveal()` calls `connected()` first and then decides:
   - `SecurityEnabled == 0` gives state `open_network`;
   - `OneX != 0` gives state `enterprise`;
   - otherwise it calls `WlanGetProfile` with flags `4` (`WLAN_PROFILE_GET_PLAINTEXT_KEY`) and reads at most 65,536 UTF-16 units.
6. `internal/wifi/profile.go:10-30`: `secretFromProfile` rejects profiles over 128 KiB. It maps `protected=true` to state `permission_required`, an empty `keyMaterial` to `no_saved_key`, and otherwise returns `available` with the value.
7. `internal/wifi/wifi.go:3-15` defines `Network` and `Secret{State, Value}`.
8. `internal/wifi/wifi_windows_test.go:9-11` pins the native struct sizes: `iface` 532, `association` 68, `connection` 604 bytes.
9. `internal/wifi/profile_test.go` covers `available`, `permission_required` and `no_saved_key` with synthetic profiles.
10. `internal/app/capabilities.go:9-26`: `Capabilities()` is static. On Windows, `wifi` is always `available` with detail "Native Windows API; operational permission checks occur on request".
11. `internal/domain/domain.go:5-17`: `domain.Error{Code, Message}`. No code constants exist yet; H06 adds the `location_consent_required` state.
12. `app.go:171-172`: `CurrentWiFi()` and `RevealWiFi()` call `wifi.Current()` and `wifi.Reveal()` directly (deviation D2).
13. `cmd/kali-tools/main.go:119-120`: the `wifi` command prints `wifi.Current()` in the JSON envelope. Typed errors become `envelope.Error` with exit code 1 (`main.go:152-167`).
14. Frontend:
    - `frontend/src/app/App.tsx:236-252` clears the secret on `blur` and `visibilitychange`, and after 15,000 ms;
    - `App.tsx:696` renders a mask (`••••••••••••`) when no secret is shown;
    - `App.tsx:147` and `App.tsx:704-709` use a reveal epoch to discard late results;
    - `App.tsx:66-95` maps states and error codes to i18n labels. `location_consent_required` is not mapped.
15. `frontend/tests/workflows.spec.ts:27` holds the test "Wi-Fi key clears on navigation and blur".
16. Wails v2.17.0 `runtime.BrowserOpenURL(ctx, url)` validates the URL with `internal/frontend/utils/urlValidator.go`. That validator rejects only the `javascript`, `data`, `file`, `ftp` and empty schemes, control characters and shell metacharacters. `ms-settings:privacy-location` passes. On failure it only logs; it returns nothing to the caller.

## 4. Design

### 4.1 Error code and constants

`internal/domain/domain.go` must contain the following. Add whichever H06 did not already add, without duplicating:

```go
const (
    CodeLocationConsentRequired = "location_consent_required"
    CodePermissionRequired      = "permission_required"
)

// LocationSettingsURI is the only URI any adapter may open for Wi-Fi consent.
const LocationSettingsURI = "ms-settings:privacy-location"
```

If H06 already introduced code constants under other names, use H06's names and record the deviation in the report.

### 4.2 Provider mapping (Windows)

Add a pure mapping function in `internal/wifi/consent.go`. This file has no build tag, so it is testable on every OS:

```go
package wifi

// Windows11_24H2Build is the first build where WLAN location consent gates
// wlan_intf_opcode_current_connection (tools research §3.3).
const Windows11_24H2Build = 26100

const errorAccessDenied = 5

// currentConnectionError maps a non-zero WlanQueryInterface(current_connection)
// return code. build is RtlGetVersion().BuildNumber.
func currentConnectionError(code uintptr, build uint32) error
```

Rules:

| `code` | `build` | Result |
| --- | --- | --- |
| 5 | `>= 26100` | `&domain.Error{Code: domain.CodeLocationConsentRequired, Message: consentMessage}` |
| 5 | `< 26100` | `&domain.Error{Code: "permission_required", Message: "Windows denied WLAN access"}` (unchanged) |
| any other non-zero | any | `&domain.Error{Code: "failed", Message: fmt.Sprintf("WLAN API returned %d", code)}` (unchanged) |

The message is a fixed string with no user or system data:

```go
const consentMessage = "Windows location access is off for this app. Open Settings > Privacy & security > Location (" + domain.LocationSettingsURI + "), allow location for desktop apps, then try again."
```

In `internal/wifi/wifi_windows.go`, change only the opcode-7 failure branch in `connected()` (today `wifi_windows.go:83-86`):

```go
if code != 0 {
    closeHandle.Call(uintptr(h), 0)
    return 0, iface{}, connection{}, currentConnectionError(code, windows.RtlGetVersion().BuildNumber)
}
```

The following stay unchanged:

- `WlanOpenHandle`, `WlanEnumInterfaces` and `WlanGetProfile` failures keep using `wlanError`. Location consent does not gate these calls, so `ERROR_ACCESS_DENIED` from them remains `permission_required`.
- The opcode-8 channel query keeps its silent fallback.
- `Reveal()` calls `connected()`, so a consent denial makes `Reveal()` return the same `location_consent_required` error before any profile is read.

Make no retry. Each call to `Current()` or `Reveal()` issues exactly one `WlanQueryInterface(7)` per connected interface, as today.

### 4.3 Reveal states (unchanged contract, now pinned)

| Situation | Return |
| --- | --- |
| Location consent missing (24H2+) | error `location_consent_required` |
| No connected interface | error `no_connection` (existing) |
| `SecurityEnabled == 0` | `Secret{State: "open_network"}` |
| `OneX != 0` | `Secret{State: "enterprise"}` |
| `WlanGetProfile` returns 5 | error `permission_required` |
| Profile `protected=true` | `Secret{State: "permission_required"}` |
| Profile without `keyMaterial` | `Secret{State: "no_saved_key"}` |
| Plaintext key present | `Secret{State: "available", Value: key}` |

To make the first three rows unit-testable, extract the decision in `Reveal()` into a pure function in `internal/wifi/consent.go`:

```go
// revealState decides the reveal outcome before any profile is read.
// It returns ok=false when the profile must be fetched.
func revealState(securityEnabled, oneX uint32) (state string, ok bool)
```

`Reveal()` calls `revealState(c.SecurityEnabled, c.OneX)` and returns `Secret{State: state}` when `ok` is true.

### 4.4 Application layer

`app.WiFiProvider` (declared by H03 in `internal/app`) is unchanged. `app.Application` gains an observed-state memory and reports it through `Capabilities`:

```go
// internal/app (file: wifi.go)
type wifiObservation struct {
    mu    sync.Mutex
    state string // "" (unknown), "available", "location_consent_required", "permission_required"
}

func (a *Application) CurrentWiFi(ctx context.Context) (wifi.Network, error) // existing H04 method; now records the observation
func (a *Application) RevealWiFi(ctx context.Context) (wifi.Secret, error)   // existing H04 method; now records the observation
```

Recording rules, which live in one helper `func (o *wifiObservation) record(err error)`:

- `err == nil` sets `available`.
- `domain.Error` with code `location_consent_required` or `permission_required` sets that code.
- Any other error leaves the state unchanged.

In `internal/app/capabilities.go`, add a pure overlay:

```go
// WithObserved replaces the state and detail of capability id when state is non-empty.
func WithObserved(all []domain.Capability, id, state, detail string) []domain.Capability
```

`Application.Capabilities(ctx)` returns `WithObserved(Capabilities(), "wifi", observed, detailFor(observed))` with these details:

- `location_consent_required` → "Windows location access is off for this app; open ms-settings:privacy-location";
- `permission_required` → "Windows denied WLAN access".

`Capabilities()` itself is not changed and never calls the WLAN API. Calling WLAN while listing capabilities could trigger the one-time consent prompt without user intent.

### 4.5 Driving adapters

| Interface | Change |
| --- | --- |
| Wails (`internal/adapter/wailsui`) | New binding `OpenLocationSettings() error`. It calls the injected `browserOpenURL(ctx, domain.LocationSettingsURI)` field, which in production is Wails `runtime.BrowserOpenURL`. It takes no argument, so no arbitrary URL can be opened. On non-Windows it returns `domain.Unsupported("location settings exist only on Windows")`. Existing `CurrentWiFi`/`RevealWiFi` bindings return the mapped error unchanged. |
| HTTP (`internal/adapter/httpapi`) | `GET /api/wifi` returns HTTP 403 with body `{"code":"location_consent_required","message":<consentMessage>,"settings_uri":"ms-settings:privacy-location"}` for that code. The extra `settings_uri` field is added in the H06 error DTO as `SettingsURI string \`json:"settings_uri,omitempty"\``, set only for this code. There is no route that opens settings: the server may run under another account or on another host. |
| CLI (`internal/adapter/cli`) | `kali-tools wifi` prints the JSON envelope with `error.code = "location_consent_required"` and `error.message = consentMessage`, so the message contains the URI. Exit code 1. In human/text output mode (if C01 added one), also print the line `Open: ms-settings:privacy-location`. |
| DTO (`internal/adapter/contract`) | Add `SettingsURI` to the error DTO as above. `contract.WiFiSecret` must keep `Value` out of every log/history path. The H06 redaction test already covers that; extend its table with a `location_consent_required` case. |

If H05/W01 have not landed, add the binding to `app.go` and the CLI change to `cmd/kali-tools/main.go` until H05 lands, and record the HTTP route as a follow-up.

### 4.6 Frontend

- `frontend/src/shared/api/bridge.ts`:
  - add `OpenLocationSettings(): Promise<void>` to `Backend`;
  - add `export const LOCATION_SETTINGS_URI = "ms-settings:privacy-location";`.
  - In browser (HTTP) transport, `OpenLocationSettings` rejects with `{code: "unsupported"}`. The UI then shows the URI text instead of a button.
- New component `frontend/src/features/wifi/LocationConsentNotice.tsx`. Props: `{ canOpen: boolean; onOpen: () => void; onRetry: () => void }`. It renders:
  - a `role="alert"` paragraph with `t.locationConsentRequired`;
  - a `<code>` element with the URI;
  - a Base UI `Button` labelled `t.openLocationSettings`, only when `canOpen`;
  - a secondary button `t.checkAgain` that calls `onRetry`, which re-runs `CurrentWiFi()` once.
- `App.tsx` Wi-Fi page: when `CurrentWiFi()` or `RevealWiFi()` rejects with code `location_consent_required`, render `LocationConsentNotice` instead of the generic error. `canOpen` is `connected()` (Wails present).
- Add to `labels` and `errorLabels` in `App.tsx`: `location_consent_required: t.locationConsentRequired`.
- i18n keys, added to both `en` and `pt` (`pt: typeof en` enforces parity):

| Key | en | pt |
| --- | --- | --- |
| `locationConsentRequired` | "Windows location access is off for kali-tools, so Wi-Fi details are hidden. Allow location for desktop apps in Settings, then check again." | "O acesso à localização do Windows está desativado para o kali-tools, por isso os detalhes do Wi-Fi ficam ocultos. Permita a localização para apps da área de trabalho em Configurações e verifique novamente." |
| `openLocationSettings` | "Open location settings" | "Abrir configurações de localização" |
| `checkAgain` | "Check again" | "Verificar novamente" |

Masking (`App.tsx:696`), auto-clear (`App.tsx:248-252`, 15,000 ms), blur/visibility clearing (`App.tsx:236-247`) and the reveal epoch stay exactly as they are.

### 4.7 Follow-up (not in this spec): WinRT consent pre-check

`AppCapability.CheckAccess("wiFiControl")` lives in `Windows.Security.Authorization.AppCapabilityAccess`, which is WinRT. Calling it from Go needs:

- `RoInitialize`;
- `RoGetActivationFactory("Windows.Security.Authorization.AppCapabilityAccess.AppCapability")`;
- `IAppCapabilityStatics::Create(HSTRING)`;
- `IAppCapability::CheckAccess()`, which returns `AppCapabilityAccessStatus` (`DeniedBySystem=0`, `NotDeclaredByApp=1`, `DeniedByUser=2`, `UserPromptRequired=3`, `Allowed=4`).

That means about 150 lines of hand-written vtable code (go-ole is an indirect dependency through Wails, but it has no typed projection). Its behavior for an unpackaged desktop app is also unverified. Record it in `docs/tools-research.md` §11 as an open item: "WinRT consent pre-check (`AppCapability.CheckAccess`) for unpackaged Wails apps — needed only if the `ERROR_ACCESS_DENIED` mapping proves ambiguous on native 24H2 tests."

### 4.8 Limits

No new limits. The existing ones stay: 256 WLAN interfaces, 65,536 UTF-16 profile units, 128 KiB profile XML, 32-byte SSID, signal ≤ 100.

## 5. Files

Create:

- `internal/wifi/consent.go`: `Windows11_24H2Build`, `currentConnectionError`, `revealState`, `consentMessage`.
- `internal/wifi/consent_test.go`: table tests for the mapping and the reveal decisions.
- `internal/app/wifi.go`: `wifiObservation` and recording.
- `internal/app/wifi_test.go`: observation and capability overlay tests with a fake `WiFiProvider`.
- `frontend/src/features/wifi/LocationConsentNotice.tsx`: consent notice component.

Modify:

- `internal/domain/domain.go`: constants from §4.1, if missing.
- `internal/wifi/wifi_windows.go`: the opcode-7 failure branch; `Reveal()` uses `revealState`.
- `internal/app/capabilities.go`: `WithObserved`.
- `internal/app/<application file from H04>`: record observations in `CurrentWiFi`/`RevealWiFi`; overlay in `Capabilities`.
- `internal/adapter/wailsui/<bindings>.go`: `OpenLocationSettings` (or `app.go` until H05).
- `internal/adapter/httpapi/<errors>.go`: 403 mapping with `settings_uri`.
- `internal/adapter/cli/<wifi command>.go`: text-mode hint (or `cmd/kali-tools/main.go` until H05).
- `internal/adapter/contract/<errors>.go`: `SettingsURI` field.
- `frontend/src/shared/api/bridge.ts`, `frontend/src/app/App.tsx`, `frontend/src/shared/i18n/i18n.ts`.
- `frontend/tests/workflows.spec.ts`: new consent workflow.
- `frontend/wailsjs/**`: regenerated, never hand-edited.

Delete: none.

## 6. Implementation steps

1. Add the domain constants (§4.1). Run `go build ./...`.
2. Write `internal/wifi/consent_test.go` (§8) and run it to confirm it fails. Then add `internal/wifi/consent.go`.
3. Wire `currentConnectionError` and `revealState` into `wifi_windows.go`. Run `go vet ./...` and `GOOS=windows go vet ./...`.
4. Add `WithObserved` and the observation recording in `internal/app`, test first.
5. Add the Wails binding `OpenLocationSettings` with an injectable opener. Regenerate the bindings (`wails generate module -tags desktop`).
6. Add the HTTP error mapping with `settings_uri` (if W01 is done) and the CLI text hint.
7. Frontend: bridge method and constant, `LocationConsentNotice`, label mapping, i18n keys, then the Playwright test.
8. Run the full verification (§9).
9. Run the native acceptance steps (§7, AC-8 to AC-11) on the owner's Windows 11 24H2 machine. Record the evidence in `docs/verification/`.
10. Write `docs/verification/specs/A05.md`.

## 7. Acceptance criteria

- **AC-1.** Given `WlanQueryInterface(current_connection)` returns 5 on build 26100, when `currentConnectionError` maps it, then the error code is `location_consent_required` and the message contains `ms-settings:privacy-location`.
- **AC-2.** Given code 5 on build 22631, when mapped, then the code is `permission_required` (no false consent claim on older builds).
- **AC-3.** Given any other non-zero code (for example 1168), when mapped, then the code is `failed` and the message contains the numeric code only.
- **AC-4.** Given an open network (`SecurityEnabled=0`), when the reveal decision runs, then the state is `open_network` and no profile is requested. Given `OneX=1`, the state is `enterprise`. Given a secured personal network, `ok=false` and the profile path runs.
- **AC-5.** Given synthetic profiles, when `secretFromProfile` runs, then the states are `available`, `permission_required` and `no_saved_key`, as today.
- **AC-6.** Given a fake `WiFiProvider` returning `location_consent_required`, when `Application.CurrentWiFi` and then `Application.Capabilities` run, then the `wifi` capability state is `location_consent_required`. Given a later successful call, the state returns to `available`. `Capabilities()` alone never calls the provider.
- **AC-7.** Given the Wi-Fi page and a bridge fake that rejects `CurrentWiFi` with `{code:"location_consent_required"}`, when the page loads, then an alert with the translated consent text and the URI is visible. "Open location settings" calls `OpenLocationSettings` exactly once. "Check again" calls `CurrentWiFi` exactly once more. No automatic retry happens, and the existing reveal masking test still passes.
- **AC-8 [native].** Given Windows 11 24H2+ with Settings > Privacy & security > Location > "Let desktop apps access your location" set to Off, when the desktop app opens the Wi-Fi page, then the consent notice is shown. "Open location settings" opens the Location settings page.
- **AC-9 [native].** Given the same host with the setting On, when the user presses "Check again", then the SSID, BSSID and signal are shown. Reveal on the owner's personal test network shows the key masked by default and clears it after 15 s and on window blur.
- **AC-10 [native].** Given consent Off, when `kali-tools wifi` runs from a user console, then the exit code is 1 and the JSON `error.code` is `location_consent_required`. When `kali-tools serve` runs as a different account or as a service, then `GET /api/wifi` returns 403 with `settings_uri`.
- **AC-11 [native].** Given an open network and an enterprise (802.1X) network, when reveal runs, then the states are `open_network` and `enterprise`. Given a non-elevated process on a profile whose key Windows protects, the state is `permission_required`.

## 8. Tests

| Test | Package | Asserts | ACs |
| --- | --- | --- | --- |
| `TestCurrentConnectionErrorMapping` | `internal/wifi` (no build tag) | Table: (5, 26100), (5, 26200), (5, 22631), (1168, 26100), (87, 19045) give the expected codes; the message contains the URI only for the consent case | AC-1, AC-2, AC-3 |
| `TestRevealStateDecisions` | `internal/wifi` | (0,0)→`open_network`; (1,1)→`enterprise`; (1,0)→`ok=false` | AC-4 |
| `TestSyntheticProfileStates` (existing) | `internal/wifi` | Unchanged | AC-5 |
| `TestWindowsWLANLayoutAndMetadata` (existing) | `internal/wifi` (windows) | Unchanged sizes 532/68/604 | regression |
| `TestWiFiObservationUpdatesCapability` | `internal/app` | Fake provider sequence: consent error, then success; capability state follows; a fake counting calls proves `Capabilities` does not call it | AC-6 |
| `TestWithObservedOverlay` | `internal/app` | Empty state leaves the slice unchanged; known id replaced; unknown id ignored | AC-6 |
| `TestOpenLocationSettingsUsesConstantURI` | `internal/adapter/wailsui` | The injected opener receives exactly `ms-settings:privacy-location` | AC-7 |
| `TestWiFiConsentHTTPError` | `internal/adapter/httpapi` | 403, code, `settings_uri` | AC-10 (unit part) |
| `TestCLIWiFiConsentEnvelope` | `internal/adapter/cli` | Fake app returns the consent error; exit 1; `error.code` matches | AC-10 (unit part) |
| Playwright `Wi-Fi location consent shows settings link and never retries automatically` | `frontend/tests/workflows.spec.ts` | Steps in AC-7; asserts the call counters on the fake | AC-7 |
| Playwright `Wi-Fi key clears on navigation and blur` (existing) | same | Unchanged | AC-9 (UI part) |

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
go test -run 'TestCurrentConnectionErrorMapping|TestRevealStateDecisions|TestSyntheticProfileStates' ./internal/wifi/
GOOS=windows go vet ./internal/wifi/
```

Native (manual, recorded in `docs/verification/`): AC-8 to AC-11 on Windows 11 24H2+. Record the OS build (`winver`), the account type, the consent setting, the screenshots and the CLI output. Redact SSID/BSSID if the owner requests it. Never record the key.

## 10. Documentation updates

- `docs/architecture.md` §3: change "`location_consent_required` when added" to the implemented state.
- `docs/architecture.md` §6.2: note that `GET /api/wifi` carries `settings_uri`.
- `docs/tools-research.md` §11: add the WinRT pre-check follow-up (§4.7). Mark the "Windows 24H2 location-consent behavior" item with the native evidence date once AC-8 to AC-10 pass.
- `PLAN.md` A05: tick "Windows 11 24H2 location consent…" only after AC-8 to AC-10 pass natively. Tick "Distinguish open network, enterprise…" after AC-11. Note in the item that detection uses `ERROR_ACCESS_DENIED` plus build ≥ 26100, not `AppCapability.CheckAccess`.
- `README.md`: one sentence in the Wi-Fi section about the location setting.

## 11. Risks and rollback

- **Ambiguous `ERROR_ACCESS_DENIED`.** A group-policy denial on 24H2 would be shown as a consent problem. Mitigation: the message points to a setting the user can check, and the WinRT follow-up resolves it. AC-8 must confirm the real behavior.
- **Build threshold.** If Microsoft backports the gate to builds below 26100, those hosts keep showing `permission_required`, which is less helpful but still honest. Adjust `Windows11_24H2Build` only with native evidence.
- **`BrowserOpenURL` fails silently.** If the protocol handler fails, Wails only logs. The URI is always printed in the notice, so the user can open it manually.
- **Rollback.** Revert the commit. The provider falls back to `permission_required` and no persisted data changes.

## 12. Quality and delivery assertions

### 12.1 Applicable quality gates

| Gate | Applies | Tightened threshold |
| --- | --- | --- |
| QG-1, QG-2, QG-3, QG-4, QG-5, QG-7 | yes | none |
| QG-6 | yes | `internal/wifi` statement coverage on a Linux host (where only untagged files compile) is ≥ 90% for `consent.go` and `profile.go`. `internal/app` must not drop below its pre-spec value. Record before and after. |
| QG-8 | yes | Playwright workflow count increases by at least 1 |
| QG-9 | yes | binding drift is empty after `OpenLocationSettings` is added |
| QG-10 | yes | `internal/wifi` imports no `internal/adapter/*` package |
| QG-11 | yes | the consent error, the HTTP 403 body and the CLI envelope contain no SSID, BSSID, profile name or key (asserted with the synthetic profile `synthetic-psk`) |
| QG-12 | no new bounded resource | existing limits unchanged |
| QG-13, QG-14, QG-15 | yes | none |

### 12.2 Delivery assertions

| ID | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | Consent mapping is correct for 24H2 and older builds | `go test -run TestCurrentConnectionErrorMapping ./internal/wifi/` | PASS, 5 table rows | CI |
| DA-2 | Exactly one constant URI is opened | `grep -rn "ms-settings:" --include=*.go internal app.go cmd` | one definition (`domain.LocationSettingsURI`); every other hit references the constant | reviewer |
| DA-3 | No automatic retry | Playwright `Wi-Fi location consent shows settings link and never retries automatically` | `CurrentWiFi` call counter is 1 before "Check again" and 2 after | CI |
| DA-4 | Listing capabilities never touches WLAN | `go test -run TestWiFiObservationUpdatesCapability ./internal/app/` | fake call counter is 0 after `Capabilities` | CI |
| DA-5 | Reveal masking unchanged | Playwright `Wi-Fi key clears on navigation and blur` | PASS | CI |
| DA-6 | i18n parity | `bun run typecheck` (`pt: typeof en`) | exit 0 | CI |
| DA-7 [native] | Consent denied path | Procedure: on Windows 11 24H2+, set "Let desktop apps access your location" to Off. Start the desktop build. Open Wi-Fi. Press "Open location settings". | Notice visible; the Settings Location page opens | [native] |
| DA-8 [native] | Consent granted path | Procedure: turn the setting On, press "Check again", then press Reveal on the owner's personal test network and wait 15 s | SSID shown; key masked again after 15 s and on blur | [native] |
| DA-9 [native] | CLI and server under another account | Procedure: with consent Off, run `kali-tools wifi` from a user console, and run `kali-tools serve` under a second local account, then call `GET /api/wifi` | exit 1 with `location_consent_required`; HTTP 403 with `settings_uri` | [native] |
| DA-10 [native] | Reveal states | Procedure: connect in turn to an open network, an 802.1X network and the WPA2/3-Personal test network, and press Reveal on each | `open_network`, `enterprise`, `available` (or `permission_required` when non-elevated and the key is protected) | [native] |

For every [native] row, record:

- Windows edition and build (`winver`);
- Wi-Fi adapter model and driver version;
- account type (standard or administrator) and whether the process is elevated;
- the consent setting value;
- the date (UTC) and the operator.

Never record the key.

Evidence record: `docs/verification/specs/A05.md`.
