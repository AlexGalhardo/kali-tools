# ADR 010: Linux 1.0 baseline

Status: accepted on 2026-10-23 by the repository owner (Alex Galhardo).

Context: B01 needs one documented Linux baseline before the netlink, NetworkManager, nl80211 and libpcap adapters are treated as supported. CI already builds the Linux native job on `ubuntu-24.04`.

Decision:

- Ubuntu 24.04 LTS is the documented Linux baseline. It matches the CI `ubuntu-24.04` runner.
- Debian 13 and Kali are best-effort and come with no guarantee.
- `doctor` and the README state that sentence. A best-effort run does not close B01.
- Native acceptance still requires a human on Ubuntu 24.04. Cross-compilation and the CI runner do not close it.

Consequences: setup instructions and the Linux runbook aim at Ubuntu 24.04. Other distributions may need the same GTK3 and WebKitGTK 4.1 packages, and may lack NetworkManager or libpcap. Those gaps stay explicit capability states.

Revisit when 1.0 hardware acceptance is finished, or when the owner chooses a second guaranteed distribution.
