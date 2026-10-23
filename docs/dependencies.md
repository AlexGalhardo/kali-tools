# Dependency inventory

Development baseline: 2026-10-19. Exact Go dependencies are in go.mod/go.sum; frontend dependencies are pinned in frontend/package.json and its lockfile. Project code uses the MIT license in LICENSE.md, selected under the owner's instruction to complete open-source licensing.

| Dependency | Purpose | Upstream license |
| --- | --- | --- |
| Go standard library | Portable diagnostics and networking | BSD-3-Clause |
| github.com/miekg/dns v1.1.73 | DNS wire format and DS parsing in `internal/dnsinspect` | BSD-3-Clause |
| github.com/zmap/zlint/v3 v3.7.2 | Certificate linting in `internal/tlsaudit` | Apache-2.0 |
| github.com/zmap/zcrypto v0.0.0-20260906180147-3ed30b1e9340 | zlint certificate parser only; no legacy handshake client | Apache-2.0 AND ISC AND MIT |
| github.com/sirupsen/logrus v1.10.2 | zlint transitive logger; process-global output silenced before linting | MIT |
| github.com/pelletier/go-toml v1.9.5 | zlint configuration parser, transitive | MIT AND Apache-2.0 |
| github.com/weppos/publicsuffix-go v0.50.4-0.20260821095816-b0fdb5c2d345 | Certificate lint public-suffix rules, transitive | MIT |
| Wails 2.17.0 | Desktop WebView and typed bridge | MIT |
| golang.org/x/sys | Windows native API bindings | BSD-3-Clause |
| golang.org/x/net | DNS fixture protocol | BSD-3-Clause |
| modernc.org/sqlite | Pure Go local SQLite | BSD-3-Clause; SQLite public domain |
| gopacket 1.7.4 | Packet decoding and PCAP/PCAPNG | BSD-3-Clause |
| gopacket/pcap 1.7.4 | Optional Windows capture, only with the `pcap` build tag | BSD-3-Clause |
| Npcap (not redistributed) | User-installed Windows driver; detected read-only | [Npcap License](https://npcap.com/); free edition restricts redistribution and silent installation |
| React / React DOM | Frontend | MIT |
| JetBrains Mono 2.304 | Bundled offline terminal typography (regular and semibold WOFF2) | SIL Open Font License 1.1; [license](licenses/jetbrains-mono.txt), [upstream](https://github.com/JetBrains/JetBrainsMono/tree/v2.304) |
| Base UI | Accessible button primitives | MIT |
| Tailwind / Vite | Frontend build | MIT |
| Impeccable | Development design skill | Apache-2.0 |
| Karpathy guidelines | Development coding skill | MIT |
| Ponytail | Development coding skills | MIT |

This table summarizes direct dependencies. Release packaging also generates a transitive inventory and collects license/copyright/notice files from the pinned modules and production frontend packages. `go list -m -json all` and `npm ls --all --json` provide transitive inventories. Each native archive includes THIRD_PARTY_NOTICES.txt and dependencies.json; vendored skill licenses and revisions are preserved.

Nmap is not executed, parsed or redistributed. TShark is neither installed nor redistributed. Optional inventory probes use fixed argument arrays, deadlines and bounded output. TShark 4.2+ within major 4 remains inventory-only. Service inspection is implemented in Go (ADR 006) and adds no module. No mandatory paid runtime service is required.

The module requires Go 1.27.3 and pins x/net 0.60.0, x/crypto 0.57.0 and x/text 0.42.0. The 2026-10-19 security scan used Go 1.27.2; the module directive has since moved to 1.27.3. Core and desktop-tagged govulncheck runs returned no findings; see the verification record for scope and commands. Both frontend lockfiles are retained for the verified Bun and npm installation paths.

## DNS library (B09), decided 2026-10-23

The owner chose `github.com/miekg/dns` v1 (BSD-3-Clause). The newest v1.1 tag on `proxy.golang.org` at decision time was **v1.1.73**. The Codeberg module `codeberg.org/miekg/dns` is not used. 1.0 needs a stable API. The Codeberg module is pre-1.0 (0.6.x), so breaking changes are allowed until about 2028, and its dependency set is heavier.

`go.mod` requires `github.com/miekg/dns v1.1.73`. The module cache `LICENSE` and `COPYRIGHT` identify BSD-3-Clause; packaging collects them through `scripts/notices.mjs`. The import is confined to `internal/dnsinspect/anchors.go`. Comparison, SOA/NS, DNSSEC and DoH/DoT are not implemented yet.

Revisit when `codeberg.org/miekg/dns` tags v1.0.0, or if the GitHub v1 repository is archived or stops receiving fixes.
