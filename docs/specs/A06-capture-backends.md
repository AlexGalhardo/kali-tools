# A06 Windows capture backends: Npcap and Pktmon spikes

| Field | Value |
| --- | --- |
| PLAN tasks | A06 (capture backend spike, Npcap evaluation, Pktmon evaluation, dropped-packet counters) |
| Status | In progress (2026-10-22): steps 1–6 adapters and interface exposure; UI/native acceptance pending |
| Depends on | H02 (capture session in the job manager), H03 (`capture.LiveProvider` port), H04 (`app.Application`), H05/H06 (adapters, DTOs), W01 + W04 (HTTP route) |
| Risk | High: driver licensing, process-wide DLL side effects of `gopacket/pcap`, and a system-wide Pktmon session that must always be stopped |

## 1. Goal

Architecture correction during step 6: the fixed Npcap URL is declared in domain and aliased by adapter/pcap. Wails uses domain.NpcapDownloadURL, because importing the driven adapter would violate docs/architecture.md and depguard. The URL and behavior are unchanged.

Give kali-tools two optional Windows capture backends besides the existing raw IPv4 socket, without bundling any driver and without making the default build depend on cgo:

- **(a) Npcap.** The default build detects a user-installed Npcap (registry and DLL presence) and reports it as a capability. With the opt-in build tag `pcap`, kali-tools captures Ethernet frames through `github.com/gopacket/gopacket/pcap`. Npcap is never bundled, downloaded or installed by kali-tools.
- **(b) Pktmon.** An optional adapter drives the built-in `pktmon.exe` with fixed argument arrays (`start`, `stop`, `etl2pcap`) from an elevated kali-tools process. It converts the ETL log to pcapng and imports it with the existing `capture.ReadArtifactFile`, reporting a dropped-packet count and explicit data-loss limits.

Both are spikes. They must deliver working, tested adapters plus the decision record `docs/adr/005-windows-capture-backend.md`, which states which backend becomes the alpha default and why, with native evidence.

## 2. Non-goals

- No bundling, downloading, silent installation or installer launch of Npcap. Only a link to `https://npcap.com/#download` is shown. The Npcap free license forbids redistribution and silent installation (tools research §3.2).
- No Linux/macOS libpcap capture (B01/B02). The `pcap` tag files are `windows`-only in this spec.
- No packet injection. `WritePacketData` and `pcap_sendpacket` are never called.
- No BPF filter input, no promiscuous mode and no monitor mode.
- No self-elevation, UAC prompt, scheduled task or service. The user starts kali-tools elevated if they want Pktmon.
- No change to offline import limits (64 MiB file, 1 MiB record, 10,000 packets) or to the pcap/pcapng reader in `internal/capture/offline.go`.
- No TShark execution.
- No WinRT and no ETW consumer written in Go. Pktmon's own `etl2pcap` does the conversion.
- No use of Pktmon filters (`pktmon filter add/remove`). They are system-wide state that kali-tools could clobber.
- No `pktmon unload`, `pktmon reset` or stopping of sessions kali-tools did not start.

## 3. Current state

Re-verify each fact before starting:

Checkout corrections (2026-10-22): H03 already supplies the three-result LiveProvider and Native implementation; app capture lifecycle is in `internal/app/capture.go`; contracts use `LiveCaptureRequest`/`LiveCapture`, and the HTTP start route is `POST /api/capture/start`. Backend/result fields must also update contract mappers, generated models and CLI JSON fixtures. Keep the established CLI permission exit 4. The existing capture provider field is migrated in step 2, not before its selection tests. Native spike measurements and the default-backend decision remain open; do not infer them from headless checks. The renewed owner instruction authorizes continuing implementation while prior platform-specific acceptance remains explicitly pending.

1. `internal/capture/live.go:14-18`: `LiveRequest{Address, DurationSeconds, PacketLimit}`.
2. `internal/capture/live.go:19-26`: `LiveResult{Limited, Packets, Provider, Dropped *uint64, Started, Finished}`.
3. `internal/capture/live.go:27-29` declares `type LiveProvider interface { Capture(context.Context, LiveRequest) (LiveResult, error) }`. No type implements it, and no code references it outside its declaration (verified by grep).
4. `internal/capture/live.go:31-37`: `Live` and `LiveSession` call the build-tagged `live(ctx, r, artifact)`. `LiveSession` creates `NewArtifact(layers.LinkTypeRaw)`.
5. `internal/capture/live.go:39-55`: `ValidateLiveRequest` requires:
   - a local, non-loopback IPv4 that belongs to an interface;
   - a duration of 1..60 s;
   - a packet limit of 1..`MaxPackets`.
6. `internal/capture/live_windows.go:88-153` is the raw-socket provider:
   - `SOCK_RAW` with `SIO_RCVALL` (`0x98000001`);
   - `WSAEACCES` maps to `permission_required`;
   - `Dropped` is never set (null), as ADR 002 states.
7. `internal/capture/live_other.go:10-12`: non-Windows returns `unsupported`.
8. `internal/capture/offline.go:15-16`: `MaxFileSize = 64 << 20` and `MaxPackets = 10000`. `read()` (`offline.go:107-169`) uses `pcapgo.NewNgReader(r, pcapgo.NgReaderOptions{ErrorOnMismatchingLinkType: true})`, so a pcapng with mixed link types fails to import.
9. `internal/capture/artifact.go:32-45`: `Artifact.Add` rejects records over 1 MiB and caps retained bytes at `MaxFileSize - MaxPackets*64 - 4096`.
10. `internal/capture/artifact.go:114-138`: `ReadArtifactFile(path)` requires a regular file of at most 64 MiB and returns `(ImportResult, *Artifact, error)`.
11. `internal/adapter/tools/process.go:39-76`: `runCommand(parent, path, args, limit)`:
    - caps stdout at `limit` and stderr at 8 KiB;
    - cancels on overflow;
    - never surfaces stderr;
    - uses `cmd.WaitDelay = 1s`.
12. `internal/adapter/tools/process_windows.go:13-59` starts processes suspended inside a kill-on-close Job Object (`CREATE_SUSPENDED | CREATE_NO_WINDOW`).
13. `internal/adapter/tools/tools.go:20-40`: `inspect` resolves executables with `exec.LookPath`. Pktmon must not use `LookPath` (PATH hijack); see §4.4.
14. `internal/app/capabilities.go:13-21`: `capture_live` is `permission_required` on Windows with detail "Administrator permission; raw IPv4 only, no Ethernet frames or drop counts".
15. `go.mod:6` pins `github.com/gopacket/gopacket v1.7.4` (BSD-3-Clause, `docs/dependencies.md`). In that version:
    - `pcap/pcap_windows.go` has no `import "C"`. It loads `wpcap.dll` with `LoadLibraryEx(..., LOAD_LIBRARY_SEARCH_USER_DIRS|LOAD_LIBRARY_SEARCH_SYSTEM32)` after calling `SetDllDirectoryA("<System32>\Npcap")`.
    - `init()` calls `LoadWinPCAP()` unconditionally on import.
    - `mustLoad` panics if a required export (for example `pcap_create` or `pcap_set_buffer_size`) is missing from the loaded `wpcap.dll`.
    - `pcap/pcap_unix.go` and `pcap/pcap_darwin.go` contain `import "C"`, so cgo and libpcap headers are required on non-Windows.
16. `golang.org/x/sys v0.49.0` provides:
    - `windows.RtlGetVersion()`;
    - `windows.GetCurrentProcessToken().IsElevated()`;
    - `windows.GetSystemDirectory()`;
    - `windows.GetFileVersionInfoSize`, `windows.GetFileVersionInfo` and `windows.VerQueryValue`;
    - `windows.VS_FIXEDFILEINFO`;
    - the `golang.org/x/sys/windows/registry` package.
17. `docs/adr/002-capture-backend.md` records the raw-socket prototype and states that a driver-backed provider needs a fresh license review.

## 4. Design

### 4.1 Backend identifiers and the provider port

In `internal/capture/backend.go` (new, no build tag):

```go
package capture

const (
    BackendRawSocket = "raw_socket"
    BackendPktmon    = "pktmon"
    BackendNpcap     = "npcap"
)

// Data-loss and scope notices attached to LiveResult.Limits. Stable codes.
const (
    LimitCircularBuffer    = "circular_buffer_may_overwrite_oldest"
    LimitTruncated         = "packets_truncated"
    LimitComponentIdentity = "component_identity_lost"
    LimitAllNICs           = "capture_scope_all_nics"
    LimitDuplicates        = "duplicates_possible"
    LimitDropCountMissing  = "drop_count_unavailable"
)
```

Change `internal/capture/live.go`:

```go
type LiveRequest struct {
    Backend         string `json:"backend,omitempty"` // "" means BackendRawSocket
    Address         string `json:"address"`
    DurationSeconds int    `json:"duration_seconds"`
    PacketLimit     int    `json:"packet_limit"`
}

type LiveResult struct {
    Limited  bool      `json:"limited"`
    Packets  []Packet  `json:"packets"`
    Provider string    `json:"provider"`
    Dropped  *uint64   `json:"dropped"`
    Limits   []string  `json:"limits"` // never nil; codes from backend.go
    Started  time.Time `json:"started"`
    Finished time.Time `json:"finished"`
}

// LiveProvider captures one bounded session and returns the retained artifact
// (nil when nothing was captured). Implementations: Native (this package, raw socket; added by H03),
// tools.Pktmon (adapter/tools), pcap.Npcap (adapter/pcap, build tag pcap).
type LiveProvider interface {
    Capture(ctx context.Context, r LiveRequest) (LiveResult, *Artifact, error)
}

// Native (added by H03) adapts the existing raw IPv4 provider. A06 adds only its Status method:
type Native struct{}

func (Native) Status(ctx context.Context) domain.Capability // today's static capture_live logic moved from app.Capabilities
```

H03 already changes the `LiveProvider` method set from `(LiveResult, error)` to `(LiveResult, *Artifact, error)` and adds `capture.Native`. If H03 is Done, only verify that the signature matches; otherwise apply that change here. It is safe because nothing implements the old one (fact 3).

`ValidateLiveRequest(r)` becomes backend-aware:

| `Backend` | `Address` rule | Duration | Packet limit |
| --- | --- | --- | --- |
| `""`, `raw_socket` | local non-loopback IPv4, as today | 1..60 | 1..10000 |
| `npcap` | local IPv4 or IPv6 address of an interface (loopback allowed, because Npcap has `NPF_Loopback`) | 1..60 | 1..10000 |
| `pktmon` | must be empty: Pktmon captures every NIC component | 1..60 | 1..10000 |
| anything else | `invalid_input` "backend must be raw_socket, pktmon or npcap" | — | — |

### 4.2 Application wiring

In `internal/app` (file `capture.go`):

```go
// CaptureBackend is declared by its consumer (app). Adapters return concrete types.
type CaptureBackend interface {
    capture.LiveProvider
    Status(ctx context.Context) domain.Capability // cheap: registry/stat/token only, never opens a driver
}

// Application field, set by the composition root (bootstrap):
//     CaptureBackends map[string]CaptureBackend // keys: capture.Backend*
func (a *Application) CaptureBackendStatus(ctx context.Context) []domain.Capability
```

Rules:

- `StartCapture(ctx, r)` (the H02/H04 use case) looks up `a.CaptureBackends[backendOrDefault(r.Backend)]`.
  - A missing key gives `unsupported`.
  - If `Status(ctx).State != "available"`, return `&domain.Error{Code: <that state>, Message: <detail>}` without calling `Capture`.
  - The existing "one capture at a time" rule is unchanged.
- `Capabilities` adds `capture_pktmon` and `capture_npcap` from `CaptureBackendStatus`. `capture_live` keeps its id and now comes from `capture.Native.Status`, which carries today's static logic.
- History stores only `"Capture <backend> <state>: <n> packets, <d> dropped"`. It never stores addresses, file paths or payloads.

This map replaces H03's single `app.Deps.Capture CaptureProvider` field with `app.Deps.CaptureBackends map[string]CaptureBackend`. Update the H03 compile-time assertions accordingly (`var _ app.CaptureBackend = capture.Native{}` and the same for `*tools.Pktmon` and `*pcap.Npcap`).

Composition roots (`internal/bootstrap`) register the backends:

- Windows default build: `raw_socket` → `capture.Native{}`, `pktmon` → `tools.NewPktmon()`, `npcap` → `pcap.NewNpcap()`.
- Without the `pcap` tag, `pcap.NewNpcap()` returns a stub whose `Status` reports detection results with state `unsupported` when Npcap is installed, and whose `Capture` returns `unsupported`.
- Non-Windows: all three report `unsupported`.

### 4.3 Spike (a): Npcap detection and the `pcap` build tag

New package `internal/adapter/pcap`. The name is the technology, as in architecture §2.

#### 4.3.1 Detection (default build, no gopacket import)

```go
// internal/adapter/pcap/detect.go (no build tag): pure types and mapping.
package pcap

const NpcapDownloadURL = "https://npcap.com/#download"

type NpcapStatus struct {
    Installed       bool   `json:"installed"`
    LegacyWinPcap   bool   `json:"legacy_winpcap"`
    Version         string `json:"version"`          // "1.79.0.0" from Packet.dll VS_FIXEDFILEINFO, "" if unknown
    AdminOnly       bool   `json:"admin_only"`
    LoopbackSupport bool   `json:"loopback_support"`
}

// npcapProbe is what the OS layer observed; mapping is pure and table-tested.
type npcapProbe struct {
    ServiceKey      bool   // HKLM\SYSTEM\CurrentControlSet\Services\npcap exists
    WpcapDLL        bool   // <System32>\Npcap\wpcap.dll is a regular file
    PacketDLL       bool   // <System32>\Npcap\Packet.dll is a regular file
    LegacyWpcap     bool   // <System32>\wpcap.dll exists (WinPcap or WinPcap-compatible mode)
    AdminOnly       uint32 // Services\npcap\Parameters\AdminOnly, 0 when absent
    LoopbackSupport uint32 // Services\npcap\Parameters\LoopbackSupport, 0 when absent
    Version         string
}

func statusFromProbe(p npcapProbe) NpcapStatus
// capability maps status to domain.Capability{ID: "capture_npcap", Provider: "npcap"}.
func capability(s NpcapStatus, backendBuilt, elevated bool) domain.Capability
```

```go
// internal/adapter/pcap/detect_windows.go  (//go:build windows)
func DetectNpcap() (NpcapStatus, error)
// detect_other.go (//go:build !windows): returns NpcapStatus{}, nil
```

Rules for `statusFromProbe`:

- `Installed = ServiceKey && WpcapDLL && PacketDLL`.
- `LegacyWinPcap = !Installed && LegacyWpcap`.
- `AdminOnly = AdminOnly == 1`.
- `LoopbackSupport = LoopbackSupport == 1`.

Rules for `capability` (evaluated in this order):

| Condition | State | Detail |
| --- | --- | --- |
| `!Installed && LegacyWinPcap` | `unsupported` | "WinPcap is not supported; install Npcap yourself from https://npcap.com/#download" |
| `!Installed` | `dependency_missing` | "Npcap is not installed. kali-tools never bundles or installs it; install it yourself from https://npcap.com/#download" |
| `!backendBuilt` | `unsupported` | "Npcap {version} detected; this build does not include the Npcap backend (build tag pcap)" |
| `AdminOnly && !elevated` | `permission_required` | "Npcap is installed in Administrators-only mode; start kali-tools as Administrator" |
| otherwise | `available` | "Npcap {version}; Ethernet frames with drop counters" |

OS probe details for `detect_windows.go`:

- Registry: open `HKLM\SYSTEM\CurrentControlSet\Services\npcap` with `registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)`, then its `Parameters` subkey, and read `GetIntegerValue("AdminOnly")` and `GetIntegerValue("LoopbackSupport")`. A missing key or value means false or 0. Never write the registry.
- DLL paths: `filepath.Join(windows.GetSystemDirectory(), "Npcap", "wpcap.dll")` and `"Packet.dll"`, checked with `os.Lstat` plus `Mode().IsRegular()`. Never use the PATH or the current directory.
- Version: `GetFileVersionInfoSize` on `Packet.dll`, then `GetFileVersionInfo` into a buffer of at most 64 KiB (refuse larger), then `VerQueryValue(block, "\\", &ptr, &len)` with `len >= uint32(unsafe.Sizeof(windows.VS_FIXEDFILEINFO{}))`. Format `FileVersionMS>>16 . FileVersionMS&0xffff . FileVersionLS>>16 . FileVersionLS&0xffff`. A failure leaves `Version` empty; it is not an error.
- The registry value names and the `Packet.dll` version source are **(verify)**. AC-9 confirms them on a native install.

#### 4.3.2 Capture backend (opt-in tag `pcap`)

Files:

- `internal/adapter/pcap/npcap_windows_pcap.go` (`//go:build windows && pcap`): imports `github.com/gopacket/gopacket/pcap`;
- `internal/adapter/pcap/npcap_stub.go` (`//go:build !(windows && pcap)`): stub.

```go
type Npcap struct {
    detect   func() (NpcapStatus, error)
    elevated func() bool
}

func NewNpcap() *Npcap
func (n *Npcap) Status(ctx context.Context) domain.Capability
func (n *Npcap) Capture(ctx context.Context, r capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error)

const (
    NpcapSnapLen     = 65535
    NpcapReadTimeout = 250 * time.Millisecond
    NpcapBufferBytes = 4 << 20
    MaxPcapDevices   = 256
)
```

Capture algorithm (tagged build):

1. Validate the request with `capture.ValidateLiveRequest(r)` (backend `npcap`). If `Status(ctx).State != "available"`, return that state as a `domain.Error`.
2. Call `pcap.FindAllDevs()`. If more than `MaxPcapDevices` devices come back, return `failed` "device list exceeds limit". Select the first device whose `Addresses[i].IP` equals `r.Address`, comparing with `netip.Addr` and ignoring zones. No match gives `invalid_input` "address does not belong to an Npcap device".
3. Open the handle:

    ```go
    h, err := pcap.NewInactiveHandle(dev.Name)
    h.SetSnapLen(NpcapSnapLen); h.SetPromisc(false); h.SetTimeout(NpcapReadTimeout)
    h.SetImmediateMode(true); h.SetBufferSize(NpcapBufferBytes)
    handle, err := h.Activate(); defer handle.Close(); h.CleanUp()
    ```

    An activation error whose text contains "Access is denied" maps to `permission_required`. Every other error maps to `failed` with the fixed message "Npcap could not open the device". Never echo the driver text.
4. Create `artifact := capture.NewArtifact(handle.LinkType())`. Derive a context with `time.Duration(r.DurationSeconds)*time.Second`.
5. Loop until the context is done or `len(result.Packets) == r.PacketLimit`:
    - `data, ci, err := handle.ReadPacketData()`;
    - on `pcap.NextErrorTimeoutExpired`, continue;
    - on any other error, stop with `failed`;
    - `artifact.Add(data, ci)` failing sets `Limited = true` and stops;
    - `capture.Decode(...)` appends the summary.
6. After the loop, call `handle.Stats()` and set `dropped := uint64(s.PacketsDropped) + uint64(s.PacketsIfDropped)`, then `result.Dropped = &dropped`. If `Stats` fails, append `LimitDropCountMissing`.
7. Set `Provider = "Npcap " + version + " (gopacket/pcap)"` and `Limits = []string{}`, plus `LimitTruncated` if any `ci.Length > ci.CaptureLength`.

Cancellation latency is at most `NpcapReadTimeout` plus one decode, so under 500 ms. The handle is always closed through `defer`.

#### 4.3.3 cgo implications (state these in ADR 005)

- **Windows:** `gopacket/pcap` builds with `CGO_ENABLED=0`, because `pcap_windows.go` uses `LoadLibraryEx`/`GetProcAddress`. The `pcap` tag therefore keeps Windows builds cgo-free. Prove it with `CGO_ENABLED=0 GOOS=windows go build -tags pcap ./...`.
- **Linux/macOS:** `gopacket/pcap` needs `CGO_ENABLED=1`, a C toolchain and libpcap headers (`libpcap-dev` / Xcode SDK). This spec adds no non-Windows tagged file, so `-tags pcap` on Linux/macOS still builds cgo-free and reports `unsupported`. Libpcap capture is B01/B02 and needs its own cross-compilation decision.
- **Import side effects (the reason for the tag):**
  - importing `gopacket/pcap` on Windows runs `LoadWinPCAP()` from `init()`;
  - that calls `SetDllDirectoryA("<System32>\Npcap")` for the whole process, which changes the DLL search order for every later `LoadLibrary`, including WebView2;
  - it panics through `mustLoad` if an installed `wpcap.dll` lacks a required export.
  The default build must never import it. A07/ADR 005 must decide between keeping the tag, accepting these effects, or replacing `gopacket/pcap` with a minimal `LazyDLL` shim loaded by absolute path on first use (option C in §4.5).

### 4.4 Spike (b): Pktmon adapter

Files:

- `internal/adapter/tools/pktmon.go` (no tag): constants, argv builders and the result assembly. Pure and testable on every OS.
- `internal/adapter/tools/pktmon_windows.go`: detection and the real session.
- `internal/adapter/tools/pktmon_other.go`: `Status` is `unsupported`; `Capture` returns `unsupported`.

```go
const (
    PktmonMinBuild       = 19041           // pktmon --capture syntax (Windows 10 2004)
    PktmonPacketBytes    = 512             // --pkt-size; never 0 (full payload)
    PktmonFileSizeMB     = 48              // --file-size; ETL stays well under MaxFileSize after conversion
    PktmonStepTimeout    = 15 * time.Second
    PktmonConvertTimeout = 60 * time.Second
    PktmonOutputLimit    = 64 << 10        // stdout cap per pktmon invocation; output is discarded
)

type PktmonStatus struct {
    Path     string // <System32>\PktMon.exe, resolved once; never from PATH
    Build    uint32 // RtlGetVersion().BuildNumber
    Elevated bool   // GetCurrentProcessToken().IsElevated()
}

func pktmonStartArgs(etl string) []string
func pktmonStopArgs() []string
func pktmonConvertArgs(etl, out string, dropOnly bool) []string

type commandRunner func(ctx context.Context, path string, args []string, limit int) ([]byte, error)

type Pktmon struct {
    mu     sync.Mutex // one pktmon session per process
    status func() (PktmonStatus, error)
    run    commandRunner // runCommand in production; fake in tests
    tmp    func() (string, error) // os.MkdirTemp("", "kali-tools-pktmon-*") in production
    sleep  func(ctx context.Context, d time.Duration) // waits for d or ctx.Done
}

func NewPktmon() *Pktmon
func (p *Pktmon) Status(ctx context.Context) domain.Capability
func (p *Pktmon) Capture(ctx context.Context, r capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error)
```

Exact argv. Both `etl` and `out` are absolute paths inside the temporary directory that kali-tools created:

```go
pktmonStartArgs(etl)  == []string{"start", "--capture", "--comp", "nics", "--pkt-size", "512",
                                  "--file-name", etl, "--file-size", "48", "--log-mode", "circular"}
pktmonStopArgs()      == []string{"stop"}
pktmonConvertArgs(etl, out, false) == []string{"etl2pcap", etl, "--out", out}
pktmonConvertArgs(etl, out, true)  == []string{"etl2pcap", etl, "--out", out, "--drop-only"}
```

`Status` (evaluated in order):

| Condition | State | Detail |
| --- | --- | --- |
| not Windows | `unsupported` | "Pktmon exists only on Windows" |
| `Build < PktmonMinBuild` or `PktMon.exe` missing | `unsupported` | "Pktmon requires Windows 10 2004 (build 19041) or later" |
| `!Elevated` | `permission_required` | "Pktmon requires an elevated (Administrator) kali-tools process; kali-tools never self-elevates" |
| otherwise | `available` | "Built-in Pktmon; all NICs, packets truncated to 512 bytes" |

Session sequence in `Capture` (Windows). `p.mu` is held for the whole sequence; a second call while it is held returns `conflict` "a Pktmon capture is already running":

1. Validate (backend `pktmon`) and check `Status`.
2. `dir, err := p.tmp()`, then `defer os.RemoveAll(dir)`. Set `etl := filepath.Join(dir, "capture.etl")`, `out := filepath.Join(dir, "capture.pcapng")` and `drops := filepath.Join(dir, "drops.pcapng")`.
3. Run `p.run(stepCtx, path, pktmonStartArgs(etl), PktmonOutputLimit)` with `stepCtx` = `ctx` plus `PktmonStepTimeout`. On error, return `failed` "pktmon start failed; another Pktmon session may be active (check with 'pktmon status' in an elevated prompt)". Return the error without calling stop: the session did not start.
4. `p.sleep(ctx, time.Duration(r.DurationSeconds)*time.Second)`, which returns early on cancellation.
5. **Always** stop, even after cancellation: run `p.run(stopCtx, path, pktmonStopArgs(), PktmonOutputLimit)` with `stopCtx := context.WithTimeout(context.WithoutCancel(ctx), PktmonStepTimeout)`. On stop failure, return `failed` "pktmon stop failed; run 'pktmon stop' in an elevated prompt". This step runs inside a `defer` registered immediately after step 3 succeeds, guarded by a `stopped` flag so it runs exactly once.
6. If `ctx` was cancelled, return `context.Canceled` after the stop, without converting.
7. Run `p.run(convCtx, path, pktmonConvertArgs(etl, out, false), PktmonOutputLimit)` with `PktmonConvertTimeout`.
8. Call `result, artifact, err := capture.ReadArtifactFile(out)`. Its existing errors (over 64 MiB, malformed, mixed link types) are returned unchanged, wrapped as `failed` with the fixed message "Pktmon output could not be imported".
9. Drop count: run `p.run(convCtx, path, pktmonConvertArgs(etl, drops, true), PktmonOutputLimit)`, then call `capture.ReadFile(drops)`. On success, set `Dropped = uint64(len(r.Packets))` and append `LimitTruncated`-style notes as applicable. On any failure, `Dropped = nil` and append `LimitDropCountMissing`.
10. Truncate `result.Packets` to `r.PacketLimit`. Set `Limited = true` if truncation happened or `ImportResult.Limited` is true. Do not trim the artifact; the artifact limits already apply.
11. Set `Provider = "Windows Pktmon (build <n>)"` and `Limits = []string{LimitCircularBuffer, LimitTruncated, LimitComponentIdentity, LimitAllNICs, LimitDuplicates}` (plus `LimitDropCountMissing` if set). Set `Started` and `Finished` from the clock around steps 3 and 5.

Privacy: the ETL and pcapng files hold up to 512 bytes of payload per packet. They live only in the `0700` temporary directory and are removed by the `defer` in step 2 on every path. The retained artifact lives in memory, as with the raw-socket backend. Paths never appear in errors, history or logs.

Crash behavior, recorded in ADR 005: if kali-tools dies between steps 3 and 5, the Pktmon session keeps running in circular mode, bounded to 48 MB. The next `start` fails with the step-3 message. kali-tools never stops a session it cannot prove it started.

### 4.5 Decision record: `docs/adr/005-windows-capture-backend.md`

The implementer writes this ADR from native measurements. Required sections and content:

1. **Status:** "accepted for alpha" or "proposed", with the date and the commit.
2. **Context:** ADR 002 limits (raw IPv4, no drop counts); the Npcap license (free edition: ≤ 5 systems, no redistribution, no silent install; OEM is paid); Pktmon availability (build ≥ 19041, elevation).
3. **Options**, each with license, distribution, privilege, cgo, link type, drop accounting and data-loss facts:
    - A. raw socket (ADR 002, unchanged);
    - B. Npcap via `gopacket/pcap` behind the `pcap` tag;
    - C. Npcap via a kali-tools minimal `windows.NewLazyDLL` shim of about 12 `wpcap.dll` exports, loaded by absolute path `<System32>\Npcap\wpcap.dll` on first use, with no `init` side effects;
    - D. Pktmon via `pktmon.exe` and `etl2pcap`.
4. **Measurements (native, required):** for each of B and D on the same host, capturing controlled traffic (`curl` of a 10 MB file from a local HTTP server, plus 1,000 pings):
    - packets captured versus sent;
    - reported drops;
    - stop latency (ms from cancel to return);
    - ETL size, pcapng size and conversion time (D);
    - peak working set;
    - link type(s) seen;
    - duplicate count, measured as identical IPv4 ID plus 5-tuple within 1 ms.
5. **`gopacket/pcap` side effects:**
    - the observed effect of `SetDllDirectoryA` on the Wails desktop build (does WebView2 still start?);
    - the behavior when only legacy WinPcap is installed (panic or not).
6. **Pktmon facts confirmed:**
    - elevation requirement (exit code and message when not elevated);
    - behavior when a session is already running;
    - `etl2pcap --drop-only` output with zero drops;
    - whether pcapng output passes `ErrorOnMismatchingLinkType`.
7. **Decision:** the alpha default backend order, and whether B, C or neither ships in default builds.
8. **Consequences:** the capability states shown to users, the install instructions (link only), and the follow-ups for B01/B02 libpcap with cgo.
9. **Revisit triggers:**
    - Npcap license change;
    - `gopacket/pcap` removing its `init` side effect;
    - a pure-Go ETW reader;
    - Pktmon syntax change.

### 4.6 Driving adapters and UI

| Interface | Change |
| --- | --- |
| Wails (`internal/adapter/wailsui`) | `StartCapture(r contract.CaptureRequest)` gains `backend`. New `CaptureBackends() []contract.Capability`. New `OpenNpcapDownload() error`, which calls `BrowserOpenURL(ctx, pcap.NpcapDownloadURL)` with the constant; there is no URL parameter. |
| HTTP (`internal/adapter/httpapi`) | `POST /api/capture` body gains `backend`. `GET /api/capture/backends` returns the three capabilities. No download-opening route. |
| CLI (`internal/adapter/cli`) | `kali-tools capture --backend raw_socket\|pktmon\|npcap` (default `raw_socket`). `--target` is required for `raw_socket`/`npcap` and rejected for `pktmon`. `kali-tools doctor` lists `capture_live`, `capture_pktmon` and `capture_npcap` and replaces the fixed `capture` string at `cmd/kali-tools/main.go:65`. |
| DTO (`internal/adapter/contract`) | `CaptureRequest.Backend string`, `CaptureResult.Limits []string`, `CaptureResult.Dropped *uint64`. |
| Frontend | Packet inspector: a backend `<select>` whose options are disabled unless the capability state is `available`, with the detail as the description. For `dependency_missing`, an "Install Npcap (opens npcap.com)" button (Wails) or the URL text (browser). Limits render as a translated list. "Dropped: n" shows when `dropped !== null`. |

If H05/W01 have not landed, add these to `app.go` / `cmd/kali-tools` until H05 lands and record the HTTP route as a follow-up.

i18n keys (both catalogs):

- `captureBackend`, `backendRawSocket`, `backendPktmon`, `backendNpcap`;
- `installNpcap`, `droppedPackets`;
- `limit_circular_buffer_may_overwrite_oldest`, `limit_packets_truncated`, `limit_component_identity_lost`, `limit_capture_scope_all_nics`, `limit_duplicates_possible`, `limit_drop_count_unavailable`.

### 4.7 Limits summary

| Limit | Value | Enforced in |
| --- | --- | --- |
| Duration | 1..60 s | `capture.ValidateLiveRequest` |
| Packets | 1..10,000 | same |
| Npcap snap length | 65,535 B | `pcap.NpcapSnapLen` |
| Npcap kernel buffer | 4 MiB | `pcap.NpcapBufferBytes` |
| Npcap read timeout | 250 ms | `pcap.NpcapReadTimeout` |
| Npcap devices scanned | 256 | `pcap.MaxPcapDevices` |
| Pktmon bytes per packet | 512 | `tools.PktmonPacketBytes` |
| Pktmon ETL size | 48 MB circular | `tools.PktmonFileSizeMB` |
| Pktmon step timeout | 15 s (start, stop) | `tools.PktmonStepTimeout` |
| Pktmon conversion timeout | 60 s | `tools.PktmonConvertTimeout` |
| Pktmon stdout per call | 64 KiB, discarded | `tools.PktmonOutputLimit` |
| Imported pcapng | 64 MiB / 1 MiB record / 10,000 packets | existing `capture` limits |
| Version-info buffer | 64 KiB | `pcap` detection |

## 5. Files

Create:

- `internal/capture/backend.go`: backend and limit constants.
- `internal/capture/backend_test.go`: validation table per backend.
- `internal/adapter/pcap/detect.go`: `NpcapStatus`, `npcapProbe`, `statusFromProbe`, `capability`, `NpcapDownloadURL`.
- `internal/adapter/pcap/detect_windows.go`: registry, DLL and version probe.
- `internal/adapter/pcap/detect_other.go`: non-Windows probe.
- `internal/adapter/pcap/detect_test.go`: mapping tables.
- `internal/adapter/pcap/npcap_windows_pcap.go`: tagged capture backend.
- `internal/adapter/pcap/npcap_stub.go`: untagged stub.
- `internal/adapter/pcap/injection_test.go`: AST guard against injection calls.
- `internal/adapter/tools/pktmon.go`: constants, argv builders, Pktmon type and assembly.
- `internal/adapter/tools/pktmon_windows.go`: detection, the real `tmp`/`sleep`/`run`.
- `internal/adapter/tools/pktmon_other.go`: unsupported.
- `internal/adapter/tools/pktmon_test.go`: fake-runner tests.
- `internal/app/capture.go` and `internal/app/capture_test.go`: `CaptureBackend` and selection.
- `docs/adr/005-windows-capture-backend.md`: decision record (§4.5).

Modify:

- `internal/capture/live.go`: request/result fields, `LiveProvider`, `Native.Status`, backend-aware validation.
- `internal/capture/live_windows.go`, `internal/capture/live_other.go`: `Limits: []string{}`.
- `internal/app/capabilities.go`: drop the static `capture_live` branch in favor of backend statuses.
- `internal/bootstrap/*`: backend registration.
- Driving adapters, contract DTOs, `frontend/src/shared/api/bridge.ts`, `frontend/src/app/App.tsx` (or `frontend/src/features/capture/`), `frontend/src/shared/i18n/i18n.ts`, `frontend/tests/workflows.spec.ts`.
- `docs/dependencies.md`, `docs/adr/002-capture-backend.md` (link to 005), `docs/architecture.md`, `PLAN.md`.

Delete: none.

## 6. Implementation steps

1. Add `capture/backend.go`. Extend `LiveRequest`/`LiveResult`, `ValidateLiveRequest`, `LiveProvider` and `Native.Status`. Update the existing callers (`app.go`, the CLI) to pass through unchanged. Run `go test ./internal/capture/`.
2. Add `internal/app/capture.go` with `CaptureBackend` and selection. Write tests with fake backends first.
3. Add the Npcap detection (`detect*.go`) with table tests. Register the stub backend in bootstrap. Check that `go list -deps ./cmd/kali-tools` contains no `gopacket/pcap`.
4. Add the tagged backend `npcap_windows_pcap.go` and the injection guard test. Run `CGO_ENABLED=0 GOOS=windows go vet -tags pcap ./...`.
5. Add the Pktmon pure parts and fake-runner tests, then `pktmon_windows.go`. Register in bootstrap.
6. Wire capabilities, the CLI `--backend` and `doctor`, the Wails bindings, the HTTP route and the DTOs.
7. Frontend: backend selector, limits list, Npcap link, i18n, Playwright.
8. Run the native spikes (§7 AC-9 to AC-13) and write ADR 005 with the measurements.
9. Update the docs (§10) and run the full verification.
10. Write `docs/verification/specs/A06.md`.

## 7. Acceptance criteria

- **AC-1.** Given each backend value, when `ValidateLiveRequest` runs, then the rules of §4.1 hold. For example, `pktmon` with a non-empty address is `invalid_input`, and backend `"x"` is `invalid_input`.
- **AC-2.** Given npcap probes (none, legacy-only, installed+admin-only, installed), when mapped with `backendBuilt` and `elevated` combinations, then the states are `dependency_missing`, `unsupported`, `unsupported` (not built), `permission_required` and `available`, and each `dependency_missing` detail contains `https://npcap.com/#download`.
- **AC-3.** Given the default build, when the dependencies of `./cmd/kali-tools` and the root desktop package are listed, then `github.com/gopacket/gopacket/pcap` is absent.
- **AC-4.** Given `CGO_ENABLED=0 GOOS=windows`, when building with `-tags pcap`, then the build succeeds (Windows backend is cgo-free).
- **AC-5.** Given the sources of `internal/adapter/pcap`, when the injection guard parses them, then no call to `WritePacketData`, `ZeroCopyWritePacketData` or any identifier containing `SendPacket` exists.
- **AC-6.** Given a fake runner, when a Pktmon capture of 1 s completes, then:
  - the runner receives start, stop, etl2pcap and etl2pcap `--drop-only`, in that order, with exactly the §4.4 argv and the path `<System32>\PktMon.exe`;
  - packets come from the fixture pcapng;
  - `Dropped` equals the drop fixture count;
  - `Limits` contains the five fixed codes;
  - the temporary directory no longer exists.
- **AC-7.** Given a fake runner and a context cancelled during the wait, when `Capture` returns, then `stop` was called exactly once, no conversion ran, the error is `context.Canceled` and the temporary directory is removed.
- **AC-8.** Given a fake runner whose `start` fails, then `stop` is not called and the error code is `failed`. Given `stop` failing, the error code is `failed` with the manual-stop hint. Given a second concurrent `Capture`, the error code is `conflict`.
- **AC-9 [native].** Given Windows 11 with Npcap installed by the user (default options), when `kali-tools doctor` runs, then `capture_npcap` reports the installed version. After setting Npcap "Administrators only", a non-elevated run reports `permission_required`.
- **AC-10 [native].** Given a `-tags pcap` desktop build on that host, when capturing 10 s on the Wi-Fi or Ethernet address while pinging the gateway 20 times, then at least 20 ICMP echo requests are decoded, `dropped` is a number, stop returns within 1 s of pressing Stop, and the desktop WebView keeps working after capture.
- **AC-11 [native].** Given an elevated kali-tools on Windows 11, when a 10 s Pktmon capture runs while pinging the gateway, then packets are imported, `pktmon status` afterwards shows no running session, and `%TEMP%` contains no `kali-tools-pktmon-*` directory.
- **AC-12 [native].** Given a non-elevated kali-tools, when Pktmon capture is requested, then `permission_required` is returned and no `pktmon.exe` process is started (checked with Process Monitor or `Get-Process pktmon` during the request).
- **AC-13 [native].** Given an elevated kali-tools and the Pktmon capture cancelled after 2 s, then the session is stopped (`pktmon status` shows none) within 15 s.

## 8. Tests

| Test | Package | Asserts | ACs |
| --- | --- | --- | --- |
| `TestLiveRequestBackendValidation` | `internal/capture` | Table over backends, addresses, duration 0/1/60/61, limit 0/1/10000/10001 | AC-1 |
| `TestCaptureRequestBounds` (existing) | `internal/capture` | Unchanged for the raw socket | AC-1 |
| `TestNpcapStatusMapping` | `internal/adapter/pcap` | Probe-to-status-to-capability table; URL in details | AC-2 |
| `TestDefaultBuildExcludesGopacketPcap` | `internal/adapter/pcap` | Runs `go list -deps github.com/AlexGalhardo/kali-tools/cmd/kali-tools` via `exec.Command(goBinary, ...)` (skipped with a reason when the `go` binary is not found) and asserts no line equals `github.com/gopacket/gopacket/pcap` | AC-3 |
| `TestNoPacketInjectionCalls` | `internal/adapter/pcap` | `go/parser` over `*.go` in the package dir; fails on forbidden selectors | AC-5 |
| `TestPktmonArgv` | `internal/adapter/tools` | Exact slices from §4.4 | AC-6 |
| `TestPktmonSessionSequence` | `internal/adapter/tools` | Fake runner records calls; writes fixture pcapng files (built with `pcapgo.NgWriter`, 3 packets and 2 drop packets) to the `--out` path; asserts order, `Dropped == 2`, `Limits`, temp dir removed | AC-6 |
| `TestPktmonCancelAlwaysStops` | `internal/adapter/tools` | Cancel during `sleep`; one stop; no etl2pcap; dir removed | AC-7 |
| `TestPktmonStartAndStopFailures` | `internal/adapter/tools` | Failure branches and the `conflict` on concurrent calls | AC-8 |
| `TestPktmonStatusStates` | `internal/adapter/tools` | Build 19040/19041, elevated true/false, missing exe | AC-12 (unit part) |
| `TestStartCaptureSelectsBackendAndRefusesUnavailable` | `internal/app` | Fake backends; non-available state returned as error code without calling `Capture` | AC-2, AC-12 |
| Playwright `capture backend selector reflects capability states` | `frontend/tests/workflows.spec.ts` | Disabled options, Npcap link, limits list, dropped count | UI |

Bound tests (QG-12) are at limit and limit+1: duration, packet count, `MaxPcapDevices` (via a pure helper `selectDevice(devs, addr)` taking a slice of 256 and 257 entries) and the version-info buffer (pure size check helper).

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
CGO_ENABLED=0 GOOS=windows go vet -tags pcap ./...
CGO_ENABLED=0 GOOS=windows go build -tags pcap -o /dev/null ./cmd/kali-tools
GOOS=linux go vet -tags pcap ./... && GOOS=darwin go vet -tags pcap ./...
go list -deps ./cmd/kali-tools | grep -x 'github.com/gopacket/gopacket/pcap' && exit 1 || true
go test -run 'Pktmon|Npcap|Backend|Injection' ./internal/...
```

Native: AC-9 to AC-13 and the ADR 005 measurements on the owner's Windows 11 host (Npcap installed manually by the owner from npcap.com).

## 10. Documentation updates

- `docs/adr/005-windows-capture-backend.md`: new (§4.5).
- `docs/adr/002-capture-backend.md`: add "Superseded in part by ADR 005" with a link.
- `docs/dependencies.md`:
  - add a row "Npcap (not redistributed) — user-installed driver, Npcap License; detected only; free edition forbids redistribution and silent install";
  - add a row "Windows Pktmon — OS component; invoked via argv; nothing redistributed";
  - note that `gopacket/pcap` (BSD-3-Clause, part of the pinned gopacket v1.7.4) is compiled only with `-tags pcap`.
- `docs/architecture.md`:
  - §4.2/§4.3: `CaptureProvider` row lists Native (raw socket), Pktmon and Npcap with their files;
  - §2: add `internal/adapter/pcap`.
- `docs/tools-research.md` §3.4 and §11: replace the "(verify)" on Pktmon elevation with the native result.
- `PLAN.md` A06:
  - tick the two spike sub-items only with ADR 005 and the native evidence;
  - tick "dropped-packet counters" only if AC-10 or AC-11 shows a non-null `dropped`.
- `README.md`: capture section explains the backends, the elevation requirement and the user-installed Npcap with a link.

## 11. Risks and rollback

- **`gopacket/pcap` init side effects** (`SetDllDirectoryA`, `mustLoad` panic). Contained by the `pcap` tag and the default-build dependency test (DA-1). ADR 005 option C is the fix if B is rejected.
- **Pktmon session left running** after a crash. Bounded at 48 MB circular, and the user gets an actionable message. Never auto-stop.
- **Pktmon output imports fail** because of mixed link types or the 64 MiB cap. This surfaces as `failed` "could not be imported". The reader is not changed in this spec. If AC-11 shows mixed link types, open a follow-up to import per interface.
- **Localized `pktmon` output.** kali-tools never parses `pktmon` stdout; it relies only on exit status. The risk is limited to exit-code semantics, which AC-11 to AC-13 confirm.
- **Registry value names (verify).** If they are wrong, detection reports `dependency_missing` on a host with Npcap. AC-9 catches it.
- **Rollback.** Revert the commits. The raw-socket backend and the import path are unchanged by the revert, and no persistent state or registry change exists.

## 12. Quality and delivery assertions

### 12.1 Applicable quality gates

| Gate | Applies | Tightened threshold |
| --- | --- | --- |
| QG-1, QG-2, QG-3, QG-4, QG-5, QG-7, QG-13, QG-14, QG-15 | yes | none |
| QG-3 | yes | additionally `CGO_ENABLED=0 GOOS=windows go vet -tags pcap ./...` and `GOOS=linux go vet -tags pcap ./...` |
| QG-6 | yes | `internal/adapter/pcap` (untagged files) ≥ 80%; `pktmon.go` ≥ 85%; `internal/capture` not lower than before (record before/after) |
| QG-8 | yes | +1 Playwright workflow |
| QG-9 | yes | bindings regenerated after `CaptureBackends`/`OpenNpcapDownload` |
| QG-10 | yes | the default build has no `gopacket/pcap` edge (DA-1); `internal/capture` imports no `internal/adapter/*` |
| QG-11 | yes | no temp path, ETL path or packet bytes in errors, history or DTO JSON (`TestPktmonSessionSequence` asserts that error strings contain no `kali-tools-pktmon`) |
| QG-12 | yes | every constant in §4.7 has a limit and limit+1 test or a named pure helper test |

### 12.2 Delivery assertions

| ID | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | Default build never links `gopacket/pcap` | `go list -deps ./cmd/kali-tools \| grep -cx github.com/gopacket/gopacket/pcap` | prints `0` | CI |
| DA-2 | Tagged Windows build is cgo-free | `CGO_ENABLED=0 GOOS=windows go build -tags pcap -o /dev/null ./cmd/kali-tools` | exit 0 | CI |
| DA-3 | No injection call exists | `go test -run TestNoPacketInjectionCalls ./internal/adapter/pcap/` | PASS | CI |
| DA-4 | Npcap is never bundled | `git ls-files \| grep -iE 'npcap.*\.(exe\|msi\|dll\|sys)$'` and the release archive listing in CI | no output | CI, reviewer |
| DA-5 | Pktmon argv is fixed | `go test -run TestPktmonArgv ./internal/adapter/tools/` | PASS | CI |
| DA-6 | Pktmon always stops | `go test -run TestPktmonCancelAlwaysStops ./internal/adapter/tools/` | PASS; stop call count == 1 | CI |
| DA-7 | ADR exists with measurements | `docs/adr/005-windows-capture-backend.md` sections 1–9 present; §4 table filled for B and D | all 9 headings; no "TBD" | reviewer |
| DA-8 [native] | Npcap detection | Procedure: the owner installs Npcap (default options) from npcap.com, runs `kali-tools doctor`, then reinstalls with "Restrict to Administrators" and reruns non-elevated | `available` with version, then `permission_required` | [native] |
| DA-9 [native] | Npcap capture | Procedure: `-tags pcap` desktop build; select the Wi-Fi/Ethernet IPv4; capture 10 s while running `ping -n 20 <gateway>`; press Stop; navigate to Overview | ≥ 20 ICMP rows; `dropped` numeric; stop < 1 s; UI responsive | [native] |
| DA-10 [native] | Pktmon capture and cleanup | Procedure: elevated PowerShell runs `kali-tools capture --backend pktmon --duration 10`, then `pktmon status`, then `Get-ChildItem $env:TEMP -Filter kali-tools-pktmon-*` | packets > 0; no session; no directory | [native] |
| DA-11 [native] | Pktmon refused when not elevated | Procedure: non-elevated `kali-tools capture --backend pktmon --duration 5` | exit 1, `permission_required` | [native] |

For every [native] row, record:

- Windows edition and build;
- NIC model and driver;
- Npcap version and install options;
- elevation;
- date (UTC);
- operator.

Evidence record: `docs/verification/specs/A06.md`.
