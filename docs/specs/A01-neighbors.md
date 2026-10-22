# A01 Passive neighbor inventory

| Field | Value |
| --- | --- |
| PLAN tasks | A01 (passive neighbor inventory from the OS ARP/ND cache) |
| Status | In progress (2026-10-22): steps 1–2 normalization and bounded parsers verified |
| Depends on | H03 (`app.HostNetwork` port), H04 (`app.Application`), H05/H06 (adapters, DTOs), W01 + W04 (HTTP route) |
| Risk | Medium: a hand-written Win32 struct layout (`MIB_IPNET_ROW2`) and a hand-written netlink parser read OS memory and kernel messages |

## 1. Goal

Answer "what is on my LAN" with least privilege, by reading the operating system's existing neighbor cache (ARP for IPv4, Neighbor Discovery for IPv6) and sending no packets.

- On Windows, use `GetIpNetTable2` from `iphlpapi.dll`.
- On Linux, use an `RTM_GETNEIGH` netlink dump, with `/proc/net/arp` (IPv4 only) as a fallback.
- Other operating systems report `unsupported`.

The result is bounded to 4,096 rows, deterministic and exposed through the Wails GUI, the web server and a new CLI command, `kali-tools neighbors`.

## 2. Non-goals

- No active probing: no ARP requests, no NDP solicitations, no ping sweep, no `mdlayher/arp`/`ndp`, no raw sockets.
- No vendor (OUI) lookup database.
- No macOS sysctl implementation (B02). It returns `unsupported`.
- No hostname resolution of neighbor addresses.
- No persistence of addresses or MACs. History stores counts only.
- No change notifications (`NotifyIpInterfaceChange`), polling or live updates. The user refreshes explicitly.
- No modification of the neighbor cache (no `FlushIpNetTable2`, no `CreateIpNetEntry2`).

## 3. Current state

Implementation reconciliation (2026-10-22): H03/H04 already moved Adapter/Snapshot types to app with platform aliases and injected platform.Host. The CLI has an unsupported neighbors stub. References below describe the pre-H03 checkout; use the completed consumer port and driving-adapter locations. x/sys 0.49.0 still has no GetIpNetTable2 binding. A06 automated implementation is complete with native gates pending; owner-authorized headless continuation applies.

Re-verify each fact before starting:

1. `internal/adapter/platform/platform.go:5-14` declares `Adapter{Index, Name, Gateways, DNS}` and `Snapshot{Adapters, Counters}`. There is no neighbor type anywhere in the module (`grep -rn -i neighbor internal` finds nothing).
2. `internal/adapter/platform/platform_windows.go:16-68`: `Inventory()` uses `windows.GetAdaptersAddresses` (buffer capped at 4 MiB, 3 attempts) and `windows.GetIfTable2Ex` plus `windows.FreeMibTable`, with a 65,536-entry cap.
3. `internal/adapter/platform/platform_linux.go:16-77` reads `/etc/resolv.conf`, `/proc/net/route` and `/proc/net/dev` with `os.ReadFile`/`bufio.Scanner`, without size limits.
4. `internal/adapter/platform/platform_other.go:7-9` returns `domain.Unsupported(...)`.
5. `golang.org/x/sys v0.49.0` (`go.mod:9`):
    - has `windows.FreeMibTable`, `windows.GetIpForwardTable2`, `windows.RawSockaddrInet` (`Family uint16; Port uint16; Data [6]uint32`, 28 bytes), `windows.RawSockaddrInet4` and `windows.RawSockaddrInet6`;
    - has **no** `GetIpNetTable2` and no `MibIpNetRow2` type (verified by grep in the module cache).
6. `golang.org/x/sys/unix` (same module) has `unix.NdMsg` (12 bytes), `unix.NDA_DST = 1`, `unix.NDA_LLADDR = 2`, the `unix.NUD_*` states and `unix.NTF_ROUTER = 0x80`. It does **not** have `NetlinkRIB`.
7. The standard library `syscall` on Linux has `NetlinkRIB(proto, family int) ([]byte, error)`, `ParseNetlinkMessage` and `RTM_GETNEIGH`/`RTM_NEWNEIGH`. `syscall.ParseNetlinkRouteAttr` returns `EINVAL` for `RTM_NEWNEIGH`, so the route attributes must be parsed by hand.
8. `app.go:122` and `cmd/kali-tools/main.go:68-69` call `platform.Inventory()` directly (deviation D2). H03 moves this behind `app.HostNetwork`.
9. `frontend/src/app/App.tsx` has an Interfaces page and a discovery table paginated at 100 rows (PLAN A08).

## 4. Design

### 4.1 Domain types (`internal/network/neighbor.go`, new)

```go
package network

const MaxNeighbors = 4096

type Neighbor struct {
    Address        string `json:"address"`         // netip.Addr.String(); IPv6 link-local includes "%<interface name>"
    MAC            string `json:"mac"`             // net.HardwareAddr.String(); "" when unknown/incomplete
    Family         string `json:"family"`          // "ipv4" | "ipv6"
    InterfaceIndex uint32 `json:"interface_index"`
    InterfaceName  string `json:"interface_name"`  // "" when the index no longer exists
    State          string `json:"state"`           // see NeighborState values
    Router         bool   `json:"router"`
}

type NeighborTable struct {
    Neighbors []Neighbor `json:"neighbors"` // never nil
    Limited   bool       `json:"limited"`   // true when more than MaxNeighbors usable rows existed
    Provider  string     `json:"provider"`  // e.g. "Windows GetIpNetTable2", "Linux netlink RTM_GETNEIGH", "Linux /proc/net/arp (IPv4 only)"
}

// Neighbor states (stable strings).
const (
    NeighborReachable   = "reachable"
    NeighborStale       = "stale"
    NeighborDelay       = "delay"
    NeighborProbe       = "probe"
    NeighborIncomplete  = "incomplete"
    NeighborUnreachable = "unreachable"
    NeighborPermanent   = "permanent"
    NeighborUnknown     = "unknown"
)

// ValidateNeighborFamily accepts "all", "ipv4", "ipv6".
func ValidateNeighborFamily(family string) error

// NormalizeNeighbors filters, sorts and bounds raw rows. It is pure.
func NormalizeNeighbors(rows []Neighbor, family string) NeighborTable
```

`NormalizeNeighbors` rules, applied in this order:

1. Drop a row if its `Address` does not parse with `netip.ParseAddr`, or if it is unspecified, loopback or multicast, or `255.255.255.255`.
2. Drop a row whose MAC is `ff:ff:ff:ff:ff:ff` (subnet broadcast entries).
3. Drop a row whose family does not match the filter (`all` keeps both).
4. Remove duplicates by (`InterfaceIndex`, `Address`), keeping the first row.
5. Sort by `InterfaceIndex` ascending, then `ipv4` before `ipv6`, then `netip.Addr.Less`.
6. Keep the first `MaxNeighbors` rows. Set `Limited` if any usable row was cut.

`Neighbors` is `[]Neighbor{}` when empty, never nil.

### 4.2 Port extension (`internal/app`)

H03 declares `HostNetwork` in `internal/app`. Add one method:

```go
type HostNetwork interface {
    // ... existing H03 methods (routes, DNS servers, counters)
    Neighbors(ctx context.Context) (network.NeighborTable, error) // unfiltered except for MaxNeighbors bounding by the adapter
}
```

Use case on `app.Application`:

```go
// Neighbors validates the family, calls HostNetwork.Neighbors and applies NormalizeNeighbors.
func (a *Application) Neighbors(ctx context.Context, family string) (network.NeighborTable, error)
```

The use case is synchronous, with a deadline of 5 s (`context.WithTimeout(ctx, NeighborTimeout)`, `const NeighborTimeout = 5 * time.Second`). It writes no history row: the operation is instant and passive. The context is checked before and after the adapter call, because the OS calls themselves cannot be interrupted.

### 4.3 Windows adapter (`internal/adapter/platform/neighbors_windows.go`)

The concrete type is H03's `HostNetwork` adapter in `internal/adapter/platform` (expected name `platform.Host`). Add `func (Host) Neighbors(ctx context.Context) (network.NeighborTable, error)`. If H03 used another name, add the method to that type and record the deviation.

A minimal `LazyProc` shim, because x/sys has no binding (fact 5):

```go
var procGetIpNetTable2 = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetIpNetTable2")

// mibIpNetRow2 mirrors MIB_IPNET_ROW2 (netioapi.h). Layout is identical on
// amd64, arm64 and 386 because the uint64 LUID falls on offset 32.
type mibIpNetRow2 struct {
    Address               windows.RawSockaddrInet // SOCKADDR_INET, offset 0, 28 bytes
    InterfaceIndex        uint32                  // NET_IFINDEX, offset 28
    InterfaceLuid         uint64                  // NET_LUID, offset 32
    PhysicalAddress       [32]byte                // IF_MAX_PHYS_ADDRESS_LENGTH, offset 40
    PhysicalAddressLength uint32                  // offset 72
    State                 uint32                  // NL_NEIGHBOR_STATE, offset 76
    Flags                 uint8                   // bit0 IsRouter, bit1 IsUnreachable, offset 80
    _                     [3]byte                 // padding, offset 81
    ReachabilityTime      uint32                  // union LastReachable/LastUnreachable (ms), offset 84
}                                                 // sizeof == 88

// mibIpNetTable2 mirrors MIB_IPNET_TABLE2: ULONG NumEntries; then rows aligned to 8.
type mibIpNetTable2 struct {
    NumEntries uint32 // offset 0
    _          uint32 // padding, offset 4
    Table      [1]mibIpNetRow2 // offset 8
}

const (
    errorNotFound = 1168 // ERROR_NOT_FOUND: no entries
    afUnspec      = 0
)
```

Call sequence:

```go
var table *mibIpNetTable2
r, _, _ := procGetIpNetTable2.Call(afUnspec, uintptr(unsafe.Pointer(&table)))
switch {
case r == errorNotFound:
    return network.NeighborTable{Neighbors: []network.Neighbor{}, Provider: providerWindows}, nil
case r != 0:
    return network.NeighborTable{}, &domain.Error{Code: "failed", Message: fmt.Sprintf("GetIpNetTable2 returned %d", r)}
}
defer windows.FreeMibTable(unsafe.Pointer(table))
if table.NumEntries > maxRawNeighborRows { /* 65536 */ return ..., &domain.Error{Code: "failed", Message: "neighbor table exceeds limit"} }
rows := unsafe.Slice(&table.Table[0], int(table.NumEntries))
```

Before the call, check `procGetIpNetTable2.Find()`. On error, return `unsupported`.

Row conversion is a pure function in `internal/adapter/platform/neighbors_windows.go`, testable with synthetic rows:

```go
func neighborFromRow(row *mibIpNetRow2, names map[uint32]string) (network.Neighbor, bool)
```

- `Address.Family == windows.AF_INET` (2): `(*windows.RawSockaddrInet4)(unsafe.Pointer(&row.Address)).Addr` gives `netip.AddrFrom4`. Family is `ipv4`.
- `Address.Family == windows.AF_INET6` (23): `(*windows.RawSockaddrInet6)(unsafe.Pointer(&row.Address)).Addr` gives `netip.AddrFrom16`. If it is link-local unicast, add `.WithZone(names[row.InterfaceIndex])` when the name is non-empty. Family is `ipv6`.
- Any other family gives `ok=false`.
- MAC: if `PhysicalAddressLength` is 0, MAC is `""`. If it is greater than 32, the row is rejected (`ok=false`). Otherwise MAC is `net.HardwareAddr(row.PhysicalAddress[:n]).String()`. A MAC of all zeros becomes `""`.
- State (`NL_NEIGHBOR_STATE`): 0 `unreachable`, 1 `incomplete`, 2 `probe`, 3 `delay`, 4 `stale`, 5 `reachable`, 6 `permanent`, other `unknown`.
- `Router = row.Flags&0x01 != 0`.

Interface names come from one `net.Interfaces()` call per request, building `map[uint32]string` of index to `Name`.

Size checks live in `neighbors_windows_test.go`:

- `unsafe.Sizeof(mibIpNetRow2{}) == 88`;
- `unsafe.Offsetof` for `InterfaceIndex == 28`, `InterfaceLuid == 32`, `PhysicalAddress == 40`, `PhysicalAddressLength == 72`, `State == 76`, `Flags == 80` and `ReachabilityTime == 84`;
- `unsafe.Offsetof(mibIpNetTable2{}.Table) == 8`.

### 4.4 Linux adapter (`internal/adapter/platform/neighbors_linux.go` and `neighbors_netlink.go`)

Pure parser, in a file with **no build tag** so it is unit-tested on every OS:

```go
// neighbors_netlink.go
const (
    maxNetlinkBytes  = 4 << 20 // 4 MiB of dump input
    maxRawNeighborRows = 65536
    nlmsgHeaderLen   = 16
    ndmsgLen         = 12
    rtattrHeaderLen  = 4
    rtmNewNeigh      = 28
    nlmsgDone        = 3
    nlmsgError       = 2
    ndaDst           = 1
    ndaLLAddr        = 2
    afInet           = 2
    afInet6          = 10
    nudIncomplete    = 0x01
    nudReachable     = 0x02
    nudStale         = 0x04
    nudDelay         = 0x08
    nudProbe         = 0x10
    nudFailed        = 0x20
    nudNoARP         = 0x40
    nudPermanent     = 0x80
    ntfRouter        = 0x80
)

// parseNetlinkNeighbors parses a concatenated RTM_NEWNEIGH dump (host byte
// order, little-endian on supported architectures). It never panics on
// malformed input; it returns invalid_input for any length inconsistency.
func parseNetlinkNeighbors(raw []byte, names map[uint32]string) ([]network.Neighbor, error)
```

Parsing rules:

- If `len(raw) > maxNetlinkBytes`, return `invalid_input`.
- Loop over `nlmsghdr{len u32, type u16, flags u16, seq u32, pid u32}`.
  - Reject `len < 16` and `len > remaining`.
  - Advance by `align4(len)`.
  - Stop at `NLMSG_DONE`. Return `failed` at `NLMSG_ERROR`.
  - Skip types other than `RTM_NEWNEIGH`.
- In each `RTM_NEWNEIGH` message:
  - read the `ndmsg` (`family u8, pad u8, pad u16, ifindex i32, state u16, flags u8, type u8`), requiring at least 12 bytes;
  - then loop over the `rtattr` items (`len u16, type u16`), rejecting `len < 4` and `len > remaining`, and advancing by `align4(len)`;
  - `NDA_DST` must be 4 bytes (`afInet`) or 16 bytes (`afInet6`);
  - `NDA_LLADDR` may be 0..32 bytes.
- Skip rows with `state & nudNoARP != 0` (they are not real neighbors).
- State precedence: `permanent` > `reachable` > `stale` > `delay` > `probe` > `incomplete` > `failed`→`unreachable`, else `unknown`.
- `Router = flags & ntfRouter != 0`.
- Stop collecting after `maxRawNeighborRows` rows; the use case bounds to 4,096 afterwards.
- Read integers with `binary.NativeEndian`.

Linux glue, `neighbors_linux.go` (`//go:build linux`):

```go
func (Host) Neighbors(ctx context.Context) (network.NeighborTable, error) {
    raw, err := syscall.NetlinkRIB(syscall.RTM_GETNEIGH, syscall.AF_UNSPEC)
    if err == nil { rows, perr := parseNetlinkNeighbors(raw, names); ... Provider: "Linux netlink RTM_GETNEIGH" }
    // fallback only when netlink itself failed (not when parsing failed)
    return procNetARP("/proc/net/arp", names) // Provider: "Linux /proc/net/arp (IPv4 only)"
}
```

`parseProcNetARP(r io.Reader, names map[string]uint32 /* name→index */) ([]network.Neighbor, error)` is also pure, in the untagged file.

- Read at most 1 MiB (`io.LimitReader(f, 1<<20+1)`). Exceeding it gives `invalid_input`.
- Skip the header line.
- Fields: `IP address`, `HW type`, `Flags` (hex), `HW address`, `Mask`, `Device`.
- Flags `0x0` gives `incomplete`. `0x2` (ATF_COM) gives `unknown`, because reachability is not exposed. Flags containing `0x4` (ATF_PERM) give `permanent`.
- A MAC of `00:00:00:00:00:00` becomes `""`.
- The interface index comes from the `Device` name via the names map (0 if unknown).

### 4.5 Other platforms

`internal/adapter/platform/neighbors_other.go` (`//go:build !windows && !linux`) returns `domain.Unsupported("neighbor cache provider is not implemented on this platform")`.

### 4.6 Capability

Register `neighbors` in `app.Capabilities()`:

- Windows: `available`, provider `windows`, detail "OS ARP/ND cache via GetIpNetTable2; no packets sent".
- Linux: `available`, provider `linux`, detail "netlink neighbor table (procfs IPv4 fallback); no packets sent".
- Other: `unsupported`, detail "Provider is not implemented".

### 4.7 Driving adapters and UI

| Interface | Change |
| --- | --- |
| Wails (`internal/adapter/wailsui`) | `Neighbors(family string) (contract.NeighborTable, error)` |
| HTTP (`internal/adapter/httpapi`) | `GET /api/neighbors?family=all\|ipv4\|ipv6` (default `all`); 400 on an invalid family |
| CLI (`internal/adapter/cli`) | `kali-tools neighbors [--family all\|ipv4\|ipv6] [--format json\|csv]`. JSON uses the standard envelope. CSV header: `schema_version,interface_index,interface_name,family,address,mac,state,router` |
| DTO (`internal/adapter/contract`) | `NeighborTable` and `Neighbor` mirror §4.1, with the same JSON names |

If H05/W01 have not landed, add these to `app.go` / `cmd/kali-tools` until H05 lands and record the HTTP route as a follow-up. Update the CLI usage line (`cmd/kali-tools/main.go:37` or its H05 successor) to list `neighbors`.

Frontend:

- New `frontend/src/features/neighbors/NeighborsTable.tsx`, rendered on the Interfaces page below the interface list:
  - a family `<select>` (All / IPv4 / IPv6) and a Refresh button;
  - a table with the columns Interface, Address, MAC, State, Router;
  - 100 rows per page with the existing pagination pattern;
  - a notice when `limited`;
  - a provider label;
  - the empty state `t.noNeighbors`.
- `bridge.ts`: types `Neighbor` and `NeighborTable`, plus `Neighbors(family: string): Promise<NeighborTable>`.
- i18n keys (both catalogs): `neighbors`, `neighborsIntro` ("Devices your computer recently exchanged traffic with, read from the operating system cache. No packets are sent."), `noNeighbors`, `neighborsLimited`, `family`, `familyAll`, `router`, and one key per state: `state_reachable`, `state_stale`, `state_delay`, `state_probe`, `state_incomplete`, `state_unreachable`, `state_permanent`, `state_unknown`.

### 4.8 Limits

| Limit | Value | Constant |
| --- | --- | --- |
| Rows returned | 4,096 | `network.MaxNeighbors` |
| Raw rows accepted from the OS | 65,536 | `platform.maxRawNeighborRows` |
| Netlink dump size | 4 MiB | `platform.maxNetlinkBytes` |
| `/proc/net/arp` size | 1 MiB | `platform.maxProcARPBytes` |
| MAC length | 32 bytes | `IF_MAX_PHYS_ADDRESS_LENGTH` |
| Use-case deadline | 5 s | `app.NeighborTimeout` |
| UI page | 100 rows | existing pagination |

## 5. Files

Create:

- `internal/network/neighbor.go`: types, `ValidateNeighborFamily`, `NormalizeNeighbors`.
- `internal/network/neighbor_test.go`: normalization tables and bounds.
- `internal/adapter/platform/neighbors_windows.go`: shim, `neighborFromRow`, `Host.Neighbors`.
- `internal/adapter/platform/neighbors_windows_test.go`: layout and synthetic rows; one native smoke test.
- `internal/adapter/platform/neighbors_netlink.go`: pure netlink and procfs parsers (no tag).
- `internal/adapter/platform/neighbors_netlink_test.go`: synthetic dumps, malformed input, procfs.
- `internal/adapter/platform/neighbors_netlink_fuzz_test.go`: `FuzzParseNetlinkNeighbors`.
- `internal/adapter/platform/neighbors_linux.go`: netlink call and procfs fallback.
- `internal/adapter/platform/neighbors_other.go`: unsupported.
- `internal/app/neighbors.go` and `internal/app/neighbors_test.go`: use case with a fake `HostNetwork`.
- `frontend/src/features/neighbors/NeighborsTable.tsx`.

Modify:

- `internal/app/<ports file from H03>`: `HostNetwork.Neighbors`.
- `internal/app/capabilities.go`: `neighbors` capability.
- Driving adapters, contract DTOs, `frontend/src/shared/api/bridge.ts`, `frontend/src/app/App.tsx` (Interfaces page), `frontend/src/shared/i18n/i18n.ts`, `frontend/tests/workflows.spec.ts`.
- `internal/porttest/<hostnetwork suite>`: add the `Neighbors` contract (non-nil slice, sorted, ≤ 4,096, no multicast).
- `scripts/task.mjs`: add `FuzzParseNetlinkNeighbors` to the fuzz smoke target, if the task lists fuzzers explicitly.

Delete: none.

## 6. Implementation steps

1. Add the `network` neighbor types and `NormalizeNeighbors`, test first.
2. Add the pure netlink and procfs parsers with table tests and the fuzz test.
3. Add the Windows shim and `neighborFromRow` with the layout tests. Run `GOOS=windows go vet ./internal/adapter/platform/`.
4. Add the Linux glue and the other-OS fallback. Run `GOOS=linux go vet ./...` and `GOOS=darwin go vet ./...`.
5. Extend `HostNetwork`, the fake and the porttest suite. Add the `Application.Neighbors` use case and the capability.
6. Add the Wails binding, HTTP route, CLI command (JSON and CSV) and DTOs.
7. Frontend: table, bridge, i18n, Playwright.
8. Run the native checks (AC-8, AC-9) and the Docker check (AC-10).
9. Write `docs/verification/specs/A01.md`.

## 7. Acceptance criteria

- **AC-1.** Given rows that include multicast `224.0.0.22`, `255.255.255.255`, a subnet broadcast with MAC `ff:ff:ff:ff:ff:ff`, `::1`, a duplicate and an unparsable address, when `NormalizeNeighbors(rows, "all")` runs, then only unicast, non-duplicate rows remain, sorted by interface, family and address.
- **AC-2.** Given 4,097 valid rows, when normalized, then 4,096 rows are returned and `Limited == true`. Given exactly 4,096 rows, `Limited == false`.
- **AC-3.** Given the `mibIpNetRow2` struct, when the layout test runs on `GOOS=windows`, then the size is 88 and every offset matches §4.3.
- **AC-4.** Given synthetic Windows rows for IPv4 reachable, IPv6 link-local stale with router flag, MAC length 0 and MAC length 33, when converted, then the outputs are respectively:
  - `ipv4`/`reachable` with the MAC;
  - `ipv6` with zone `%<name>`, `stale` and `Router=true`;
  - an empty MAC;
  - rejected.
- **AC-5.** Given a synthetic netlink dump with one IPv4 `REACHABLE` row, one IPv6 `STALE|ROUTER` row, one `NOARP` row and an `NLMSG_DONE`, when parsed, then two neighbors are returned with the correct fields.
- **AC-6.** Given truncated headers, `rtattr.len < 4`, `rtattr.len` beyond the buffer, `NDA_DST` of 5 bytes, or input over 4 MiB, when parsed, then `invalid_input` is returned and the parser does not panic. The fuzz smoke runs 30 s without a crash.
- **AC-7.** Given a `/proc/net/arp` fixture with complete, incomplete and permanent rows, when parsed, then the states are `unknown`, `incomplete` and `permanent`, the family is `ipv4`, and an input over 1 MiB is rejected.
- **AC-8 [native].** Given a Windows 11 host on a LAN, after `ping <gateway>`, when `kali-tools neighbors --family ipv4` runs non-elevated, then the gateway appears with the MAC shown by `Get-NetNeighbor -AddressFamily IPv4 -IPAddress <gateway>` and a matching state, and no packets are sent by kali-tools.
- **AC-9 [native].** Given the same host, when the desktop Interfaces page loads neighbors and the user switches the family to IPv6, then the rows match `Get-NetNeighbor -AddressFamily IPv6`, excluding multicast entries.
- **AC-10.** Given the Docker lab (Linux, `cap_drop: [ALL]`), when `kali-tools neighbors` runs in the `test` container after the integration test has contacted `fixture`, then the exit code is 0, the provider is `Linux netlink RTM_GETNEIGH` and the fixture's IPv4 address is listed.
- **AC-11.** Given macOS (cross-compiled check plus unit test of the fallback), when `Neighbors` is called, then the error code is `unsupported`.

## 8. Tests

| Test | Package | Asserts | ACs |
| --- | --- | --- | --- |
| `TestNormalizeNeighborsFiltersAndSorts` | `internal/network` | Filter/sort/dedupe table | AC-1 |
| `TestNormalizeNeighborsBound` | `internal/network` | 4,096 vs 4,097 rows | AC-2 |
| `TestValidateNeighborFamily` | `internal/network` | `all`/`ipv4`/`ipv6` ok; `""`, `IPv4`, `x` invalid | AC-1 |
| `TestMibIpNetRow2Layout` | `internal/adapter/platform` (windows) | Size and offsets | AC-3 |
| `TestNeighborFromRow` | `internal/adapter/platform` (windows) | Synthetic rows | AC-4 |
| `TestNativeNeighbors` | `internal/adapter/platform` (windows) | Real call returns no error and ≤ 4,096 rows; every row parses (host smoke, not a hardware claim) | AC-8 (partial) |
| `TestParseNetlinkNeighbors` | `internal/adapter/platform` (no tag) | Synthetic dump | AC-5 |
| `TestParseNetlinkNeighborsMalformed` | same | Error table, no panic | AC-6 |
| `FuzzParseNetlinkNeighbors` | same | No panic; output ≤ `maxRawNeighborRows` | AC-6 |
| `TestParseProcNetARP` | same | States, size cap | AC-7 |
| `TestApplicationNeighborsUsesPortAndBounds` | `internal/app` | Fake `HostNetwork`, family filter, deadline, no history write | AC-1, AC-2 |
| `TestNeighborsUnsupportedFallback` | `internal/adapter/platform` (`!windows && !linux`) | Error code | AC-11 |
| `TestCLINeighborsJSONAndCSV` | `internal/adapter/cli` | Envelope, CSV header, exit 2 on a bad family | CLI |
| `TestNeighborsRouteRejectsBadFamily` | `internal/adapter/httpapi` | 400 and error code | HTTP |
| `TestPrivateLabNeighbors` | `tests/integration` (`integration` tag) | AC-10 | AC-10 |
| Playwright `neighbors table paginates and filters by family` | `frontend/tests/workflows.spec.ts` | 205 synthetic rows give 3 pages; family filter calls the bridge with `ipv6` | UI |

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
GOOS=windows GOARCH=amd64 go vet ./internal/adapter/platform/ && GOOS=windows GOARCH=arm64 go vet ./internal/adapter/platform/
go test -run '^$' -fuzz FuzzParseNetlinkNeighbors -fuzztime 30s ./internal/adapter/platform/
bash scripts/setup/setup-linux-docker.sh test
```

## 10. Documentation updates

- `docs/architecture.md`:
  - §4.3: `HostNetwork` row lists `Neighbors`;
  - §2: mention neighbors under `adapter/platform`;
  - §6.3: CLI command list adds `neighbors`.
- `docs/tools-research.md` §4.1(b): replace "Check whether the `x/sys/windows` binding exists… **(verify)**" with "x/sys v0.49.0 has no binding; kali-tools uses a LazyProc shim (A01)".
- `PLAN.md` A01: tick "Passive neighbor inventory…" after AC-8 to AC-10 have evidence.
- `README.md`: CLI command list.

## 11. Risks and rollback

- **Struct layout drift or a wrong offset** reads garbage memory. Mitigated by the size and offset tests, the `PhysicalAddressLength ≤ 32` check and the bounded `NumEntries`. The rows are read only, from the OS-owned allocation, before `FreeMibTable`.
- **Netlink parser bugs.** Mitigated by strict length checks and fuzzing. `NetlinkRIB` returns the entire dump at once and cannot be bounded beforehand, so the 4 MiB check runs after the read. Kernel neighbor tables default to `gc_thresh3 = 1024` entries per family, far below the limit.
- **Privacy.** MAC addresses are personal data on shared networks. They are kept in memory and in explicit exports only, never in history or logs.
- **Rollback.** Revert the commits. No persisted data changes.

## 12. Quality and delivery assertions

### 12.1 Applicable quality gates

| Gate | Applies | Tightened threshold |
| --- | --- | --- |
| QG-1, QG-2, QG-3, QG-4, QG-5, QG-7, QG-13, QG-14, QG-15 | yes | none |
| QG-3 | yes | add `GOOS=windows GOARCH=arm64 go vet ./internal/adapter/platform/` |
| QG-6 | yes | `neighbors_netlink.go` ≥ 90%; `internal/network` not lower than before; `internal/adapter/platform` not lower than before (record before/after) |
| QG-8 | yes | +1 Playwright workflow |
| QG-9 | yes | bindings regenerated |
| QG-10 | yes | `internal/network` imports no `internal/adapter/*` and no `golang.org/x/sys` |
| QG-11 | yes | the history table has no new rows after `Neighbors` (asserted in `TestApplicationNeighborsUsesPortAndBounds`) |
| QG-12 | yes | the limits in §4.8 each have a limit and limit+1 test |

### 12.2 Delivery assertions

| ID | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | Native struct layout pinned | `GOOS=windows go test -run TestMibIpNetRow2Layout ./internal/adapter/platform/` on a Windows CI runner | PASS | CI (windows job) |
| DA-2 | Parser survives fuzzing | `go test -run '^$' -fuzz FuzzParseNetlinkNeighbors -fuzztime 30s ./internal/adapter/platform/` | no crasher | CI, `node scripts/task.mjs fuzz` |
| DA-3 | No packets are sent | `go list -deps ./internal/adapter/platform \| grep -E 'mdlayher/(arp\|ndp)\|gopacket/pcap'` | no output | CI |
| DA-4 | Row bound | `go test -run TestNormalizeNeighborsBound ./internal/network/` | PASS | CI |
| DA-5 | Docker lab lists the fixture | `TestPrivateLabNeighbors` in the Docker lab | PASS | CI (Docker job) |
| DA-6 | CLI contract | `kali-tools neighbors --family bogus; echo $?` | exit code `2`, `error.code == "invalid_input"` | CI (`TestCLINeighborsJSONAndCSV`) |
| DA-7 [native] | Windows matches the OS cache | Procedure: on Windows 11 run `ping -n 1 <gateway>`, then `kali-tools neighbors --family ipv4`, then `Get-NetNeighbor -AddressFamily IPv4 -IPAddress <gateway> \| Format-List` | gateway MAC and state equal | [native] |
| DA-8 [native] | IPv6 view in the GUI | Procedure: desktop Interfaces page with the family set to IPv6, compared with `Get-NetNeighbor -AddressFamily IPv6 \| Where-Object { -not $_.IPAddress.StartsWith('ff') }` | every non-multicast row present | [native] |

For each [native] row, record:

- Windows build;
- adapter model;
- network type (home or lab);
- elevation (expected: not elevated);
- date (UTC);
- operator.

MACs may be partially redacted in the evidence.

Evidence record: `docs/verification/specs/A01.md`.
