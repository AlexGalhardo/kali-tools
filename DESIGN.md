---
name: kali-tools
description: A local network workspace with a precise terminal identity
colors:
  background: "#000000"
  sidebar: "#050805"
  surface: "#080d09"
  raised: "#101b12"
  border: "#203627"
  control-border: "#3d6345"
  text: "#e2f4e5"
  muted: "#9bb5a0"
  accent: "#39ff14"
  accent-hover: "#87ff70"
  accent-ink: "#061000"
  accent-border: "#356b31"
  selected: "#10290f"
  hover: "#142018"
  warning: "#efc47c"
  warning-surface: "#211b0e"
  warning-border: "#6e572b"
  danger: "#ffb0b8"
  danger-surface: "#260d12"
  danger-border: "#854550"
  graph-secondary: "#65dce4"
typography:
  body:
    fontFamily: '"JetBrains Mono", "Cascadia Code", Consolas, monospace'
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.6
  title:
    fontFamily: '"JetBrains Mono", "Cascadia Code", Consolas, monospace'
    fontSize: "30px"
    fontWeight: 550
    lineHeight: 1.2
    letterSpacing: "-0.02em"
  label:
    fontFamily: '"JetBrains Mono", "Cascadia Code", Consolas, monospace'
    fontSize: "12px"
    fontWeight: 500
rounded:
  control: "4px"
  panel: "6px"
  pill: "5px"
spacing:
  compact: "8px"
  normal: "16px"
  section: "24px"
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.accent-ink}"
    rounded: "{rounded.control}"
    padding: "10px 16px"
  button-primary-hover:
    backgroundColor: "{colors.accent-hover}"
  button-secondary:
    backgroundColor: "{colors.raised}"
    textColor: "{colors.muted}"
    rounded: "{rounded.control}"
    padding: "10px 16px"
  input:
    backgroundColor: "{colors.background}"
    textColor: "{colors.text}"
    rounded: "{rounded.panel}"
    padding: "11px 12px"
  card:
    backgroundColor: "{colors.surface}"
    rounded: "{rounded.panel}"
  pill:
    backgroundColor: "{colors.hover}"
    textColor: "{colors.muted}"
    rounded: "{rounded.pill}"
    padding: "4px 8px"
---

## Overview

**Creative North Star: "The Precise Terminal"**
Pure black, neon green and terminal typography give the shared desktop and browser interface its hacker/cyberpunk identity. This is an Operate surface: the identity supports real network tasks through familiar controls, clear results and explicit capability states.

**Key Characteristics:**

- Black workspace with green-tinted tonal layers.
- Offline monospace typography and static terminal prompts.
- Compact controls, visible focus and text-backed status colors.

## Colors

The accent identifies actions, current navigation, key measurements and keyboard focus. Near-black surfaces separate the sidebar, panels and raised controls from the workspace. Pale green text and muted green-gray metadata preserve hierarchy. Amber marks permission requirements; pale red marks failures. Cyan distinguishes the secondary graph series.

**The Status Rule.** Pair semantic color with a readable state label; retain the distinct warning and failure palettes.

## Typography

Use the terminal stack throughout. Bundle unmodified JetBrains Mono 2.304 regular and semibold WOFF2 under OFL-1.1; no font CDN is required. Regular supplies body text; semibold supplies emphasized roles. Titles step down to 26px in compact layouts and 24px on mobile. Data and controls generally use 12px; table headers use 11px. Tables and measurements use tabular numerals.

## Layout

Desktop navigation is fixed at 248px beside a flexible workspace; content is capped at 1550px. At 1100px and below, navigation narrows to 218px and secondary columns stack. At 720px and below, navigation moves into normal flow with two columns, metrics stack and headings/actions wrap. Tables scroll within their containers. Packet rows remain paginated.

## Elevation & Depth

**The Tonal Depth Rule.** Separate work areas through borders and green-tinted surface layers, without decorative shadows, glow or blur.

## Shapes

Buttons use compact control corners; panels, inputs and code/secret containers use panel corners. Status pills use their own modest rounding. Metrics form a flat ruled strip. Retain the shared authored SVG stroke vocabulary.

## Components

Base UI Button retains button behavior. Primary buttons use accent fill, secondary buttons use raised surfaces and visible borders, and text actions remain quiet until hover. Selected navigation combines accent text, a dark selected surface and an accent border. Inputs have programmatic labels and dark backgrounds. Keyboard focus uses a 2px accent outline with a 4px offset.

Panel titles have a distinct sidebar-toned strip. Page headings carry a static green prompt. Warning and error notices retain readable text and semantic borders. Loading, empty and unavailable states remain explicit; secret reveal retains its existing clearing behavior. The only motion is the 150ms button background transition, removed for reduced-motion preferences.

## Do's and Don'ts

- Do keep targets and scope next to the action.
- Do preserve visible keyboard focus and text-backed capability states.
- Do use the shared semantic palette for charts and controls.
- Don't add decorative animation, terminal rain or fabricated output.
- Don't hide permission requirements or unavailable capabilities behind the theme.
- Don't replace shared SVG icons with arbitrary Unicode glyphs.
