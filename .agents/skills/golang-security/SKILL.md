---
name: golang-security
description: "Security for kali-tools Go code — the project baseline (no shell, argv from validated values, secrets and packet payloads out of history and logs, loopback/Host/token/CrossOriginProtection rules for `kali-tools serve`, no desktop listener), plus injection, crypto, filesystem, cookies, logging, memory safety, STRIDE/DREAD and govulncheck. Use when writing, reviewing or auditing code that touches input, executables, secrets, files, the web server or untrusted bytes."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.2.2"
---

# Go Security

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md`; they win over anything below.

**Persona:** You are a senior Go security engineer. You apply security thinking both when auditing existing code and when writing new code — threats are easier to prevent than to fix.

**Thinking mode:** reason thoroughly for security audits — security bugs hide in subtle interactions.

## Modes

- **Review mode** — reviewing a PR for security issues. Start from the changed files, then trace call sites and data flows into adjacent code — a vulnerability may live outside the diff but be triggered by it. Sequential.
- **Audit mode** — full codebase security scan. Cover independent domains (they can be split across parallel reviewers): (1) command and SQL injection, (2) secrets, payloads and logging, (3) the local web server (loopback, Host, token, CSRF, headers, limits), (4) untrusted parser input and resource bounds, (5) concurrency and dependency vulnerabilities. Aggregate findings, score with DREAD and report by severity. Keep each fix a separate, focused, independently revertible change.
- **Coding mode** — use when writing new code or fixing a reported vulnerability. Follow the skill's sequential guidance. Optionally launch a background agent to grep for common vulnerability patterns in newly written code while the main agent continues implementing the feature.

## Tools

govulncheck v1.9.0 is pinned in CI and Lefthook; never install `@latest`.

## kali-tools security baseline

These rules come from `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md` §8. They are requirements, not suggestions.

### Scope

kali-tools is a defensive diagnostics toolkit. Offensive categories are out of scope (`docs/tools-research.md`). Do not add exploitation, credential attacks or traffic injection.

### Executables

- Never call a shell and never concatenate a command string. Use `exec.CommandContext(ctx, path, argv...)` through `internal/adapter/tools`.
- Resolve executable paths once at bootstrap; never take a path or an argument list from the UI, HTTP or CLI. Build argv from validated domain values only.
- Every run has a deadline, stdout/stderr caps and descendant cancellation (Job Objects on Windows, process groups on Unix).
- Reject targets and options that would be parsed as flags by the tool (values starting with `-`).

### Secrets and payloads

- Wi-Fi keys are read only on an explicit user action and returned to that caller only. They never enter history, logs, the database, errors, events or crash output.
- Packet payloads never enter diagnostic history or logs. Packet files are separate, opt-in artifacts written only to an explicit path.
- The web-server token is never logged, persisted or echoed in a response.
- Type separation is the control: persisted and logged structs have no field that can hold a secret or payload (`docs/architecture-research.md` §6.6). Do not claim guaranteed zeroization of Go strings.

### Local web server (`kali-tools serve`, PLAN W04/W05)

Mandatory before any release that contains `serve`:

1. Bind to `127.0.0.1` or `[::1]` by default. Non-loopback requires an explicit flag, TLS and a visible warning.
2. Host allowlist: only `127.0.0.1:<port>`, `localhost:<port>`, `[::1]:<port>` (DNS rebinding defense).
3. Per-launch 32-byte `crypto/rand` token, printed in a URL fragment (`/#token=...`), exchanged once for an `HttpOnly; SameSite=Strict` cookie (or `Authorization: Bearer`), compared with `subtle.ConstantTimeCompare`, required on every `/api` route including SSE.
4. `http.NewCrossOriginProtection()` on unsafe methods; no state change on GET.
5. No CORS headers.
6. `ReadHeaderTimeout`, `http.MaxBytesReader` on every body, caps on concurrent jobs and SSE subscribers, a Content-Security-Policy and `X-Content-Type-Options: nosniff`. No global `WriteTimeout` (it breaks SSE); use per-write deadlines.
7. No endpoint accepts an arbitrary command, SQL statement or file path.
8. Report capability states honestly in server mode (permission or location consent may differ from the desktop).

```go
func secure(next http.Handler, port int, token []byte) http.Handler {
    allowed := map[string]bool{
        fmt.Sprintf("127.0.0.1:%d", port): true,
        fmt.Sprintf("localhost:%d", port): true,
        fmt.Sprintf("[::1]:%d", port):     true,
    }
    cop := http.NewCrossOriginProtection()
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !allowed[r.Host] {
            http.Error(w, "forbidden host", http.StatusForbidden)
            return
        }
        if strings.HasPrefix(r.URL.Path, "/api/") && !validToken(r, token) {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        cop.Handler(next).ServeHTTP(w, r)
    })
}
```

Tests must reject a foreign Host, a cross-origin POST, a missing or invalid token and an oversized body, and must show that the token never appears in logs, history or responses.

### Desktop

The desktop build must never open a TCP listener; Wails bindings use in-process IPC.

### Untrusted input

PCAP/PCAPNG files, packets, DNS/HTTP/TLS responses and tool output are untrusted. Bound reads, validate length fields, cap counts and retained bytes, and fuzz parsers (see `golang-safety`).

## Overview

Security in Go follows the principle of **defense in depth**: protect at multiple layers, validate all inputs, use secure defaults, and leverage the standard library's security-aware design. Go's type system and concurrency model provide some inherent protections, but vigilance is still required.

## Security Thinking Model

Before writing or reviewing code, ask three questions:

1. **What are the trust boundaries?** — Where does untrusted data enter? In kali-tools: any web page the user visits (CSRF, DNS rebinding against `serve`), HTTP requests, Wails binding arguments, CLI flags, imported PCAP/PCAPNG files, packets on the wire, DNS/HTTP/TLS responses from targets, and Nmap/TShark output
2. **What can an attacker control?** — Which inputs flow into sensitive operations? (subprocess argument arrays, SQL, file paths for import/export, diagnostic targets, parser allocations)
3. **What is the blast radius?** — kali-tools can start network diagnostics, packet capture and reveal Wi-Fi keys, sometimes with administrator rights. A bypass can mean remote-triggered scanning from the user's machine, secret disclosure or RCE through a tool invocation

## Severity Levels

| Level | DREAD | Meaning |
| --- | --- | --- |
| Critical | 8-10 | RCE, full data breach, credential theft — fix immediately |
| High | 6-7.9 | Auth bypass, significant data exposure, broken crypto — fix in current sprint |
| Medium | 4-5.9 | Limited exposure, session issues, defense weakening — fix in next sprint |
| Low | 1-3.9 | Minor info disclosure, best-practice deviations — fix opportunistically |

Levels align with [DREAD scoring](./references/threat-modeling.md).

## Research Before Reporting

Before flagging a security issue, trace the full data flow through the codebase — don't assess a code snippet in isolation.

1. **Trace the data origin** — follow the variable back to where it enters the system. Is it user input, a hardcoded constant, or an internal-only value?
2. **Check for upstream validation** — look for input validation, sanitization, type parsing, or allow-listing earlier in the call chain.
3. **Examine the trust boundary** — if the data never crosses a trust boundary (e.g., internal service-to-service with mTLS), the risk profile is different.
4. **Read the surrounding code, not just the diff** — middleware, interceptors, or wrapper functions may already provide a layer of defense.

**Severity adjustment, not dismissal:** upstream protection does not eliminate a finding — defense in depth means every layer should protect itself. But it changes severity: a SQL concatenation reachable only through a strict input parser is medium, not critical. Always report the finding with adjusted severity and note which upstream defenses exist and what would happen if they were removed or bypassed.

**When downgrading or skipping a finding:** add a brief inline comment (e.g., `// security: SQL concat safe here — input is validated by parseUserID() which returns int`) so the decision is documented, reviewable, and won't be re-flagged by future audits.

## Threat Modeling (STRIDE)

Apply STRIDE to every trust boundary crossing and data flow in your system: **S**poofing (authentication), **T**ampering (integrity), **R**epudiation (audit logging), **I**nformation Disclosure (encryption), **D**enial of Service (rate limiting), **E**levation of Privilege (authorization). Score each threat using DREAD (Damage, Reproducibility, Exploitability, Affected users, Discoverability) to prioritize remediation — Critical (8-10) demands immediate action.

For the full methodology with Go examples, DFD trust boundaries, DREAD scoring, and OWASP Top 10 mapping, see **[Threat Modeling Guide](./references/threat-modeling.md)**.

## Quick Reference

| Severity | Vulnerability | Defense | Standard Library Solution |
| --- | --- | --- | --- |
| Critical | SQL Injection | Parameterized queries separate data from code | `database/sql` with `?` placeholders |
| Critical | Command Injection | Pass args separately, never via shell concatenation | `exec.Command` with separate args |
| High | XSS | Auto-escaping renders user data as text, not HTML/JS | `html/template`, `text/template` |
| High | Path Traversal | Scope untrusted file access to an allowed root | Go 1.24+: use `os.Root`. Pre-Go 1.24: use `filepath.IsLocal` + `filepath.Rel` + separator-aware checks; never rely on `filepath.Clean` + `strings.HasPrefix` alone. |
| Medium | Timing Attacks | Constant-time comparison avoids byte-by-byte leaks | `crypto/subtle.ConstantTimeCompare` |
| High | Crypto Issues | Use vetted algorithms; never roll your own | `crypto/aes`, `crypto/rand` |
| Medium | HTTP Security | TLS + security headers prevent downgrade attacks | `net/http`, configure TLSConfig |
| Low | Missing Headers | HSTS, CSP, X-Frame-Options prevent browser attacks | Security headers middleware |
| Medium | Resource exhaustion | Caps on jobs, subscribers, body size, parser input and subprocess output | `http.MaxBytesReader`, `io.LimitReader`, server timeouts, bounded channels |
| High | Race Conditions | Protect shared state to prevent data corruption | `sync.Mutex`, channels, avoid shared state |

## Detailed Categories

For complete examples, code snippets, and CWE mappings, see:

- **[Cryptography](./references/cryptography.md)** — Algorithms, key derivation, TLS configuration.
- **[Injection Vulnerabilities](./references/injection.md)** — SQL, command, template injection, XSS, SSRF.
- **[Filesystem Security](./references/filesystem.md)** — Path traversal, zip bombs, file permissions, symlinks.
- **[Network/Web Security](./references/network.md)** — SSRF, open redirects, HTTP headers, timing attacks, session fixation.
- **[Cookie Security](./references/cookies.md)** — Secure, HttpOnly, SameSite flags.
- **[Memory Safety](./references/memory-safety.md)** — Integer overflow, memory aliasing, `unsafe` usage.
- **[Secrets and Payloads](./references/secrets.md)** — Wi-Fi keys, the web-server token, packet payloads, history and redaction.
- **[Logging Security](./references/logging.md)** — PII in logs, log injection, sanitization.
- **[Threat Modeling Guide](./references/threat-modeling.md)** — STRIDE, DREAD scoring, trust boundaries, OWASP Top 10.
- **Local web server** — the mandatory baseline is below and in `docs/architecture-research.md` §8.

## Code Review Checklist

For the full security review checklist organized by domain (input handling, database, crypto, web, auth, errors, dependencies, concurrency), see **[Security Review Checklist](./references/checklist.md)** — a comprehensive checklist for code review with coverage of all major vulnerability categories.

## Tooling & Verification

### Static Analysis & Linting

Security-relevant linters enabled in `.golangci.yml`: `bodyclose`, `sqlclosecheck`, `rowserrcheck`, `nilerr`, `errcheck`, `errorlint`, `govet`, `staticcheck`. `gosec` is not enabled; running it ad hoc at a pinned version is fine, adopting it is a configuration change. See the `golang-lint` skill.

```bash
govulncheck ./...               # pinned v1.9.0
govulncheck -tags desktop ./...
```

### Security Testing

```bash
# Race detector
go test -race ./...

# Parser fuzz smoke (PCAP and Nmap XML)
node scripts/task.mjs fuzz
```

## Common Mistakes

| Severity | Mistake | Fix |
| --- | --- | --- |
| High | `math/rand` for tokens | Output is predictable — attacker can reproduce the sequence. Use `crypto/rand` |
| Critical | SQL string concatenation | Attacker can modify query logic. Parameterized queries keep data and code separate |
| Critical | `exec.Command("bash -c")` | Shell interprets metacharacters (`;`, `\|`, `` ` ``). Pass args separately to avoid shell parsing |
| High | Trusting unsanitized input | Validate at trust boundaries — internal code trusts the boundary, so catching bad input there protects everything |
| Critical | Hardcoded secrets | Secrets in source end up in history, logs and backups. kali-tools has no service credentials; the only secrets are Wi-Fi keys (read on explicit request) and the per-launch token (generated at runtime) |
| Medium | Comparing secrets with `==` | `==` short-circuits on first differing byte, leaking timing info. Use `crypto/subtle.ConstantTimeCompare` |
| Medium | Returning detailed errors | Stack traces and DB errors help attackers map your system. Return generic messages, log details server-side |
| High | Ignoring `-race` findings | Races cause data corruption and can bypass authorization checks under concurrency. Fix all races |
| High | MD5/SHA1 for passwords | Both have known collision attacks and are fast to brute-force. Use Argon2id or bcrypt (intentionally slow, memory-hard) |
| High | AES without GCM | ECB/CBC modes lack authentication — attacker can modify ciphertext undetected. GCM provides encrypt+authenticate |
| High | Binding `serve` to 0.0.0.0 | Exposes diagnostics and capture to the LAN. Loopback by default; non-loopback needs an explicit flag, TLS and a warning (PLAN W05) |

## Security Anti-Patterns

| Severity | Anti-Pattern | Why It Fails | Fix |
| --- | --- | --- | --- |
| High | Security through obscurity | Hidden URLs are discoverable via fuzzing, logs, or source | Authentication + authorization on all endpoints |
| High | Trusting client headers | `X-Forwarded-For`, `X-Is-Admin` are trivially forged | Server-side identity verification |
| High | Client-side authorization | JavaScript checks are bypassed by any HTTP client | Server-side permission checks on every handler |
| Critical | Ignoring crypto errors | `_, _ = encrypt(data)` silently proceeds unencrypted | Always check errors — fail closed, never open |
| Critical | Rolling your own crypto | Custom encryption hasn't been analyzed by cryptographers | Use `crypto/aes` GCM, `golang.org/x/crypto/argon2` |

See `docs/architecture-research.md` §8 and §12 for the project-specific anti-patterns.

## Cross-References

See `golang-database`, `golang-safety`, `golang-error-handling`, `golang-context` and `golang-dependency-management` skills.

## Additional Resources

- [Go Security Best Practices](https://go.dev/doc/security/best-practices)
- [gosec Security Linter](https://github.com/securego/gosec)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)
- [OWASP Go Secure Coding Practices](https://owasp.org/www-project-go-secure-coding-practices-guide/)
