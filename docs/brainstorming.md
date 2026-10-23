# Brainstorming

Status: first round completed on 2026-10-19. Research: `tools-research.md`. Execution plan: `../PLAN.md`.

## Owner decisions

1. Windows 11 first, with abstractions for Linux/macOS.
2. Implement as much as practical in Go; integrate Nmap/TShark for remaining gaps. Use open-source tools and keep the project noncommercial.
3. Include Wi-Fi password display and live packet capture in alpha.
4. Use local SQLite for history and preferences.
5. PT-BR and English interface; English code and documentation.
6. After creating PLAN.md, configure GitHub authentication/access for ongoing pushes.
7. On 2026-10-23, cancel Nmap service inspection because of the NPSL 0.95 execute-and-parse clause. This supersedes the Nmap half of decision 2. Identify scoped TCP services with native Go checks. See ADR 006.

The earlier recommendations/questions below are retained as brainstorming history. The decisions above supersede conflicting proposals, including postponing capture until beta.

## Confirmed requirements

- Go, Wails, React, Tailwind CSS v4; Base UI from the shared conversation.
- Windows 11, Linux and macOS as the intended desktop platforms.
- Research first, collaborative brainstorming second, then a detailed PLAN.md.
- SemVer, Keep a Changelog, Conventional Commits and EditorConfig.
- GitHub Actions code prepared but inactive; Docker for CLI tool tests.
- Git remote: https://github.com/AlexGalhardo/kali-tools.git

## Recommendations to discuss

1. Hybrid architecture: native Go diagnostics and monitoring, optional Nmap/TShark/iperf3 adapters. Rewriting full equivalents would substantially expand scope.
2. Deliver a useful alpha with interfaces, addresses, passive bandwidth, ping, DNS and basic TCP checks. Capture and advanced integrations follow in beta.
3. Share one Go core between desktop and CLI. No network service is necessary for the packaged desktop.
4. Start with built-in modular adapters; consider third-party plugins after 1.0.
5. Keep credentials ephemeral, capture files opt-in, and history local with retention controls.
6. Validate all three OSes before 1.0, while choosing one lead platform for the first vertical slice.

## Questions for round one

- Which is the lead platform: Windows 11, Linux, or all three with equal alpha acceptance gates?
- Is the hybrid approach acceptable, or is native Go reimplementation an explicit project goal?
- Which alpha features are essential? Suggested: interfaces/IP/MAC, passive traffic graphs, ping/DNS/TCP. Should current Wi-Fi password display and packet capture already be mandatory in alpha?
- Should history be persisted locally in SQLite? Suggested: preferences and diagnostic summaries only; packets and secrets excluded by default.
- Preferred interface languages: PT-BR, English, or both? Suggested: both, with English code/docs as in the reference.

## Planning rules

Record answers here, then build PLAN.md with structure/alpha/beta/stable phases, nested tasks, delivery assertions, quality gates and evidence. Do not mark unimplemented features complete. License choice, signing credentials, minimum OS versions and redistribution policy remain explicit decisions before release packaging.
