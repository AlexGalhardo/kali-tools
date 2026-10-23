# Security policy

This development alpha has no stable security support commitment yet. Report suspected vulnerabilities privately to the repository owner through GitHub private vulnerability reporting where enabled. Do not publish real passwords, captured packets, tokens or personal diagnostic data in issues.

Input boundaries include Wails methods, diagnostic targets, capture bytes, SQLite migrations and external program output. Diagnostics have deadlines and scope limits. The UI never receives arbitrary SQL or command execution methods. Wi-Fi reveal is explicit, ephemeral and excluded from history. Capture requires an explicit start and is never started on application launch. A diagnostic bundle is written only when the operator asks for it, stays on that computer, and omits Wi-Fi keys, scan results and capture packets.

The application does not elevate itself or install a privileged helper. Windows raw capture reports when administrator authorization is required. Do not treat this alpha as a hardened privileged application; the privilege boundary and native acceptance remain release gates in PLAN.md.

## Web server mode

`kali-tools serve` binds loopback by default. A cryptographically random launch token lives in the printed URL fragment, never a query parameter. The browser exchanges it for an HttpOnly SameSite=Strict cookie; bearer authentication also supports automation. Tokens and cookies must not be shared or recorded. Restarting the process invalidates sessions.

A token holder can run network probes, discovery, service inspection and packet capture and reveal the current user's saved Wi-Fi key. This intentionally powerful surface is restricted by a Host allowlist, Go CrossOriginProtection, Fetch Metadata, no CORS, strict CSP, request deadlines and bounded bodies/concurrency. It is not an authorization boundary between multiple users sharing the token. Non-loopback binding requires explicit `--allow-remote` plus a TLS certificate/key and emits a warning. Validate the printed certificate fingerprint over a trusted channel. Capability states come from the server account, which may lack location consent or capture privileges.

Report security issues using the private reporting procedure above. Include redacted reproduction steps; never send actual launch tokens, cookies, saved keys or packet payloads.
