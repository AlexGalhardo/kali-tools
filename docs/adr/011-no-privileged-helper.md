# ADR 011: No privileged helper for 1.0

Status: accepted on 2026-10-23 by the repository owner (Alex Galhardo).

Context: B03 asked whether kali-tools should ship a privileged helper with authenticated local IPC for capture and other operations the ordinary process cannot perform. The UI and the web server must stay unprivileged.

Decision:

- kali-tools 1.0 ships with no privileged helper and no local helper IPC.
- Privilege comes only from what each operating system already provides.
- Linux: a one-time `sudo setcap cap_net_raw+eip` on the capture binary. Do not run the UI or the web server as root.
- macOS: detect whether this user can open `/dev/bpf` through the `access_bpf` group (ChmodBPF-style). Never change device permissions.
- Windows: the user installs Npcap. Do not bundle or redistribute Npcap. Its free license forbids redistribution without a commercial license. Point to https://npcap.com/#download and detect whether it is present.
- When permission is missing, the UI, CLI and `doctor` show an explicit capability state and the exact fix. Pktmon and raw IPv4 capture say to start kali-tools from an elevated terminal. kali-tools never elevates itself.

Consequences: the helper and its IPC are out of scope for 1.0. Install, update, uninstall and recovery of a helper are not built. The UI and the web server stay unprivileged. Native acceptance of each OS privilege state remains open. This decision does not implement BPF capture or close B02.

Revisit only if the owner explicitly asks for a helper. That helper would need its own ADR.
