# Capture decoding

Offline import and live capture share `capture.Decode`. Each packet is decoded on its own. TCP streams are not reassembled. IPv6 fragments are not reassembled. The packet payload is omitted from the summary and from history.

## Supported levels

- Link: Ethernet or raw IP, as recorded in the file or by the live backend.
- Network: IPv4 and IPv6. A complete IPv6 hop-by-hop header is followed to the next header. An IPv6 fragment stays a fragment; its payload is not decoded as UDP or as any other protocol.
- Transport: TCP, UDP, ICMPv4 and ICMPv6. The summary is the flow or the ICMP type.
- DNS: question names from one packet, stopped after 512 bytes of summary text. A truncated DNS packet does not show the question name.
- HTTP: a plaintext method is recognized from the first 8 bytes of one TCP segment when those bytes are `GET`, `POST`, `HEAD`, `PUT`, `DELETE`, `OPTIONS`, `PATCH`, `CONNECT`, `TRACE`, or `HTTP/` followed by a space. The summary is `Plaintext HTTP message (payload omitted)`. Port 443 adds `Port 443 (encryption not verified)`.

TShark is inventory-only. It is not run for `-T ek`, `-T fields`, or any other decode ([ADR 006](adr/006-native-service-inspection.md)).

## Truncation

A packet is truncated when the original length is greater than the captured bytes, when `CaptureLength` is set and smaller than the original length, or when decoding reports an error layer. An empty summary becomes `Truncated or malformed packet`. A summary that already exists gains `; truncated`.

## Bounds

These limits are the backpressure. Reaching one stops the import or the live artifact and sets `Limited` where the caller records it.

| Limit | Value |
| --- | --- |
| File | 64 MiB |
| Packets | 10000 |
| One packet | 1 MiB |
| Snaplen | at most 1 MiB |

Import rejects a short header, a bad magic, a snaplen above 1 MiB, and a record whose captured length is greater than its original length. Export returns the writer's error, including a disk-full error, without rewriting it. Pktmon start and stop failures do not include private paths. A capture provider error is the job error. History keeps the backend, state and counts, not packet text.

Pulling a cable is a native check in [the Windows runbook](verification/native-acceptance-windows.md). Unit tests cover the provider-error path only.
