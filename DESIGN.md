---
name: kali-tools
description: A focused local network workspace
colors:
  background: "#0c1118"
  sidebar: "#10161e"
  surface: "#111923"
  border: "#24303c"
  text: "#dbe4ec"
  muted: "#94a6b8"
  primary: "#61e6af"
  primary-ink: "#062a20"
typography:
  body:
    fontFamily: "Segoe UI, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 400
  title:
    fontFamily: "Segoe UI, system-ui, sans-serif"
    fontSize: "30px"
    fontWeight: 550
rounded:
  control: "7px"
  panel: "10px"
spacing:
  compact: "8px"
  normal: "16px"
  section: "24px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.primary-ink}"
    rounded: "{rounded.control}"
    padding: "10px 16px"
---

## Overview

Operate mode: fast reading, familiar desktop controls, precise states. Preserve the existing dark neutral surfaces and restrained green accent. No decorative terminal costume or marketing content.

## Colors

Green identifies the primary action, current navigation and successful state. Amber identifies authorization requirements. Muted text must retain WCAG AA contrast. Do not rely on color alone for status.

## Typography

Use the Windows system sans for labels and prose. Monospace is reserved for addresses, code and measurements. Data uses tabular numerals. Keep metadata at least 11px and controls comfortably readable.

## Layout

Fixed navigation on desktop, with a flexible main work area. Tables scroll horizontally and packets are bounded to pages of 100 rows. Compact windows reduce navigation width and collapse secondary columns.

## Elevation & Depth

Borders and tonal layers separate work areas. No decorative shadows, glow or blur.

## Shapes

Controls share a 7px radius; work panels use 10px. Status labels remain compact. Icons use one authored SVG stroke vocabulary.

## Components

Base UI Button owns button behavior. Every input has a programmatic label and visible focus. Permission, loading, error and empty states are explicit. Secret reveal clears on timeout, navigation and loss of focus. Network rates are computed by Go and received as batched events.

## Do's and Don'ts

Prefer one clear action per operation. Keep targets and scope next to the action. Display unavailable capabilities honestly. Avoid decorative eyebrows, arbitrary glyph icons, nested panel shells and invented example traffic in production.
