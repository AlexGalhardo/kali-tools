# General Debugging Methodology

For any bug, follow this systematic process:

## Table of Contents

- [Step 1: Understand Expected vs Actual](#step-1-understand-expected-vs-actual)
- [Step 2: Get the Full Error](#step-2-get-the-full-error)
- [Step 3: Isolate the Problem](#step-3-isolate-the-problem)
- [Step 4: Check External Dependencies](#step-4-check-external-dependencies)
- [Step 5: Check Local Evidence](#step-5-check-local-evidence)
- [Step 6: Compare with Working Code](#step-6-compare-with-working-code)
- [Step 7: Form a Hypothesis and Test It](#step-7-form-a-hypothesis-and-test-it)
- [Step 8: Trace to Root Cause](#step-8-trace-to-root-cause)
- [Step 9: Fix and Verify](#step-9-fix-and-verify)
- [Step 10: Defense-in-Depth](#step-10-defense-in-depth)
- [When You're Stuck: Escalation Protocol](#when-youre-stuck-escalation-protocol)

## Step 1: Understand Expected vs Actual

Before touching code, articulate clearly:

- What **should** happen?
- What **actually** happens?
- What **changed** recently?

```bash
# What changed recently?
git log --oneline -20
git diff HEAD~5

# Binary search for the breaking commit
git bisect start
git bisect bad          # current commit is broken
git bisect good abc123  # this commit was working
# git bisect will walk you to the breaking commit
```

## Step 2: Get the Full Error

```bash
# Full build errors
go build ./... 2>&1

# Verbose test output
go test ./... -v 2>&1

# Static analysis
go vet ./...

# Run linters — see the golang-lint skill for configuration
golangci-lint run ./...
```

Run `golangci-lint` early in your debugging workflow. It catches unchecked errors, suspicious constructs, and many other issues that are easy to miss by reading code. See the `golang-lint` skill for configuration and usage.

## Step 3: Isolate the Problem

Narrow the scope before investigating deeper:

```bash
# Does a single test fail?
go test -run TestSpecificName -v ./pkg/...

# Does it fail without cache?
go test -count=1 -run TestSpecificName ./pkg/...

# Is it a specific package?
go build ./pkg/suspect/...

# Is it flaky? Run multiple times
go test -count=10 -run TestSuspect ./pkg/...
```

Write more tests if you suspect missing test cases or need to test something in different conditions.

## Step 4: Check External Dependencies

Sometimes the bug is not in kali-tools code. Before diving deeper, check what the code depends on:

```bash
# Capabilities and optional tool inventory as the app sees them (JSON output)
kali-tools doctor
nmap --version
tshark --version

# Host routes, DNS servers and interface inventory
kali-tools inventory

# Connectivity to a diagnostic target, outside the app (lab or loopback only)
nc -zv 127.0.0.1 443
openssl s_client -connect 127.0.0.1:443 -brief
```

Command names above follow the current CLI; check `kali-tools` usage if they change.

### Common external causes in kali-tools

- Missing privilege: raw capture and some ICMP paths need administrator rights; the capability state should say `permission_required`
- Optional tool missing or outside the recognized version range (Nmap 7.80+ within major 7; TShark 4.2+ within major 4)
- Windows location consent not granted for Wi-Fi metadata, or the process runs under another account
- Firewall or antivirus blocking raw sockets, ICMP or the lab network
- DNS resolver differences between Windows, Linux and macOS (`adapter/platform` reads them differently)
- Docker lab not running or `LAB_FIXTURE` unset for integration tests
- WebView2 runtime or Wails bindings out of date for the desktop build
- SQLite file locked by a second instance, or created by a newer schema version
- Disk full or the data directory not writable

Never print secrets while checking configuration: do not dump Wi-Fi profiles with keys, the web-server token or environment variables wholesale.

## Step 5: Check Local Evidence

kali-tools has no remote observability stack. The evidence is local:

- The job history (state, timestamps, parameters, tool version, summary) through the UI or CLI.
- The capability states reported by `doctor` and the UI.
- Bounded, redacted logs on stderr from the CLI or `serve`.
- Recorded verification evidence in `docs/verification/` for what was proven on real hardware before.

If none of that explains the failure, reproduce it in a test or in the Docker lab.

## Step 6: Compare with Working Code

Before forming a hypothesis, find similar code that **works**:

- Search the codebase for analogous functionality that doesn't have the bug
- Read the working reference implementation **completely** — don't skim
- List **every difference** between the working code and the broken code
- Check: are the dependencies the same? The config? The initialization order? The error handling?

Often the bug becomes obvious when you see what the working version does differently.

## Step 7: Form a Hypothesis and Test It

- Form a **single, specific** hypothesis with clear reasoning
- Add targeted logging or a focused test
- Change **one thing**, observe, confirm or reject
- If the hypothesis was wrong, **revert the change** — don't stack fixes on top of failed attempts

## Step 8: Trace to Root Cause

When the symptom appears deep in the call stack, don't fix where the error surfaces. Trace backward:

1. **Find the immediate cause** — what line panics or returns the wrong value?
2. **Ask "what called this?"** — trace one level up the call chain
3. **Keep tracing** — repeat until you find where the invalid data **originated**, not where it was **consumed**
4. **Fix at the source** — the fix belongs where the bad value was created, not where it caused a crash

```go
// Example: panic in handler — but the bug is in the constructor
// ✗ Bad — fixing at the symptom
func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
    if s.db == nil {  // nil check masks the real bug
        http.Error(w, "db unavailable", 500)
        return
    }
    // ...
}

// ✓ Good — fixing at the source
func NewServer(db *sql.DB) *Server {
    if db == nil {
        panic("NewServer: db must not be nil")  // fail fast at construction
    }
    return &Server{db: db}
}
```

When you can't trace manually, add temporary instrumentation:

```go
// Log the full call chain before the dangerous operation
func suspectFunction(val string) {
    fmt.Fprintf(os.Stderr, "DEBUG suspectFunction: val=%q\n%s\n", val, debug.Stack())
    // ...
}
```

## Step 9: Fix and Verify

- Fix the root cause, not the symptom
- The failing test from step 1 should now pass
- Run the full test suite to check for regressions

## Step 10: Defense-in-Depth

After fixing a bug, ask: "How do I make this bug structurally impossible?" A single fix at one layer can be bypassed by different code paths or future refactoring. Add validation at multiple layers:

1. **Entry point** — reject invalid input at public API boundaries (`New*` constructors, exported functions)
2. **Business logic** — assert preconditions inside internal functions that receive the data
3. **Runtime guards** — use build tags or env checks to catch dangerous operations in tests (e.g., refuse writes outside temp dirs)
4. **Observability** — add structured logging or metrics so the same class of bug is instantly visible if it recurs

Not every fix needs all four layers — use judgment. But when a bug could cause data loss, corruption, or security issues, multi-layer defense is worth the cost.

## When You're Stuck: Escalation Protocol

If your fix doesn't work:

- **< 3 failed attempts:** Return to Step 1. You misidentified the root cause. Gather more evidence.
- **>= 3 failed attempts:** Stop fixing. The problem is likely architectural, not a simple bug. Step back and question your assumptions about how the system works. Ask: "Is the design fundamentally sound, or am I patching a broken abstraction?"
- **Each fix reveals a new problem:** You're chasing symptoms, not the root cause. See the Red Flags section in [SKILL.md](../SKILL.md).
