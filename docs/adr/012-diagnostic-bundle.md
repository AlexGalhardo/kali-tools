# ADR 012: Diagnostic bundle

Status: accepted on 2026-10-23. The owner specified the bundle shape; this ADR records the smallest reading of that decision while ADR 010 and ADR 011 are still open on other branches.

Context: B06 requires an on-demand diagnostic bundle that a person can save and inspect without sending it anywhere. The bundle has to stay useful and must not become a second copy of scans, captures or Wi-Fi keys.

Decision:

- `kali-tools doctor --bundle PATH` writes one `.zip` on this computer. The Settings action does the same through a save dialog or a browser download. `GET /api/doctor/bundle` returns those bytes to an authenticated session. Nothing is uploaded.
- The archive contains `summary.txt` and `report.json` at schema version 1. The JSON holds the app/Go/OS/arch versions, the doctor text, capability states, language and retention, and at most 50 recent history rows.
- IPv4, IPv6 and MAC addresses become short pseudonyms (`ip1`, `mac1`) that stay stable inside one archive. The machine hostname, dotted hostnames, labelled `ssid`/`hostname` values, and any extra names passed to the writer become `name1`, `name2`, and so on.
- Secrets passed to the writer are deleted, not renamed. The application does not read Wi-Fi keys, scan results or capture packets, so those values are never copied in. A key that is not already in history and is not passed as a secret cannot appear.
- The bundle does not query Wi-Fi. A query would change the observed capability state and, on some systems, the location prompt. An SSID is pseudonymized only when it is already present as a labelled value or as a supplied name.
- The zip is capped at 1 MiB. A larger result is refused.

Consequences: Stock doctor text that names `npcap.com` is pseudonymized inside the archive, because that host is a hostname. The CLI doctor command without `--bundle` is unchanged. Compatibility fixtures for other export formats remain a separate B06 item.

Revisit only if the owner wants the bundle to include a current SSID or to keep documentation hostnames intact.
