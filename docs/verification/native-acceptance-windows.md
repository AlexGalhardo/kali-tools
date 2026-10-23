# Windows 11 native acceptance

Use this on a real Windows 11 machine. Install the latest unsigned CI artifact for `main` (desktop build and `kali-tools.exe`). SmartScreen shows **More info**, then **Run anyway**. Development builds are unsigned and are not a qualified release.

Record `winver` once. Save screenshots and command output under a folder you keep with the verification notes. Redact SSIDs, BSSIDs, keys and packet payloads. Do not tick PLAN boxes from this file; tick a box only after the captured evidence is written into `docs/verification`.

H07's Windows neighbor, ICMP, inventory and elevated-capture data branches already have evidence from 2026-10-21. This list does not repeat them. B09 has no separate Windows gate; step 12 only confirms the panel is present. Step 13 is the B10 native check.

There is no privileged helper ([ADR 011](../adr/011-no-privileged-helper.md)). Install Npcap yourself from https://npcap.com/#download. kali-tools detects it and never bundles or redistributes it. Pktmon and raw IPv4 capture need an elevated terminal. kali-tools never elevates itself.

1. **Record the host.** Run `winver`. Expected: Windows 11, build 26100 or newer for the location-consent steps. Evidence: `01-winver.png`. Supports every step below.

2. **Launch the desktop app.** Open the CI desktop build. Expected: Overview lists this machine's adapters and the sidebar does not say "Not connected". Open Interfaces and confirm the addresses, gateway and DNS match `Get-NetIPConfiguration`. Evidence: `02-overview.png`, `02-interfaces.png`, and the PowerShell output. Closes the practical native-bridge check for H01 AC-10, H03 AC-11, H04 AC-12, H05 AC-17/18 and H06 AC-17 on the installed artifact. A source checkout can also run `node scripts/task.mjs bridge`; the CI artifact does not include that test binary.

3. **Confirm the desktop opens no listener.** With the desktop app running, in PowerShell: `Get-NetTCPConnection -OwningProcess (Get-Process kali-tools | Select-Object -ExpandProperty Id) -State Listen`. Expected: no rows. Evidence: paste the command output as `03-netstat.txt`. Closes W01 AC-17.

4. **Exercise a desktop job.** In Diagnostics, run a TCP check to `127.0.0.1` port `8080` (or another closed local port). Open History. Expected: the job reaches a terminal state and History shows the redacted summary, not the target. Leave the app open and confirm the Overview rate line still updates. Evidence: `04-history.png`. Closes H02 AC-16 for job state and desktop rates. Capture in step 8 covers the rest of that item.

5. **Compare neighbors with the OS cache.** In PowerShell: `ping -n 1 <gateway>`, then `.\kali-tools.exe neighbors --family ipv4 --json`, then `Get-NetNeighbor -AddressFamily IPv4 -IPAddress <gateway> | Format-List`. Expected: exit 0, the gateway MAC and state match, and kali-tools does not send its own scan. In the app, open Interfaces, set the neighbor family to IPv6, and compare with `Get-NetNeighbor -AddressFamily IPv6 | Where-Object { -not $_.IPAddress.StartsWith('ff') }`. Expected: the same non-multicast rows. Evidence: `05-neighbors-ipv4.txt`, `05-neighbors-ipv6.png`. Closes A01 AC-8, A01 AC-9 and the open interface/gateway comparison on the A01 parent.

6. **Location consent off.** Settings, Privacy & security, Location, turn **Let desktop apps access your location** Off. Open the app's Wi-Fi page. Expected: the consent notice is visible and no SSID is shown. Press **Open location settings**. Expected: Windows opens the Location page. In a user console run `.\kali-tools.exe wifi show --json`. Expected: exit 4 and `error.code` is `location_consent_required`. Evidence: `06-consent-off.png`, `06-wifi-cli.txt`. Closes A05 AC-8 and the CLI half of A05 AC-10.

7. **Location consent on, then reveal.** Turn **Let desktop apps access your location** On. Return to the app and press **Check again**. Expected: SSID, BSSID and signal appear. Press the reveal action on your own test network. Expected: the key is masked by default, disappears after 15 seconds, and disappears if the window loses focus. Do not photograph the key. If you can join an open network and an 802.1X network, reveal on each. Expected states: `open_network` and `enterprise`. A non-elevated reveal of a Windows-protected profile is `permission_required`. Evidence: `07-consent-on.png` and a note of the state, with the key cropped out. Closes A05 AC-9 and A05 AC-11. The same Wi-Fi captures are the A09 Wi-Fi evidence.

8. **Capture backends.** Run `.\kali-tools.exe doctor --json` and `.\kali-tools.exe capabilities --json`. Expected: Npcap is reported from detection only. The default CI artifact is not built with `-tags pcap`, so it cannot capture through Npcap; record that limit instead of claiming A06 AC-10. If Npcap is installed from https://npcap.com/#download with default options, doctor shows it available with a version; after a reinstall with "Restrict Npcap driver's access to Administrators only", a non-elevated doctor shows `permission_required`. For Pktmon, from a non-elevated shell run `.\kali-tools.exe capture live --backend pktmon --duration 5 --json`. Expected: exit 4, `permission_required`, and `Get-Process pktmon` shows nothing. From an elevated shell run `.\kali-tools.exe capture live --backend pktmon --duration 10 --json` while `ping -n 20 <gateway>` runs. Then run `pktmon status` and `Get-ChildItem $env:TEMP -Filter 'kali-tools-pktmon-*'`. Expected: imported packets, no running Pktmon session, and no leftover directory. Start another elevated capture and cancel it after about 2 seconds. Expected: `pktmon status` shows no session within 15 seconds. In the app's Packet inspector, start and stop a capture on the default backend. Expected: Stop returns promptly and the rest of the UI still responds. Evidence: `08-doctor.json`, `08-pktmon-denied.txt`, `08-pktmon-ok.txt`, `08-capture-ui.png`. Closes A06 AC-9 and AC-11 through AC-13, and the detection half of AC-10. A06 AC-10 itself stays open until a `-tags pcap` build is tested. These captures are also the A09 capture evidence. In Discovery & tools, confirm service inspection does not offer Nmap and TShark stays inventory-only. Evidence: `08-tools.png`. That is the remaining A07 native qualification note; TShark is not executed. While an elevated capture is running, unplug the adapter only when that will not drop the machine you are using. Expected: the job fails, history contains no packet text, and `pktmon status` shows no leftover session. Say so if you skip this. Unit tests do not close this physical check (B05).

9. **Platform TLS verifier.** With internet access, run `.\kali-tools.exe tls-audit --target example.com --port 443 --json`. Expected: exit 0, `verifier` is `windows-platform`, `verified` is true, and TLS 1.2 or 1.3 is `accepted`. Open Diagnostics, TLS audit, and run the same host. Expected: the protocol matrix and chain render. Evidence: `09-tls-audit.json`, `09-tls-audit.png`. Closes B08 AC-12.

10. **Edge and Chrome cross-origin.** Start `.\kali-tools.exe serve` and open the printed URL in Edge. From another origin (a page you serve locally, not the kali-tools origin), run in the browser console: `fetch("http://127.0.0.1:<port>/api/history", {credentials: "include"})` and submit a cross-origin form POST to `/api/diagnostics`. Expected: both fail, and the server log shows 403 or 401. Repeat in Chrome. Evidence: `10-edge.txt`, `10-chrome.txt`. Closes W04 AC-15.

11. **Server account and consent.** As your normal user, run `.\kali-tools.exe serve` and open the URL. Record the Wi-Fi and capture rows on Overview. Stop it. Start it again with `runas /user:<other-account>`. Open that server's URL in the browser. Expected: the capability rows are the other account's states, and the page says they come from the server process. If location consent or capture permission differs, the UI shows `location_consent_required` or `permission_required` rather than the desktop's state. Evidence: `11-serve-user.png`, `11-serve-other.png`. Closes W01 AC-16 and W06.

12. **DNS panel smoke.** Open Diagnostics, then the **DNS inspection** tab. Expected: the form is in English, the intro says observed validation is not a trust decision, and switching the app language to PT-BR translates the labels. Do not send a query unless you intend to. This does not close a PLAN box. B09's lab check is the Docker job in the evidence record, not this machine.

13. **Local discovery and SNMP.** On your own LAN, open Discovery & tools. Pick the Wi-Fi or Ethernet adapter, set the scope to that adapter's IPv4 /24, and run local discovery for a few seconds. Expected: responders stay inside that prefix, and a LOCATION value is text, not a fetched page. Then run `.\kali-tools.exe local-discover --interface <name> --scope <cidr> --json`. For SNMP, tick **I own or administer this device** and query one device you administer, using `--secret-file` (a mode-600 file). Expected: system and interface rows appear, and the command line, history and logs do not contain the community or passphrase. Evidence: `13-local-discover.json`, `13-snmp.json` with the secret redacted. Closes B10 AC-11 and AC-12. Skip SNMP if you have no device you administer, and say so.

## Paste-back template

```text
Host:
OS build:
Artifact:
Steps run:
Steps skipped and why:
PLAN items this evidence can close:
Problems:
```
