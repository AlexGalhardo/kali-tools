# A08 Terminal visual theme

| Field | Value |
| --- | --- |
| PLAN tasks | A08 |
| Status | Implemented and visually verified 2026-10-21; evidence in docs/verification/specs/A08-theme.md |
| Depends on | Existing shared React interface |
| Risk | Low; shared styling, bundled fonts and responsive layout |

## 1. Goal

Apply the owner's explicit black, neon-green, Matrix/terminal/cyberpunk direction to the shared desktop and browser UI, using Impeccable. Improve hierarchy and readability without changing capability behavior.

## 2. Non-goals

No fake terminal output, fake data, flashing rain, new network operations, transport implementation, or theme picker. The web transport remains W01/W04.

## 3. Current state

`frontend/src/styles/style.css` uses blue-gray surfaces and system sans, with many repeated literal colors. `App.tsx` shares all pages and hardcodes graph colors. The same frontend is embedded in desktop; browser mode currently reports the missing native bridge. Existing Playwright coverage has 18 workflows and captures 1320/960 layouts.

## 4. Design

Use pure black workspace, near-black green-tinted surfaces, neon green `#39ff14` for actions/selection/focus, muted green-gray text, amber permission and red failure states. Bundle unmodified JetBrains Mono 2.304 regular/semibold WOFF2 under OFL-1.1 for offline use. Controls retain familiar behavior, clear labels and keyboard focus. Use a static terminal prompt treatment in headings, precise 4px control/6px panel corners, readable 12px+ data and tabular numerals. Reuse semantic CSS variables for graph colors. Support 1320, 960 and 390px widths with table scrolling and a compact two-column mobile navigation. No decorative animation.

## 5. Files

Modify `frontend/src/styles/style.css`, `frontend/src/app/App.tsx`, `frontend/tests/workflows.spec.ts`, DESIGN.md, `.impeccable/design.json`, `docs/dependencies.md`, `scripts/notices.mjs`, PLAN.md and CHANGELOG.md. Add `frontend/src/shared/assets/fonts/JetBrainsMono-Regular.woff2`, `JetBrainsMono-SemiBold.woff2`, `docs/licenses/jetbrains-mono.txt`, and `docs/verification/specs/A08-theme.md`.

Finish-review correction: extend the existing SVG vocabulary in `frontend/src/shared/ui/Icon.tsx` to replace refresh/open/forward Unicode icon substitutes; preserve labels and behavior.

Accessible-name correction: add `viewInterfaces` to both catalogs in `frontend/src/shared/i18n/i18n.ts`; the icon-only overview action uses it.

## 6. Implementation steps

1. Inspect current desktop screenshot; preserve the current workflow suite.
2. Bundle fonts/license; replace styling tokens and update responsive layout and graph colors.
3. Run all frontend gates, inspect desktop/compact/mobile renders and keyboard focus; run the Impeccable detector once.
4. Record evidence, synchronize design documentation and commit with hooks.

## 7. Acceptance criteria

- AC-1: Workspace is black; neon green consistently marks actions, selection and focus.
- AC-2: Terminal typography loads locally in both shared build modes, with no font CDN.
- AC-3: Existing workflows and serious/critical accessibility checks pass.
- AC-4: No viewport-level horizontal overflow at 1320, 960 or 390px; tables scroll within their container.
- AC-5: Errors/permission states remain distinguishable by text and color; keyboard focus is visible.

## 8. Tests

Retain all 18 workflows; extend visual-evidence workflow with mobile overflow and font checks. Axe continues to inspect every page. Manually inspect generated screenshots and focused controls.

## 9. Verification

`bun run typecheck`, `bun run lint`, `bun run build`, `bunx playwright test`, `node scripts/markdown.mjs`, `node scripts/release-policy.mjs`, `node scripts/notices.mjs`; Impeccable detector and rendered review. Run Go regression checks before push through existing hooks.

## 10. Documentation updates

Update DESIGN.md and its sidecar with the authorized visual direction, record source/font license, and record actual checks in A08-theme evidence. Pagination satisfies the 1.0 virtualization requirement ([ADR 007](../adr/007-paginated-tables.md)). A08's broader native and permission acceptance remains open.

## 11. Risks and rollback

Monospace expands labels; inspect PT-BR and narrow layouts. Revert the theme commit to restore the previous style; no stored settings or public API changes.

## 12. Quality and delivery assertions

Applicable global frontend, Markdown, release, scope and hook gates apply. No new Go package or production logic. DA-1: all browser workflows pass, count never decreases. DA-2: local WOFF2 assets and OFL notice ship in the frontend and notices. DA-3: screenshots show the requested palette/typography at all target widths; no viewport overflow. DA-4: no claims of a completed HTTP transport or native bridge qualification from browser fakes.

Evidence record: `docs/verification/specs/A08-theme.md`.
