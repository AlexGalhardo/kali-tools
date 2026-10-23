# Linux native acceptance

Use this on a real Linux machine. Download the unsigned CI artifact `native-linux-x64` (a `.tar.gz` or `.zip` in the workflow artifacts). It contains the desktop build and the `kali-tools` binary. Development builds are unsigned and are not a qualified release.

The CI artifact is linux/amd64. Do not treat an aarch64 run of that binary as a pass. Ubuntu 24.04 LTS is the documented baseline ([ADR 010](../adr/010-linux-baseline.md)). Debian 13 and Kali are best-effort and come with no guarantee; a run on either does not close B01. Record the distro once. Save screenshots and command output under a folder you keep with the verification notes. Redact SSIDs, BSSIDs, keys and packet payloads. Do not tick PLAN boxes from this file; tick a box only after the captured evidence is written into `docs/verification`.

Wi-Fi metadata uses nl80211. Live capture uses libpcap only in a build with `-tags pcap`. The CI Linux artifact is that build. Opening an interface needs `CAP_NET_RAW` on the capture process. Do not run the UI or the server as root. Those steps do not close A05, A06 or B01 until the evidence is pasted.

Privilege for 1.0 is a one-time `sudo setcap cap_net_raw+eip` on the capture binary ([ADR 011](../adr/011-no-privileged-helper.md)). Do not run the UI or `kali-tools serve` as root. There is no privileged helper. `capture_live` stays `unsupported` on Linux and its doctor detail names that command. `setcap` is what lets the libpcap backend in step 7 open an interface.

1. **Record the host.** Run `cat /etc/os-release` and `uname -m`. Expected for a B01 baseline run: Ubuntu 24.04 and `x86_64`. If the file says Debian 13 or Kali, record that and mark the run best-effort. Evidence: `01-os-release.txt`. Supports every step below.

2. **Launch the desktop app.** Install WebKitGTK 4.1 if the app fails to start (`libwebkit2gtk-4.1-0` on Debian/Ubuntu). Open the desktop build. Expected: Overview lists this machine's adapters and the sidebar does not say "Not connected". Evidence: `02-overview.png`. If the library is missing, save the error text as `02-start.txt` and stop; that does not close a PLAN box. A successful launch closes the practical native-bridge check for H01 AC-10, H03 AC-11, H04 AC-12, H05 AC-17/18 and H06 AC-17 on the installed artifact.

3. **Confirm the desktop opens no listener.** With the app running: `ss -ltnp | grep kali-tools` or the same filter on the app's pid. Expected: no rows. Evidence: `03-ss.txt`. Closes W01 AC-17 on Linux.

4. **Exercise a desktop job.** In Diagnostics, run a TCP check to `127.0.0.1` port `8080` (or another closed local port). Open History. Expected: the job reaches a terminal state and History shows the redacted summary, not the target. Evidence: `04-history.png`. Closes H02 AC-16 for job state on Linux.

5. **Compare neighbors, routes and counters with the kernel.** `ping -c 1 <gateway>`, then `./kali-tools neighbors --family ipv4 --json`, then `ip neigh show <gateway>`. Expected: exit 0, `provider` is `Linux netlink RTM_GETNEIGH`, and the gateway address and MAC match. The command reads the cache; it does not scan. Then `./kali-tools inventory --json`, `ip -4 route show default` and `ip -s link`. Expected: the default gateway matches, and a counter name matches an interface. Evidence: `05-neighbors.txt`, `05-inventory.json`. This is the B01 routes, counters and neighbors check. It does not close B01 until the pasted evidence is recorded, and a best-effort distro does not close it.

6. **Wi-Fi metadata and key reveal.** On Ubuntu 24.04 with a connected Wi-Fi interface, run `./kali-tools wifi show --json`. Expected: exit 0, an SSID, and a signal from 0 to 100. If NetworkManager is not installed, `./kali-tools wifi reveal --confirm --json` exits with `dependency_missing` and `wifi show` can still succeed. If NetworkManager refuses the secret, reveal is `permission_required`. Do not save the key. Evidence: `06-wifi.json` with the key removed, plus a note of the reveal state. A machine without a radio records `unsupported` or `no_connection`. This does not close A05 or B01 until the pasted evidence is recorded.

7. **libpcap capture.** The CI `native-linux-x64` artifact is built with `-tags pcap` and links libpcap. Install `libpcap0.8t64` if the dynamic linker says it is missing. Run `./kali-tools doctor --json` and `./kali-tools capabilities --json`. Expected: `capture_libpcap` is `available` and its detail names `CAP_NET_RAW`. `capture_live`, `capture_pktmon` and `capture_npcap` are not working Linux backends. `./kali-tools capture live --backend raw_socket --duration 5 --json` exits 3 with `unsupported`.

   As your normal user, not as root, run `./kali-tools capture live --backend libpcap --target <local-ipv4> --duration 5 --json`. Expected: exit 4, `permission_required`, and the message names `CAP_NET_RAW`. `id -u` is not 0. Do not prefix the command with `sudo`.

   To capture, grant the capability to that binary only: `sudo setcap cap_net_raw+eip ./kali-tools`. Run the same capture command again without `sudo` while `ping -c 5 <gateway>` runs. Expected: exit 0 and a packet count. `id -u` is still your user. Remove the capability with `sudo setcap -r ./kali-tools`. Evidence: `07-doctor.json`, `07-capture-denied.json`, `07-capture.json` with payloads removed. This does not close A06 or B01 until the pasted evidence is recorded. An untagged local build reports `capture_libpcap` as `unsupported` ("this build does not include libpcap") and does not close B01 either.

   In Discovery & tools, confirm service inspection does not offer Nmap and TShark stays inventory-only. Evidence: `07-tools.png`.

8. **TLS audit.** With internet access, run `./kali-tools tls-audit --target example.com --port 443 --json`. Expected: exit 0, `verifier` is `go-x509`, `verified` is true, and TLS 1.2 or 1.3 is `accepted`. Open Diagnostics, TLS audit, and run the same host. Expected: the protocol matrix and chain render. Evidence: `08-tls-audit.json`, `08-tls-audit.png`. This is the Linux observation for B08. It does not close the Windows platform-verifier check.

9. **Chrome cross-origin.** Start `./kali-tools serve` and open the printed URL in Chrome or Chromium. From another origin, run in the browser console: `fetch("http://127.0.0.1:<port>/api/history", {credentials: "include"})` and submit a cross-origin form POST to `/api/diagnostics`. Expected: both fail, and the server log shows 403 or 401. Evidence: `09-chrome.txt`. Closes the Chrome half of W04 AC-15 on Linux. Edge is not required on Linux.

10. **Server account.** As your normal user, run `./kali-tools serve` and open the URL. Record the Wi-Fi and capture rows. Stop it. Start it again as another local account. Expected: the capability rows are that account's states. Wi-Fi follows nl80211 and NetworkManager for that account. `capture_libpcap` follows whether that process was built with `-tags pcap` and whether it has `CAP_NET_RAW`. The page says the states come from the server process. Do not start the server as root. Evidence: `10-serve-user.png`, `10-serve-other.png`. Closes W01 AC-16 and the Linux half of W06.

11. **DNS panel smoke.** Open Diagnostics, then the **DNS inspection** tab. Expected: the form is in English, the intro says observed validation is not a trust decision, and switching the app language to PT-BR translates the labels. Do not send a query unless you intend to. This does not close a PLAN box.

12. **Local discovery and SNMP.** On your own LAN, open Discovery & tools. Pick an up, non-loopback adapter, set the scope to that adapter's IPv4 /24, and run local discovery for a few seconds. Expected: responders stay inside that prefix, and a LOCATION value is text, not a fetched page. Then run `./kali-tools local-discover --interface <name> --scope <cidr> --json`. For SNMP, tick **I own or administer this device** and query one device you administer, using `--secret-file` (mode 600). Expected: system and interface rows appear, and the command line, history and logs do not contain the community or passphrase. Evidence: `12-local-discover.json`, `12-snmp.json` with the secret redacted. Closes B10 AC-11 and AC-12 on Linux. Skip SNMP if you have no device you administer, and say so.

13. **Install, upgrade, uninstall.** Unpacking the CI archive is not the qualified install path. If you only unpacked it, write "not run" for R02's native install, upgrade, rollback and uninstall checks.

## Paste-back template

```text
Host:
Distro and uname -m:
Artifact:
Steps run:
Steps skipped and why:
PLAN items this evidence can close:
Problems:
```
