---
name: golang-naming
description: "Go naming conventions for kali-tools — packages named for capabilities, layers and adapter technologies, no stutter, consumer-named ports, MixedCaps identifiers, receivers, errors, enums, tests, and stable wire names (JSON tags, error codes, job and capability states). Use when naming or renaming Go identifiers or packages, or reviewing names."
license: MIT
metadata:
  author: samber
  upstream: https://github.com/samber/cc-skills-golang
  version: "1.2.1"
---

# Go Naming Conventions

Adapted for kali-tools from AlexGalhardo/vukem@cf7e016eacfa17d184af364f820e59974a799b75 (originally samber/cc-skills-golang, MIT). Binding project rules are in `AGENTS.md`, `docs/architecture.md` and `docs/architecture-research.md`; they win over anything below.

Go favors short, readable names. Capitalization controls visibility — uppercase is exported, lowercase is unexported. All identifiers MUST use MixedCaps, NEVER underscores.

> "Clear is better than clever." — Go Proverbs
>
> "Design the architecture, name the components, document the details." — Go Proverbs

To ignore a rule, just add a comment to the code.

## Quick Reference

| Element | Convention | Example |
| --- | --- | --- |
| Package | lowercase, single word, \_test suffix OK for test files | `json`, `http`, `tabwriter`, `http_test` |
| File | lowercase, underscores OK | `user_handler.go` |
| Exported name | UpperCamelCase | `ReadAll`, `HTTPClient` |
| Unexported | lowerCamelCase | `parseToken`, `userCount` |
| Interface | method name + `-er` | `Reader`, `Closer`, `Stringer` |
| Struct | MixedCaps noun | `Request`, `FileHeader` |
| Constant | MixedCaps (not ALL_CAPS) | `MaxRetries`, `defaultTimeout` |
| Receiver | 1-2 letter abbreviation | `func (s *Server)`, `func (b *Buffer)` |
| Error variable | `Err` prefix | `ErrNotFound`, `ErrTimeout` |
| Error type | `Error` suffix | `PathError`, `SyntaxError` |
| Constructor | `New` (single type) or `NewTypeName` (multi-type) | `ring.New`, `http.NewRequest` |
| Boolean field | `is`, `has`, `can` prefix on **fields** and methods | `isReady`, `IsConnected()` |
| Test function | `Test` + function name | `TestParseToken` |
| Acronym | all caps or all lower | `URL`, `HTTPServer`, `xmlParser` |
| Variant: context | `WithContext` suffix | `FetchWithContext`, `QueryContext` |
| Variant: in-place | `In` suffix | `SortIn()`, `ReverseIn()` |
| Variant: error | `Must` prefix | `MustParse()`, `MustLoadConfig()` |
| Option func | `With` + field name | `WithPort()`, `WithLogger()` |
| Enum (iota) | type name prefix, zero-value = unknown | `StatusUnknown` at 0, `StatusReady` |
| Named return | descriptive, for docs only | `(n int, err error)` |
| Error string | lowercase (incl. acronyms), no punctuation | `"image: unknown format"`, `"invalid id"` |
| Import alias | short, only on collision | `mrand "math/rand"`, `pb "app/proto"` |
| Format func | `f` suffix | `Errorf`, `Wrapf`, `Logf` |
| Test table fields | `got`/`expected` prefixes | `input string`, `expected int` |

## MixedCaps

All Go identifiers MUST use `MixedCaps` (or `mixedCaps`). NEVER use underscores in identifiers — the only exceptions are test function subcases (`TestFoo_InvalidInput`), generated code, and OS/cgo interop. This is load-bearing, not cosmetic — Go's export mechanism relies on capitalization, and tooling assumes MixedCaps throughout.

```go
// ✓ Good
MaxPacketSize
userCount
parseHTTPResponse

// ✗ Bad — these conventions conflict with Go's export mechanism and tooling expectations
MAX_PACKET_SIZE   // C/Python style
max_packet_size   // snake_case
kMaxBufferSize    // Hungarian notation
```

## Avoid Stuttering

Go call sites always include the package name, so repeating it in the identifier wastes the reader's time — `http.HTTPClient` forces parsing "HTTP" twice. A name MUST NOT repeat information already present in the package name, type name, or surrounding context.

```go
// Good — clean at the call site
http.Client       // not http.HTTPClient
json.Decoder      // not json.JSONDecoder
user.New()        // not user.NewUser()
config.Parse()    // not config.ParseConfig()

// In package sqldb:
type Connection struct{}  // not DBConnection — "db" is already in the package name

// Anti-stutter applies to ALL exported types, not just the primary struct:
// In package dbpool:
type Pool struct{}        // not DBPool
type Status struct{}      // not PoolStatus — callers write dbpool.Status
type Option func(*Pool)   // not PoolOption
```

## Frequently Missed Conventions

These conventions are correct but non-obvious — they are the most common source of naming mistakes:

**Constructor naming:** When a package exports a single primary type, the constructor is `New()`, not `NewTypeName()`. This avoids stuttering — callers write `apiclient.New()` not `apiclient.NewClient()`. Use `NewTypeName()` only when a package has multiple constructible types (like `http.NewRequest`, `http.NewServeMux`).

**Boolean struct fields:** Unexported boolean fields MUST use `is`/`has`/`can` prefix — `isConnected`, `hasPermission`, not bare `connected` or `permission`. The exported getter keeps the prefix: `IsConnected() bool`. This reads naturally as a question and distinguishes booleans from other types.

**Error strings are fully lowercase — including acronyms.** Write `"invalid message id"` not `"invalid message ID"`, because error strings are often concatenated with other context (`fmt.Errorf("parsing token: %w", err)`) and mixed case looks wrong mid-sentence. Sentinel errors should include the package name as prefix: `errors.New("apiclient: not found")`.

**Enum zero values:** Always place an explicit `Unknown`/`Invalid` sentinel at iota position 0. A `var s Status` silently becomes 0 — if that maps to a real state like `StatusReady`, code can behave as if a status was deliberately chosen when it wasn't.

**Subtest names:** Table-driven test case names in `t.Run()` should be fully lowercase descriptive phrases: `"valid id"`, `"empty input"` — not `"valid ID"` or `"Valid Input"`.

## kali-tools naming

- Packages are named for what they provide: capabilities (`network`, `capture`, `wifi`), layers (`domain`, `app`, `job`, `bootstrap`, `porttest`) and adapters by technology (`adapter/storage`, `adapter/tools`, `adapter/probe`, `adapter/platform`, `adapter/httpapi`, `adapter/wailsui`, `adapter/cli`). Never `util`, `common`, `misc`, `types`, `models`, `interfaces`, `ports`, `services` or `controllers`.
- Avoid stutter: `scan.Result`, not `scan.ScanResult`; `job.NewManager`, not `job.NewJobManager`; `network.Result`, not `network.NetworkResult`.
- Ports are named for the role the consumer needs (`JobStore`, `Prober`, `HostNetwork`, `ToolRunner`), never `IFoo` or `FooInterface`.
- Platform files use the `_windows.go`, `_linux.go`, `_other.go` suffixes beside a shared constructor and method set.
- Wire-format names are a contract, not Go identifiers: JSON tags, `schema_version`, error codes (`invalid_input`, `unsupported`), job states and capability states stay stable and snake_case where already defined. Renaming one is a breaking change for the UI, HTTP and CLI and needs a PLAN task.
- Identifiers, comments and error text are in English.

## Detailed Categories

For complete rules, examples, and rationale, see:

- **[Packages, Files & Import Aliasing](./references/packages-files.md)** — Package naming (single word, lowercase, no plurals), file naming conventions, import alias patterns (only use on collision to avoid cognitive load), and directory structure.

- **[Variables, Booleans, Receivers & Acronyms](./references/identifiers.md)** — Scope-based naming (length matches scope: `i` for 3-line loops, longer names for package-level), single-letter receiver conventions (`s` for Server), acronym casing (URL not Url, HTTPServer not HttpServer), and boolean naming patterns (isReady, hasPrefix).

- **[Functions, Methods & Options](./references/functions-methods.md)** — Getter/setter patterns (Go omits `Get` so `user.Name()` reads naturally), constructor conventions (`New` or `NewTypeName`), named returns (for documentation only), format function suffixes (`Errorf`, `Wrapf`), and functional options (`WithPort`, `WithLogger`).

- **[Types, Constants & Errors](./references/types-errors.md)** — Interface naming (`Reader`, `Closer` suffix with `-er`), struct naming (nouns, MixedCaps), constants (MixedCaps, not ALL_CAPS), enums (type name prefix like `StatusReady`), sentinel errors (`ErrNotFound` variables), error types (`PathError` suffix), and error message conventions (lowercase, no punctuation).

- **[Test Naming](./references/testing.md)** — Test function naming (`TestFunctionName`), table-driven test field conventions (`input`, `expected`), test helper naming, and subcase naming patterns.

## Common Mistakes

| Mistake | Fix |
| --- | --- |
| `ALL_CAPS` constants | Go reserves casing for visibility, not emphasis — use `MixedCaps` (`MaxRetries`) |
| `GetName()` getter | Go omits `Get` because `user.Name()` reads naturally at call sites. But `Is`/`Has`/`Can` prefixes are kept for boolean predicates: `IsHealthy() bool` not `Healthy() bool` |
| `Url`, `Http`, `Json` acronyms | Mixed-case acronyms create ambiguity (`HttpsUrl` — is it `Https+Url`?). Use all caps or all lower |
| `this` or `self` receiver | Go methods are called frequently — use 1-2 letter abbreviation (`s` for `Server`) to reduce visual noise |
| `util`, `helper` packages | These names say nothing about content — use specific names that describe the abstraction |
| `http.HTTPClient` stuttering | Package name is always present at call site — `http.Client` avoids reading "HTTP" twice |
| `user.NewUser()` constructor | Single primary type uses `New()` — `user.New()` avoids repeating the type name |
| `connected bool` field | Bare adjective is ambiguous — use `isConnected` so the field reads as a true/false question |
| `"invalid message ID"` error | Error strings must be fully lowercase including acronyms — `"invalid message id"` |
| `StatusReady` at iota 0 | Zero value should be a sentinel — `StatusUnknown` at 0 catches uninitialized values |
| `"not found"` error string | Sentinel errors should include the package name — `"mypackage: not found"` identifies the origin |
| `userSlice` type-in-name | Types encode implementation detail — `users` describes what it holds, not how |
| Inconsistent receiver names | Switching names across methods of the same type confuses readers — use one name consistently |
| `snake_case` identifiers | Underscores conflict with Go's MixedCaps convention and tooling expectations — use `mixedCaps` |
| Long names for short scopes | Name length should match scope — `i` is fine for a 3-line loop, `userIndex` is noise |
| Naming constants by value | Values change, roles don't — `DefaultPort` survives a port change, `Port8080` doesn't |
| `FetchCtx()` context variant | `WithContext` is the standard Go suffix — `FetchWithContext()` is instantly recognizable |
| `sort()` in-place but no `In` | Readers assume functions return new values. `SortIn()` signals mutation |
| `parse()` panicking on error | `MustParse()` warns callers that failure panics — surprises belong in the name |
| Mixing `With*`, `Set*`, `Use*` | Consistency across the codebase — `With*` is the Go convention for functional options |
| Plural package names | Go convention is singular (`net/url` not `net/urls`) — keeps import paths consistent |
| `Wrapf` without `f` suffix | The `f` suffix signals format-string semantics — `Wrapf`, `Errorf` tell callers to pass format args |
| Unnecessary import aliases | Aliases add cognitive load. Only alias on collision — `mrand "math/rand"` |
| Inconsistent concept names | Using `user`/`account`/`person` for the same concept forces readers to track synonyms — pick one name |

Applying these fixes means renaming existing identifiers. Use `gopls rename`, which updates every call site and refuses a rename that would break interface satisfaction, then vet every `GOOS` and the `desktop` tag, because platform files only compile for their own target. Renaming an exported method on the Wails facade changes the generated bindings in `frontend/wailsjs`; regenerate them and update `shared/api/bridge.ts` in the same change.

## Enforce with Linters

`predeclared` and `misspell` are enabled in `.golangci.yml`; `revive` and `errname` are not, so review enforces the rest. See `golang-lint` skill.

## Cross-References

- → See `golang-code-style` skill for broader formatting and style decisions
- → See `golang-structs-interfaces` skill for interface naming depth and receiver design
- → See `golang-lint` skill for automated enforcement (predeclared, misspell)
- → See `golang-project-layout` skill for package placement
