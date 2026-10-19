# Tools research

Research date: 2026-10-19. Status: initial research complete; product decisions pending brainstorming.

## Context recovered

The shared conversation was read in the browser. The requested product combines network monitoring, packet inspection, LAN discovery, Wi-Fi information and an extensible security toolkit on Windows 11, Linux and macOS. The chosen stack is Go + Wails + React + TypeScript + Base UI + Tailwind CSS v4. Go supersedes the earlier Rust recommendation.

Source: https://chatgpt.com/share/6ad626c6-5a54-83e9-837b-064b7a148a55?ogimg=plain

## What Kali tools have underneath

Kali is an integration and distribution ecosystem, not a common application engine. Its catalog points to independently developed tools and packages. Their internals range from OS networking APIs and packet libraries to protocol implementations and subprocess orchestration. A unified desktop must normalize capabilities, dependencies and results instead of assuming a universal Kali API.

Source: https://www.kali.org/tools/

### Nmap: scheduling and inference, beyond socket connections

Nmap's scanning engine handles multiple probe types, host/port parallelism, adaptive round-trip estimates, retries and congestion control. A timeout alone does not establish that a port is closed. A small Go TCP-connect scanner is feasible, but is not feature-equivalent to Nmap's service detection or OS fingerprinting. Recommendation: native Go for a constrained connect diagnostic; an optional Nmap adapter for advanced scans. Parse structured output and preserve tool version and scan settings with each result.

Source: https://nmap.org/book/port-scanning-algorithms.html

### Wireshark/TShark: capture and decoding are separate layers

Capture uses platform facilities through pcap; dissectors interpret captured bytes. TShark supports live capture and saved files, protocol fields and structured output. Capture filters and display filters have different syntax and execution costs. Ring files and stop conditions bound disk use. Recommendation: begin with offline PCAP/PCAPNG import and optional TShark decoding; add live capture with explicit capability checks. Avoid implementing hundreds of protocol decoders for 1.0.

Sources:

- https://www.wireshark.org/docs/man-pages/tshark.html
- https://www.wireshark.org/docs/wsdg_html_chunked/ChapterCapture.html
- https://github.com/wireshark/wireshark

HTTPS payloads remain encrypted without separately supplied session secrets. Observing TLS metadata is not equivalent to reading application content. Packet capture, process attribution and per-process bandwidth accounting also require different mechanisms.

### Gobuster: a useful Go architecture reference

Gobuster is implemented in Go and separates command modes and reusable execution logic. It illustrates how a specialized CLI can expose different discovery operations while reusing concurrency and configuration infrastructure. For this product, bounded workers, request deadlines, cancellation and explicit result models should live in the Go core, shared by CLI and desktop.

Source: https://github.com/OJ/gobuster

### iperf3: active throughput measurement

iperf3 uses a client/server model, with control and traffic flows and JSON output for integration. It measures throughput to a selected endpoint, not an inherent universal internet speed. Recommendation: an optional adapter with duration and bandwidth limits and a local test server. Interface counters provide passive usage, not connection capacity.

Source: https://software.es.net/iperf/invoking.html

## Native Go versus external adapters

| Capability | Proposed approach | Important boundary |
| --- | --- | --- |
| Interfaces, IPv4/IPv6, MAC | Go `net` | Routes, DNS configuration and counters need additional platform APIs |
| TCP connect, DNS, HTTP, certificates | Go core | A TLS certificate check is not a complete vulnerability audit |
| Ping and traceroute | Platform adapters | Privileges and ICMP behavior differ by OS |
| Passive traffic rate | Counter deltas per interface | Handle resets, interface replacement and monotonic elapsed time |
| Advanced port/service discovery | Optional Nmap process | Versioned parser and availability checks |
| Protocol inspection | Optional TShark process | Bounded output, file size, capture and display filters |
| Wi-Fi metadata and saved key | Native platform adapter | User/session permissions and network type determine availability |
| Throughput test | Optional iperf3 process | Requires a suitable endpoint |

Go networking reference: https://pkg.go.dev/net

External execution must use an executable plus an argument array, without a shell. Validate operations and targets in Go. Bound runtime, output and concurrency; cancel descendant processes on each OS. Record missing, unsupported and permission-denied capabilities distinctly. Never expose an arbitrary command executor through Wails bindings.

## Platform feasibility and spikes

| Area | Windows 11 | Linux | macOS |
| --- | --- | --- | --- |
| Wi-Fi | WLAN APIs | NetworkManager/D-Bus where available | Native Wi-Fi APIs; availability depends on OS permissions |
| Saved Wi-Fi key | WLAN profile permissions | NetworkManager secret access and authorization | Keychain; feasibility must be tested on supported macOS releases |
| Capture | Npcap-dependent tool setup | libpcap and capture permissions | BPF/libpcap permissions |
| Native desktop | WebView2 | GTK/WebKit dependencies | System WebKit and packaging requirements |

The table is a feasibility map, not a verified support guarantee. Decide minimum OS versions, then run native spikes. Enterprise Wi-Fi may have no reusable PSK. Password retrieval is an explicit action for the current saved network, with OS authorization; no background collection, persistence, telemetry or log output. Do not promise that a successful API call necessarily returned plaintext.

Sources:

- https://learn.microsoft.com/en-us/windows/win32/api/wlanapi/nf-wlanapi-wlangetprofile
- https://networkmanager.dev/docs/api/latest/gdbus-org.freedesktop.NetworkManager.Settings.Connection.html
- https://developer.apple.com/documentation/security/keychain-services

Keep the UI and ordinary diagnostics unprivileged. If required, design a minimal privileged helper with authenticated local IPC, narrowly typed operations, request limits and platform-specific lifecycle management. Helper design needs its own architecture decision and tests.

## Desktop architecture proposal

Wails connects Go methods and events to a frontend rendered by the OS WebView. Use thin bindings, services in Go and batched metric events rather than sending each captured packet to React. The v2 documentation currently identifies v2.17.0 and states v3 is beta. Proposed baseline: v2 stable, with an explicit review of release status before implementation.

Sources:

- https://wails.io/docs/introduction/
- https://wails.io/docs/gettingstarted/installation/
- https://tailwindcss.com/docs/installation/using-vite

Proposed layout (not yet implemented):

```text
main.go                 Wails entry point
cmd/kali-tools/       headless CLI
internal/app/           shared use cases and job lifecycle
internal/network/       diagnostic domain and contracts
internal/platform/      OS-specific adapters and build constraints
internal/tools/         versioned external tool adapters
internal/storage/       optional history, migrations and retention
frontend/               React, TypeScript, Base UI, Tailwind v4
tests/fixtures/         synthetic captures and parser outputs
tests/integration/      isolated CLI tests
docs/adr/               architecture decisions
.github/workflows/      eventually active workflow location
```

A registry of built-in adapters is sufficient initially. Avoid arbitrary downloadable plugins for alpha. Proposed capability states: available, missing dependency, permission required, unsupported, failed. Jobs need stable IDs, queued/running/completed/failed/cancelled states, bounded event buffers and deterministic cleanup.

## Docker test strategy

Use containers for the headless Go CLI and isolated fixture services: HTTP, DNS, known open/closed TCP ports and iperf3. Use a private Compose network, deterministic assertions and health checks. Prefer non-root containers and no published ports unless required. Docker Desktop networking differs from native host networking.

Docker does not validate the actual Windows/macOS Wi-Fi adapter, credential store, capture driver, elevation flow or Wails WebView. Those require native tests and a documented manual acceptance matrix. Avoid privileged containers or host networking as the default test environment.

Source: https://docs.docker.com/engine/network/

## Reference repository inspected

Repository: https://github.com/AlexGalhardo/vukem-marketplace-reactjs

Inspected snapshot: `114790f016dbf2fadc6b6227f4ec44673dbd0be0` (shallow clone). Read package.json, README, AGENTS.md, EditorConfig, Lefthook, git-cliff and CI configuration. Its agent rules describe that repository; they are reference material, not instructions for this project.

Adopt: Bun with committed lockfile and exact dependency versions; TypeScript; Base UI; Tailwind v4 Vite plugin; Biome; Go formatting/lint/vet/race/vulnerability checks; meaningful Playwright accessibility tests; Conventional Commits; SemVer; Keep a Changelog generated through git-cliff; EditorConfig and contribution/security guides.

Adapt: CLI and desktop native build matrix instead of web hosting deployment. Avoid copying marketplace infrastructure such as Redis, MongoDB, payment integrations or Inertia. A local application can start with configuration files and optional SQLite history.

CI/CD requirement: write workflows but leave them inactive. Proposed mechanism: valid workflow source files under `docs/ci/` with a `.yml.disabled` suffix, outside `.github/workflows/`. Manual dispatch alone still registers an active workflow, so it does not satisfy the strict interpretation. Activation is a later explicit move/rename. Pin action references when implementation is prepared; do not blindly copy versions from the reference.

## Release and quality proposal

- Structure: reproducible setup, core contracts, CLI skeleton, desktop shell, dormant CI/CD and Docker lab.
- Alpha (`0.1.0-alpha.1` onward): interfaces, passive metrics, diagnostics and first platform adapters.
- Beta (`0.1.0-beta.1` onward): capture/import, external adapters, permissions, history and three-OS validation.
- Stable (`1.0.0`): agreed capability matrix, native installers, upgrade/recovery checks, dependency notices and documented limitations.
- Only completed, verified deliverables receive `[x]`; planned work stays `[ ]`. Every task must name its assertion and evidence.
- Performance budgets need an explicit workload and baseline before being used as release gates.
- Dependency redistribution, signing and notarization requirements need a separate review before packaging; research here does not establish redistribution rights.

## Pending decisions

See `docs/brainstorming.md`. The final PLAN.md must follow the user's brainstorming answers, not silently treat these proposals as settled scope.
