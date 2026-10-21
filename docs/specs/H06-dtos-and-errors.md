# H06 Transport DTOs and error mapping

| Field | Value |
| --- | --- |
| PLAN tasks | H06 |
| Status | Ready |
| Depends on | H05 |
| Risk | Medium: the frontend contract and the generated models change namespace; a wrong JSON tag or field order breaks the UI or CLI bytes silently |

## 1. Goal

Every driving adapter speaks one frontend contract, defined once in `internal/adapter/contract`, separate from storage rows, application types and capability types (D4). Errors reach users through one function, `contract.FromError`, which keeps stable codes and never leaks raw internal text. The shared kernel gains named constants for capability states and error codes, including the new `location_consent_required` state, and both i18n catalogs render it. Redaction tests prove that no DTO can carry a secret or packet payload, except the explicit Wi-Fi reveal DTO, which also refuses to print its value.

## 2. Non-goals

- Do not detect Windows location consent. A05 produces `location_consent_required`; this spec only defines and renders it.
- Do not create `internal/adapter/httpapi` (W01). The contract is designed for it, nothing more.
- Do not change any JSON field name, field order or value that the frontend or the CLI emits today, except the documented changes in §4.6.
- Do not change the shape of rejected Wails promises: they stay `"<code>: <message>"` strings. Do not configure Wails `ErrorFormatter`.
- Do not add CLI commands, flags or binding methods.
- Do not move `internal/domain.Error`; adapters keep using it inside the core.
- Do not change capability detection logic in `app.Capabilities`, only its literals.

## 3. Current state

Facts at commit `8cbf25e` (re-verify):

- `internal/domain/domain.go:5-24`: `SchemaVersion = 1`, `Version`, `Error{Code "code", Message "message"}` with `Error()` returning `Code + ": " + Message`, `Invalid(format, args...)`, `Unsupported(message)`, `Capability{ID, State, Detail, Provider}`. There are no constants for states or codes.
- Capability states are string literals in `internal/app/capabilities.go:10-22` (`available`, `unsupported`, `permission_required`) and in `internal/adapter/tools/tools.go:20-36` (`dependency_missing`, `failed`, `unsupported`, `available`).
- Error-code literals are spread over `internal/network/network.go:327-350`, `internal/wifi/wifi_windows.go:53-97`, `internal/capture/live_windows.go:37`, `internal/adapter/probe/probe_windows.go:54` and `internal/adapter/tools/tools.go:57`.
- Adapter types used as DTOs (D4): `storage.Job` and `storage.Settings` appear in the generated `frontend/wailsjs/go/main/App.d.ts` (`History`, `StartDiagnostic`, `Settings`, `SaveSettings`); `tools.Nmap`, `tools.Tool` and `platform.Snapshot` also appear there. After H03–H05 these become application or capability types, still not a dedicated contract.
- The CLI maps unknown errors to `{failed, err.Error()}` (`cmd/kali-tools/main.go:164`, moved to `internal/adapter/cli` by H05), which can echo raw SQL, subprocess or OS text.
- Settings validation returns `fmt.Errorf("invalid settings")` (`internal/adapter/storage/storage.go:183-188`), so an invalid save reaches the UI without a stable code.
- The frontend parses the code from the rejection string: `errorText` matches `/(?:^|\s)([a-z_]+):/` (`frontend/src/app/App.tsx:98-101`). State labels live in `App.tsx:67-84`, capability labels in `App.tsx:497-502`, error labels in `App.tsx:86-97`. The pill is amber only for `permission_required` (`App.tsx:511`). There is no label for `location_consent_required`, `conflict`, `unauthorized` or `forbidden`.
- Catalogs: `frontend/src/shared/i18n/i18n.ts` exports `en` (line 1) and `pt: typeof en` (line 131), so a key added to `en` must be added to `pt` or `tsc` fails.
- `frontend/src/shared/api/bridge.ts:2-93` defines the frontend types; JSON names there are the contract to preserve. `Backend` (`bridge.ts:94-122`) types the generated module.
- `frontend/tests/workflows.spec.ts:34-47` already tests capability-state labels for `unsupported` and `dependency_missing`; Playwright fakes reject with `Error("permission_required: ...")` (`workflows.spec.ts:141`, `:151`).
- Wi-Fi reveal returns `wifi.Secret{State, Value}` (`internal/wifi/wifi.go:12-15`); `Value` is the key material (`internal/wifi/profile.go:29`).
- H05 delivered `internal/adapter/wailsui.Bindings` and `internal/adapter/cli.Run`, each with a local error mapping (`toError` in `cli`), and bindings returning application types.

Source Go types whose JSON this contract must reproduce byte for byte (field order matters):

| Source type | Fields in order (JSON name) |
| --- | --- |
| `network.InterfaceInfo` | index, name, mac, mtu, up, loopback, addresses |
| `network.Request` | record_type (omitempty), kind, target, port, timeout_ms |
| `network.Result` | error_code (omitempty), kind, target, status, elapsed_ms (float64), values, provider |
| `network.PortResult` | host, port, state |
| `network.Counter` | id, received (uint64), sent (uint64) |
| `network.Rate` | id, receive_bps, send_bps, timestamp |
| `network.Metrics` | timestamp, receive_bps, send_bps, interfaces |
| `platform.Adapter` / `platform.Snapshot` (or their H03 successors) | index (uint32), name, gateways, dns / adapters, counters |
| `storage.Job` / `job.Status` | id, kind, state, created, finished (omitempty pointer), summary |
| `storage.Settings` (or its H03 successor) | language, retention_days |
| `capture.Packet` | number, timestamp, length, source, destination, protocol, info |
| `capture.ImportResult` | packets, limited, format |
| `capture.LiveRequest` | address, duration_seconds, packet_limit |
| `capture.LiveResult` | limited, packets, provider, dropped (`*uint64`, no omitempty), started, finished |
| `wifi.Network` | channel (`*uint32`, omitempty), profile, ssid, bssid, signal (uint32), authentication (uint32), state |
| `wifi.Secret` | state, value |
| `tools.Tool` | name, state, version, path |
| `tools.Nmap` | scanner, version, finished{exit}, hosts[{status{state}, addresses[{address, type}], ports[{service{name, product, version, method}, port, protocol, state{state}}]}]; `XMLName` is `json:"-"` |

## 4. Design

### 4.1 Domain constants (`internal/domain/domain.go`)

```go
// Capability states (Capability.State).
const (
    CapabilityAvailable               = "available"
    CapabilityDependencyMissing       = "dependency_missing"
    CapabilityPermissionRequired      = "permission_required"
    CapabilityLocationConsentRequired = "location_consent_required"
    CapabilityUnsupported             = "unsupported"
    CapabilityFailed                  = "failed"
)

// Stable error codes (Error.Code). Adding one requires both i18n catalogs.
const (
    CodeInvalidInput            = "invalid_input"
    CodeUnsupported             = "unsupported"
    CodePermissionRequired      = "permission_required"
    CodeDependencyMissing       = "dependency_missing"
    CodeLocationConsentRequired = "location_consent_required"
    CodeNotFound                = "not_found"
    CodeTimeout                 = "timeout"
    CodeCancelled               = "cancelled"
    CodeFailed                  = "failed"
    CodeCertificateInvalid      = "certificate_invalid"
    CodeNoConnection            = "no_connection"
    CodeConflict                = "conflict"
    CodeUnauthorized            = "unauthorized"
    CodeForbidden               = "forbidden"
)

func CapabilityStates() []string        // new slice, the 6 states in the order above
func ErrorCodes() []string              // new slice, the 14 codes in the order above
func IsCapabilityState(s string) bool
func IsErrorCode(code string) bool
```

The constants are untyped strings, so `Capability.State` and `Error.Code` keep type `string` and their JSON is unchanged. `Invalid` and `Unsupported` use `CodeInvalidInput` and `CodeUnsupported`. `internal/app/capabilities.go` uses the `Capability*` constants instead of literals. Replacing the other literals listed in §3 is optional in this spec; do it only in files this spec already modifies.

### 4.2 Package `internal/adapter/contract`

One package, imported by `wailsui`, `cli` and later `httpapi`. It may import `internal/domain`, `internal/app`, `internal/job`, `internal/network`, `internal/capture`, `internal/wifi` and stdlib. It must not import any other `internal/adapter/*` package, Wails, `net/http`, `os/exec` or `database/sql`.

```go
package contract

// Error is the only error shape that leaves a driving adapter.
type Error struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

// Error keeps the "<code>: <message>" form that the frontend parses (App.tsx errorText).
func (e Error) Error() string { return e.Code + ": " + e.Message }

type Capability struct {
    ID       string `json:"id"`
    State    string `json:"state"`
    Detail   string `json:"detail"`
    Provider string `json:"provider"`
}

type Interface struct {
    Index     int      `json:"index"`
    Name      string   `json:"name"`
    MAC       string   `json:"mac"`
    MTU       int      `json:"mtu"`
    Up        bool     `json:"up"`
    Loopback  bool     `json:"loopback"`
    Addresses []string `json:"addresses"`
}

type Counter struct {
    ID       string `json:"id"`
    Received uint64 `json:"received"`
    Sent     uint64 `json:"sent"`
}

type Adapter struct {
    Index    uint32   `json:"index"`
    Name     string   `json:"name"`
    Gateways []string `json:"gateways"`
    DNS      []string `json:"dns"`
}

type Inventory struct {
    Adapters []Adapter `json:"adapters"`
    Counters []Counter `json:"counters"`
}

type DiagnosticRequest struct {
    RecordType string `json:"record_type,omitempty"`
    Kind       string `json:"kind"`
    Target     string `json:"target"`
    Port       int    `json:"port"`
    TimeoutMS  int    `json:"timeout_ms"`
}

type DiagnosticResult struct {
    ErrorCode string   `json:"error_code,omitempty"`
    Kind      string   `json:"kind"`
    Target    string   `json:"target"`
    Status    string   `json:"status"`
    ElapsedMS float64  `json:"elapsed_ms"`
    Values    []string `json:"values"`
    Provider  string   `json:"provider"`
}

type Job struct {
    ID       string     `json:"id"`
    Kind     string     `json:"kind"`
    State    string     `json:"state"`
    Created  time.Time  `json:"created"`
    Finished *time.Time `json:"finished,omitempty"`
    Summary  string     `json:"summary"`
}

// JobEvent is the payload of the "job" event.
type JobEvent struct {
    SchemaVersion int               `json:"schema_version"`
    Seq           uint64            `json:"seq"`
    Job           Job               `json:"job"`
    Result        *DiagnosticResult `json:"result,omitempty"`
}

type Settings struct {
    Language      string `json:"language"`
    RetentionDays int    `json:"retention_days"`
}

type PortResult struct {
    Host  string `json:"host"`
    Port  int    `json:"port"`
    State string `json:"state"`
}

type Packet struct {
    Number      int       `json:"number"`
    Timestamp   time.Time `json:"timestamp"`
    Length      int       `json:"length"`
    Source      string    `json:"source"`
    Destination string    `json:"destination"`
    Protocol    string    `json:"protocol"`
    Info        string    `json:"info"`
}

// Capture is an imported packet file summary (bridge.ts Capture).
type Capture struct {
    Packets []Packet `json:"packets"`
    Limited bool     `json:"limited"`
    Format  string   `json:"format"`
}

type LiveCaptureRequest struct {
    Address         string `json:"address"`
    DurationSeconds int    `json:"duration_seconds"`
    PacketLimit     int    `json:"packet_limit"`
}

type LiveCapture struct {
    Limited  bool      `json:"limited"`
    Packets  []Packet  `json:"packets"`
    Provider string    `json:"provider"`
    Dropped  *uint64   `json:"dropped"`
    Started  time.Time `json:"started"`
    Finished time.Time `json:"finished"`
}

// CaptureEvent is the payload of the "capture" event. Error is "" on success,
// otherwise FromError(err).Error().
type CaptureEvent struct {
    Result LiveCapture `json:"result"`
    Error  string      `json:"error"`
}

type Rate struct {
    ID         string    `json:"id"`
    ReceiveBPS float64   `json:"receive_bps"`
    SendBPS    float64   `json:"send_bps"`
    Timestamp  time.Time `json:"timestamp"`
}

// Metrics is the payload of the "metrics" event.
type Metrics struct {
    Timestamp  time.Time `json:"timestamp"`
    ReceiveBPS float64   `json:"receive_bps"`
    SendBPS    float64   `json:"send_bps"`
    Interfaces []Rate    `json:"interfaces"`
}

type WiFi struct {
    Channel        *uint32 `json:"channel,omitempty"`
    Profile        string  `json:"profile"`
    SSID           string  `json:"ssid"`
    BSSID          string  `json:"bssid"`
    Signal         uint32  `json:"signal"`
    Authentication uint32  `json:"authentication"`
    State          string  `json:"state"`
}

// WiFiSecret is the only DTO allowed to carry secret material, and only in Value.
type WiFiSecret struct {
    State string `json:"state"`
    Value string `json:"value"`
}

// String and GoString never print Value, so %v, %+v and %#v cannot leak it into logs.
func (s WiFiSecret) String() string   { return "{state:" + s.State + " value:[redacted]}" }
func (s WiFiSecret) GoString() string { return s.String() }

type Tool struct {
    Name    string `json:"name"`
    State   string `json:"state"`
    Version string `json:"version"`
    Path    string `json:"path"`
}

type ServiceInspection struct {
    Scanner  string       `json:"scanner"`
    Version  string       `json:"version"`
    Finished NmapFinished `json:"finished"`
    Hosts    []NmapHost   `json:"hosts"`
}
type NmapFinished struct {
    Exit string `json:"exit"`
}
type NmapState struct {
    State string `json:"state"`
}
type NmapAddress struct {
    Address string `json:"address"`
    Type    string `json:"type"`
}
type NmapService struct {
    Name    string `json:"name"`
    Product string `json:"product"`
    Version string `json:"version"`
    Method  string `json:"method"`
}
type NmapPort struct {
    Service  NmapService `json:"service"`
    Port     int         `json:"port"`
    Protocol string      `json:"protocol"`
    State    NmapState   `json:"state"`
}
type NmapHost struct {
    Status    NmapState     `json:"status"`
    Addresses []NmapAddress `json:"addresses"`
    Ports     []NmapPort    `json:"ports"`
}
```

### 4.3 Mapping functions (`internal/adapter/contract/map.go`)

Rules for every mapper: copy field by field (no `json` round trip, no reflection); preserve `nil` versus empty slices (`nil` in → `nil` out, empty in → empty non-nil out); copy pointers into new values so the DTO never aliases application memory.

```go
func FromCapabilities(v []domain.Capability) []Capability
func FromInterfaces(v []network.InterfaceInfo) []Interface
func FromInventory(v HostSnapshot) Inventory            // HostSnapshot = the H04 type returned by Application.Inventory
func (r DiagnosticRequest) Request() network.Request
func FromDiagnosticResult(v network.Result) DiagnosticResult
func FromStatus(v job.Status) Job
func FromStatuses(v []job.Status) []Job
func FromSettings(v AppSettings) Settings               // AppSettings = the H04 settings type
func (s Settings) AppSettings() AppSettings
func FromPortResults(v []network.PortResult) []PortResult
func FromImportResult(v capture.ImportResult) Capture
func (r LiveCaptureRequest) LiveRequest() capture.LiveRequest
func FromLiveResult(v capture.LiveResult) LiveCapture
func FromMetrics(v network.Metrics) Metrics
func FromWiFi(v wifi.Network) WiFi
func FromWiFiSecret(v wifi.Secret) WiFiSecret
func FromTools(v []ToolInfo) []Tool                      // ToolInfo = the H04 tool inventory element type
func FromServiceInspection(v Inspection) ServiceInspection // Inspection = the H04 type returned by InspectServices
```

`HostSnapshot`, `AppSettings`, `ToolInfo` and `Inspection` are placeholders in this document only. Write the mappers against the real H04 type names (check with `go doc ./internal/app`); do not create aliases with these names.

Event mapping, used by `wailsui` now and by `httpapi` later:

```go
const (
    EventJob     = "job"
    EventMetrics = "metrics"
    EventCapture = "capture"
)

// EventPayload maps one job.Event to the frontend event name and payload.
// ok is false for an unknown topic or an unexpected Data type; the caller drops the event.
func EventPayload(ev job.Event) (name string, payload any, ok bool)
```

| Topic | Name | Payload |
| --- | --- | --- |
| `job` | `job` | `JobEvent{SchemaVersion: ev.SchemaVersion, Seq: ev.Seq, Job: FromStatus(*ev.Job), Result: FromDiagnosticResult(*r) when ev.Data is a non-nil *network.Result}`; `ev.Job == nil` → `ok == false` |
| `metrics` | `metrics` | `FromMetrics(ev.Data.(network.Metrics))` |
| `capture` | `capture` | `CaptureEvent{Result: FromLiveResult(...), Error: ...}` built from the H04 capture outcome; a typed error is converted with `FromError(err).Error()`; the legacy string form is copied unchanged |

### 4.4 Error mapping (`internal/adapter/contract/errors.go`)

```go
// MaxErrorMessageBytes bounds every Message that leaves an adapter.
const MaxErrorMessageBytes = 512

const (
    msgCancelled = "Operation cancelled"
    msgTimeout   = "Operation timed out"
    msgFailed    = "Operation failed"
)

// FromError maps any error to the contract. It is the only error mapping used
// by wailsui, cli and httpapi. FromError(nil) returns Error{}.
func FromError(err error) Error
```

Algorithm, in this order:

1. `err == nil` → `Error{}`.
2. `errors.As(err, &d)` with `d *domain.Error` → `Code = d.Code` if `domain.IsErrorCode(d.Code)`, else `domain.CodeFailed`; `Message = sanitize(d.Message)`.
3. `errors.Is(err, context.Canceled)` → `{CodeCancelled, msgCancelled}`.
4. `errors.Is(err, context.DeadlineExceeded)` → `{CodeTimeout, msgTimeout}`.
5. Anything else → `{CodeFailed, msgFailed}`. The raw text is never copied.

`sanitize(s string) string`: replace every Unicode control character (`unicode.IsControl`) with a space, then truncate to at most `MaxErrorMessageBytes` bytes at a rune boundary (never split a UTF-8 sequence). An empty message after sanitizing becomes `msgFailed`.

### 4.5 Adapter changes

`internal/adapter/wailsui`:

- Every bound method returns contract types. Arguments become `contract.DiagnosticRequest`, `contract.Settings` and `contract.LiveCaptureRequest`, converted with their methods. JavaScript names and argument order do not change.
- One unexported helper is the adapter's only error mapping:

```go
func fail(err error) error {
    if err == nil {
        return nil
    }
    return contract.FromError(err) // value type, never a typed nil pointer
}
```

- The forwarder emits `contract.EventPayload(ev)` results and drops `ok == false`. The local `jobPayload` from H05 is deleted.

`internal/adapter/cli`:

- `envelope.Error` becomes `*contract.Error` (same JSON). `toError` is deleted; failures use `e := contract.FromError(err)` and `exitCode(e)`:

```go
func exitCode(e contract.Error) int {
    if e.Code == domain.CodeInvalidInput {
        return exitInvalid
    }
    return exitFailure
}
```

- `data` values are contract DTOs (`FromInterfaces`, `FromDiagnosticResult`, `FromPortResults`, ...). `version` and `doctor` keep their maps; `doctor.capabilities` and `doctor.tools` hold contract slices.
- `parseCodedError` uses `domain.IsErrorCode` instead of its local list.

Settings validation (wherever H03/H04 placed it; today `storage.validateSettings`, `storage.go:183-188`) returns `domain.Invalid("invalid settings")`. The stored-settings corruption errors (`storage.go:175-180`) stay as they are and map to `failed`.

### 4.6 Documented wire changes

| Change | Where | Compatibility |
| --- | --- | --- |
| `job` event gains `"seq": <uint64>` after `schema_version` | Wails `job` event | additive; the UI ignores unknown keys |
| Non-domain errors become `{"code":"failed","message":"Operation failed"}` instead of raw `err.Error()` | CLI envelope, Wails rejections | same code; message no longer leaks internals |
| `context.DeadlineExceeded` that is not a `domain.Error` becomes `timeout` instead of `failed` | CLI, Wails | code is more precise |
| Invalid settings save becomes `invalid_input: invalid settings` (CLI exit 2 once C01 adds `settings set`) | Wails rejection | UI now shows the localized invalid-input text |
| Generated models namespace becomes `contract` (`frontend/wailsjs/go/models.ts`) | generated code | `bridge.ts` exported API unchanged |

Update the H05 CLI goldens only where a row above applies, and list each changed golden in the commit message.

### 4.7 Frontend

`frontend/src/shared/i18n/i18n.ts`, add to `en` and `pt`:

| Key | `en` | `pt` |
| --- | --- | --- |
| `locationConsent` | `Location consent required` | `Consentimento de localização necessário` |
| `locationConsentError` | `Windows location consent is required to read Wi-Fi details.` | `É necessário o consentimento de localização do Windows para ler os detalhes do Wi-Fi.` |
| `conflictError` | `The operation conflicts with one already running.` | `A operação conflita com outra em execução.` |
| `unauthorizedError` | `Authentication is required.` | `Autenticação necessária.` |
| `forbiddenError` | `This operation is not allowed.` | `Esta operação não é permitida.` |

`frontend/src/app/App.tsx`:

- `labels` (`App.tsx:67-84`) and the capability `labels` (`App.tsx:497-502`): add `location_consent_required: t.locationConsent`.
- `errorLabels` (`App.tsx:86-97`): add `location_consent_required: t.locationConsentError`, `conflict: t.conflictError`, `unauthorized: t.unauthorizedError`, `forbidden: t.forbiddenError`.
- Pill class (`App.tsx:511`): amber for `permission_required` and `location_consent_required`.

`frontend/src/shared/api/bridge.ts`: exported API unchanged. Add above `Capability` a comment listing the six states; keep `state: string`.

## 5. Files

Create:

- `internal/adapter/contract/contract.go`: DTO types, `Error`, `WiFiSecret` formatting.
- `internal/adapter/contract/map.go`: mapping functions.
- `internal/adapter/contract/events.go`: event names and `EventPayload`.
- `internal/adapter/contract/errors.go`: `FromError`, `sanitize`, limits.
- `internal/adapter/contract/contract_test.go`: JSON compatibility and mapping tests.
- `internal/adapter/contract/errors_test.go`: error mapping and limit tests.
- `internal/adapter/contract/redaction_test.go`: DTO redaction tests.
- `internal/domain/domain_test.go`: constant list tests.

Modify:

- `internal/domain/domain.go`: constants and helpers.
- `internal/app/capabilities.go`: use constants.
- The settings validation file (today `internal/adapter/storage/storage.go`): `domain.Invalid`.
- `internal/adapter/wailsui/bindings.go`, `events.go`, `bindings_test.go`: contract types, `fail`, `EventPayload`.
- `internal/adapter/cli/cli.go`, `capture.go`, `cli_test.go`, affected goldens: contract types, `FromError`, `exitCode`.
- `frontend/wailsjs/go/models.ts`, `frontend/wailsjs/go/wailsui/Bindings.d.ts`: regenerated.
- `frontend/src/shared/i18n/i18n.ts`, `frontend/src/app/App.tsx`, `frontend/src/shared/api/bridge.ts` (comment only), `frontend/tests/workflows.spec.ts` (one new test).
- `docs/architecture.md`, `PLAN.md`: §10.

Delete: `internal/adapter/contract/.gitkeep` if it exists. Nothing else.

## 6. Implementation steps

1. **Domain constants.** Add §4.1 and `internal/domain/domain_test.go`; switch `Invalid`, `Unsupported` and `app/capabilities.go` to the constants. `go test ./...` passes with no other change.
2. **Contract package.** Add `contract.go`, `errors.go`, `map.go`, `events.go` and their tests. Nothing imports it yet. `go test ./internal/adapter/contract` passes.
3. **Settings validation code.** Switch it to `domain.Invalid("invalid settings")`; existing storage tests still pass (they assert only `err != nil` and no echo of the input).
4. **CLI.** Switch to contract DTOs, `FromError` and `exitCode`. Run `go test ./internal/adapter/cli`; for each failing golden, confirm that §4.6 explains the difference, then regenerate only that golden with `-update`.
5. **Wails bindings.** Switch to contract DTOs, `fail` and `EventPayload`. Run `wails generate module -tags desktop`, commit the regenerated files, then `node scripts/task.mjs bindings` must exit 0. `bun run typecheck` must pass without editing the exported types of `bridge.ts`; a failure there means a DTO diverged from the frontend type: fix the DTO, not `bridge.ts`.
6. **Frontend states.** Apply §4.7 and add the Playwright test. Run the frontend commands.
7. **Docs.** Apply §10. Run the full §9 block.
8. **Evidence.** Write `docs/verification/specs/H06.md` with the README §5.3 structure, including the baseline coverage of `internal/domain`, `internal/adapter/cli` and `internal/adapter/wailsui` taken before step 1 and the list of changed goldens.

Commit: `refactor(contract): add frontend DTOs and single error mapping (H06)`, or one commit per step with the same scope.

## 7. Acceptance criteria

- AC-1: Given every pair (source type, contract DTO) in §3, when both are marshalled from the same populated fixture, then the JSON bytes are identical.
- AC-2: Given `nil` and empty slices in a source value, when mapped, then the DTO JSON shows `null` and `[]` respectively, exactly like the source.
- AC-3: Given `&domain.Error{Code: "permission_required", Message: "x"}` wrapped with `fmt.Errorf("...: %w", err)`, when `FromError` runs, then it returns `{permission_required, x}`.
- AC-4: Given `&domain.Error{Code: "made_up", Message: "x"}`, when `FromError` runs, then the code is `failed` and the message is `x`.
- AC-5: Given `context.Canceled` (possibly wrapped), `context.DeadlineExceeded`, or `errors.New("sql: password=synthetic-secret")`, when `FromError` runs, then the results are `{cancelled, "Operation cancelled"}`, `{timeout, "Operation timed out"}` and `{failed, "Operation failed"}`; the last contains no `synthetic-secret`.
- AC-6: Given a domain message of 512 bytes, 513 bytes, a 513-byte message whose 512th byte is inside a multi-byte rune, and a message with `\n` and `\x1b`, when sanitized, then lengths are 512, 512, ≤ 512 with valid UTF-8, and control characters are spaces.
- AC-7: Given every exported struct type in `internal/adapter/contract`, when the redaction test inspects it, then no field has type `[]byte`, and no JSON name is one of `key`, `psk`, `password`, `passphrase`, `secret`, `token`, `payload`, `raw`, `material`, `bytes`, except the allow-listed `WiFiSecret.value`.
- AC-8: Given a `WiFiSecret{State: "available", Value: "synthetic-test-key"}`, when formatted with `%v`, `%+v`, `%#v` and `%s`, then no output contains `synthetic-test-key`; when marshalled to JSON, then `value` is present (the UI needs it).
- AC-9: Given a synthetic packet whose payload contains `synthetic-private`, when imported and mapped to `contract.Capture` and `contract.LiveCapture`, then their JSON does not contain `synthetic-private`.
- AC-10: Given a `job.Event` per topic, when `EventPayload` runs, then names are `job`, `metrics`, `capture`, the `job` payload has keys `schema_version`, `seq`, `job` and `result` only when a result exists, and an unknown topic yields `ok == false`.
- AC-11: Given the `*wailsui.Bindings` method set, when the reflection test walks every parameter and result type recursively, then every named struct type belongs to package `internal/adapter/contract` (or is `time.Time`), and none belongs to `storage`, `tools`, `platform`, `app`, `job`, `network`, `capture` or `wifi`.
- AC-12: Given a binding call that fails with a domain error, when the error is returned, then its `Error()` is `"<code>: <message>"` and `errors.As(err, &contract.Error{})` succeeds.
- AC-13: Given `domain.CapabilityStates()` and `domain.ErrorCodes()`, when listed, then they contain exactly the 6 states and 14 codes of §4.1, without duplicates, and each call returns a new slice.
- AC-14: Given a CLI failure with a non-domain error, when `cli.Run` encodes it, then the envelope error is `{failed, "Operation failed"}` and the exit code is 1; `invalid_input` still exits 2.
- AC-15: Given the Playwright fake returns capability `wifi` with state `location_consent_required`, when the overview renders in `en` and then `pt-BR`, then the Wi-Fi row shows `Location consent required` and `Consentimento de localização necessário`.
- AC-16: Given the regenerated bindings, when `node scripts/task.mjs bindings` runs, then there is no drift and `frontend/wailsjs/go/models.ts` declares namespace `contract` and no namespace `storage`, `tools` or `platform`.
- AC-17 [native]: Given the Windows desktop build, when `node scripts/task.mjs bridge` runs, then `TestNativeBridge` still passes (it asserts `settings.language === 'en'`, the version string, `CancelJob("missing") === false` and two rejections).

## 8. Tests

`internal/domain` (`package domain_test`):

| Test | Asserts | AC |
| --- | --- | --- |
| `TestCapabilityStatesAndErrorCodes` | exact lists, no duplicates, fresh slices (mutating one result does not affect the next), `IsCapabilityState`/`IsErrorCode` true for each and false for `""` and `"made_up"` | AC-13 |

`internal/adapter/contract` (`package contract_test` except where noted):

| Test | Asserts | AC |
| --- | --- | --- |
| `TestDTOJSONMatchesSourceTypes` | table: for each pair, build a fully populated source value (every field non-zero, pointers set), map it, `json.Marshal` both, compare bytes | AC-1 |
| `TestMappersPreserveNilAndEmptySlices` | table over every slice-bearing mapper | AC-2 |
| `TestMappersDoNotAlias` | mutate the source slice or pointer after mapping; DTO unchanged | AC-2 |
| `TestFromError` | table: nil, domain, wrapped domain, unknown code, canceled, wrapped canceled, deadline, plain error with `synthetic-secret` | AC-3, AC-4, AC-5 |
| `TestFromErrorBoundsMessage` | 512, 513, rune-boundary and control-character cases; `utf8.ValidString` | AC-6 |
| `TestEveryDTOIsRedactionSafe` | parses the package's non-test files with `go/parser` to list exported struct types; asserts the registry slice `allDTOs` in the test covers exactly that set (fails when a DTO is added without registering it); walks fields recursively with `reflect` | AC-7 |
| `TestWiFiSecretNeverFormatsValue` | the four verbs and JSON | AC-8 |
| `TestCaptureDTOsCarryNoPayload` | synthetic artifact as in `TestCLICaptureConversion`; import, map, marshal | AC-9 |
| `TestEventPayload` | one event per topic plus an unknown topic and a `job` event with nil `Job` | AC-10 |
| `TestErrorImplementsErrorString` | `contract.Error{"timeout","x"}.Error() == "timeout: x"` | AC-12 |

`internal/adapter/wailsui`:

| Test | Asserts | AC |
| --- | --- | --- |
| `TestBindingSignaturesUseContractTypes` | AC-11 via `reflect` | AC-11 |
| `TestBindingErrorsAreContractErrors` | `Discover("0.0.0.0/0", []int{80})` error: `errors.As` to `contract.Error`, code `invalid_input`, `Error()` starts with the prefix `"invalid_input: "` | AC-12 |
| H05 tests | still pass; payload assertions updated for `seq` | AC-10 |

`internal/adapter/cli`:

| Test | Asserts | AC |
| --- | --- | --- |
| `TestCLIMapsUnknownErrorsGenerically` | `import --file <path to a directory>` (the OS error is not a `domain.Error`): envelope `{failed, "Operation failed"}`, exit 1, stdout does not contain the path | AC-14 |
| H05 tests and goldens | pass; changed goldens listed in the commit | AC-14 |

Frontend (`frontend/tests/workflows.spec.ts`):

| Test | Asserts | AC |
| --- | --- | --- |
| `overview renders location consent state` | override `window.go.wailsui.Bindings.Capabilities` as in the existing test at lines 34-47; check both languages | AC-15 |

AC-16 is the `bindings` task. AC-17 is `TestNativeBridge` on Windows.

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
go test -race -timeout 120s ./internal/adapter/... ./internal/domain/...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
```

Binding regeneration and drift check:

```bash
node scripts/task.mjs bindings
grep -n "export namespace" frontend/wailsjs/go/models.ts   # expect contract (and no storage, tools, platform)
```

D4 check until H08 lands:

```bash
go list -f '{{join .Imports "\n"}}' ./internal/adapter/contract ./internal/adapter/cli ./internal/adapter/wailsui | grep -E 'internal/adapter/(storage|tools|probe|platform)$' && exit 1 || true
```

Native, Windows with WebView2 only (`[native]`):

```bash
node scripts/task.mjs bridge
```

## 10. Documentation updates

- `docs/architecture.md`: §2 map (add `adapter/contract`: frontend DTOs and error mapping); §3 table (driving adapters may import `adapter/contract`; add a `contract` row: may import `domain`, `app`, `job`, capability packages; must not import other adapters, Wails, `net/http`, `os/exec`, `database/sql`); §3 capability-state rule (drop "when added"); §6.1–6.3 (adapters return `contract` DTOs and map errors with `contract.FromError`); §8 D4 marked resolved.
- `docs/architecture-research.md` §6.5: name `contract.FromError` as the implementation of the mapping table.
- `PLAN.md` §2 "States" (location consent required now defined) and H06 checkboxes with evidence.

## 11. Risks and rollback

- Field order or tag drift changes CLI bytes or breaks the UI: `TestDTOJSONMatchesSourceTypes`, the goldens and `tsc` (through `backend()` returning the generated module as `Backend`) catch it.
- Returning a typed nil `*contract.Error` as `error` makes every call fail: `fail` returns the value type, and `TestBindingErrorsAreContractErrors` plus the H05 success-path tests catch it.
- Generic `Operation failed` messages hide useful diagnostics from users. Mitigation: capability packages keep returning `domain.Error` with authored messages; turning more raw errors into domain errors is follow-up work, not part of this spec.
- `location_consent_required` is defined but never produced until A05; the UI test proves rendering only.
- Rollback: `git revert` the H06 commit(s); goldens, generated files and frontend labels revert together.

## 12. Quality and delivery assertions

Applicable global gates (README §5.1):

| Gate | Threshold for H06 |
| --- | --- |
| QG-1, QG-2, QG-3, QG-4, QG-5, QG-7, QG-13, QG-14, QG-15 | as defined; commit messages contain `(H06)` |
| QG-6 | `internal/adapter/contract` ≥ 90 % statements (tightened: the package is pure mapping); `internal/domain` 100 %; `internal/adapter/cli` and `internal/adapter/wailsui` not lower than their H05 values (record before/after) |
| QG-8 | Playwright workflow count = H05 count + 1 |
| QG-9 | `node scripts/task.mjs bindings` exits 0 after regeneration |
| QG-10 | `go list` check of §9 prints nothing |
| QG-11 | `TestFromError` (`synthetic-secret`), `TestWiFiSecretNeverFormatsValue` (`synthetic-test-key`), `TestCaptureDTOsCarryNoPayload` (`synthetic-private`), `TestEveryDTOIsRedactionSafe` |
| QG-12 | `MaxErrorMessageBytes` (512) tested at 512 and 513 in `TestFromErrorBoundsMessage` |

Delivery assertions:

| DA | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | No storage, tools or platform type is in the frontend contract | `grep -nE 'export namespace (storage\|tools\|platform)' frontend/wailsjs/go/models.ts` | no match | CI, reviewer |
| DA-2 | Binding signatures use only contract types | `go test ./internal/adapter/wailsui -run TestBindingSignaturesUseContractTypes` | pass | CI |
| DA-3 | DTO JSON is byte-identical to the previous wire format | `go test ./internal/adapter/contract -run TestDTOJSONMatchesSourceTypes` | pass for every row of §3 | CI |
| DA-4 | Every DTO is covered by the redaction test | `go test ./internal/adapter/contract -run TestEveryDTOIsRedactionSafe` | pass (fails when a DTO is unregistered) | CI |
| DA-5 | One error mapping per adapter | `grep -rn 'errors.As\|domain.Error{' internal/adapter/cli internal/adapter/wailsui --include='*.go' --exclude='*_test.go'` | no match outside `contract` | reviewer |
| DA-6 | Constants cover every documented code | `go test ./internal/domain -run TestCapabilityStatesAndErrorCodes` | pass | CI |
| DA-7 | Both catalogs render the consent state | `bunx playwright test -g "location consent"` | pass | CI |
| DA-8 | Desktop IPC still works with contract types | `node scripts/task.mjs bridge` on Windows with WebView2 | `TestNativeBridge` passes | [native] |

Evidence record: `docs/verification/specs/H06.md`.
