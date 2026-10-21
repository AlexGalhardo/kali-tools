# Cookie Security Rules

Cookie security is critical for preventing session hijacking and XSS exploitation.

## Rules

1. Cookies MUST set `HttpOnly` for session and authentication cookies.
2. Cookies MUST set `Secure` in production (HTTPS only).
3. `SameSite` SHOULD be `Lax` or `Strict` — use `None` only when cross-site access is required.

---

## Table of Contents

- [HTTP-Only Flag Missing — Medium](#http-only-flag-missing--medium)
- [Insecure Cookie Configuration (Missing Secure Flag) — Medium](#insecure-cookie-configuration-missing-secure-flag--medium)
- [SameSite Cookie Protection — Medium](#samesite-cookie-protection--medium)
- [Cookie Prefix Examples — Low](#cookie-prefix-examples--low)
- [kali-tools: token-exchange cookie — High](#kali-tools-token-exchange-cookie--high)
- [Cookie Best Practices Checklist](#cookie-best-practices-checklist)
- [CWE References](#cwe-references)

## HTTP-Only Flag Missing — Medium

Without HttpOnly flag, cookies can be accessed via JavaScript.

### Bad

```go
cookie := &http.Cookie{
    Name:  "session",
    Value: sessionID,
    // DON'T: Missing HttpOnly, Secure flags
}
```

### Good

```go
cookie := &http.Cookie{
    Name:     "session",
    Value:    sessionID,
    HttpOnly: true,   // Prevents JavaScript access
    Secure:   true,   // Only sends over HTTPS
    SameSite: http.SameSiteStrictMode,
    Path:     "/",
    MaxAge:   3600,
}
```

---

## Insecure Cookie Configuration (Missing Secure Flag) — Medium

Without Secure flag, cookies are sent over unencrypted HTTP.

### Bad

```go
http.SetCookie(w, &http.Cookie{
    Name:  "auth_token",
    Value: token,
    // Missing Secure, HttpOnly flags
})
```

### Good

```go
http.SetCookie(w, &http.Cookie{
    Name:     "auth_token",
    Value:    token,
    Secure:   true,   // HTTPS only
    HttpOnly: true,   // No JavaScript access
    SameSite: http.SameSiteLaxMode,
    Path:     "/",
    MaxAge:   86400,
    Domain:   "",  // Default: send to exact host only
})
```

---

## SameSite Cookie Protection — Medium

SameSite attribute protects against CSRF attacks.

### Bad

```go
cookie := &http.Cookie{
    Name:     "session",
    Value:    token,
    Secure:   true,
    HttpOnly: true,
    // DON'T: Missing SameSite
}
```

### Good

```go
// Strict for high-security operations
authCookie := &http.Cookie{
    Name:     "auth",
    Value:    token,
    Secure:   true,
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
}

// Lax for most applications
sessionCookie := &http.Cookie{
    Name:     "session",
    Value:    token,
    Secure:   true,
    HttpOnly: true,
    SameSite: http.SameSiteLaxMode,
}

// None for cross-site cookies (requires Secure: true)
crossSiteCookie := &http.Cookie{
    Name:     "analytics",
    Value:    trackingID,
    Secure:   true,
    HttpOnly: true,
    SameSite: http.SameSiteNoneMode,
}
```

---

## Cookie Prefix Examples — Low

Modern cookie prefixes enforce cookie behavior in browsers.

```go
// __Secure- prefix: Requires Secure flag
secureCookie := &http.Cookie{
    Name:     "__Secure-Session",
    Value:    token,
    Secure:   true,   // Required for __Secure-
    HttpOnly: true,
}

// __Host- prefix: Requires Secure, no Domain, origin-bound path
hostCookie := &http.Cookie{
    Name:     "__Host-CSRF",
    Value:    csrfToken,
    Secure:   true,    // Required
    HttpOnly: true,
    Domain:   "",      // Must be empty
    Path:     "/",     // Required
}
```

---

## kali-tools: token-exchange cookie — High

`kali-tools serve` uses one cookie: the session created when the SPA exchanges the per-launch token from the URL fragment. There is no session library.

```go
http.SetCookie(w, &http.Cookie{
    Name:     "kali_session",
    Value:    sessionValue, // random per launch; never logged
    Path:     "/",
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
    Secure:   tlsEnabled, // true whenever the server runs with TLS (required off-loopback)
})
```

- Compare session values with `subtle.ConstantTimeCompare`.
- The cookie is required on every `/api` route, including `/api/events` (SSE cannot send headers).
- The session ends when the process exits; nothing is persisted.

---

## Cookie Best Practices Checklist

- [ ] Set `HttpOnly: true` for all authentication cookies
- [ ] Set `Secure: true` for all cookies over HTTPS
- [ ] Set appropriate `SameSite` value (Strict/Lax/None)
- [ ] Use short `MaxAge` expiration
- [ ] Avoid setting cookie `Domain` unless necessary
- [ ] Validate cookie values on every request
- [ ] Use cryptographically signed cookies
- [ ] Rotate cookie secrets regularly
- [ ] Clear cookies on logout
- [ ] Use double-submit cookie pattern for CSRF protection

---

## CWE References

- **CWE-1004**: Sensitive Cookie Without 'HttpOnly' Flag
- **CWE-614**: Sensitive Cookie in HTTPS Session Without 'Secure' Attribute
- **CWE-352**: Cross-Site Request Forgery (CSRF)
- **CWE-285**: Improper Authorization
- **CWE-565**: Reliance on Cookies without Validation
