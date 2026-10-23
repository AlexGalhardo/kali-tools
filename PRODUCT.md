# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

The React interface runs inside a native Wails desktop WebView. This is not a hosted service. Windows 11 leads acceptance; Linux and macOS are later native verification gates.

## Stack

Go, Wails v2, React, TypeScript, Base UI, Tailwind CSS v4 and local SQLite. English source and documentation. English and PT-BR interface catalogs.

## Product Purpose

Provide a local network toolkit for interface inventory, traffic monitoring, diagnostics, scoped discovery, current-network Wi-Fi inspection and packet inspection. Success is a working operation with a bounded runtime, a clear result and accurate capability reporting.

## Users

The confirmed scope is the repository owner's local desktop toolkit. A broader audience and deployment environment have not been specified.

## Workflows

Inspect interfaces; start a diagnostic and cancel it; discover a bounded network scope; explicitly reveal the current saved Wi-Fi key; explicitly start or stop capture; import capture files; review redacted history; change language and retention.

## Constraints

No mandatory cloud account or paid runtime. Secrets are ephemeral and excluded from ordinary history and logs. Capture files are opt-in. Operating system permissions remain visible. Service inspection is built in and does not run Nmap. Optional TShark is separately installed and inventory-only. GitHub workflows remain inactive until explicit activation.

## Open Decisions

Project license, signed distribution, supported native OS versions and final privileged-helper architecture remain owner or qualification decisions in PLAN.md.
