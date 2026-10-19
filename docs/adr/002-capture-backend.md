# ADR 002: initial Windows capture provider

Status: implemented prototype; native acceptance and performance gate open.

Use a Windows raw IPv4 socket bound to an explicitly selected local address, followed by SIO_RCVALL. This uses the system IP stack through BSD-licensed golang.org/x/sys; it does not redistribute an additional capture driver. Administrator authorization is required. The application does not auto-elevate.

This provider captures IPv4 IP packets, not Ethernet frames, monitor-mode Wi-Fi, or complete IPv6 traffic. It does not expose trustworthy drop counters; `dropped` is null. Sessions are bounded to 60 seconds and 10000 packets, decoded summaries remain in memory, and cancellation closes the socket. The current desktop sends the batch when capture ends. It is a prototype until a controlled native capture/stop test and resource measurements pass.

Npcap is separately licensed and is not bundled or silently treated as an eligible open-source dependency. Nmap/TShark remain separate optional installations. A later driver-backed provider needs a fresh license and distribution review; a raw IPv4 prototype does not prove full Wireshark-equivalent capture.

Offline Go decoding supports PCAP/PCAPNG with preflight length validation, a 64 MiB file limit, a 1 MiB record limit and at most 10000 displayed records. Summaries omit payloads. Explicit PCAP/PCAPNG export is implemented with bounded in-memory raw records, nanosecond timestamps and temporary-file replacement to preserve an existing destination on failure. The retained-byte budget reserves space for framing so exported files remain below 64 MiB. No payload is sent through the bridge or stored in SQLite. Stream reassembly remains pending.

Sources: [Microsoft SIO_RCVALL](https://learn.microsoft.com/en-us/windows/win32/winsock/sio-rcvall), [Microsoft ICMP](https://learn.microsoft.com/en-us/windows/win32/api/icmpapi/nf-icmpapi-icmpsendecho), [gopacket](https://github.com/gopacket/gopacket).
