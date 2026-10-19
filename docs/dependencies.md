# Dependency inventory

Development baseline: 2026-10-19. Exact Go dependencies are in go.mod/go.sum; frontend dependencies are pinned in frontend/package.json and its lockfile. The project license is still an owner decision.

| Dependency | Purpose | Upstream license |
| --- | --- | --- |
| Go standard library | Portable diagnostics and networking | BSD-3-Clause |
| Wails 2.17.0 | Desktop WebView and typed bridge | MIT |
| golang.org/x/sys | Windows native API bindings | BSD-3-Clause |
| golang.org/x/net | DNS fixture protocol | BSD-3-Clause |
| modernc.org/sqlite | Pure Go local SQLite | BSD-3-Clause; SQLite public domain |
| gopacket 1.7.4 | Packet decoding and PCAP/PCAPNG | BSD-3-Clause |
| React / React DOM | Frontend | MIT |
| Base UI | Accessible button primitives | MIT |
| Tailwind / Vite | Frontend build | MIT |
| Impeccable | Development design skill | Apache-2.0 |
| Karpathy guidelines | Development coding skill | MIT |
| Ponytail | Development coding skills | MIT |

This is an initial direct-dependency inventory, not a complete redistribution notice bundle. `go list -m -json all` and `npm ls --all --json` provide transitive inventories. Review and include each dependency's notices before a distributable release.

Nmap and TShark are neither installed nor redistributed by this project. Optional inventory probes execute only their fixed version argument with a deadline and bounded output. Advanced adapters remain incomplete. No mandatory paid runtime service is required.
