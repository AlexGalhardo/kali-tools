# Secrets and Payloads in kali-tools

kali-tools has no service credentials, API keys or database passwords. Its sensitive data is:

| Data | Where it appears | Rule |
| --- | --- | --- |
| Wi-Fi keys | Explicit reveal action (Windows WLAN API) | Returned only to the caller that asked; never in history, logs, database, events, errors or crash output. The CLI requires an explicit flag and prints a warning |
| Web-server token | Generated per launch for `serve` | Printed once in a URL fragment; never logged, persisted or echoed |
| Packet payloads | Live capture, PCAP/PCAPNG import | Never in history or logs; packet files are opt-in artifacts written only to an explicit path |
| Diagnostic targets and results | History | Stored as timestamps, parameters, tool version and summary only |

## Hardcoded secrets — Critical

Never commit keys, tokens, certificates or captured traffic. Test fixtures are synthetic.

```go
// ✗ DON'T
const token = "abc123-xyz789-secret-key"

// ✓ Generate per launch
tok := make([]byte, 32)
if _, err := rand.Read(tok); err != nil { // crypto/rand
    return fmt.Errorf("generating token: %w", err)
}
```

## Keeping secrets out of history and logs

The control is type separation, not discipline at each call site:

```go
// Revealed only through this type; it has no JSON-to-storage path.
type RevealedKey struct {
    SSID string
    Key  string `json:"-"` // never marshaled by accident
}

// What history stores has no field that could hold the key.
type HistoryEntry struct {
    ID      string
    Kind    string
    Summary string
}
```

- Redact before logging: log the SSID or the error code, never the key.
- Do not put a secret in an error message, a `fmt.Errorf` cause or a panic value.
- Do not claim guaranteed zeroization of Go strings; keep secret lifetimes short instead.
- Tests assert that a run involving a secret leaves no trace of it in the store, logs, events or DTOs (PLAN H06).

## TLS material for remote exposure

When `serve` runs off-loopback (PLAN W05), the certificate is user-supplied or generated self-signed with its fingerprint displayed. Private keys are read from an explicit path and never copied into the database or logs.

## Secret detection

Before committing, check diffs for key material, tokens and capture files. `.gitignore` already excludes build artifacts; do not add real captures to `tests/fixtures`.

## CWE References

- CWE-798: Use of Hard-coded Credentials
- CWE-532: Insertion of Sensitive Information into Log File
- CWE-312: Cleartext Storage of Sensitive Information
