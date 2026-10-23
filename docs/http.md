# HTTP contract

`kali-tools serve` exposes this API and the embedded frontend. The CLI startup flags are in the [CLI contract](cli.md). Threat boundaries are in [SECURITY.md](../SECURITY.md). Response JSON uses the types in `internal/adapter/contract`. This document freezes routes, authentication, body limits, error statuses and capability honesty. It does not freeze a platform matrix.

## Authentication

`POST /api/session` exchanges the launch token from the printed URL fragment for an HttpOnly `SameSite=Strict` cookie. The cookie name is `kali_session_` plus the listen port. TLS adds `Secure`. Automation may send `Authorization: Bearer` with that same launch token. Restarting the process invalidates both.

Every other `/api/` route requires that cookie or bearer. A missing or wrong credential is `401` `unauthorized` with message `session required`. `GET /healthz` and the site root (embedded files) are public. They still pass the Host allowlist.

There is no CORS. A cross-site request is `403` `forbidden`. One token is one operator: it is not a multi-user authorization boundary.

## Routes

| Route | Session | Success |
| --- | --- | --- |
| `POST /api/session` | no | 204 |
| `GET /healthz` | no | 200 `{"status":"ok"}` |
| `GET /api/version` | yes | 200 |
| `GET /api/capabilities` | yes | 200 |
| `GET /api/capture/backends` | yes | 200 |
| `GET /api/interfaces` | yes | 200 |
| `GET /api/neighbors` | yes | 200 |
| `GET /api/connections` | yes | 200 |
| `POST /api/throughput` | yes | 200 |
| `GET /api/inventory` | yes | 200 |
| `GET /api/tools` | yes | 200 |
| `GET /api/jobs/{id}` | yes | 200 |
| `GET /api/history` | yes | 200 |
| `GET /api/settings` | yes | 200 |
| `GET /api/wifi` | yes | 200 |
| `POST /api/diagnose` | yes | 200 |
| `POST /api/tls/audit` | yes | 200 |
| `POST /api/dns/inspect` | yes | 200 |
| `POST /api/discovery/local` | yes | 200 |
| `POST /api/snmp` | yes | 200 |
| `POST /api/diagnostics` | yes | 202 |
| `POST /api/jobs/{id}/cancel` | yes | 200 |
| `GET /api/events` | yes | 200 text event stream |
| `DELETE /api/history` | yes | 204 |
| `PUT /api/settings` | yes | 204 |
| `POST /api/wifi/reveal` | yes | 200 |
| `POST /api/discovery` | yes | 200 |
| `POST /api/inspect` | yes | 200 |
| `POST /api/capture/start` | yes | 202 |
| `POST /api/capture/stop` | yes | 204 |
| `POST /api/capture/import` | yes | 200 |
| `GET /api/capture/export` | yes | 200 capture bytes |
| `GET /api/doctor/bundle` | yes | 200 zip bytes |

`GET /api/events` is the job event stream. A finished job remains available at `GET /api/jobs/{id}`.

`POST /api/wifi/reveal` sets `Cache-Control: no-store`. The saved key is not written to history.

`POST /api/capture/import` requires `Content-Type: application/octet-stream`. `GET /api/capture/export` with query `format=pcap` or `format=pcapng` returns that file with `Cache-Control: no-store`. The server does not accept a filesystem path.

`GET /api/doctor/bundle` returns `application/zip` with `Cache-Control: no-store` and filename `kali-tools-diagnostic.zip`. The body is the same local archive as `kali-tools doctor --bundle`. The server does not upload it.

`GET /api/connections` returns the socket table. Windows and macOS are `unsupported`. An unreadable Linux socket table is `permission_required`. The JSON has no interface counters and no rate.

`POST /api/throughput` accepts `duration_ms` (1 through 10000) or `iperf_json` (an iperf3 `-J` document), not both. The response fields are `endpoint`, `duration_ms`, `byte_count` and `source`. That is not a bandwidth estimate. The server does not execute iperf3. A loopback measurement uses `127.0.0.1` and stops when the request context is cancelled.

`POST /api/snmp` reads a community or SNMPv3 secret from the JSON body and does not log that body.

## Body limits

Default JSON body limit: 64 KiB.

`POST /api/session` body limit: 1 KiB.

`POST /api/tls/audit` body limit: 4 KiB.

`POST /api/dns/inspect` body limit: 8 KiB.

`POST /api/discovery/local` body limit: 4 KiB.

`POST /api/snmp` body limit: 4 KiB.

`POST /api/capture/import` body limit: 64 MiB.

JSON requests must use `Content-Type: application/json` and must not contain unknown fields. A larger body is `413` `invalid_input`. A wrong media type is `415` `invalid_input`.

## Settings

`GET /api/settings` and `PUT /api/settings` use `language` and `retention_days`. Language is `en` or `pt-BR`. Retention is 1 through 365 days. A new database defaults to `en` and 30 days. Other values are `400` `invalid_input` and are not echoed back.

## Errors

Failure bodies are `{"error":{"code","message"}}`. `location_consent_required` may add `error.settings_uri`. Statuses:

| Code | Status |
| --- | --- |
| `invalid_input` | 400 |
| `unauthorized` | 401 |
| `forbidden` | 403 |
| `permission_required` | 403 |
| `location_consent_required` | 403 |
| `not_found` | 404 |
| `conflict` | 409 |
| `dependency_missing` | 424 |
| `cancelled` | 499 |
| `unsupported` | 501 |
| `no_connection` | 502 |
| `certificate_invalid` | 502 |
| `timeout` | 504 |
| `failed` | 500 |

An unknown code is `500`. Messages are bounded and do not include secrets or rejected input.

## Unavailable capabilities

`GET /api/capabilities` and `GET /api/capture/backends` describe the server process. A desktop build on the same machine can differ when the server account lacks location consent or capture permission. The UI shows these states. It does not replace them with the desktop's.

`unsupported` means that operating system has no provider. `permission_required` means the provider exists and the process was denied. Linux and macOS Wi-Fi and live capture stay `unsupported` until their native providers exist. That is an honest state, not a missing route.

## Compatibility

Within this document, a new route or a new response field is additive. Removing a route, rejecting a previously accepted request field, retyping a response field, or changing an error status requires a changelog entry before release. Request bodies already reject unknown fields, so clients must not send extra keys.
