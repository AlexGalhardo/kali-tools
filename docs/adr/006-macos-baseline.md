# ADR 006: macOS 1.0 baseline

Status: accepted on 2026-10-23 by the repository owner (Alex Galhardo).

Context: B02 needs a supported macOS baseline and a native bridge before any Apple Silicon adapter is written. Go 1.27 already requires macOS 13. The 1.0 hardware-acceptance budget is one macOS target.

Decision:

- Support macOS 13 Ventura or later, Apple Silicon only (`darwin/arm64`).
- Do not ship Intel (`darwin/amd64`) or universal binaries for 1.0.
- The native bridge is cgo with a thin Objective-C layer (CoreWLAN and the other Apple frameworks the adapter needs). There is no separate Swift helper.
- The macOS build keeps the ad-hoc signature the Go linker applies (`codesign -s -`). That signature lets an arm64 binary run. It is not a Developer ID signature and it is not notarization. See [ADR 008](008-signing-and-provenance.md).

Consequences: CI and release assembly publish `macos-arm64` only. `GOOS=darwin` vet stays. Packaging refuses a non-arm64 macOS host. Native macOS acceptance is still open. Cross-compilation does not close it.

B02 verification, still open: on real Apple Silicon hardware, check whether an ad-hoc-signed `.app` can read the CoreWLAN SSID and BSSID and whether Location Services prompts. Since Sonoma those values are empty without Location Services authorization, and only a signed app bundle is a realistic prompt target. If the ad-hoc signature cannot obtain that authorization, the capability must be an explicit state, not an empty success. This check is not accepted from documentation or from a Linux cross-compile.

Revisit when 1.0 hardware acceptance is finished and a later release needs Intel, or when the ad-hoc signature proves insufficient for CoreWLAN and the owner reopens the signing decision.
