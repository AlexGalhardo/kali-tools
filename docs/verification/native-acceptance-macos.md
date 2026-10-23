# macOS native acceptance

Use this on a real Apple Silicon Mac running macOS 13 Ventura or later. Download the unsigned CI artifact `native-macos-arm64`. It contains the desktop `.app` and the `kali-tools` binary. Development builds are not notarized and are not a qualified release. Intel Macs are out of scope ([ADR 007](../adr/007-macos-baseline.md)).

The `.app` keeps the ad-hoc signature the Go linker applies (`codesign -s -`). That is not a Developer ID signature. Gatekeeper may block the first open. Use System Settings > Privacy & Security > Open Anyway, or `xattr -d com.apple.quarantine` on the `.app` ([ADR 009](../adr/009-signing-and-provenance.md)).

Save screenshots and command output under a folder you keep with the verification notes. Redact SSIDs, BSSIDs, keys and packet payloads. Do not tick PLAN boxes from this file; tick a box only after the captured evidence is written into `docs/verification`.

Neighbors have no macOS provider yet. Wi-Fi metadata comes from CoreWLAN when the build uses cgo, and an empty network name is location consent ([ADR 013](../adr/013-macos-wifi.md)). Live capture is not implemented. `capture_live` reports whether this user can open `/dev/bpf` ([ADR 011](../adr/011-no-privileged-helper.md)). The Location Services check is the open B02 question from ADR 007. It does not close B02 by itself.

1. **Record the host.** Run `sw_vers` and `uname -m`. Expected: macOS 13 or later and `arm64`. Evidence: `01-sw-vers.txt`. Supports every step below.

2. **Confirm the ad-hoc signature and launch.** The archive contains one `.app` and the `kali-tools` binary. `codesign -dv --verbose=2 /path/to/that.app 2>&1 | grep Signature`. Expected: `Signature=adhoc`. Open the app. Expected: Overview lists this machine's adapters and the sidebar does not say "Not connected". Evidence: `02-codesign.txt`, `02-overview.png`. A successful launch closes the practical native-bridge check for H01 AC-10, H03 AC-11, H04 AC-12, H05 AC-17/18 and H06 AC-17 on the installed artifact.

3. **Confirm the desktop opens no listener.** With the app running: `lsof -nP -iTCP -sTCP:LISTEN -p <pid>`. Expected: no rows. Evidence: `03-lsof.txt`. Closes W01 AC-17 on macOS.

4. **Exercise a desktop job.** In Diagnostics, run a TCP check to `127.0.0.1` port `8080` (or another closed local port). Open History. Expected: the job reaches a terminal state and History shows the redacted summary, not the target. Evidence: `04-history.png`. Closes H02 AC-16 for job state on macOS.

5. **Neighbors stay unsupported.** Run `./kali-tools neighbors --family ipv4 --json`. Expected: exit 3 and `error.code` is `unsupported`. `arp -an` may still show rows. Evidence: `05-neighbors.json`. This does not close A01. It records the missing macOS provider.

6. **Wi-Fi, Location Services and Keychain (ADR 007, ADR 013).** Run `./kali-tools wifi show --json`. Expected: exit 0 with an SSID, or exit 4 with `location_consent_required` or `permission_required`, or exit 3 with `unsupported`. An exit 0 whose SSID is empty is a failed check. The signal percent stays 0 because CoreWLAN reports dBm. Then open the ad-hoc `.app` and System Settings > Privacy & Security > Location Services. Record whether the app appears in the list and whether macOS shows a permission prompt. Redact any SSID or BSSID in the evidence. Run `./kali-tools wifi reveal --confirm --json` and redact the key before saving the output. Expected: `no_saved_key`, `permission_required`, or `unsupported` when the ad-hoc build cannot read Keychain. Do not record the key. Evidence: `06-wifi.json`, `06-reveal.json` with the key removed, `06-location.png`, and one sentence on whether a prompt appeared. This is the B02 observation. It does not close B02 or A05.

7. **BPF access, no capture.** Run `./kali-tools doctor --json` and `./kali-tools capabilities --json`. Expected: `capture_live` is `permission_required` when this user cannot open `/dev/bpf`, and the detail names the `access_bpf` group. When this user can open a BPF device, the state is `unsupported` and the detail says so. Do not `chmod` or `chown` `/dev/bpf*`. `./kali-tools capture live --backend raw_socket --duration 5 --json` does not capture: exit 4 when access is missing, otherwise exit 3 with `unsupported`. Pktmon and Npcap are Windows backends. Evidence: `07-capture.json`. This does not close A06 or B02. In Discovery & tools, confirm service inspection does not offer Nmap and TShark stays inventory-only. Evidence: `07-tools.png`.

8. **TLS audit.** With internet access, run `./kali-tools tls-audit --target example.com --port 443 --json`. Expected: exit 0, `verifier` is `darwin-platform`, `verified` is true, and TLS 1.2 or 1.3 is `accepted`. Open Diagnostics, TLS audit, and run the same host. Expected: the protocol matrix and chain render. Evidence: `08-tls-audit.json`, `08-tls-audit.png`. This is the macOS platform-verifier observation for B08. It does not close the Windows check.

9. **Chrome cross-origin.** Start `./kali-tools serve` and open the printed URL in Chrome. From another origin, run in the browser console: `fetch("http://127.0.0.1:<port>/api/history", {credentials: "include"})` and submit a cross-origin form POST to `/api/diagnostics`. Expected: both fail, and the server log shows 403 or 401. Evidence: `09-chrome.txt`. Closes the Chrome half of W04 AC-15 on macOS. Edge is not required.

10. **Server account.** As your normal user, run `./kali-tools serve` and open the URL. Record the Wi-Fi and capture rows. Stop it. Start it again as another local account. Expected: the capability rows are that account's states. Wi-Fi is that account's CoreWLAN result and is not a success with an empty SSID. `capture_live` follows whether that account can open `/dev/bpf`. The page says the states come from the server process. Do not start the server as root. Evidence: `10-serve-user.png`, `10-serve-other.png`. Closes W01 AC-16 and the macOS half of W06.

11. **DNS panel smoke.** Open Diagnostics, then the **DNS inspection** tab. Expected: the form is in English, the intro says observed validation is not a trust decision, and switching the app language to PT-BR translates the labels. Do not send a query unless you intend to. This does not close a PLAN box.

12. **Local discovery and SNMP.** On your own LAN, open Discovery & tools. Pick an up, non-loopback adapter, set the scope to that adapter's IPv4 /24, and run local discovery for a few seconds. Expected: responders stay inside that prefix, and a LOCATION value is text, not a fetched page. Then run `./kali-tools local-discover --interface <name> --scope <cidr> --json`. For SNMP, tick **I own or administer this device** and query one device you administer, using `--secret-file` (mode 600). Expected: system and interface rows appear, and the command line, history and logs do not contain the community or passphrase. Evidence: `12-local-discover.json`, `12-snmp.json` with the secret redacted. Closes B10 AC-11 and AC-12 on macOS. Skip SNMP if you have no device you administer, and say so.

13. **Install, upgrade, uninstall.** Opening the CI `.app` is not the qualified install path. If you only opened it, write "not run" for R02's native install, upgrade, rollback and uninstall checks.

14. **Socket inventory is unsupported.** Run `./kali-tools connections --json`. Expected: exit 3 and `error.code` is `unsupported`. Then `./kali-tools throughput --duration 1 --json`. Expected: exit 0, `endpoint` starts with `127.0.0.1:`, `duration_ms` is 1000, and `byte_count` is greater than 0. Evidence: `14-connections.json`, `14-throughput.json`. This does not close B02 or B04. macOS socket-to-process mapping was not implemented.

## Paste-back template

```text
Host:
sw_vers and uname -m:
codesign Signature line:
Artifact:
Location Services prompt (yes/no) and whether an SSID appeared:
Steps run:
Steps skipped and why:
PLAN items this evidence can close:
Problems:
```
