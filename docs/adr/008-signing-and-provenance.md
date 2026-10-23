# ADR 008: Signing and provenance

Status: accepted on 2026-10-23 by the repository owner (Alex Galhardo).

Context: R03 asked whether 1.0 artifacts would be signed. The owner does not want any involvement with Apple or Microsoft, now or for 1.0.

Decision:

- No Apple Developer Program membership, Developer ID signing, or notarization.
- No Windows Authenticode, EV, or Azure Trusted Signing certificates.
- Releases ship unsigned by vendor certificates. `BUILD.json` keeps `signed: false`. Development drafts keep the unsigned-preview status note.
- macOS arm64 binaries keep the ad-hoc signature the Go linker applies (`codesign -s -`). That signature is required for arm64 to run and needs no Apple account. It is not a vendor signature. See [ADR 006](006-macos-baseline.md).
- Every release artifact gets a SHA-256 checksum in `SHA256SUMS`.
- The release workflow attests those artifacts with `actions/attest-build-provenance` (Sigstore-backed, no paid certificate). That attestation is the R03 provenance mechanism.

Consequences: SmartScreen and Gatekeeper will warn on first launch. The README tells Windows users to choose More info, then Run anyway, and macOS users to use System Settings > Privacy & Security > Open Anyway, or `xattr -d com.apple.quarantine` on the `.app`. Linux has no equivalent prompt. No release tag is created by this decision.

Revisit only if the owner explicitly withdraws the no-Apple and no-Microsoft rule. Do not add a signing certificate, notarization step, or SmartScreen-reputation task on the way to 1.0.
