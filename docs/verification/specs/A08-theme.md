# A08 terminal theme evidence

- Date: 2026-10-21, Windows build 26300; Bun 1.4.3, shared React frontend.
- Scope: owner's explicit black/neon-green/terminal Matrix theme request, using Impeccable; spec `docs/specs/A08-terminal-theme.md`.
- Baseline: 18 browser workflows; screenshot of existing blue-gray/sans UI inspected before editing.
- Result: pure black workspace, green-tinted near-black panels, neon accent and keyboard focus, bundled JetBrains Mono regular/semibold, shared semantic graph colors, compact mobile navigation, consistent SVG action icons. Open-interfaces action now has an explicit bilingual accessible label.

| Check / acceptance | Result |
| --- | --- |
| AC-1 black/neon hierarchy | Inspected desktop 1320px, compact 960px and mobile 390px screenshots |
| AC-2 offline typography | WOFF2 files bundled by Vite, 92.16/94.47 kB; browser font readiness check passes; no font CDN |
| AC-3 behavior and accessibility | 19 browser workflows pass (18 existing plus narrow-theme workflow); Axe checks all eight screens for serious/critical violations |
| AC-4 responsiveness | No viewport overflow at 1320/960/390; all eight pages inspected by narrow-layout test, including PT-BR settings |
| AC-5 semantic states | Existing translated error/permission workflows pass; amber permission state visually distinct; 2px neon focus rules retained |
| TypeScript / lint / build | Bun typecheck/lint/build pass; initial formatting findings fixed |
| Impeccable mechanical detector | Ran once on changed CSS/App, returned `[]` |
| Impeccable finish review | Separate reviewer requested SVG action icons; replacement and alignment fixes scored resolved; final disposition ship at scored-fix scope |
| Font licensing and notices | Unmodified upstream 2.304 files, OFL notice in docs/licenses; `node scripts/notices.mjs` includes font and produces notices successfully |
| Markdown / release / scope | Markdown and release policy checks pass; no public contract or stored settings change |

Screenshots: `.impeccable/review/desktop.png`, `compact.png`, `mobile.png` (generated, ignored). The latter shows PT-BR settings. The first SVG replacement exposed a missing accessible name on the icon-only action; fixed with English/PT-BR labels before the final passing run. The reviewer also found block icon alignment; inline-flex buttons fixed it.

This qualifies shared styling and browser behavior with faithful test data, not native Wails IPC. The real HTTP transport remains W01/W04; the existing browser missing-bridge state remains honest. No fake traffic, animated rain, external font request, or secret persistence was added. Broader A08 platform and permission qualification stays open. On 2026-10-23 the owner decided that 100-row pagination satisfies the virtualization requirement ([ADR 007](../../adr/007-paginated-tables.md)).
