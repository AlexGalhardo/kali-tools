# Dependency inventory

Development baseline: 2026-10-19. Exact Go dependencies are in go.mod/go.sum; frontend dependencies are pinned in frontend/package.json and its lockfile. Project code uses the MIT license in LICENSE.md, selected under the owner's instruction to complete open-source licensing.

| Dependency | Purpose | Upstream license |
| --- | --- | --- |
| Go standard library | Portable diagnostics and networking | BSD-3-Clause |
| Wails 2.17.0 | Desktop WebView and typed bridge | MIT |
| golang.org/x/sys | Windows native API bindings | BSD-3-Clause |
| golang.org/x/net | DNS fixture protocol | BSD-3-Clause |
| modernc.org/sqlite | Pure Go local SQLite | BSD-3-Clause; SQLite public domain |
| gopacket 1.7.4 | Packet decoding and PCAP/PCAPNG | BSD-3-Clause |
| React / React DOM | Frontend | MIT |
| JetBrains Mono 2.304 | Bundled offline terminal typography (regular and semibold WOFF2) | SIL Open Font License 1.1; [license](licenses/jetbrains-mono.txt), [upstream](https://github.com/JetBrains/JetBrainsMono/tree/v2.304) |
| Base UI | Accessible button primitives | MIT |
| Tailwind / Vite | Frontend build | MIT |
| Impeccable | Development design skill | Apache-2.0 |
| Karpathy guidelines | Development coding skill | MIT |
| Ponytail | Development coding skills | MIT |

This table summarizes direct dependencies. Release packaging also generates a transitive inventory and collects license/copyright/notice files from the pinned modules and production frontend packages. `go list -m -json all` and `npm ls --all --json` provide transitive inventories. Each native archive includes THIRD_PARTY_NOTICES.txt and dependencies.json; vendored skill licenses and revisions are preserved.

Nmap and TShark are neither installed nor redistributed by this project. Optional inventory probes and explicit Nmap service inspection use fixed argument arrays, deadlines and bounded output. Nmap 7.80+ within major 7 is recognized; TShark 4.2+ within major 4 remains inventory-only. No mandatory paid runtime service is required.

The module requires Go 1.27.2 and pins x/net 0.60.0, x/crypto 0.57.0 and x/text 0.42.0 after the 2026-10-19 security scan. Core and desktop-tagged govulncheck runs returned no findings; see the verification record for scope and commands. Both frontend lockfiles are retained for the verified Bun and npm installation paths.
