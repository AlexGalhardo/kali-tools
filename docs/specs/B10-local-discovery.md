# B10 Local service discovery: mDNS, SSDP and SNMP (read-only)

| Field | Value |
| --- | --- |
| PLAN tasks | B10 (mDNS/DNS-SD, SSDP M-SEARCH, SNMP read-only against user-declared owned devices) |
| Status | In progress (2026-10-23); Docker AC-9 is the remaining automated gate |
| Depends on | B09 (`miekg/dns` v1 decision; `hashicorp/mdns` depends on it), H02, H04, H05/H06, W01 + W04 |
| Risk | Medium: multicast behavior differs per OS and Docker network; SNMP credentials are secrets that must never leak |

## 1. Goal

Add a read-only local discovery capability in a new package, `internal/localdiscovery`. It inventories devices that **announce themselves** on the selected local link and reads basic SNMP data from devices the user explicitly declares as owned. It uses:

- mDNS/DNS-SD queries (`github.com/hashicorp/mdns`) for an allowlisted set of service types;
- SSDP `M-SEARCH` with no third-party dependency;
- SNMP v2c/v3 `GET`/`GETBULK` of the system and interface MIBs (`github.com/gosnmp/gosnmp`).

Scope rules match A04 discovery:

- an explicit IPv4 CIDR of /24 or narrower;
- an explicit local interface;
- bounded duration and result counts.

The capability is exposed in the GUI, the web server and the CLI.

## 2. Non-goals

- No SNMP `SET`, no trap receiver, no community or password guessing, no default `public` community, no walk outside the listed OIDs.
- No fetching of SSDP `LOCATION` URLs (device description XML). They are reported only.
- No UPnP control actions (SOAP), no port-mapping queries.
- No mDNS responder or advertising; kali-tools only queries.
- No IPv6 multicast in this spec. IPv4 only, matching A04's IPv4 scope rule.
- No LLMNR, NetBIOS or WS-Discovery.
- No persistence of names, addresses, MACs, communities or SNMP values. History stores counts only.
- No scanning across subnets. Responders outside the scope are dropped.

## 3. Current state

Re-verify each fact before starting:

1. `internal/network/network.go:214-230`: `ValidateDiscovery(cidr, ports, timeout)` requires:
    - an IPv4 prefix of /24 or narrower;
    - 1..32 unique ports;
    - a timeout of 100 ms..5 s.

    `Discover` (`network.go:232-297`) uses 32 workers, 10 ms pacing and a 2-minute cap.
2. `internal/app/service.go:255-271`: `Service.Discover` persists counts only ("Discover persists counts only; addresses and port results remain in memory").
3. `internal/network/network.go:31-51`: `network.Interfaces()` lists the interface index, name, MAC, flags and addresses.
4. `go.mod` has neither `github.com/hashicorp/mdns` nor `github.com/gosnmp/gosnmp`. `golang.org/x/net v0.60.0` (which provides `ipv4.PacketConn`) is already required.
5. Upstream `github.com/hashicorp/mdns` (main, research date 2026-10-21):
    - `go.mod` declares `go 1.26.0`, `github.com/miekg/dns v1.1.73` and `golang.org/x/net v0.61.0`;
    - exported API: `QueryContext(ctx, *QueryParam) error`, `Query(*QueryParam) error`, `DefaultParams(service string) *QueryParam`;
    - `QueryParam{Service, Domain, Timeout, Interface *net.Interface, Entries chan<- *ServiceEntry, WantUnicastResponse, DisableIPv4, DisableIPv6, Logger *log.Logger}`;
    - `ServiceEntry{Name, Host, AddrV4, AddrV6IPAddr, Port, Info, InfoFields}`;
    - sends to `Entries` do **not** block (entries are dropped when nobody reads);
    - `QueryContext` sets `Logger = log.Default()` when nil.
6. Upstream `github.com/gosnmp/gosnmp`: the LICENSE header reads "Copyright 2012-2020 The GoSNMP Authors. All rights reserved. Use of this source code is governed by a BSD-style license", with parts derived from the Go ASN.1 library under the Go BSD license. The exact SPDX id must be confirmed in the review (§4.6).
7. `docker/compose.yml`: `fixture` and `test` share the bridge network `lab` (`internal: true`), with `cap_drop: [ALL]` and the non-root user `65532`.

## 4. Design

### 4.1 Package and types (`internal/localdiscovery`)

```go
package localdiscovery

const (
    MinDurationSeconds = 1
    MaxDurationSeconds = 10
    MaxResponders      = 256
    MaxServiceTypes    = 16 // allowlist (12) + up to 4 user types
    MaxUserServiceTypes = 4
    MaxDatagramBytes   = 8192
    MaxHeaderValue     = 1024
    MaxTXTFields       = 32
    MaxTXTBytes        = 1024
    SSDPSends          = 2
    SSDPSendGap        = 250 * time.Millisecond
    SSDPTTL            = 2
    SNMPPort           = 161
    SNMPMinTimeoutMS   = 500
    SNMPMaxTimeoutMS   = 5000
    SNMPRetries        = 1
    SNMPMaxRepetitions = 16
    SNMPMaxRows        = 256
    SNMPMaxValueBytes  = 512
    MaxSecretBytes     = 256
)

// DefaultServiceTypes is the fixed allowlist queried by mDNS.
var DefaultServiceTypes = []string{
    "_http._tcp", "_https._tcp", "_ipp._tcp", "_printer._tcp", "_airplay._tcp", "_raop._tcp",
    "_googlecast._tcp", "_ssh._tcp", "_smb._tcp", "_workstation._tcp", "_device-info._tcp", "_hap._tcp",
}

type Scope struct {
    Interface string `json:"interface"` // net.Interface.Name; must be up, non-loopback, multicast-capable
    CIDR      string `json:"cidr"`      // IPv4 /24 or narrower, must overlap an IPv4 prefix of Interface
}

type Request struct {
    Scope           Scope    `json:"scope"`
    Protocols       []string `json:"protocols"`              // subset of "mdns","ssdp"; ≥1
    DurationSeconds int      `json:"duration_seconds"`       // 1..10, per protocol
    ServiceTypes    []string `json:"service_types,omitempty"` // extra mDNS types, ≤4, regex ^_[a-z0-9-]{1,15}\._(tcp|udp)$
    SSDPTarget      string   `json:"ssdp_target,omitempty"`  // "ssdp:all" (default) | "upnp:rootdevice"
}

type Responder struct {
    Protocol    string   `json:"protocol"`    // "mdns" | "ssdp"
    Address     string   `json:"address"`     // IPv4
    Port        int      `json:"port,omitempty"`
    Name        string   `json:"name"`        // mDNS instance name or SSDP USN
    Host        string   `json:"host,omitempty"`   // mDNS target host
    ServiceType string   `json:"service_type,omitempty"` // mDNS type or SSDP ST
    TXT         []string `json:"txt,omitempty"`    // mDNS TXT, ≤ MaxTXTFields
    Server      string   `json:"server,omitempty"` // SSDP SERVER header
    Location    string   `json:"location,omitempty"` // SSDP LOCATION, reported only, never fetched
}

type Result struct {
    Responders   []Responder `json:"responders"`     // sorted by address, protocol, name
    Limited      bool        `json:"limited"`
    OutOfScope   int         `json:"out_of_scope"`   // responses dropped because the source was outside Scope.CIDR
    Malformed    int         `json:"malformed"`      // datagrams that failed to parse
    Errors       []string    `json:"errors"`         // per-protocol stable codes, e.g. "mdns_bind_failed"
}

func ValidateScope(s Scope, ifaces []network.InterfaceInfo) error
func Validate(r Request, ifaces []network.InterfaceInfo) error
func Discover(ctx context.Context, r Request) (Result, error)
```

SNMP is a separate operation, because it targets one declared device:

```go
type SNMPRequest struct {
    Scope       Scope        `json:"scope"`
    Target      string       `json:"target"`   // IPv4 inside Scope.CIDR
    Port        int          `json:"port"`     // default 161, 1..65535
    Owned       bool         `json:"owned"`    // must be true: user declares the device is theirs
    Version     string       `json:"version"`  // "v2c" | "v3"
    Community   Secret       `json:"-"`        // v2c; never serialized
    V3          *SNMPv3Creds `json:"-"`        // v3; never serialized
    TimeoutMS   int          `json:"timeout_ms"` // 500..5000
    Interfaces  bool         `json:"interfaces"` // also walk ifTable columns
}

type SNMPv3Creds struct {
    User          string // ≤ 64 bytes; not secret
    AuthProtocol  string // "SHA" | "SHA256" | "SHA512"
    AuthPass      Secret
    PrivProtocol  string // "AES" | "AES256"
    PrivPass      Secret
}

// Secret redacts itself in every formatting path.
type Secret struct{ value string }
func NewSecret(s string) (Secret, error) // 1..MaxSecretBytes, no NUL
func (Secret) String() string            // "[REDACTED]"
func (Secret) GoString() string          // "[REDACTED]"
func (Secret) MarshalJSON() ([]byte, error) // `"[REDACTED]"`
func (s Secret) reveal() string          // unexported; only the gosnmp adapter calls it

type SNMPResult struct {
    Target     string            `json:"target"`
    System     map[string]string `json:"system"`     // keys: sysDescr, sysObjectID, sysUpTime, sysContact, sysName, sysLocation
    Interfaces []SNMPInterface   `json:"interfaces"`
    Limited    bool              `json:"limited"`
}

type SNMPInterface struct {
    Index      int    `json:"index"`
    Descr      string `json:"descr"`
    OperStatus string `json:"oper_status"` // "up" | "down" | "testing" | "unknown" | "dormant" | "notPresent" | "lowerLayerDown"
    SpeedBPS   uint64 `json:"speed_bps"`   // ifHighSpeed*1e6 when present, else ifSpeed
    MAC        string `json:"mac"`         // ifPhysAddress
}

func ValidateSNMP(r SNMPRequest, ifaces []network.InterfaceInfo) error
func QuerySNMP(ctx context.Context, r SNMPRequest) (SNMPResult, error)
```

### 4.2 Scope rules (shared)

`ValidateScope`:

1. `Interface` must exist in `ifaces` and be up, not loopback, and have at least one IPv4 address.
2. `CIDR` must parse with `netip.ParsePrefix`, be IPv4 with `Bits() >= 24`, and use the existing `network.ValidateDiscovery` prefix rule (/24 or narrower).
3. `CIDR` must overlap at least one IPv4 prefix of `Interface` (`prefix.Overlaps(ifacePrefix)`). Otherwise the error is `invalid_input` "scope must be on the selected interface's link".

Every response whose source IPv4 is not in `CIDR` increments `OutOfScope` and is discarded, never stored.

### 4.3 mDNS (`mdns.go`)

- For each service type (`DefaultServiceTypes` plus validated `ServiceTypes`, at most `MaxServiceTypes`), run sequentially:

    ```go
    entries := make(chan *mdns.ServiceEntry, MaxResponders)
    params := &mdns.QueryParam{
        Service: t, Domain: "local", Timeout: perTypeTimeout,
        Interface: iface, Entries: entries,
        WantUnicastResponse: true, DisableIPv6: true,
        Logger: log.New(io.Discard, "", 0),
    }
    err := mdns.QueryContext(ctx, params)
    ```

  `perTypeTimeout = max(200ms, Duration / number of types)`. Drain `entries` after `QueryContext` returns (non-blocking loop). `Logger` must be set: the library otherwise logs to `log.Default()`.
- Convert each entry:
  - `Address = AddrV4.String()`, which must be in scope;
  - `Port`;
  - `Name` truncated to 255 bytes;
  - `Host` truncated to 255 bytes;
  - `ServiceType = t`;
  - `TXT = InfoFields`, capped at `MaxTXTFields`, each truncated to `MaxTXTBytes`.
- Deduplicate by (`Protocol`, `Address`, `Name`).
- A bind failure (the library returns an error such as "failed to bind") appends `mdns_bind_failed` to `Errors`. This is not fatal, because SSDP still runs.

### 4.4 SSDP (`ssdp.go`, no dependency)

1. Resolve the interface's first IPv4 address in scope as `src`. Open `net.ListenUDP("udp4", &net.UDPAddr{IP: src, Port: 0})`. Wrap it with `ipv4.NewPacketConn(conn)` and call `SetMulticastInterface(iface)` and `SetMulticastTTL(SSDPTTL)`.
2. Send the request below `SSDPSends` times, `SSDPSendGap` apart, to `239.255.255.250:1900`. The text is fixed except `ST` (from the allowlist) and the version:

    ```text
    M-SEARCH * HTTP/1.1\r\n
    HOST: 239.255.255.250:1900\r\n
    MAN: "ssdp:discover"\r\n
    MX: <min(DurationSeconds,5)>\r\n
    ST: <ssdp:all|upnp:rootdevice>\r\n
    USER-AGENT: kali-tools/<domain.Version> UPnP/1.1\r\n
    \r\n
    ```

3. Read until `Duration` elapses or the context is done, with `SetReadDeadline` steps of 250 ms. The buffer is `MaxDatagramBytes`; a larger datagram counts as `Malformed`.
4. Parse with `http.ReadResponse(bufio.NewReader(bytes.NewReader(b)), nil)`. The status must be 200; otherwise count it as `Malformed`. Take `ST`, `USN`, `SERVER` and `LOCATION`, each truncated to `MaxHeaderValue`. `LOCATION` must parse as an `http`/`https` URL, otherwise it is dropped (field empty). **Never request `LOCATION`.**
5. Deduplicate by `USN`. Set `Name = USN`, `ServiceType = ST`, and `Address` to the source IP, which must be in scope.

### 4.5 SNMP (`snmp.go`)

`ValidateSNMP`:

- the scope is valid;
- `Target` parses as IPv4 inside `CIDR`;
- `Owned == true`, else `invalid_input` "confirm that you own or administer this device";
- `Version` is `v2c` with a non-empty `Community`, or `v3` with non-empty `User` and `AuthPass`;
  - `PrivPass` is optional, but when present `PrivProtocol` is required;
  - protocols come from the allowlist.
- `TimeoutMS` is in range.

The `noAuthNoPriv` mode is not allowed for v3.

Query, through `gosnmp`:

```go
g := &gosnmp.GoSNMP{
    Target: r.Target, Port: uint16(r.Port), Version: gosnmp.Version2c, Community: r.Community.reveal(),
    Timeout: time.Duration(r.TimeoutMS) * time.Millisecond, Retries: SNMPRetries,
    MaxOids: 6, MaxRepetitions: SNMPMaxRepetitions, Context: ctx,
    Logger: gosnmp.NewLogger(log.New(io.Discard, "", 0)),
}
// v3: Version3, SecurityModel UserSecurityModel, MsgFlags AuthPriv/AuthNoPriv,
//     SecurityParameters &gosnmp.UsmSecurityParameters{UserName, AuthenticationProtocol, AuthenticationPassphrase, PrivacyProtocol, PrivacyPassphrase}
err := g.Connect(); defer g.Conn.Close()
pkt, err := g.Get([]string{".1.3.6.1.2.1.1.1.0", ".1.3.6.1.2.1.1.2.0", ".1.3.6.1.2.1.1.3.0",
    ".1.3.6.1.2.1.1.4.0", ".1.3.6.1.2.1.1.5.0", ".1.3.6.1.2.1.1.6.0"})
```

- When `Interfaces` is set, run `g.BulkWalk` on each of these columns, stopping after `SNMPMaxRows` rows per column via a sentinel error:
  - `.1.3.6.1.2.1.2.2.1.2` (ifDescr);
  - `.1.3.6.1.2.1.2.2.1.8` (ifOperStatus);
  - `.1.3.6.1.2.1.2.2.1.5` (ifSpeed);
  - `.1.3.6.1.2.1.2.2.1.6` (ifPhysAddress);
  - `.1.3.6.1.2.1.31.1.1.1.15` (ifHighSpeed, optional).

  Join the rows by index. Set `Limited` when any column was cut.
- Values are converted to strings truncated to `SNMPMaxValueBytes`. `OctetString` values that are not valid UTF-8 are hex-encoded. `sysUpTime` is rendered as a duration.
- Only `Connect`, `Get`, `BulkWalk` and `Conn.Close` are used. A guard test forbids `Set`, `SendTrap`, `Walk` without a bound, and `WalkAll`.
- Errors map to stable codes with fixed messages that never contain gosnmp error text, because it can echo the request:
  - a timeout gives `timeout` "SNMP agent did not answer";
  - `authorizationError`, `noAccess` or a v3 report on an unknown user or wrong digest gives `unauthorized` "SNMP credentials were rejected";
  - anything else gives `failed` "SNMP query failed".

Secrets:

- `Community`, `AuthPass` and `PrivPass` are `Secret` values;
- they are excluded from JSON (`json:"-"`) and redact themselves in fmt;
- they are zeroed (`r.Community = Secret{}`) after `Connect` returns.

DTOs carry them only on the request path:

- `contract.SNMPRequest.Community string`, write-only;
- the HTTP handler decodes it and converts it immediately to `Secret`;
- the request is never logged.

### 4.6 Dependency and license review (before §6 step 2)

1. `go get github.com/hashicorp/mdns@<latest tag or main pseudo-version>` and `go get github.com/gosnmp/gosnmp@<latest v1 tag>`. Record the exact versions.
2. Note that `hashicorp/mdns` requires `golang.org/x/net v0.61.0`. Accept the bump of the pinned `x/net` (0.60.0 → ≥ 0.61.0) and re-run `govulncheck ./...` and `govulncheck -tags desktop .`. Update the pin line in `docs/dependencies.md`.
3. Open each LICENSE in the module cache:
    - `hashicorp/mdns` (expected MIT; verify — HashiCorp relicensed some projects to BUSL, so a non-MIT license stops the spec);
    - `gosnmp/gosnmp` (expected BSD-2-Clause or BSD-3-Clause, plus Go-derived BSD-3; confirm the SPDX id from the full text).

    `miekg/dns` was reviewed in B09.
4. Run `go mod why -m` for each module and list any new transitive modules. Stop on any non-permissive license.
5. Record the rows in `docs/dependencies.md` (§10) and resolve the "gosnmp (BSD-2 **(verify)**)" note in `docs/tools-research.md` §4.1.

### 4.7 Application, adapters and UI

```go
// internal/app
func (a *Application) DiscoverLocal(ctx context.Context, r localdiscovery.Request) (localdiscovery.Result, error)
func (a *Application) QuerySNMP(ctx context.Context, r localdiscovery.SNMPRequest) (localdiscovery.SNMPResult, error)
```

- Both run as operation jobs, with kinds `local_discovery` ("Local discovery") and `snmp` ("SNMP query"), under the 8-operation cap.
- Summaries:
  - `"Local discovery <state>: <n> responders, <o> out of scope"`;
  - `"SNMP query <state>: <k> interfaces"`.
- Both validate against `network.Interfaces()` (passed in as a slice; the capability package does not call the OS for validation).
- Capabilities:
  - `local_discovery`: `available` on all OSes, detail "mDNS and SSDP on one interface, IPv4 /24 or narrower; read-only";
  - `snmp`: `available`, detail "SNMP v2c/v3 GET/GETBULK of system and interface MIBs on owned devices".

| Interface | Change |
| --- | --- |
| Wails | `DiscoverLocal(r contract.LocalDiscoveryRequest)`, `QuerySNMP(r contract.SNMPRequest)` |
| HTTP | `POST /api/discovery/local` (body ≤ 4 KiB); `POST /api/snmp` (body ≤ 4 KiB; the handler never logs the body) |
| CLI | `kali-tools local-discover --interface <name> --scope <cidr> [--protocols mdns,ssdp] [--duration 3] [--service-types _foo._tcp,…] [--ssdp-target ssdp:all]` |
| CLI | `kali-tools snmp --interface <name> --scope <cidr> --target <ip> --owned [--port 161] [--version v2c\|v3] [--secret-file <path>] [--v3-user u --auth SHA256 --priv AES] [--interfaces] [--timeout 2000]` |
| DTO | Mirrors §4.1; `SNMPRequest` secret fields are request-only |

CLI secret input, so no secrets appear in argv or shell history:

- `--secret-file` names a regular file of at most 4 KiB, owned by the user.
- Line 1 is the v2c community, or the v3 auth passphrase.
- Line 2 (v3 only, optional) is the privacy passphrase.
- Lines are trimmed of `\r\n` and limited to `MaxSecretBytes`.
- A `--community` or `--password` flag does **not** exist. If one is passed, the stdlib flag error gives exit 2.
- Environment variables are not read (H01 rule).

If H05/W01 have not landed, add these to `app.go` / `cmd/kali-tools` until H05 lands and record the HTTP routes as follow-ups.

Frontend: `frontend/src/features/localdiscovery/LocalDiscoveryPanel.tsx` on the "Discovery & tools" page. It contains:

- an interface `<select>` fed by `Interfaces()`;
- the scope CIDR, protocol toggles and duration;
- a results table (Protocol, Address, Name, Type, Server/TXT, Location as plain text, not a link), 100 rows per page;
- counters for out-of-scope and malformed responses.

`SNMPPanel.tsx` contains:

- the target, version, a password-type input for the community/passphrases (`autocomplete="off"`);
- a required "I own or administer this device" checkbox;
- the system table and the interface table.

The secret inputs are cleared after submit and on navigation, as with the Wi-Fi reveal.

i18n keys (both catalogs): `localDiscovery`, `snmp`, `scopeInterface`, `scopeCidr`, `protocols`, `serviceTypes`, `ssdpTarget`, `responders`, `outOfScope`, `malformed`, `ownedConfirm`, `snmpVersion`, `community`, `authPass`, `privPass`, `snmpSystem`, `snmpInterfaces`, `mdns_bind_failed`, `ssdp_bind_failed`.

### 4.8 Lab fixtures

Unit tests (no multicast needed):

- `ssdp_test.go` exercises the parser with raw datagrams. A loopback test sends to a local UDP socket by overriding the destination through an unexported `ssdpDest` field (default `239.255.255.250:1900`).
- `mdns_test.go` exercises entry conversion and scope filtering with synthetic `*mdns.ServiceEntry` values.
- `snmp_test.go` uses an in-process fake SNMP v2c agent on `127.0.0.1:0`. It decodes requests with `gosnmp.GoSNMP.SnmpDecodePacket` and answers with `MarshalMsg`, covering system values, 300 ifTable rows (bound) and a wrong community (`noSuchName`/timeout → `timeout`).

Docker lab (`integration` tag, `tests/integration/localdiscovery_test.go`):

- `cmd/lab-fixture` adds an SSDP responder on the `fixture` container:
  - it joins `239.255.255.250` on its `eth0` with `ipv4.PacketConn.JoinGroup` on UDP 1900;
  - **1900 is ≥ 1024, so a non-root bind works**;
  - it answers `ST: upnp:rootdevice` with `USN: uuid:kali-tools-lab::upnp:rootdevice` and `LOCATION: http://<fixture-ip>:8080/description.xml`.
- It adds an mDNS responder using `mdns.NewServer` for `_kali-lab._tcp` (instance `fixture`, port 8080). mDNS port 5353 conflicts with the existing UDP DNS fixture on `:5353`, so **move the existing plain DNS fixture to `:5300`** and update `--health`, `tests/integration/lab_test.go` and the B09 references in the same change.
- It adds an SNMP v2c agent on UDP 1161 serving the fixed system MIB values (community from a fixture-only constant `lab-community`), using the same fake agent code as the unit test, moved to `internal/localdiscovery/snmpfake` (test-only package, imported by the fixture).
- The test, run in the `test` container:
  - discovers on `eth0` with scope = the lab subnet /24;
  - asserts the SSDP USN and the mDNS instance are present;
  - asserts SNMP `sysName == "kali-tools-lab"` with `Port: 1161`.
- If multicast does not traverse the lab bridge (Docker bridges normally flood multicast, but IGMP snooping can interfere), the test fails with a clear message. It is **not** skipped. Record the result; if it fails for infrastructure reasons, the spec stays In progress until the lab is fixed.

## 5. Files

Create:

- `internal/localdiscovery/localdiscovery.go`: types, constants, `Validate`, `ValidateScope`, `Discover`.
- `internal/localdiscovery/mdns.go`, `ssdp.go`, `snmp.go`, `secret.go`.
- `internal/localdiscovery/*_test.go`: unit tests; `guard_test.go` forbids the SNMP write APIs and `LOCATION` fetching.
- `internal/localdiscovery/snmpfake/agent.go`: fake v2c agent for tests and the lab.
- `cmd/lab-fixture/ssdp.go`, `cmd/lab-fixture/mdns.go`, `cmd/lab-fixture/snmp.go`.
- `tests/integration/localdiscovery_test.go`.
- `internal/app/localdiscovery.go` and `internal/app/localdiscovery_test.go`.
- `frontend/src/features/localdiscovery/LocalDiscoveryPanel.tsx`, `frontend/src/features/localdiscovery/SNMPPanel.tsx`.

Modify:

- `go.mod`, `go.sum`: `hashicorp/mdns`, `gosnmp/gosnmp`, `x/net` bump.
- `cmd/lab-fixture/main.go`: DNS port 5353 → 5300; new listeners; health.
- `tests/integration/lab_test.go`: DNS port 5300.
- `internal/app/capabilities.go`, driving adapters, DTOs, `bridge.ts`, `App.tsx`, `i18n.ts`, `workflows.spec.ts`.
- `docs/dependencies.md`, `docs/architecture.md`, `docs/tools-research.md`, `PLAN.md`, `README.md`.

Delete: none.

## 6. Implementation steps

1. Run the dependency and license review (§4.6). Stop on a non-permissive license.
2. Write the types, scope validation and the bound tests.
3. Write the SSDP sender and parser with loopback tests.
4. Write the mDNS wrapper with conversion tests, plus the guard on `Logger`.
5. Write `Secret` and SNMP with the fake agent, and the redaction tests.
6. Add the app use cases, capabilities, adapters and DTOs. The CLI uses `--secret-file`.
7. Lab fixtures: move the DNS port, add SSDP/mDNS/SNMP, then the integration test. Run the Docker lab.
8. Frontend panels, i18n, Playwright.
9. Update the docs.
10. Write `docs/verification/specs/B10.md`.

## 7. Acceptance criteria

- **AC-1.** Given scopes that are invalid (/23, IPv6, an interface that is down or loopback, a CIDR not overlapping the interface), when validated, then `invalid_input`. Given /24 and /32 on the interface's link, they are valid.
- **AC-2.** Given SSDP datagrams (a valid 200 response, a 404, garbage, 8,193 bytes, a source outside the scope, a duplicate USN), when parsed and filtered, then the result holds 1 responder, `Malformed == 3` and `OutOfScope == 1`, and `LOCATION` is reported but no HTTP request is made (the test server counts 0 requests).
- **AC-3.** Given synthetic mDNS entries, including one outside the scope and one with 40 TXT fields, then the out-of-scope entry is dropped and the TXT list is capped at 32.
- **AC-4.** Given 300 responders, then 256 are returned and `Limited=true`. Given exactly 256, `Limited=false`.
- **AC-5.** Given the fake SNMP agent, when `QuerySNMP` runs with the correct community and `Interfaces=true`, then the 6 system keys are filled and the interfaces are capped at 256 with `Limited=true` (the agent serves 300).
- **AC-6.** Given a wrong community (the agent drops the request), then `timeout`. Given a v3 wrong passphrase (the agent returns a `usmStatsWrongDigests` report), then `unauthorized`. In both cases the error JSON, the job summary and `fmt.Sprintf("%+v", req)` contain neither `synthetic-community` nor `synthetic-pass`.
- **AC-7.** Given `Owned=false`, then `invalid_input` with the ownership message, and no packet is sent (the fake agent counts 0 requests).
- **AC-8.** Given the package sources, when the guard test parses them, then there are no calls to `Set`, `SendTrap`, `WalkAll` or `Walk`, and no `http.Get`/`http.NewRequest` in `ssdp.go`.
- **AC-9.** Given the Docker lab, when `TestPrivateLabLocalDiscovery` runs, then the SSDP USN `uuid:kali-tools-lab::upnp:rootdevice`, the mDNS instance `fixture._kali-lab._tcp.local.` and SNMP `sysName` are found.
- **AC-10.** Given `kali-tools snmp ... --community x` (a nonexistent flag), then exit 2. Given `--secret-file` pointing at a 5 KiB file, `invalid_input`.
- **AC-11 [native].** Given the owner's Windows 11 host on the home LAN, when Local discovery runs on the Wi-Fi interface with the home /24 for 5 s, then at least one known device (for example a printer or a cast device) appears via mDNS or SSDP, and nothing outside the /24 is shown.
- **AC-12 [native].** Given an owned SNMP-enabled device (for example the home router with SNMP enabled by the owner), when `kali-tools snmp --owned --secret-file …` runs, then `sysDescr` and the interfaces are listed, and the secret file content appears in no output or log.

## 8. Tests

| Test | Package | Asserts | ACs |
| --- | --- | --- | --- |
| `TestValidateScope` | `internal/localdiscovery` | AC-1 | AC-1 |
| `TestSSDPParseAndFilter` | same | AC-2 | AC-2 |
| `TestSSDPLoopbackRoundTrip` | same | Real UDP via the `ssdpDest` override; M-SEARCH bytes match the template exactly | AC-2 |
| `TestMDNSEntryConversion` | same | AC-3 | AC-3 |
| `TestResponderBound` | same | AC-4 | AC-4 |
| `TestSNMPSystemAndInterfaces` | same | AC-5 | AC-5 |
| `TestSNMPErrorsAndRedaction` | same | AC-6 | AC-6 |
| `TestSNMPRequiresOwnership` | same | AC-7 | AC-7 |
| `TestSecretRedaction` | same | `%v`, `%+v`, `%#v` and `json.Marshal` all print `[REDACTED]` | QG-11 |
| `TestReadOnlyGuard` | same | AC-8 | AC-8 |
| `TestPrivateLabLocalDiscovery` | `tests/integration` | AC-9 | AC-9 |
| `TestCLISNMPSecretFile` | `internal/adapter/cli` | AC-10 | AC-10 |
| `TestSNMPRouteNeverLogsBody` | `internal/adapter/httpapi` | A captured logger has no community | QG-11 |
| Playwright `local discovery requires scope and SNMP requires ownership` | `frontend/tests/workflows.spec.ts` | Disabled submit without the checkbox; secret field cleared after submit | UI |

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
go test -race ./internal/localdiscovery/...
go vet -tags integration ./...
bash scripts/setup/setup-linux-docker.sh test
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

## 10. Documentation updates

- `docs/dependencies.md`:
  - rows for `github.com/hashicorp/mdns` (verified SPDX, mDNS queries, logger silenced);
  - `github.com/gosnmp/gosnmp` (verified SPDX, SNMP read-only);
  - the updated `x/net` pin sentence.
- `docs/tools-research.md`:
  - §4.1(b): replace "gosnmp (BSD-2 **(verify)**)" with the verified license;
  - §11: remove gosnmp from the "Licenses marked (verify)" list.
- `docs/architecture.md`: add `internal/localdiscovery`; note the lab DNS port 5300.
- `PLAN.md` B10: tick after AC-1 to AC-10 pass and AC-11/AC-12 have native evidence or are explicitly "not run".
- `README.md`: CLI list adds `local-discover` and `snmp`, with the `--secret-file` guidance.

## 11. Risks and rollback

- **HashiCorp license change.** Verified in §4.6 before any code. If `hashicorp/mdns` is no longer MIT, stop and replace it with a small query client on `miekg/dns` (a follow-up spec).
- **Multicast in Docker or Windows.** Windows' own mDNS service may hold UDP 5353. `WantUnicastResponse` avoids needing that port, and a bind failure is reported as `mdns_bind_failed`, never faked.
- **Credential leakage.** Mitigated by the `Secret` type, `json:"-"`, fixed error messages, the discarded gosnmp logger, `--secret-file` instead of argv, and the redaction tests.
- **Lab DNS port move** (5353 → 5300) touches A03 integration assertions. It is done in the same commit, and the lab test proves it.
- **Rollback.** Revert the commits, run `go mod tidy`, and restore the lab DNS port.

## 12. Quality and delivery assertions

### 12.1 Applicable quality gates

| Gate | Applies | Tightened threshold |
| --- | --- | --- |
| QG-1, QG-2, QG-3, QG-4, QG-5, QG-9, QG-13, QG-14, QG-15 | yes | none |
| QG-6 | yes | `internal/localdiscovery` ≥ 85% statements |
| QG-7 | yes | govulncheck after the `x/net` bump: no reachable vulnerability |
| QG-8 | yes | +1 Playwright workflow |
| QG-10 | yes | `internal/localdiscovery` imports no `internal/adapter/*` or `internal/app` |
| QG-11 | yes | synthetic `synthetic-community` / `synthetic-pass` absent from history, logs, errors and every DTO JSON (`TestSNMPErrorsAndRedaction`, `TestSecretRedaction`, `TestSNMPRouteNeverLogsBody`) |
| QG-12 | yes | every constant in §4.1 has a limit and limit+1 test |

### 12.2 Delivery assertions

| ID | Assertion | Command, test or artifact | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | Licenses verified | `docs/dependencies.md` rows for mdns and gosnmp with SPDX ids; "(verify)" removed for gosnmp in tools research | present | reviewer |
| DA-2 | Read-only SNMP | `go test -run TestReadOnlyGuard ./internal/localdiscovery/` | PASS | CI |
| DA-3 | No secrets in argv | `kali-tools snmp --help 2>&1 \| grep -ciE 'community\|password'` | `0` flag names (only `--secret-file`) | CI (`TestCLISNMPSecretFile`) |
| DA-4 | Scope enforced | `go test -run 'TestValidateScope\|TestSSDPParseAndFilter' ./internal/localdiscovery/` | PASS | CI |
| DA-5 | Lab inventory matches fixtures | `TestPrivateLabLocalDiscovery` in the Docker job | PASS (3 findings) | CI (Docker) |
| DA-6 | Logger silenced | `grep -n "Logger:" internal/localdiscovery/mdns.go internal/localdiscovery/snmp.go` | both set to an `io.Discard` logger | reviewer |
| DA-7 [native] | Home LAN discovery | Procedure: desktop, Discovery & tools → Local discovery, choose the Wi-Fi interface and its /24, mDNS+SSDP, 5 s | ≥ 1 responder; all addresses inside the /24 | [native] |
| DA-8 [native] | Owned SNMP device | Procedure: enable SNMP v2c or v3 on an owned router, write the secret to a file, run `kali-tools snmp --owned --secret-file <f> …`, then search the output, `%APPDATA%` logs and the history DB for the secret | `sysDescr` present; secret found nowhere | [native] |

For [native] rows, record:

- OS build;
- adapter;
- device model and firmware (SNMP);
- SNMP version;
- date (UTC);
- operator.

Never record the community or passphrase.

Evidence record: `docs/verification/specs/B10.md`.
