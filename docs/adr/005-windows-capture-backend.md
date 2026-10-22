# ADR 005: optional Windows capture backends

## 1. Status

Proposed, 2026-10-22. Implementation baseline: [0b9802b](https://github.com/AlexGalhardo/kali-tools/commit/0b9802b). Automated adapter/interface checks pass; native measurements and the default-backend decision are open. This record does not qualify a release.

## 2. Context

ADR 002's raw IPv4 socket has no Ethernet frames, IPv6 or trustworthy drop counters. A06 evaluates two optional alternatives without distributing a driver or elevating the process. Npcap is separately installed by the user; its free license restricts installation count, redistribution and silent installation. See the [upstream license](https://npcap.com/). Built-in Pktmon requires Windows build 19041 or later and administrator privileges for the capture flow.

## 3. Options

| Option | License/distribution | Privilege | cgo | Link/loss behavior |
| --- | --- | --- | --- | --- |
| A: raw socket | Windows API; x/sys BSD; no extra driver | Administrator | no | Raw IPv4, no reliable drops |
| B: gopacket/pcap | BSD Go wrapper; separately licensed Npcap is never bundled | Installation's AdminOnly setting | no on Windows | Driver link type, native drop counters; read timeout 250 ms |
| C: minimal lazy DLL shim | Project MIT wrapper; separately installed Npcap | Installation dependent | no | Candidate only; not implemented or measured |
| D: Pktmon subprocess | Built-in Windows executable; not redistributed | Administrator | no | All NICs, 512-byte truncation, circular overwrite, duplicates, lost component identity |

Option C would load an absolute System32/Npcap DLL path only on explicit use, avoiding upstream initialization side effects; its export surface and implementation remain a follow-up, not a claim of completed work. Pktmon uses fixed arguments with 15-second start/stop and 60-second conversion deadlines, bounded stdout/stderr and a 48 MB circular ETL. See [Microsoft start syntax](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/pktmon-start) and [conversion syntax](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/pktmon-etl2pcap).

## 4. Measurements

No controlled native capture was run in the headless implementation. The same host must serve a controlled 10 MB HTTP transfer plus 1,000 pings for both candidates.

| Measurement | B: Npcap | D: Pktmon |
| --- | --- | --- |
| Packets captured/sent | Not measured | Not measured |
| Reported drops | Not measured | Not measured |
| Cancel-to-return latency | Not measured | Not measured |
| ETL/PCAPNG size and conversion time | Not applicable | Not measured |
| Peak working set | Not measured | Not measured |
| Link types | Not measured | Not measured |
| Duplicate IPv4 ID + 5-tuple within 1 ms | Not measured | Not measured |

These rows keep DA-7 open. Synthetic fixture counts and cross-compilation cannot substitute for these measurements.

## 5. gopacket/pcap side effects

The pinned Windows implementation initializes its DLL loader and changes the process DLL search directory. The default desktop/CLI dependency graphs exclude that package; only `windows && pcap` imports it. Tagged compilation succeeds with CGO_ENABLED=0. WebView2 startup and legacy-WinPcap behavior have not been executed or qualified. Default detection reads fixed paths, registry values and bounded version resources without loading the driver.

## 6. Pktmon facts requiring native confirmation

| Native observation | Status |
| --- | --- |
| Non-elevated exit code/message | Not run; synthetic capability mapping passes |
| Existing foreign-session start failure | Not run; tests prove no stop after failed start |
| Zero-drop conversion output | Not run; failed/incomplete imports remain unknown |
| Converted mixed link types | Not run; existing reader rejects mismatches |
| Owned-session cancellation/temporary cleanup | Synthetic tests pass; native check open |

An application crash can leave its Pktmon session running in bounded circular mode. A later start failure instructs the user to inspect `pktmon status`; the application never stops a session it cannot prove it started. A failed owned stop reports the explicit manual `pktmon stop` recovery instruction.

## 7. Decision

Retain the existing raw_socket default while qualification is pending. Pktmon requires explicit selection; Npcap requires explicit selection and a `pcap`-tagged build. Neither B nor C is added to default builds. The final alpha backend ordering remains undecided until the native measurements and WebView checks pass.

## 8. Consequences

All interfaces report independent capture_live, capture_pktmon and capture_npcap capability states. The UI disables unavailable options and renders stable translated loss codes; CLI accepts --backend. Npcap guidance opens only the fixed upstream download page in desktop mode, or presents a browser link. No installer is downloaded or launched. Capture payloads remain transient except an explicit export; job history contains only backend/state/counts. Temporary Pktmon files are removed on all tested return paths. Linux/macOS providers remain unsupported; B01/B02 require a separately qualified libpcap/cgo design.

## 9. Revisit triggers

- Npcap license or redistribution terms change.
- gopacket/pcap removes its initialization side effects.
- A bounded pure-Go ETW reader becomes available.
- Pktmon syntax or conversion/link-type behavior changes.
- Native measurements establish an eligible default with acceptable loss and stop latency.
