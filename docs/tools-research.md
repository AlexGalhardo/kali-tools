# Tools research

Research dates: 2026-10-19 (baseline), 2026-10-21 (technical deepening). Scope: defensive, diagnostic and authorized-testing capabilities. Credential attacks, exploitation frameworks, Wi-Fi injection/monitor-mode attacks and evasion remain out of scope (see PLAN.md §9).

Items marked **(verify)** come from secondary sources or conflicting reports. Confirm them against the upstream LICENSE or a native test before relying on them in a decision, release note or license inventory.

## 1. Context recovered

The shared conversation was read in the browser. The requested product combines network monitoring, packet inspection, LAN discovery, Wi-Fi information and an extensible security toolkit on Windows 11, Linux and macOS. The chosen stack is Go + Wails + React + TypeScript + Base UI + Tailwind CSS v4. Go supersedes the earlier Rust recommendation. On 2026-10-21 the owner confirmed three interfaces sharing one Go core: native Wails GUI (primary), a web server serving the same UI, and a complete CLI for headless/SSH use. See [architecture](architecture.md).

Source: https://chatgpt.com/share/6ad626c6-5a54-83e9-837b-064b7a148a55?ogimg=plain

## 2. What Kali tools have underneath

Kali is an integration and distribution ecosystem, not a common application engine. Its catalog points to independently developed tools and packages. Their internals range from OS networking APIs and packet libraries to protocol implementations and subprocess orchestration. A unified desktop must therefore normalize three things instead of assuming a universal Kali API:

- capabilities;
- dependencies;
- results.

Source: https://www.kali.org/tools/

### 2.1 Nmap: scheduling and inference, beyond socket connections

Nmap's scanning engine handles:

- multiple probe types;
- host and port parallelism;
- adaptive round-trip estimates;
- retries;
- congestion control.

A timeout alone does not establish that a port is closed. A small Go TCP-connect scanner is feasible, but it is not feature-equivalent to Nmap's service detection or OS fingerprinting.

Recommendation, as updated on 2026-10-23 (see §3.1 and ADR 006):

- Use native Go for a constrained connect diagnostic.
- Identify a protocol with a banner read, one TLS handshake and one HTTP request. Do not execute Nmap.
- Preserve the provider name and the probe-set revision with each result. Do not store probe bytes.

Source: https://nmap.org/book/port-scanning-algorithms.html

### 2.2 Wireshark/TShark: capture and decoding are separate layers

Capture uses platform facilities through pcap; dissectors interpret the captured bytes. TShark supports live capture and saved files, protocol fields and structured output. Capture filters and display filters differ in syntax and execution cost. Ring files and stop conditions bound disk use.

Recommendation:

- Keep offline PCAP/PCAPNG import and Go decoding as the base.
- Use TShark only for justified decoding depth.
- Avoid implementing hundreds of protocol decoders for 1.0.

Sources:

- https://www.wireshark.org/docs/man-pages/tshark.html
- https://www.wireshark.org/docs/wsdg_html_chunked/ChapterCapture.html

HTTPS payloads remain encrypted unless session secrets are supplied separately. Observing TLS metadata is not the same as reading application content. Packet capture, process attribution and per-process bandwidth accounting each need a different mechanism.

### 2.3 Gobuster: a Go architecture reference

Gobuster is implemented in Go. It separates command modes from reusable execution logic, which shows how a specialized CLI can expose several discovery operations while sharing concurrency and configuration infrastructure. For kali-tools, the following belong in the Go core, shared by every interface:

- bounded workers;
- request deadlines;
- cancellation;
- explicit result models.

Source: https://github.com/OJ/gobuster

### 2.4 iperf3: active throughput measurement

iperf3 uses a client/server model with control and traffic flows, and JSON output for integration. It measures throughput to a selected endpoint, not a universal "internet speed". Interface counters give passive usage, not connection capacity.

Recommendation: an optional adapter with duration and bandwidth limits.

Source: https://software.es.net/iperf/invoking.html

## 3. Findings that change earlier assumptions (2026-10-21)

1. **Nmap license (NPSL 0.95) and the dedicated adapter.**
   - NPSL lists software "designed specifically to execute Covered Software and parse the results" as a derivative-work trigger. It also lists reading `nmap-service-probes`/`nmap-os-db`. The licensor reads "derivative work" broadly, though the annotation says the definition only binds parties that accept the license.
   - The former `InspectServices` Nmap adapter fell into that wording.
   - Owner decision (Alex Galhardo, 2026-10-23): cancel that adapter. Do not accept the clause, request a waiver, or import user-produced Nmap XML. Never bundle Nmap, never execute it, and never read its data files. `Ullaakut/nmap` (MIT) wraps the same executable and does not change the analysis.
   - The replacement is native TCP/banner/TLS inspection (ADR 006). The option comparison is below.
   - Sources: https://nmap.org/npsl/npsl-annotated.html, https://github.com/Ullaakut/nmap
2. **Npcap free edition is not redistributable.**
   - The free license allows up to 5 systems, no external redistribution and no silent installer. The OEM edition is paid.
   - The maintainers advise free projects to ask users to install Npcap themselves.
   - Npcap supports loopback capture (`NPF_Loopback`) and an "Administrators only" install mode.
   - kali-tools must detect Npcap and link to it, never install it. Sources: https://npcap.com/, https://npcap.com/oem/
3. **Windows 11 24H2 location consent gates Wi-Fi metadata.**
   - Without precise-location consent, the following return `ERROR_ACCESS_DENIED`, even when the process is elevated:
     - `WlanGetAvailableNetworkList`;
     - `WlanGetNetworkBssList`;
     - `WlanQueryInterface(wlan_intf_opcode_current_connection)`;
     - `WlanScan`;
     - `netsh wlan show network mode=bssid`.
   - Windows prompts once per app, and only for user-context processes outside `System32`.
   - Microsoft's guidance:
     - check consent with `AppCapability.CheckAccess("wiFiControl")`;
     - deep-link to `ms-settings:privacy-location`;
     - use `WlanConnectionProfileDetails.GetConnectedSsid` when only the SSID is needed;
     - throttle scans.
   - Model a `location_consent_required` capability state in every interface. A web server running under another account may never receive the prompt.
   - Source: https://learn.microsoft.com/en-us/windows/win32/nativewifi/wi-fi-access-location-changes
4. **Pktmon is a built-in Windows capture path that needs no Npcap.**
   - `pktmon.exe` has shipped since build 19041. It supports packet capture, counters and drop reasons, Ethernet/Wi-Fi/MBB, and real-time and circular logging.
   - `pktmon etl2pcap` converts ETL to pcapng, but drops component identity and omits dropped packets unless `--drop-only` is passed. Microsoft's `etl2pcapng` keeps components as pcapng interfaces.
   - Candidate optional adapter: an elevated pktmon session → pcapng → existing `pcapgo.NgReader` import. Elevation required **(verify)**. Use argv arrays only.
   - Sources: https://learn.microsoft.com/en-us/windows-server/networking/technologies/pktmon/pktmon, https://github.com/microsoft/etl2pcapng

### 3.5 Service inspection without Nmap (evaluated 2026-10-23)

The capability to replace is: identify the service or protocol on TCP ports inside an explicit IPv4 /24 or narrower, at most 32 ports, with deadlines and output caps, and without writing targets or probe content to history. SYN scans, OS fingerprinting, brute force, exploit checks and evasion stay out of scope. Licenses were read from the upstream LICENSE files on 2026-10-23.

| Option | License | Decision |
| --- | --- | --- |
| praetorian-inc/fingerprintx v1.1.19 | Apache-2.0. pkg.go.dev also lists BSD-3-Clause, matching the vendored Go crypto tree | Reject |
| zmap/zgrab2 | Apache-2.0 for University of Michigan code, ISC for the Google/BoringSSL-derived files | Reject |
| Go ports that embed or parse `nmap-service-probes` or `nmap-os-db` | Varies; the data files are NPSL | Reject |
| projectdiscovery/naabu | MIT | Reject for this capability |
| masscan | AGPL-3.0 | Reject (already §4.1) |
| Ullaakut/nmap | MIT | Reject |
| Native `net` and `crypto/tls` | Project MIT; no new module | Accept |

fingerprintx is the closest library. It fingerprints application protocols over TCP (and UDP, which this project does not want) and its README states that it does not need the caller to name the protocol. Apache-2.0 can be combined with this MIT project if notices are kept. It is rejected because `go.mod` directly requires `github.com/ory/dockertest/v3` and `github.com/projectdiscovery/wappalyzergo`, and the README documents a `third_party/cryptolib` fork of Go's `crypto/ssh` used to export unexported handshake functions. That is a second SSH stack beside this module's `golang.org/x/crypto` v0.57.0 pin (fingerprintx requires v0.44.0). Fast mode also returns nothing unless the port's conventional service answers, which is the port-number inference this project does not report as identification. Checked sources: https://github.com/praetorian-inc/fingerprintx/blob/main/LICENSE, https://github.com/praetorian-inc/fingerprintx/blob/main/go.mod, https://github.com/praetorian-inc/fingerprintx/blob/main/README.md, https://pkg.go.dev/github.com/praetorian-inc/fingerprintx

zgrab2 is actively maintained and its license is compatible, but it is a survey framework. The caller names the protocol (`zgrab2 http`). The module requires `github.com/zmap/zcrypto`, Prometheus, RabbitMQ, JARM and a flag library. This repository already limits zcrypto to certificate parsing for zlint and records "no legacy handshake client". Importing zgrab2 would expand that use. Checked sources: https://github.com/zmap/zgrab2/blob/master/LICENSE, https://github.com/zmap/zgrab2/blob/v1.0.0/go.mod, https://pkg.go.dev/github.com/zmap/zgrab2

naabu is a port scanner. Its SYN mode needs libpcap or Npcap. Its connect mode repeats native TCP discovery and does not identify the protocol. masscan is AGPL and is built around high-rate SYN scanning. Ullaakut/nmap still executes Nmap, so the NPSL execute-and-parse wording still applies.

The accepted implementation is standard-library TCP, a capped banner classification, one TLS handshake and one HTTP/1.1 request. It runs on Windows, Linux and macOS because it uses `net` and `crypto/tls`. It adds no module, no Nmap code and no Nmap data file. It does not claim Nmap or fingerprintx coverage: a silent port that is neither TLS nor HTTP is reported open and unidentified.

## 4. Capability catalog by category

Each category records:

- (a) representative tools;
- (b) Go building blocks;
- (c) native/adapter/out-of-scope recommendation;
- (d) platform and permission boundaries;
- (e) structured formats useful to adapters.

In the architecture, each category maps to one module under `internal/modules/` (see [architecture](architecture.md)).

### 4.1 Discovery and port/service inventory

**(a) Representative tools.**

| Tool | License | Language | Notes |
| --- | --- | --- | --- |
| Nmap | NPSL 0.95 | C/C++/Lua | See §3.1 |
| masscan | AGPL-3.0 | C | `-oX`, `-oJ`, `-oL` output |
| naabu | MIT | Go | SYN mode needs libpcap/Npcap; CONNECT fallback without root; library runner; JSONL |
| arp-scan | GPL **(verify)** | C | |
| netdiscover | GPL **(verify)** | C | Depends on libpcap and libnet |
| fping, hping3 | BSD-style / GPL-2 **(verify)** | C | |

**(b) Go building blocks.**

- `net.Dialer` with context deadlines: implemented, unprivileged and portable.
- `mdlayher/arp` (MIT): ARP on the local L2 segment. Linux `AF_PACKET`, needs `CAP_NET_RAW`.
- `mdlayher/ndp` (MIT): IPv6 neighbor discovery. Upstream is less active; consider the `antrea.io/ndp` fork.
- `mdlayher/netlink` (MIT): Linux neighbor and route tables.
- Windows `GetIpNetTable2` (iphlpapi): lists the OS ARP/ND cache without raw packets. This is the least-privilege answer to "what is on my LAN". x/sys v0.49.0 has no binding; kali-tools uses a read-only LazyProc shim with pinned layout tests (A01). Gateway/GUI native acceptance remains pending.
- `hashicorp/mdns` (MIT): mDNS/DNS-SD. `grandcat/zeroconf` is stale (last release about 6 years ago).
- `gosnmp` (BSD-2 **(verify)**): read-only system and interface MIB queries against owned devices.
- SSDP/UPnP: an M-SEARCH listener on `net.ListenMulticastUDP`, with no dependency.

**(c) Recommendation.**

- **Native Go:**
  - TCP connect (done);
  - service identification by banner, one TLS handshake and one HTTP request (ADR 006);
  - OS neighbor cache per OS;
  - read-only mDNS/SSDP/SNMP inventory.
- **Not used:** Nmap, under the decision in §3.1. masscan is not recommended (AGPL, scan-rate focus). A port number is not reported as a service.
- **Out of scope:**
  - half-open SYN scanning;
  - OS fingerprinting;
  - service-probe databases, including `nmap-service-probes`.

**(d) Platform and permission boundaries.**

- **Windows:**
  - `SOCK_RAW` requires Administrators.
  - Client SKUs block TCP data over raw sockets and drop spoofed-source UDP.
  - Anything beyond connect probing therefore needs Npcap at Ethernet level.
  - Source: https://learn.microsoft.com/en-us/windows/win32/winsock/tcp-ip-raw-sockets-2
- **Linux:** raw/ARP needs `CAP_NET_RAW`.
- **macOS:** raw sockets need root.

**(e) Structured output.**

- Nmap `-oX`, following the `nmap.dtd` structure: `nmaprun` → `host` → `ports` → `runstats`. Source: https://nmap.org/book/app-nmap-dtd.html
- naabu `-json` writes JSON lines.
- Parse every format with `io.LimitReader` plus streaming decoders.

### 4.2 Packet capture and analysis (read-only)

**(a) Representative tools.**

- Wireshark/TShark (GPLv2+): runs as a separate process only, which keeps the MIT core clean.
- tcpdump/libpcap: BSD **(verify)**.
- Zeek (BSD-3): TSV/JSON logs.
- Suricata: GPLv2, EVE JSON **(verify)**.

**(b) Go building blocks.**

- `github.com/gopacket/gopacket` (BSD-3, already pinned at v1.7.4):
  - the actively maintained community fork of Google's gopacket;
  - `pcap` works with Npcap/libpcap and supports BPF;
  - `pcapgo` provides pure-Go PCAP/PCAPNG `Reader`/`Writer`/`NgReader`/`NgWriter`; its `EthernetHandle` live capture is Linux-only.
  - Sources: https://github.com/gopacket/gopacket, https://pkg.go.dev/github.com/gopacket/gopacket/pcapgo
- `malivvan/gopacket`: purego (cgo-free) libpcap loading. Watch it; do not adopt yet.
- Windows Pktmon → pcapng (§3.4).

**(c) Recommendation.**

- **Native Go:**
  - decoding;
  - PCAP/PCAPNG I/O;
  - flow/conversation summaries;
  - protocol counters;
  - BPF-filtered live capture when Npcap/libpcap exists.
- **Optional adapters:**
  - TShark for dissector depth;
  - Pktmon on Windows;
  - Zeek/Suricata as offline JSON log importers only.
- **Out of scope:** spoofing, ARP poisoning, MITM and anything else that writes to the wire.

**(d) Platform and permission boundaries.**

- **Windows:** Npcap is user-installed and possibly admin-only.
- **Linux:** `CAP_NET_RAW`, plus `CAP_NET_ADMIN` for promiscuous mode.
- **macOS:** `/dev/bpf*` access through Wireshark's ChmodBPF group (`access_bpf`). kali-tools reports BPF access and never changes device permissions.
- **All platforms:**
  - never store payloads in history;
  - bound snaplen, packet count and duration.

**(e) Structured output (TShark).**

| Option | Format |
| --- | --- |
| `-T ek` | Newline-delimited, all values are strings, so map types explicitly |
| `-T fields -e …` | Explicit field list |
| `-T json` | One large array; harder to stream |
| `-T pdml` / `-T psml` | XML detail / XML summary |
| `-G fields` | Field glossary for validating `-e` names at adapter start-up |
| `-G elastic-mapping` | Typed mappings |

Prefer `ek` or `fields` for bounded streaming. Source: https://www.wireshark.org/docs/man-pages/tshark.html

### 4.3 Name resolution and DNS inspection

**(a) Representative tools.**

- dnsrecon, dnsenum and fierce (GPL family **(verify)**).
- `dig`/`host`.
- ProjectDiscovery `dnsx` (MIT, Go):
  - record types A, AAAA, CNAME, PTR, NS, MX, TXT, SRV and SOA;
  - DoH/DoT;
  - JSONL;
  - a library API.

**(b) Go building blocks.**

- Stdlib `net.Resolver`, already used, gives the OS-resolver view.
- `miekg/dns` (BSD-3) has moved to `codeberg.org/miekg/dns` (new API, pre-1.0). Pending decision: pin GitHub v1.1.x short-term or adopt Codeberg. Record the choice in `docs/dependencies.md`.
- DoH: `net/http` plus the wire format. DoT: `crypto/tls` plus `miekg/dns`.

**(c) Recommendation.**

- **Native Go:**
  - system vs explicit resolver comparison;
  - TTL/authority checks;
  - SOA/NS consistency across authoritative servers;
  - read-only DNSSEC chain display;
  - DoH/DoT reachability.
- **Out of scope:** subdomain brute force and third-party zone-transfer attempts. AXFR is only possible later, against a user-declared owned zone.

**(d) Platform and permission boundaries.** No privileges are needed. Report blocked port-53 egress as its own failure class.

**(e) Structured output.**

- dnsx JSONL.
- Align exported field names with RFC 8427 (DNS in JSON) where practical **(verify fit)**.

### 4.4 TLS and certificate auditing

**(a) Representative tools.**

| Tool | License | Language | Notes |
| --- | --- | --- | --- |
| testssl.sh | GPL-2.0 | Bash | Windows only via MSYS2/Cygwin/WSL; JSON/CSV/HTML output |
| SSLyze | AGPL | Python | |
| sslscan | GPL-3 **(verify)** | C | |
| tlsx | MIT | Go | SAN/CN/fingerprints; expired/self-signed/mismatch/revoked detection; version/cipher enumeration; JARM/JA3; JSONL |

**(b) Go building blocks.**

- Stdlib `crypto/tls` + `crypto/x509` cover:
  - handshake, negotiated version and cipher;
  - chain building against the system roots (on Windows, `SystemCertPool` delegates to the platform verifier, so record which verifier produced each verdict);
  - stapled OCSP and SCT presence.
- `zmap/zlint` (Apache-2.0, active): lints certificates and CRLs against:
  - RFC 5280;
  - CA/B Forum BR/EV;
  - ETSI;
  - Mozilla/Apple CT policy.
  It is high-value, low-risk audit content.
- `zmap/zcrypto` (Apache-2.0): a research fork that speaks legacy TLS/export suites. It has no releases and is explicitly not for production security. Isolate it in an adapter used only for observing legacy servers, never for kali-tools' own connections.
- `google/certificate-transparency-go` (Apache-2.0): verifies embedded SCTs. CT-log querying for third-party domains is OSINT and stays optional.
- TLS fingerprints:
  - JA3 (BSD-3) and JA4 client (BSD-3) may be implemented.
  - JA4S/JA4X/JA4H/JA4L/JA4SSH use the FoxIO License 1.1, are patent pending and restrict monetization. They need an owner decision before implementation.

**(c) Recommendation.**

- **Native Go:**
  - chain report: validity, SAN coverage, key and signature algorithms, AIA/OCSP/CRL URLs, stapling, SCT count;
  - protocol support matrix via `MinVersion`/`MaxVersion` per attempt;
  - zlint findings;
  - JA3/JA4 for ClientHellos in imported PCAPs.
- **Optional:**
  - zcrypto legacy probes behind a labelled switch;
  - testssl.sh adapter on Linux/macOS.
- **Out of scope:** exploit checks such as Heartbleed-style memory reads.

**(d) Platform and permission boundaries.** No privileges are needed.

**(e) Structured output.**

- tlsx JSONL.
- testssl.sh JSON/CSV **(verify flag names)**.
- zlint result maps keyed by lint name.

### 4.5 Path, latency and throughput

**(a) Representative tools.**

- mtr (GPL-2 **(verify)**), traceroute, fping.
- iperf3 (BSD-3, ESnet):
  - `-J` JSON output;
  - official builds only for Linux/FreeBSD/macOS; the last official Windows binary is 3.1.3 (2016);
  - current release 3.20 (2025-11).
  - Source: https://software.es.net/iperf/news.html

**(b) Go building blocks.**

- `golang.org/x/net/icmp`: unprivileged `udp4`/`udp6` endpoints on Linux (needs `ping_group_range`) and Darwin only; raw endpoints are privileged.
- `prometheus-community/pro-bing` (MIT **(verify)**): sources conflict on whether Windows needs elevation.
- Windows `IcmpSendEcho2Ex`, already used by the native probe adapter: TTL is set through `IP_OPTION_INFORMATION`. Some Windows 10 setups block ICMP Time Exceeded, so report "no reply", not "host down".
- No maintained Go iperf3-protocol library exists. A native kali-tools-to-kali-tools throughput test is simple but is **not** iperf3-wire-compatible.

**(c) Recommendation.**

- **Native Go:** ping/traceroute/TCP RTT (done).
- **Optional adapter:** iperf3.
- **Out of scope:** flood/stress modes.

**(d) Platform and permission boundaries.** Covered in (b). Unelevated `IcmpSendEcho2` on Windows 10/11 needs a native acceptance test.

**(e) Structured output.**

- iperf3 `-J`: `start`, `intervals[]`, `end`, and `error` on failure **(verify per version)**.
- mtr `--json` **(verify)**.

### 4.6 Wi-Fi metadata (managed mode only)

**(a) Representative tools.**

- Kismet (GPL-2 **(verify)**).
- `iw`, `nmcli`, wavemon on Linux.
- `netsh wlan` on Windows.
- The aircrack-ng suite and monitor-mode injection are excluded.

**(b) Go building blocks.**

- **Windows:** wlanapi, subject to the 24H2 location gate (§3.3).
- **Linux:** `mdlayher/wifi` (MIT, nl80211). Triggering a scan likely needs `CAP_NET_ADMIN`; cached BSS reads likely do not **(verify)**.
- **macOS:** CoreWLAN through cgo.
  - Since Sonoma, SSID/BSSID are nil without Location Services authorization.
  - CLI binaries and LaunchAgents struggle to get that authorization.
  - Some 15.1 reports show empty values even when authorized.
  - Only a signed `.app` with a usage string is realistic.
  - Source: https://developer.apple.com/forums/thread/732431

**(c) Recommendation.**

- **Native:**
  - current connection: SSID, BSSID, band, channel, RSSI, PHY, auth/cipher;
  - visible BSS list;
  - channel overlap view.
- **Consent-state model:** `granted`, `prompt_required`, `denied`, `unsupported`.
- **Key reveal:** stays explicit, current-network-only and never persisted.
- **Out of scope:** monitor mode, deauthentication, handshake capture, cracking.

**(d) Platform and permission boundaries.** As above. Web-server mode under another account may not get consent prompts.

**(e) Structured output.** No standard exists. Define the JSON schema in-repo. Text parsing of `netsh`/`iw` is a last-resort adapter.

### 4.7 HTTP and web metadata (read-only)

**(a) Representative tools.**

- httpx (MIT, Go): status, title, TLS, tech detection, favicon hash; JSONL.
- WhatWeb (GPL-2 **(verify)**).
- Nuclei (MIT): an SDK exists, but its templates are vulnerability-oriented.

**(b) Go building blocks.** `net/http` with a custom `Transport`:

- timeouts;
- `httptrace` for DNS/connect/TLS/TTFB phases;
- a redirect cap;
- `io.LimitReader` bodies.

**(c) Recommendation.**

- **Native Go:**
  - the existing HTTP diagnostic, extended with `httptrace` timings;
  - a security-header presence report (HSTS, CSP, and others);
  - the redirect chain, reported without following it;
  - hand-off to the TLS module.
- **Out of scope:** Nuclei templates, fuzzing and directory brute force.

## 5. Native Go versus external adapters (summary)

| Capability | Approach | Module | Boundary |
| --- | --- | --- | --- |
| Interfaces, IPv4/IPv6, MAC | Go `net` | network | Routes, DNS configuration and counters need platform adapters |
| Routes, counters, neighbors | Platform adapters | platform | `GetIpNetTable2`, netlink, sysctl |
| TCP connect, DNS, HTTP, certificates | Go core | network / future tls | A TLS summary is not a vulnerability audit |
| Ping and traceroute | Platform adapters | probe | Privileges and ICMP behavior differ by OS |
| Passive traffic rate | Counter deltas | network | Resets, interface replacement, monotonic time |
| TCP service identification | Native banner, TLS and HTTP | tools | ADR 006; no Nmap execution or data files |
| Protocol inspection | Go decoding + optional TShark | capture / tools | Bounded output, file size, filters |
| Built-in Windows capture | Optional Pktmon | capture | Elevation; ETL→pcapng conversion limits |
| Wi-Fi metadata and saved key | Native adapter | wifi | Location consent, network type, OS authorization |
| Throughput | Optional iperf3 | tools | Requires a suitable endpoint |
| Certificate linting | zlint | future tls | Apache-2.0; uses the zcrypto parser |

External execution rules:

- Use an executable plus an argument array, never a shell.
- Validate operations and targets in Go.
- Bound runtime, output and concurrency.
- Cancel descendant processes on every OS.
- Record missing, unsupported, permission-denied and consent-required capabilities distinctly.
- Never expose an arbitrary command executor through Wails bindings, HTTP routes or CLI flags.

## 6. Platform feasibility

| Area | Windows 11 | Linux | macOS |
| --- | --- | --- | --- |
| Wi-Fi metadata | WLAN APIs + 24H2 location consent | nl80211 (`mdlayher/wifi`) or NetworkManager D-Bus | CoreWLAN + Location Services; signed `.app` |
| Saved Wi-Fi key | WLAN profile permissions | NetworkManager secret agent authorization | Keychain; feasibility spike required |
| Capture | Npcap (user-installed) or Pktmon | libpcap + `CAP_NET_RAW` | BPF + ChmodBPF group |
| Neighbors | `GetIpNetTable2` | netlink | sysctl/route |
| Native desktop | WebView2 | GTK/WebKit dependencies | System WebKit |

This table is a feasibility map, not a verified support guarantee. Note the following:

- Enterprise Wi-Fi may have no reusable PSK.
- Password retrieval is an explicit action on the current saved network, with OS authorization. It never involves background collection, persistence, telemetry or log output.

Sources:

- https://learn.microsoft.com/en-us/windows/win32/api/wlanapi/nf-wlanapi-wlangetprofile
- https://networkmanager.dev/docs/api/latest/gdbus-org.freedesktop.NetworkManager.Settings.Connection.html
- https://developer.apple.com/documentation/security/keychain-services

Keep the UI and ordinary diagnostics unprivileged. If a privileged helper becomes necessary, it requires:

- authenticated local IPC;
- narrowly typed operations;
- request limits;
- platform lifecycle management;
- its own ADR.

## 7. Cross-cutting adapter guidance

1. **Process isolation and licensing.**
   - A GPL/AGPL tool run as a separate process with argv arrays keeps the MIT core clean.
   - NPSL is not accepted (§3.1). Do not execute Nmap.
   - Never bundle Nmap, Npcap, Wireshark or masscan.
2. **Bounded parsing.**
   - Wrap adapter stdout in `io.LimitReader`.
   - Use streaming decoders: `json.Decoder` for JSONL and `ek`. Do not parse Nmap XML.
   - Apply `exec.CommandContext` deadlines.
   - Send results to the event queue through a bounded channel.
3. **Single capability probe.** One capability service, shared by CLI, desktop and web, reports:
   - Npcap presence and its admin-only flag;
   - Pktmon availability;
   - elevation;
   - location consent;
   - BPF access on macOS;
   - `CAP_NET_RAW` on Linux;
   - TShark and iperf3 versions. Nmap is not inventoried.
4. **Normalized result model.**
   - Map naabu JSONL, TShark `ek`, tlsx JSONL and iperf3 JSON into domain structs, so the UI never depends on adapter schemas. Service inspection already returns the application result directly.
   - Keep raw adapter output in memory, or in an explicit user export only.
5. **Dependency watch list** (record in `docs/dependencies.md`):

| Dependency | Status |
| --- | --- |
| gopacket/gopacket | Active fork, BSD-3 |
| miekg/dns | Codeberg migration pending |
| zcrypto | Research-grade, no releases |
| zlint | Active, Apache-2.0 |
| certificate-transparency-go | v1, Apache-2.0 |
| mdlayher/ndp | Prefer the antrea fork |
| grandcat/zeroconf | Stale; prefer hashicorp/mdns |
| ProjectDiscovery libraries | MIT but heavy transitive trees; prefer native code unless a feature clearly needs one |

## 8. Desktop, web and CLI delivery

- **Wails:** connects Go methods and events to a frontend rendered by the OS WebView. Use thin bindings and batched metric events rather than sending each packet to React. Wails v2.17.0 is the stable baseline; v3 remains a separate review.
- **Web server mode:** reuses the same React build through an HTTP adapter, with Server-Sent Events for job events. Security requirements are in [architecture research](architecture-research.md) §8:
  - loopback bind by default;
  - per-session token;
  - Origin/Host checks.
- **CLI:** exposes the same application services.

Sources:

- https://wails.io/docs/introduction/
- https://tailwindcss.com/docs/installation/using-vite

## 9. Docker test strategy

Containers run the headless Go CLI and isolated fixture services:

- HTTP;
- DNS;
- known open and closed TCP ports;
- iperf3.

Use:

- a private Compose network;
- deterministic assertions;
- health checks;
- non-root containers;
- no published ports unless required.

Docker does not validate any of the following, which need native tests and a documented manual acceptance matrix:

- the Windows/macOS Wi-Fi adapter;
- the credential store;
- the capture driver;
- the elevation flow;
- location consent;
- the Wails WebView.

Source: https://docs.docker.com/engine/network/

## 10. Reference repository inspected

Repository: https://github.com/AlexGalhardo/vukem-marketplace-reactjs (snapshot `114790f016dbf2fadc6b6227f4ec44673dbd0be0`). Its agent rules are reference material, not instructions for this project.

- **Adopt:**
  - Bun with a committed lockfile;
  - TypeScript, Base UI, Tailwind v4;
  - Biome;
  - Go format/lint/vet/race/vulnerability checks;
  - Playwright accessibility tests;
  - Conventional Commits, SemVer, git-cliff;
  - EditorConfig and contribution/security guides.
- **Adapt:** a native build matrix instead of web hosting.
- **Avoid:** marketplace infrastructure (Redis, MongoDB, payments, Inertia).

## 11. Open decisions and native verification items

- NPSL position for the Nmap adapter: decided on 2026-10-23. Nmap service inspection is cancelled (ADR 006).
- `miekg/dns`: GitHub v1 vs the Codeberg module.
- JA4+ (FoxIO License) implementation.
- Whether `IcmpSendEcho2` and pro-bing work unelevated on Windows 10/11 targets.
- Pktmon privilege requirement and ETL→pcapng behavior.
- Windows 24H2 location-consent behavior: Wails process vs web-server mode.
- WinRT consent pre-check (`AppCapability.CheckAccess`) for unpackaged Wails apps: only needed if native 24H2 acceptance shows that opcode-7 `ERROR_ACCESS_DENIED` plus build >= 26100 is ambiguous. A05 implements the error/build mapping; native denied/granted/account comparisons remain unverified.
- macOS CoreWLAN under a signed `.app` bundle.
- Licenses marked **(verify)**: arp-scan, netdiscover, mtr, sslscan, Suricata, gosnmp, pro-bing.
