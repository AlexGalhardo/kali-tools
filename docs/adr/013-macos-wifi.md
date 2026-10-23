# ADR 013: macOS routes and Wi-Fi metadata

Status: accepted on 2026-10-23. The hardware result is still open.

Context: B02 needs routes, counters and current Wi-Fi metadata on Apple Silicon, plus a Keychain spike. ADR 007 already fixed the baseline: macOS 13 or later, arm64, cgo, ad-hoc signature. An empty CoreWLAN SSID is not a success.

Decision:

- Read interfaces, default-route gateways and nameservers through the routing sysctl and `/etc/resolv.conf`. Read interface byte counters from `RTM_IFINFO2`. Neighbors stay unsupported.
- Read the current SSID, BSSID and channel through CoreWLAN when cgo is enabled. A missing interface is `unsupported`. An empty network name is `location_consent_required`.
- Leave the shared signal field unset when CoreWLAN returns dBm. That field is a percent on Windows. Do not invent a conversion.
- Look up a saved key as an AirPort generic password and set `kSecUseAuthenticationUIFail`, so the call does not prompt. A missing entitlement is `unsupported`. A result that would have prompted is `permission_required`. A missing item is `no_saved_key`.
- A build without cgo keeps the explicit unsupported states. It does not pretend to have read CoreWLAN.

Consequences: native macOS CI compiles the cgo file. `CGO_ENABLED=0 GOOS=darwin go vet` compiles the fallback. Neither closes B02. The Apple Silicon runbook records the SSID, the Location Services prompt and the redacted Keychain status.

Revisit only if the ad-hoc signature cannot obtain Location Services or the AirPort item name is wrong on the hardware Alex tests. Do not add a prompt or a privileged helper to make the lookup succeed.
