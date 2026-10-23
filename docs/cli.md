# CLI contract

## Synopsis

```text
kali-tools [global flags] <command> [<subcommand>] [<positional>] [flags]
```

Human output is the default. Scripts must pass `--json`. Positional kinds and job IDs precede flags. Help is English and exits 0; no command exits 2. The binary implements `serve` through the secure HTTP composition described below.

## Global flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--json` | false | One versioned JSON envelope on stdout. |
| `--timeout` | unset | Integer milliseconds or Go duration, 100ms..10m. Diagnose defaults to 3000ms, with an application maximum of 30s; discovery defaults to 3000ms, with an application maximum of 5s. Capture live uses duration. Other commands use a context deadline. |
| `--no-color` | false | Disable state colors. Redirected output never receives ANSI sequences. |
| `--lang` | en | Human labels, states, warnings and errors: `en` or `pt-BR`. Help stays English. |
| `--detach` | false | Rejected: there is no daemon mode. |

Global flags work before the command or after its positional arguments. Boolean values use `--flag=false`. A `--` terminator ends flag recognition. Parse errors never echo rejected values.

Command defaults: port 443, ports 80,443, duration 5 seconds, limit 1000 packets, format json for discovery, all false, confirm false; other strings are empty. Export requires explicit output and format pcap or pcapng. Capture accepts duration 1..60 and limit 1..10000. Port lists contain 1..32 ports, at most 191 bytes. Job IDs are exactly 32 lowercase hexadecimal characters. Unset settings flags preserve current values.

## version

[JSON paths and types](../internal/adapter/cli/testdata/contract/version.json).

`kali-tools version [flags]`

Flags / positional: none.

JSON `data`: `{"version","go","os","arch"}` (unchanged keys). The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
kali-tools 0.4.0 (goX OS/ARCH)
```

## doctor

[JSON paths and types](../internal/adapter/cli/testdata/contract/doctor.json).

`kali-tools doctor [flags]`

Flags / positional: `--bundle PATH`. Without `--bundle`, output is unchanged.

JSON `data` without `--bundle`: `{"os","arch","baseline","capabilities","tools","capture","wifi"}`. `baseline` states that Ubuntu 24.04 LTS is the documented baseline and that Debian 13 and Kali are best-effort with no guarantee. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

`--bundle PATH` writes a redacted `.zip` and does not upload it. JSON `data` is `{"path","format":"zip"}`. Human output uses the saved-file line. The archive contains `summary.txt` and `report.json` (schema version 1): versions, doctor output, capability states, language, retention and at most 50 history rows. IPs, MAC addresses, dotted hostnames and labelled SSIDs are stable pseudonyms for that file. Wi-Fi keys, scan results and capture packets are not included. A directory path is `invalid_input` and the error does not repeat the path.

Human example (fixture values):

```text
System: OS/ARCH
Ubuntu 24.04 LTS is the documented Linux baseline. Debian 13 and Kali are best-effort and come with no guarantee.

CAPABILITY STATE PROVIDER DETAIL
interfaces available go/net Go interface inventory
diagnostics available go/net DNS, bounded TCP, HTTP (phase timings, security headers, redirect target; explicit hop trace at most 5) and TLS; redirects are never followed automatically
discovery available go/net IPv4 TCP connect, /24 or narrower, at most 32 ports
capture_import available gopacket PCAP and PCAPNG, 64 MiB and 10000 decoded packets

NAME    STATE               VERSION  PATH
tshark  dependency_missing
```

## capabilities

[JSON paths and types](../internal/adapter/cli/testdata/contract/capabilities.json).

`kali-tools capabilities [flags]`

Flags / positional: none.

JSON `data`: `[]Capability`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
CAPABILITY STATE PROVIDER DETAIL
interfaces available go/net Go interface inventory
diagnostics available go/net DNS, bounded TCP, HTTP (phase timings, security headers, redirect target; explicit hop trace at most 5) and TLS; redirects are never followed automatically
discovery available go/net IPv4 TCP connect, /24 or narrower, at most 32 ports
capture_import available gopacket PCAP and PCAPNG, 64 MiB and 10000 decoded packets
```

## interfaces

[JSON paths and types](../internal/adapter/cli/testdata/contract/interfaces.json).

`kali-tools interfaces [flags]`

Flags / positional: none.

JSON `data`: `[]Interface`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
<native interfaces>
```

## inventory

[JSON paths and types](../internal/adapter/cli/testdata/contract/inventory.json).

`kali-tools inventory [flags]`

Flags / positional: none.

JSON `data`: `Inventory`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
ADAPTER           GATEWAYS   DNS
Fixture Ethernet  192.0.2.1  192.0.2.53

COUNTER  RECEIVED  SENT
1        200       100
```

## neighbors

[JSON paths and types](../internal/adapter/cli/testdata/contract/neighbors.json).

`kali-tools neighbors [flags]`

Flags / positional: none.

JSON `data`: error `unsupported` "neighbor inventory is not implemented yet". The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (stderr for errors):

```text
error: unsupported: neighbor inventory is not implemented yet
```

## diagnose

[JSON paths and types](../internal/adapter/cli/testdata/contract/diagnose.json).

`kali-tools diagnose [flags]`

Flags / positional: kind ∈ `dns tcp http tls ping trace traceroute`; `--target` (required), `--port` int default 443, `--record` string, `--redirects` int default 0 (HTTP only, 0..5), `--kind` (deprecated). Redirects are never followed unless `--redirects` asks for that many hops.

JSON `data`: `DiagnosticResult`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
ping fixture.invalid: completed (0 ms, fake)
  echo reply
```

## discover

[JSON paths and types](../internal/adapter/cli/testdata/contract/discover.json).

`kali-tools discover [flags]`

Flags / positional: `--target` CIDR, `--ports` default `80,443`, `--format json\|csv` default `json`.

JSON `data`: `[]PortResult`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (stderr for errors):

```text
HOST       PORT   STATE
127.0.0.1  443  open
```

## inspect

[JSON paths and types](../internal/adapter/cli/testdata/contract/inspect.json).

`kali-tools inspect [flags]`

Flags / positional: `--target` CIDR, `--ports` default `80,443`, `--format json` (deprecated).

JSON `data`: `ServiceInspection`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (stderr for errors):

```text
native 1
HOST       PORT  STATE  SERVICE  METHOD
192.0.2.1  443   open   https    banner
```

## wifi show

[JSON paths and types](../internal/adapter/cli/testdata/contract/wifi-show.json).

`kali-tools wifi show [flags]`

Flags / positional: none.

JSON `data`: `WiFi`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
SSID: Fixture Network
BSSID: 02:00:00:00:00:02
profile: Fixture
signal: 80%
authentication: 7
state: connected
```

## wifi reveal

[JSON paths and types](../internal/adapter/cli/testdata/contract/wifi-reveal.json).

`kali-tools wifi reveal [flags]`

Flags / positional: `--confirm` bool (required).

JSON `data`: `{"state","value"}`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
state: available
key: synthetic-test-key
```

## capture live

[JSON paths and types](../internal/adapter/cli/testdata/contract/capture-live.json).

`kali-tools capture live [flags]`

Flags / positional: `--target` local IPv4, `--duration` int 1..60 default 5, `--limit` int 1..10000 default 1000, `--output`, `--format pcap\|pcapng`.

JSON `data`: capture result. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
0 packets, format live
NO  TIME  LENGTH  SOURCE  DESTINATION  PROTOCOL  INFO
```

## capture import

[JSON paths and types](../internal/adapter/cli/testdata/contract/capture-import.json).

`kali-tools capture import [flags]`

Flags / positional: `--file` (required), `--output`, `--format pcap\|pcapng`.

JSON `data`: `Capture`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
1 packets, format pcap
NO  TIME                  LENGTH  SOURCE     DESTINATION  PROTOCOL  INFO
1   1970-01-01T00:01:40Z  20      192.0.2.1  192.0.2.2    IPv4
```

## capture export

[JSON paths and types](../internal/adapter/cli/testdata/contract/capture-export.json).

`kali-tools capture export [flags]`

Flags / positional: `--file`, `--output`, `--format` (all required).

JSON `data`: `{"path","format"}`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
saved <fixture> (pcapng)
```

## jobs list

[JSON paths and types](../internal/adapter/cli/testdata/contract/jobs-list.json).

`kali-tools jobs list [flags]`

Flags / positional: `--all` bool.

JSON `data`: `[]Job`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
ID                                CREATED               STATE      KIND  SUMMARY
0123456789abcdef0123456789abcdef  2026-10-19T12:00:00Z  completed  tcp   Diagnostic completed
```

## jobs show

[JSON paths and types](../internal/adapter/cli/testdata/contract/jobs-show.json).

`kali-tools jobs show [flags]`

Flags / positional: id = 32 lowercase hex.

JSON `data`: `Job`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
id: 0123456789abcdef0123456789abcdef
kind: tcp
state: completed
created: 2026-10-19T12:00:00Z
finished: 2026-10-19T12:00:00Z
summary: Diagnostic completed
```

## jobs cancel

[JSON paths and types](../internal/adapter/cli/testdata/contract/jobs-cancel.json).

`kali-tools jobs cancel [flags]`

Flags / positional: id = 32 lowercase hex.

JSON `data`: `{"id","cancelled":true}`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (stderr for errors):

```text
cancellation requested: 0123456789abcdef0123456789abcdef
```

## history list

[JSON paths and types](../internal/adapter/cli/testdata/contract/history-list.json).

`kali-tools history list [flags]`

Flags / positional: none.

JSON `data`: `[]Job`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
ID                                CREATED               STATE      KIND  SUMMARY
0123456789abcdef0123456789abcdef  2026-10-19T12:00:00Z  completed  tcp   Diagnostic completed
```

## history clear

[JSON paths and types](../internal/adapter/cli/testdata/contract/history-clear.json).

`kali-tools history clear [flags]`

Flags / positional: none.

JSON `data`: `{"cleared":true}`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
history cleared (queued and running jobs are kept)
```

## settings get

[JSON paths and types](../internal/adapter/cli/testdata/contract/settings-get.json).

`kali-tools settings get [flags]`

Flags / positional: none.

JSON `data`: `Settings`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
language: en
retention_days: 30
```

## settings set

[JSON paths and types](../internal/adapter/cli/testdata/contract/settings-set.json).

`kali-tools settings set [flags]`

Flags / positional: `--language en\|pt-BR`, `--retention-days` int.

JSON `data`: `Settings` (after save). The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
language: pt-BR
retention_days: 7
```

## tools

[JSON paths and types](../internal/adapter/cli/testdata/contract/tools.json).

`kali-tools tools [flags]`

Flags / positional: none.

JSON `data`: `[]Tool`. The envelope contains `schema_version: 1`, `version`, optional `data` and optional `error: {code, message}`. Partial diagnostic/capture data may accompany an error.

Human example (fixture values):

```text
NAME    STATE               VERSION  PATH
tshark  dependency_missing
```

## serve

`kali-tools serve [--listen 127.0.0.1:0] [--json]`

The binary root starts the HTTP/SSE server with the shared persistent application and embedded frontend. Open the printed URL, which contains a secret per-launch fragment; never share it. Ctrl-C stops the server cleanly. Remote mode requires a specific address plus `--allow-remote --tls-cert file --tls-key file`. Wildcard binds are rejected. TLS prints its SHA-256 certificate fingerprint. `--timeout` and `--detach` are rejected; `--lang` and `--no-color` are accepted.

Human output prints the address, token URL and shutdown instruction. JSON prints one versioned startup envelope with `data.url`, `data.address`, `data.tls`, `data.remote` and `data.certificate_sha256`. The URL is printed only on stdout. API requests thereafter use separate contract DTOs, not CLI envelopes.

The adapter-only [reserved-command fixture](../internal/adapter/cli/testdata/contract/serve.json) verifies that `cli.Run` delegates listener ownership to the binary; it is not the binary's startup contract. Routes, body limits, error statuses and capability states are in the [HTTP contract](http.md). See [web server specification](specs/W01-web-server.md) and [security](../SECURITY.md).

## help

`kali-tools help [flags]`

Flags / positional: command path.

Help prints plain text on stdout even with `--json`; it does not emit an envelope.

```text
Usage: kali-tools [global flags] <command> [flags]
Commands:
```

## wifi

[JSON paths and types](../internal/adapter/cli/testdata/contract/wifi.json).

Deprecated alias for `wifi show`. Flags, JSON data and human output match that command; one warning goes to stderr.

## capture

[JSON paths and types](../internal/adapter/cli/testdata/contract/capture.json).

Deprecated alias for `capture live`. Flags, JSON data and human output match that command; one warning goes to stderr.

## import

[JSON paths and types](../internal/adapter/cli/testdata/contract/import.json).

Deprecated alias for `capture import`. Flags, JSON data and human output match that command; one warning goes to stderr.

## Legacy aliases

`wifi` means `wifi show`; `capture` means `capture live`; `import` means `capture import`. `diagnose --kind <kind>` still works and `trace` aliases `traceroute`. Explicit discovery/inspection format json is deprecated in favor of `--json`. Each deprecated form emits exactly one stderr warning. Aliases preserve canonical flags and results.

## Exit codes

| Exit | Meaning |
| --- | --- |
| 0 | Success or help |
| 1 | failed, no_connection, certificate_invalid, unknown errors or output failure |
| 2 | invalid_input, usage error or no command |
| 3 | unsupported |
| 4 | permission_required, location_consent_required |
| 5 | dependency_missing |
| 6 | not_found |
| 7 | timeout |
| 8 | conflict |
| 9 | unauthorized, forbidden |
| 130 | cancelled |

## Output streams

stdout carries only results, help or discovery CSV. `discover --format csv` keeps the header `schema_version,host,port,state` and cannot be combined with `--json`. stderr carries warnings, human errors and human job progress. JSON mode emits one envelope and no progress. Progress is capped at 1000 lines per invocation. Times are RFC 3339 UTC. Ctrl-C and SIGTERM cancel work; a second signal restores normal termination. Cancelled capture returns partial data, exits 130 and never exports.

## Environment

Non-empty `NO_COLOR` disables color. Settings and redacted history share the desktop SQLite database in the OS user configuration directory. Version, help and rejected invocations never open it. The CLI never reads stdin or prompts, including over SSH without a TTY.

## Security

Wi-Fi reveal requires `--confirm`, warns on stderr before revealing, writes the key only to stdout and creates no history row. Terminal recordings may retain the key. Diagnostic history stores state and safe summaries, never Wi-Fi keys or packet payloads. Packet export writes only to an explicit path. Jobs can be cancelled only by the process that owns them; a new CLI process cannot cancel another invocation or the desktop. Retained jobs are queryable with jobs show; jobs list --all includes terminal history. Capture artifacts are process-local: capture export imports --file first. History clear preserves active jobs.

## Compatibility policy

Within one `schema_version`, JSON changes are additive only (new keys or commands). Removing or retyping a key, renaming a command or flag, or changing an exit code requires a schema_version bump, an entry here and in CHANGELOG.md, and a deprecation period of at least one minor release with a stderr warning. Fixtures in internal/adapter/cli/testdata/contract enforce JSON paths and types. Native inventory values, elapsed times and generated IDs are normalized only in golden tests.

## Migration from 0.1.0-alpha.1-dev

1. Output defaults to human text. Add `--json` to scripts.
2. Flags are command-specific: `version --port 1` now fails instead of being ignored.
3. Errors use the distinct exit codes above, beyond the previous 0/1/2 mapping.
4. `-h` and `--help` exit 0 instead of 2.
5. Parse errors produce an envelope with `--json` and never echo rejected values.
6. `diagnose` runs as a cancellable job and records redacted state/summary history.
7. `doctor` describes key reveal as `Windows WLAN API; key reveal requires wifi reveal --confirm or the desktop reveal action`.

Development artifacts remain unsigned. Native SSH, Wi-Fi, capture and desktop acceptance require real-host evidence; unit tests, Docker and cross compilation do not qualify them.

## tls-audit

`kali-tools tls-audit --target <host> [--port 443] [--server-name <sni>] [--timeout 5000] [--lint=true] [--json]`

Reports certificate metadata, explicit verification, protocol support and zlint findings. SNI must be a DNS name. Per-attempt timeout is 500..15000 ms and the overall audit is bounded to 60 seconds. No application data or exploit probes are sent. Certificate distrust and lint findings still exit 0; connection failure exits 1 and invalid input exits 2. History contains state and certificate/error/warning counts only. The Windows/macOS verifier follows the platform trust store; native acceptance is tracked separately.

## dns-inspect

`kali-tools dns-inspect --target <name> [--record A] [--resolvers ip[:port],…] [--doh url,…] [--dot host[:port],…] [--checks compare,soa_ns,dnssec,encrypted] [--trust-anchor "<DS RR>"] [--timeout 2000] [--json]`

Compares the system resolver with explicit resolvers and can check SOA/NS consistency, a read-only DNSSEC chain, and DoH/DoT reachability. `--record` defaults to A. `--checks` defaults to `compare`. Each comma-separated list and the trust-anchor text is limited to 1,024 bytes. The per-query timeout is 200..5000 ms and reuses the global `--timeout` flag; the overall inspection stops at 30 seconds. Invalid input exits 2. Network failures stay on the result and still exit 0. History stores the state, query count and check count only. Observed DNSSEC validation is not a trust decision. Zone transfer, NSEC walking and name guessing are not implemented.

## local-discover

`kali-tools local-discover --interface <name> --scope <cidr> [--protocols mdns,ssdp] [--duration 3] [--service-types _foo._tcp,…] [--ssdp-target ssdp:all] [--json]`

Reads mDNS and SSDP on one up, non-loopback interface. The scope is an IPv4 prefix of /24 or narrower that overlaps that interface. Duration is 1..10 seconds. At most four extra service types are accepted. Responses from outside the scope are counted and discarded. `LOCATION` is shown as text and is never requested. History stores the responder count and the out-of-scope count only.

## snmp

`kali-tools snmp --interface <name> --scope <cidr> --target <ip> --owned --secret-file <path> [--port 161] [--version v2c|v3] [--v3-user <user> --auth SHA256 --priv AES] [--interfaces] [--timeout 2000] [--json]`

Reads SNMP system and, with `--interfaces`, interface MIB columns from one owned device inside the scope. `--secret-file` is a regular file of at most 4 KiB owned by the user. Line 1 is the v2c community or the v3 auth passphrase. Line 2, for v3 only, is the optional privacy passphrase. There is no community or password flag, and environment variables are not read. Timeout is 500..5000 ms. History stores the interface count only. Write operations are not implemented.
