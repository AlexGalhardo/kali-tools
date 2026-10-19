# Brainstorming

Status: awaiting the owner's first round. Research: `tools-research.md`.

## Confirmed requirements

- Go, Wails, React, Tailwind CSS v4; Base UI from the shared conversation.
- Windows 11, Linux and macOS as the intended desktop platforms.
- Research first, collaborative brainstorming second, then a detailed PLAN.md.
- SemVer, Keep a Changelog, Conventional Commits and EditorConfig.
- GitHub Actions code prepared but inactive; Docker for CLI tool tests.
- Git remote: https://github.com/AlexGalhardo/hacker-tools.git

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
