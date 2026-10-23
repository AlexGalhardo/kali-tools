# Web-server threat model for external review

This is the brief for a reviewer who is not the author. Nobody external has reviewed it. The R02 child stays open until that review is recorded. The operational policy remains [SECURITY.md](../../SECURITY.md).

## Boundary

`kali-tools serve` binds loopback unless the operator passes `--allow-remote` with a TLS certificate and key. The launch token is in the URL fragment. The browser exchanges it for an HttpOnly `SameSite=Strict` cookie. Automation may send `Authorization: Bearer` with the same token. Restarting the process invalidates both. There is no CORS. A cross-site request is rejected. The Host header is allowlisted.

One token is one operator. It is not a multi-user authorization boundary. A token holder can run network probes, discovery, service inspection and packet capture, and can reveal the current user's saved Wi-Fi key.

The UI and the web server are not elevated. There is no privileged helper and no helper IPC ([ADR 011](../adr/011-no-privileged-helper.md)). Capture starts only when the operator starts it. A diagnostic bundle is written only on request, stays on that computer, and omits Wi-Fi keys, scan results and capture packets.

Capability rows come from the server process. They can differ from the desktop user when the server account lacks location consent or capture permission.

## What the reviewer is asked to judge

- Whether the loopback default, fragment token, cookie flags and missing CORS match the routes in the [HTTP contract](../http.md).
- Whether a token holder is described as powerful on purpose, and whether any route grants a second user a narrower right. None does.
- Whether non-loopback binding is refused without both `--allow-remote` and TLS.
- Whether any path persists a Wi-Fi secret or a packet payload in history. The contract says they do not.
- Whether the absent helper matches ADR 011.

## What this note does not do

It does not record a reviewer's name, date or conclusion. It does not close a web-server security gate. Automated Host, Origin and token tests are not a substitute for that review.
