# Architecture research: Clean and Hexagonal Architecture in Go

Research date: 2026-10-21. Audience: AI agents and contributors who change kali-tools.

This document records what the primary sources say, how the ideas translate to idiomatic Go, and which choices apply to kali-tools. The binding description of the current repository is [architecture.md](architecture.md); this file explains the reasoning behind it. When the two disagree, architecture.md wins and this file must be updated.

Sources are cited inline. A source is marked *(search summary)* when only a search excerpt could be read, not the page itself.

## Contents

1. Reading guide for agents
2. The canonical ideas
3. What Go changes
4. The book: *Arquitetura Limpa na Prática*
5. Layout options and the chosen layout
6. Core patterns with Go code
7. Driving adapters: Wails, HTTP, CLI
8. Local web server security
9. Driven adapters: platform, processes, SQLite
10. Testing strategy
11. Enforcing the dependency rule
12. Anti-patterns
13. Decision table
14. Sources

## 1. Reading guide for agents

- If you only read one section, read §13 (decision table) and §12 (anti-patterns).
- Before adding a package, read §5 and [architecture.md](architecture.md).
- Before adding an interface, read §3.2 and §4.6. Interfaces belong to the consumer and exist only at real boundaries.
- Before touching the web server, read §8. The checks there are mandatory.
- The skill `.agents/skills/clean-architecture-in-practice/` holds the book's frameworks, glossary and per-chapter notes. Load it for deeper questions about layers, use cases or DTOs.

## 2. The canonical ideas

### 2.1 Hexagonal architecture (Ports and Adapters), Alistair Cockburn

Cockburn's article is about **inside versus outside**, not about stacked layers: "The asymmetry to exploit is not that between left and right sides of the application but between inside and outside." ([Cockburn](https://alistair.cockburn.us/hexagonal-architecture/))

| Term | Meaning | kali-tools examples |
| --- | --- | --- |
| Port | A purposeful conversation between the application and the outside world | "run a diagnostic", "store a job", "list Wi-Fi networks" |
| Adapter | Converts one technology to or from a port | SQLite store, Nmap executable runner, Windows WLAN API |
| Primary (driving) actor | Starts the conversation and drives the application | Wails UI, browser UI through the HTTP server, CLI, tests |
| Secondary (driven) actor | Is driven by the application | SQLite, Nmap, TShark, pcap, OS network APIs |

Cockburn's test is to "create your application to work without either a UI or a database so you can run automated regression-tests". For kali-tools, every use case must be runnable from a Go test without Wails, a browser, SQLite or a real network adapter.

### 2.2 Clean Architecture, Robert C. Martin

The dependency rule: "source code dependencies can only point inwards." ([Martin, 2012](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html))

```text
 Frameworks & Drivers   (Wails, net/http, SQLite driver, os/exec, x/sys/windows)
   Interface Adapters   (bindings, HTTP handlers, CLI commands, repositories, parsers)
     Use Cases          (application rules: start diagnostic, cancel job, export capture)
       Entities         (domain rules: valid targets, rate deltas, capability states)
```

Three points that matter for implementation:

- The circles are schematic. The number of rings is not fixed.
- Control flow may go outward, for example a use case calling a repository. The **source** dependency still points inward because the inner ring declares the interface and the outer ring implements it. This is dependency inversion.
- Data crossing a boundary uses simple structures shaped for the inner ring. Database rows and framework request types never travel inward.

### 2.3 Common ancestors

Clean Architecture is a synthesis of Hexagonal Architecture, DCI (Reenskaug and Coplien) and BCE (Jacobson). All of them separate concerns by splitting software into layers and isolating business rules from technology (Lemos, ch. 1 and 3; see §4).

## 3. What Go changes

Go makes most of the ceremony from Java or C# unnecessary. These rules come from the Go project and well-known Go practitioners.

### 3.1 Module layout

From [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout):

- Server-style projects keep their logic in `internal/`. The compiler then stops other modules from importing it, so its API can be refactored freely.
- `cmd/<name>/` holds one `main` package per binary.
- There is no `pkg/` requirement.

golang-standards/project-layout is **not** an official standard. Russ Cox wrote that it "is unfortunate that this is being put forth as 'golang-standards' when it really is not", and that most Go repositories do not use `pkg/` ([issue #117](https://github.com/golang-standards/project-layout/issues/117)). Follow go.dev instead.

### 3.2 Interfaces belong to the consumer

From [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments):

- "Go interfaces generally belong in the package that uses values of the interface type."
- "The implementing package should return concrete (usually pointer or struct) types."
- "Do not define interfaces before they are used."
- "Do not define interfaces on the implementor side of an API 'for mocking'."

Together these give the rule "accept interfaces, return structs", which Dave Cheney calls "a great rule of thumb" ([SOLID Go Design](https://dave.cheney.net/2016/08/20/solid-go-design)).

Consequence for hexagonal Go: a **port is a small interface declared in the package that consumes it** (the application layer). A separate `ports` or `interfaces` package is not needed and is discouraged. Implicit interface satisfaction means an adapter never imports the port's package just to say that it implements it.

### 3.3 Import graph equals architecture

Cheney: "In Go, your import graph must be acyclic". Push specific dependencies upward toward `main`, and prefer a wide, flat graph to a tall, narrow one. In practice, the dependency rule is a property of the import graph. It can be checked mechanically (§11).

### 3.4 Explicit dependencies

Peter Bourgon ([Go best practices](https://peter.bourgon.org/go-best-practices-2016/)):

- "Make dependencies explicit!"
- "Loggers are dependencies."
- "Only func main has the right to decide which flags are available to the user."

So inner packages never read `os.Getenv`, global flags or global loggers. They receive values through constructors.

### 3.5 Naming

From [Effective Go](https://go.dev/doc/effective_go) and Code Review Comments:

- Avoid stutter: `scan.Result`, not `scan.ScanResult`; `job.NewManager`, not `job.NewJobManager`.
- Avoid package names such as `util`, `common`, `misc`, `types`, `models` and `interfaces`.
- Name a package for what it provides.

### 3.6 Context

- Pass `context.Context` as the first parameter of anything that can block.
- Never store a context in a struct. The single accepted exception in kali-tools is the Wails runtime context, and it stays confined to the Wails adapter (§7.3).
- Never put dependencies such as a database pool or a transaction in a context.

## 4. The book: *Arquitetura Limpa na Prática* (Otávio Lemos, 2022)

The owner supplied this Portuguese book, with TypeScript examples, as a reference. It was converted with the `book-to-skill` skill into `.agents/skills/clean-architecture-in-practice/`. The PDF itself is copyrighted and stays out of Git (`/docs/*.pdf` is ignored). The notes below paraphrase the book and add the Go translation.

### 4.1 Positioning

- Architecture is "the shape given to the system by those who build it": how it is divided into components, how they are arranged and how they communicate.
- Its purpose is to minimize the human effort needed to build and maintain the system, keeping it *soft* and therefore cheap to change.
- Lemos uses Dan North's framing: minimize lead time to positive business impact, sustainably.
- He separates three abstraction levels:
  1. technical details;
  2. **application business rules** (use cases);
  3. **domain business rules** (policies that would exist even without software).
- A "screaming architecture" shows its use cases, not its framework.

**Go mapping:** package names should describe capabilities (`network`, `capture`, `wifi`), not technologies. The technology names live at the edge (`adapter/storage`, `adapter/httpapi`).

### 4.2 The dependency rule is the only non-negotiable rule

Inner code must not name anything declared in an outer ring, and that includes data structures. Lemos applies this strictly: code that calls a driver directly (database driver, bcrypt, JWT) belongs to Frameworks and Drivers, even if it looks like an "adapter".

**Go mapping:**

- Domain and application packages must not import `net/http`, `database/sql` drivers, `os/exec`, `x/sys/windows`, Wails or third-party SDKs.
- Struct tags such as `db:"…"` are the Go equivalent of ORM annotations and belong only on adapter-local structs.

### 4.3 Five layers

| Layer | Holds | kali-tools equivalent |
| --- | --- | --- |
| Entities | Critical data and rules; self-validating value objects; no identity or persistence | `internal/domain`; validation and rate logic in `internal/network`, `internal/capture`, `internal/wifi` |
| Use cases | Application rules that orchestrate entities; **own the ports** they need; DTOs | `internal/app` (job lifecycle, history, discovery orchestration) |
| Interface adapters | Controllers and presenters; convert between use cases and the outside world | `app.go` Wails facade, `cmd/kali-tools` CLI, future `internal/adapter/{wailsui,httpapi,cli}` |
| Frameworks and drivers | Glue to databases, crypto, OS and processes | `internal/adapter/{storage,tools,probe,platform}` |
| Main and configuration | The "dirty" composition root: factories, configuration, entry points | `main.go`, `cmd/*/main.go`, future `internal/bootstrap` |

### 4.4 Entities and value objects

Value objects such as `Email`, `Password` and `Title` have a private constructor and a factory that validates and returns either an error or a value. They are immutable and compared by value. Entities carry no database IDs.

**Go mapping:**

```go
type Target struct{ host string } // unexported field: callers cannot build an invalid value

func NewTarget(raw string) (Target, error) {
    raw = strings.TrimSpace(raw)
    if raw == "" || len(raw) > 253 {
        return Target{}, ErrInvalidTarget
    }
    return Target{host: raw}, nil
}
```

The zero value `Target{}` is still constructible, so document it as invalid or check it at the boundary. Validation in the type turns the AGENTS.md rule "never concatenate shell commands" into a type-level guarantee, because adapters receive a `Target`, not a raw string.

### 4.5 Use cases own their ports

Use cases build entities and call their rules. They declare the repository, encoder and token-manager interfaces they need. Rules fixed for the domain go in entities; rules that vary per application go in use cases.

**Go mapping:**

- Use a struct with injected dependencies and a method `Handle(ctx, Request) (Response, error)`, or a method on an application service.
- Declare the small interfaces it consumes in the same package.

### 4.6 When **not** to use interfaces

This is the book's first warning: "Clean Architecture is not filling the system with interfaces." That practice is cargo-cult over-engineering. Add an interface only when a call crosses an architectural boundary or a test needs a stand-in. Inside one layer, call directly. Below roughly 5,000 lines, question whether the layering is worth it at all.

Lemos chose one generic `UseCase.perform` interface. **Do not copy that into Go.** It loses type safety, and Go needs no such interface. Concrete handler types, or narrow single-method interfaces at the consumer, are better.

### 4.7 Errors as values

Lemos returns expected failures as `Either<Error, Value>`, keeps exceptions for truly exceptional cases and catches them only at the entry points.

**Go mapping:**

- `(T, error)` already is this pattern.
- Use sentinel or typed errors, wrap them with `%w`, and map them once per driving adapter (§6.5).
- Return `ErrNotFound`, not a nil pointer.
- `recover` belongs only at the outermost boundary.

### 4.8 Controllers, presenters and DTOs

- Controllers translate a transport request into a use-case request and map results to status codes.
- Presenters (Humble Object) format output so that the view only renders it.
- DTOs carry data across boundaries so that entities never leave the core.
- Partial-update DTOs with optional fields make use cases branchy; prefer full replacements.

**Go mapping:**

- Use plain exported structs without behavior.
- Formatting for a CLI table or a Wails view model belongs in the adapter, not the use case.

### 4.9 Testing and delivery

- Every layer is testable in isolation, with use cases tested against **stubs** of their ports.
- The case study was built with TDD, in **vertical slices**: one use case through all layers at a time.
- For greenfield work Lemos recommends a walking skeleton with an acceptance test.

### 4.10 Closing principle

"Simple, but not simplistic." Separating infrastructure from rules is cheap and always pays off. Thinking "it will never change" is as risky as thinking "make it fully flexible now".

## 5. Layout options and the chosen layout

### 5.1 Options surveyed

| Layout | Shape | Fit for kali-tools |
| --- | --- | --- |
| Flat | All code in `main` | Too small for three interfaces |
| Layered by kind | `controllers/`, `services/`, `repositories/`, `models/` | Rejected; reads as Java and couples unrelated features ([Kat Zien, GopherCon 2018](https://about.sourcegraph.com/go/gophercon-2018-how-do-you-structure-your-go-apps)) |
| Ben Johnson's Standard Package Layout | Domain types in the root package; one subpackage per dependency (`sqlite`, `http`); `mock`; `cmd/` ([gobeyond.dev](https://www.gobeyond.dev/standard-package-layout/)) | Good ideas: dependency-named adapters, `cmd` wiring |
| Three Dots Labs | `domain/`, `app/` (commands and queries), `adapters/` (driven), `ports/` (driving) ([introducing clean architecture](https://threedots.tech/post/introducing-clean-architecture/)) | Good ideas: one `Application` used by every port |
| evrone/go-clean-template | `cmd/app`, `internal/{entity,usecase,repo,controller}` ([repo](https://github.com/evrone/go-clean-template)) | Rule worth keeping: "add extra layers only when they are really needed" |
| bxcodec/go-clean-arch v4 | Consumer-side interfaces, `internal/`, service-focused packages ([repo](https://github.com/bxcodec/go-clean-arch)) | Confirms the idiomatic direction |

### 5.2 Chosen layout (summary)

The full annotated tree is in [architecture.md](architecture.md). The principles:

1. **Capability packages are the inner hexagon.** `internal/domain` is the shared kernel. `internal/network`, `internal/capture` and `internal/wifi` hold the domain types and rules of each tool family.
2. **`internal/app` is the application layer.** It holds the use cases, job lifecycle and history, and declares the driven ports it consumes.
3. **`internal/adapter/` holds every adapter**, named after its technology. It holds driven adapters (`storage`, `tools`, `probe`, `platform`) now, and driving adapters (`wailsui`, `httpapi`, `cli`) as their code is extracted.
4. **Composition roots stay minimal.** `main.go` at the repository root is the Wails root; Wails v2 builds the project-root package and `go:embed` cannot reach parent directories. `cmd/kali-tools` is the CLI root. `internal/bootstrap` will hold the wiring shared by all roots.
5. **Cross-cutting core packages.** `internal/job` will hold the job manager and event fan-out. `internal/porttest` will hold the contract suites.

The layout grows by adding a capability package plus adapters, never by adding a layer.

## 6. Core patterns with Go code

### 6.1 One Application, many ports

Three Dots shows an `app.Application{Commands, Queries}` struct whose handlers "can be called this way from any port: HTTP, gRPC, or CLI" ([basic CQRS](https://threedots.tech/post/basic-cqrs-in-go/)). Full CQRS is not needed in kali-tools. The useful part is that all three driving adapters receive the **same** application value.

```go
package app

type Application struct {
    Jobs        *job.Manager
    Diagnostics DiagnosticsHandler
    Discovery   DiscoveryHandler
    History     HistoryHandler
}
```

H04 implementation note (2026-10-21): the preceding Application sketch is illustrative. The implemented `app.Application` is a single struct with unexported dependencies and ctx-first methods, not exported handler fields. Both driving roots receive it from bootstrap; only native dialogs, event translation and CLI encoding stay in the adapters.

### 6.2 Driven ports are declared by the consumer

```go
package app

// Declared here because app consumes them. Adapters return concrete types.
type JobStore interface {
    PutJob(ctx context.Context, j Job) error
    History(ctx context.Context) ([]Job, error)
}

type ServiceInspector interface {
    Inspect(ctx context.Context, scope network.Scope) (network.Inventory, error)
}
```

To get a compile-time check without an import from adapter to app, put it in the composition root or a test: `var _ app.JobStore = (*storage.Store)(nil)`.

### 6.3 Job lifecycle and event streaming in the core

Three interfaces require one job model. Streaming is a **core** concept. Driving adapters only forward the events, through Wails events, Server-Sent Events or stdout.

```go
package job

type Event struct {
    JobID ID        `json:"jobId"`
    Seq   uint64    `json:"seq"`  // detect gaps; resync after reconnect
    Kind  string    `json:"kind"` // state | progress | log
    State State     `json:"state,omitempty"`
    At    time.Time `json:"at"`
}

// Subscribe returns a bounded channel. A slow consumer loses events, which
// are counted; the producer never blocks.
func (m *Manager) Subscribe(ctx context.Context, buf int) <-chan Event
func (m *Manager) Cancel(id ID) error
func (m *Manager) Snapshot(id ID) (Status, error)
```

The rules this design follows:

- A job runs under a context derived from the **manager**, not from the request. A job outlives the binding call or HTTP request that started it, and the application shutdown cancels it.
- Cancellation is a use case, reachable from every adapter.
- `Seq` plus `Snapshot` let a reconnecting browser resynchronize without a replay log.
- Every queue is bounded, which follows the AGENTS.md rule to bound event queues.

The current `internal/app.Service` already implements stable job IDs, bounded concurrency and an event callback. Extracting it into `internal/job` is a PLAN task, not a rewrite.

Implemented in H02 (2026-10-21): `Subscribe(ctx)` uses `Limits.SubscriberBuffer` and returns a drop counter, `Cancel` returns `bool`, `Snapshot` returns `(Status, bool)`, and `Wait(ctx, id)` returns terminal status plus operation/persistence errors. Metrics and capture producers now live in `internal/app`, with Wails translating and forwarding events only. The earlier API sketch above is illustrative; the implementation contract is in the H02 spec.

### 6.4 Repository and transactions

- **Single-aggregate writes:** use the update-function pattern, `Update(ctx, key, func(*T) (*T, error))`. The transaction stays in the adapter and the rule stays in the closure ([repository pattern](https://threedots.tech/post/repository-pattern-in-go/), [database transactions](https://threedots.tech/post/database-transactions-in-go/)).
- **Multi-repository writes:** use a unit-of-work port, `RunInTx(ctx, func(Stores) error) error`. Repositories accept a small `dbtx` interface satisfied by both `*sql.DB` and `*sql.Tx` ([Rednafi](https://rednafi.com/go/repo-txn-uow/)). Add it only when the first multi-repository write appears.
- Never put a transaction in the context. Three Dots says that "only masks the root issue".

### 6.5 Error mapping

- The domain and app return sentinel or typed errors. The project uses `domain.Error{Code, Message}` with stable codes such as `invalid_input` and `unsupported`.
- Wrap errors with `%w`.
- H06 implements the shared error mapping as `internal/adapter/contract.FromError`; adapters delegate once, with transport-specific exit/status handling. Wails retains `"<code>: <message>"` rejected-promise strings. Unknown internal text is never copied; authored messages are bounded to 512 UTF-8 bytes.
- Each driving adapter has **one** mapping function:

| Adapter | Maps an error to |
| --- | --- |
| HTTP | Status code plus `{code, message}` |
| Wails | `{code, message}` |
| CLI | Exit code plus message |

- Never return raw subprocess stderr, SQL errors or secrets to a client. Log a bounded, redacted form instead.

### 6.6 DRY applies to behavior, not data

"Usually, DRY is better applied to behaviors, not data." ([Three Dots](https://threedots.tech/post/things-to-know-about-dry/)) Separate structs for domain, storage rows and transport DTOs are deliberate. In kali-tools this separation is a security control: sharing one struct is exactly how a Wi-Fi key or packet bytes could reach history or logs.

The single exception is the **frontend contract**. The Wails and HTTP adapters serve the same React app, so they share one set of DTO shapes, defined once.

### 6.7 Dependency injection

Wire manually, with plain constructors in dependency order, in `main` or in a shared `bootstrap` function.

- Google Wire is archived and "no longer maintained" ([github.com/google/wire](https://github.com/google/wire)).
- Uber fx resolves dependencies at runtime with reflection, so wiring errors appear late ([comparison](https://leapcell.io/blog/go-dependency-injection-approaches-wire-vs-fx-and-manual-best-practices)).
- At kali-tools' size, manual wiring is clearer, compile-checked and greppable.

Mat Ryer's `run` pattern makes composition roots testable ([Grafana blog](https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/)):

```go
func main() {
    if err := run(context.Background(), os.Args, os.Getenv, os.Stdout, os.Stderr); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

## 7. Driving adapters: Wails, HTTP, CLI

### 7.1 Shared rule

A driving adapter does three things only:

1. decode and validate transport input into an application request;
2. call the application;
3. encode the result or map the error.

If two adapters contain the same `if`, that logic belongs in `internal/app` or a capability package.

### 7.2 CLI

| Option | Pros | Cons |
| --- | --- | --- |
| Stdlib `flag` with one `FlagSet` per subcommand | No dependency, about 50 lines of dispatch | No completion, non-POSIX syntax |
| Cobra (on pflag) | Nested commands, generated help, shell completion ([intro](https://blog.gopheracademy.com/advent-2014/introducing-cobra/)) | New dependency to license-review and pin |

The current CLI uses stdlib `flag`. Re-evaluate when the command tree grows beyond about six subcommands or shell completion is required, and record the decision in an ADR. Either way:

- commands take `(ctx, *app.Application, args, io.Writer)`;
- Ctrl-C cancels `ctx`, which cancels the job through the same manager path the UIs use;
- `--json` output carries `schema_version`.

### 7.3 Wails v2 bindings

Wails v2 `Bind` exposes a struct's exported methods as generated JS/TS functions. `OnStartup(ctx)` provides the runtime context that `runtime.EventsEmit` needs ([options](https://raw.githubusercontent.com/wailsapp/wails/master/website/docs/reference/options.mdx), [events](https://raw.githubusercontent.com/wailsapp/wails/master/website/docs/reference/runtime/events.mdx)). Wails v3 moves to plain service structs so they can be tested "without mocking a Wails context" ([v3 migration](https://v3.wails.io/migration/v2-to-v3/), *search summary*). Apply the same discipline in v2:

- Bindings hold only the application and the stored runtime context. They contain no logic.
- They return DTOs and stable error values, never domain structs with unexported fields.
- One separate forwarder goroutine subscribes to the job manager and emits a single `job` event.
- Wrap `runtime.EventsEmit` in a `func(ctx, string, ...any)` field so that the binding can be tested without Wails.

Generated bindings live in `frontend/wailsjs/` and must be regenerated and checked for drift. Their namespace follows the Go package (`main` today). Moving the facade into `internal/adapter/wailsui` therefore changes the generated import path, and the frontend bridge must be updated in the same change.

### 7.4 HTTP server: REST plus Server-Sent Events

- **Same UI.** Expose the frontend `embed.FS` from one small package, so that Wails `AssetServer.Assets` and `httpapi` serve the same build.
- **Transport abstraction in React.** `frontend/src/shared/api/bridge.ts` is the single seam. In Wails it calls the generated bindings and `EventsOn`; in a browser it calls `fetch` and `EventSource`. Choose at runtime, for example based on whether `window.go` exists.
- **SSE for events.** One-way job events need no library, and `EventSource` reconnects automatically. Use WebSocket only for real client-to-server streaming. If you do, keep the library's same-origin default: `coder/websocket` rejects cross-origin upgrades by default and warns that misused `OriginPatterns` enables CSRF ([pkg.go.dev](https://pkg.go.dev/github.com/coder/websocket)).
- **Routing.** Use the Go 1.22+ `http.ServeMux` patterns and keep all routes in one `routes.go`:

```go
func addRoutes(mux *http.ServeMux, a *app.Application) {
    mux.Handle("GET /api/capabilities", handleCapabilities(a))
    mux.Handle("POST /api/diagnostics", handleStartDiagnostic(a))
    mux.Handle("POST /api/jobs/{id}/cancel", handleCancel(a))
    mux.Handle("GET /api/jobs/{id}", handleJobStatus(a))
    mux.Handle("GET /api/events", handleEvents(a)) // SSE
}
```

- **SSE handler.** Use `http.NewResponseController`, flush after each event, send a keep-alive comment every 15 s, and stop on `r.Context().Done()`. Do not set a global `WriteTimeout`, which would kill SSE streams; use `rc.SetWriteDeadline` per write instead. Set `ReadHeaderTimeout` and apply `http.MaxBytesReader` to every request body.
- **Delivery.** Ship it as a `serve` subcommand of the single CLI binary, so a headless SSH host runs `kali-tools serve`. The reasoning is in [architecture.md](architecture.md).

## 8. Local web server security (mandatory)

A local HTTP server is attack surface reachable from **any web page the user visits**, through CSRF and DNS rebinding. Recent advisories against loopback development servers show the pattern ([CVE-2026-97875](https://app.opencve.io/cve/CVE-2026-97875), [mcp-go DNS rebinding](https://www.vulncheck.com/advisories/mcp-go-before-0.56.0-missing-host-header-validation-enables-dns-rebinding)). The stakes are higher here because kali-tools can start network diagnostics and capture. Apply every layer:

1. **Bind to loopback.** Listen on `127.0.0.1:<port>` (or `[::1]`), never `0.0.0.0` by default. LAN exposure requires an explicit flag, TLS and a visible warning.
2. **Host allowlist (DNS rebinding).** Accept only `127.0.0.1:<port>`, `localhost:<port>` and `[::1]:<port>`, and reject everything else ([NCC Group Singularity](https://github.com/nccgroup/singularity/wiki/Preventing-DNS-Rebinding-Attacks), [Jupyter](https://jupyter-server.readthedocs.io/en/latest/operators/security.html)). A Host check alone is not enough.
3. **Per-launch token.**
   - Generate 32 bytes from `crypto/rand` and print `http://127.0.0.1:<port>/#token=…`. The fragment is never sent to the server or in a Referer header.
   - The SPA exchanges the token once for an `HttpOnly; SameSite=Strict` cookie, or sends `Authorization: Bearer`.
   - Compare with `subtle.ConstantTimeCompare`.
   - Require the token on every `/api` route, **including SSE**. `EventSource` cannot set headers, so SSE relies on the cookie.
   - Never log or persist the token.
4. **CSRF.** Go 1.25 `http.NewCrossOriginProtection()` rejects cross-origin unsafe requests using `Sec-Fetch-Site`, falling back to an Origin-versus-Host comparison ([Filippo Valsorda](https://words.filippo.io/csrf/), [Alex Edwards](https://www.alexedwards.net/blog/preventing-csrf-in-go)). No state changes on GET.
5. **No CORS headers.** The SPA is same-origin.
6. **Hardening.**
   - `ReadHeaderTimeout` and `MaxBytesReader`.
   - Caps on concurrent jobs and SSE subscribers.
   - A Content-Security-Policy and `X-Content-Type-Options: nosniff`.
   - No arbitrary command, SQL or file-path endpoints.

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

Two kali-tools-specific consequences:

- Windows Wi-Fi location consent (see [tools research](tools-research.md) §3.3) is granted to the user-context desktop process. A server running under another account may never be prompted. Report this as its own capability state.
- The desktop build must never open an extra TCP listener by accident. Wails bindings use in-process IPC.

## 9. Driven adapters: platform, processes, SQLite

### 9.1 Platform adapters through build constraints

- `//go:build` goes before the package clause, followed by a blank line. Filename suffixes (`_windows.go`, `_linux.go`) are implicit constraints ([go help buildconstraint](https://pkg.go.dev/cmd/go#hdr-Build_constraints)).
- Every OS file exports the same constructor and method set. The fallback file returns an explicit unsupported error, never a fake success.
- CI runs `GOOS=windows|linux|darwin go vet ./...`. As AGENTS.md says, cross-compilation proves that the code compiles, not that it works on real hardware.

### 9.2 External executables (Nmap, TShark, future iperf3/pktmon)

- Resolve the executable path once, at bootstrap.
- Build the argument array from validated domain values and call `exec.CommandContext(ctx, path, argv...)`. Never call a shell.
- Cap stdout and stderr. Parse with size-limited streaming decoders.
- Cancel descendant processes: Windows uses Job Objects and Unix uses process groups, as already implemented in `internal/adapter/tools`.
- Licensing for each tool is in [tools research](tools-research.md) §3 and §7.

### 9.3 SQLite

- Keep row structs and SQL inside `internal/adapter/storage`. Map them to application types at the boundary.
- Use explicit migrations, parameterized SQL and a single coordinated writer.
- Secrets and payloads never reach this adapter, a rule enforced by type separation (§6.6).

## 10. Testing strategy

| Level | Target | Technique |
| --- | --- | --- |
| Domain unit | Capability packages | Table-driven, black-box `_test` packages, no fakes |
| Use case | `internal/app` | Hand-written fakes of driven ports; job lifecycle, cancellation and bounded-drop tests |
| Contract | Each driven port | One exported suite in `internal/porttest`, run against the real adapter **and** the fake, so the fake cannot drift |
| Acceptance | Each driving adapter | One specification and one driver per adapter: direct `Application`, `httptest` plus SSE client, CLI `run` with a buffer ([Learn Go with Tests](https://quii.gitbook.io/learn-go-with-tests/testing-fundamentals/scaling-acceptance-tests)) |
| Process | Composition roots | `run()` in a goroutine with a cancellable context; poll `/healthz` (Mat Ryer) |
| Integration | Docker lab | `//go:build integration`, private Compose network (existing `tests/integration`) |
| Native | Real hardware | Manual or hosted evidence recorded in `docs/verification/`; never inferred |
| Browser | React | Playwright with contract-faithful fakes; does not prove Wails IPC |

H07 implements the shared suites in `internal/porttest`, with fakes in `internal/porttest/fake`. The real exported signature is:

```go
func JobStoreContract(t *testing.T, newStore func(t *testing.T) app.JobStore)
```

`storage` and `fake` both run this suite and `SettingsStoreContract`. The store round-trip contract is verbatim: redaction is an upstream application responsibility. Five additional suites cover host inventory, tools, Wi-Fi, ICMP and capture. Direct, CLI and headless Wails acceptance drivers run identical diagnostic and discovery scenarios; CLI history remains explicitly unsupported until C01, and HTTP is W01. See [H07 evidence](verification/specs/H07.md) for verified branches and the pending ICMP validation issue.

## 11. Enforcing the dependency rule

Pick one tool, pin it and run it in CI next to `go vet`:

- **depguard**, already available through golangci-lint ([configuration](https://golangci-lint.run/docs/linters/configuration/)). Deny Wails, `os/exec`, `database/sql` and `net/http` in domain and application packages. Deny Wails everywhere except the Wails adapter and the desktop root.
- **go-arch-lint** ([repo](https://github.com/fe3dback/go-arch-lint)) declares components and a `mayDependOn` graph in `.go-arch-lint.yml`.
- **A Go test** that runs `go list -deps` and asserts the forbidden edges. It has no new dependency but gives weaker messages.

The target rules for kali-tools:

```text
domain        -> stdlib only
network, capture, wifi -> domain (+ stdlib, gopacket for capture)   [no adapter imports]
app           -> domain, capability packages, job
adapter/*     -> domain, capability packages, app (types only)
bootstrap     -> everything except cmd
cmd/*, main   -> bootstrap, adapter/*
```

Violation recorded and fixed on 2026-10-21: `internal/network` imported `internal/adapter/probe`. It now declares the consumer-side `network.Prober` port; composition roots inject `probe.ICMP{}` into `network.Diagnostics`. This is the reference example of the port pattern in this repository.

## 12. Anti-patterns

- **Packages named by kind:** `controllers/`, `services/`, `repositories/`, `models/`, `utils/`, `common/`, `types/`.
- **Producer-side interfaces "for mocking"** (`IFooService` next to its only implementation), and interfaces for every struct. Add one at a real boundary, or when a second implementation exists.
- **Stutter:** `network.NetworkResult`, `job.NewJobManager`.
- **One struct for JSON, SQL and the domain.** This leaks fields, and in kali-tools could leak secrets.
- **`pkg/` and the full golang-standards tree.**
- **Context misuse:** a context in a struct (except the Wails runtime context in its adapter), dependencies in a context, or long jobs tied to request contexts.
- **DI frameworks at this size.**
- **Business logic in bindings, handlers or CLI commands.**
- **Unbounded channels, buffers, subprocess output or parser input.**
- **Premature patterns:** unit of work, CQRS or a separate domain layer before their trigger appears. Rednafi and community threads warn that they cost more than they return in small codebases ([dev.to](https://dev.to/gregalc99/why-clean-architecture-struggles-in-golang-and-what-works-better-2n9i)).
- **Generic `UseCase` interfaces** copied from the TypeScript book (§4.6).

## 13. Decision table

| Decision | Choice | Reason |
| --- | --- | --- |
| Layout | go.dev server layout: `internal/` and `cmd/`, no `pkg/` | Official guidance |
| Inner hexagon | `internal/domain` plus capability packages | Screams capabilities; flat import graph |
| Application layer | `internal/app` with consumer-side ports | Idiomatic Go; adapters stay concrete |
| Adapters | `internal/adapter/<technology>` | Named by dependency; driving and driven together |
| Composition | Manual wiring in `internal/bootstrap`, `run()` in each root | Compile-checked; Wire archived |
| Events | Core `job` manager with bounded fan-out; adapters forward | One lifecycle, three transports |
| Web transport | REST plus SSE; WebSocket only if bidirectional | Stdlib only; automatic reconnect |
| Web delivery | `kali-tools serve` subcommand | One headless binary for SSH hosts |
| CLI parser | Stdlib `flag` until completion or depth justifies Cobra (ADR) | Fewer dependencies |
| Transactions | Update function first; unit of work on the first multi-repository write | Keeps SQL out of `app` |
| Tests | Fakes, contract suites, per-adapter acceptance drivers | Fakes cannot drift; adapters stay thin |
| Enforcement | depguard in golangci-lint (or go-arch-lint) | Rules hold as the code grows |
| Local server security | Loopback, Host allowlist, token, cross-origin protection | Blocks DNS rebinding and CSRF |

## 14. Sources

### Go official

- https://go.dev/doc/modules/layout
- https://go.dev/wiki/CodeReviewComments
- https://go.dev/doc/effective_go
- https://pkg.go.dev/cmd/go#hdr-Build_constraints

### Architecture originals

- https://alistair.cockburn.us/hexagonal-architecture/
- https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html
- Otávio Lemos, *Arquitetura Limpa na Prática*, 2022 (local PDF, not redistributed; skill `.agents/skills/clean-architecture-in-practice/`)

### Three Dots Labs

- https://threedots.tech/post/introducing-clean-architecture/
- https://threedots.tech/post/basic-cqrs-in-go/
- https://threedots.tech/post/ddd-lite-in-go-introduction/
- https://threedots.tech/post/repository-pattern-in-go/
- https://threedots.tech/post/database-transactions-in-go/
- https://threedots.tech/post/things-to-know-about-dry/
- https://threedots.tech/post/common-anti-patterns-in-go-web-applications/
- https://threedots.tech/post/microservices-test-architecture/

### Layouts and templates

- https://www.gobeyond.dev/standard-package-layout/
- https://www.gobeyond.dev/wtf-dial/
- https://github.com/bxcodec/go-clean-arch
- https://github.com/evrone/go-clean-template
- https://github.com/golang-standards/project-layout/issues/117

### Practitioners

- https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/
- https://peter.bourgon.org/go-best-practices-2016/
- https://dave.cheney.net/2016/08/20/solid-go-design
- https://www.alexedwards.net/blog/organising-database-access
- https://quii.gitbook.io/learn-go-with-tests/testing-fundamentals/scaling-acceptance-tests
- https://about.sourcegraph.com/go/gophercon-2018-how-do-you-structure-your-go-apps
- https://rednafi.com/go/repo-txn-uow/

### Dependency injection

- https://github.com/google/wire
- https://leapcell.io/blog/go-dependency-injection-approaches-wire-vs-fx-and-manual-best-practices

### Wails

- https://raw.githubusercontent.com/wailsapp/wails/master/website/docs/reference/options.mdx
- https://raw.githubusercontent.com/wailsapp/wails/master/website/docs/reference/runtime/events.mdx
- https://v3.wails.io/migration/v2-to-v3/ *(search summary)*

### Web transport and CLI

- https://pkg.go.dev/github.com/coder/websocket
- https://blog.gopheracademy.com/advent-2014/introducing-cobra/

### Security

- https://words.filippo.io/csrf/
- https://www.alexedwards.net/blog/preventing-csrf-in-go
- https://github.com/nccgroup/singularity/wiki/Preventing-DNS-Rebinding-Attacks
- https://jupyter-server.readthedocs.io/en/latest/operators/security.html
- https://app.opencve.io/cve/CVE-2026-97875
- https://www.vulncheck.com/advisories/mcp-go-before-0.56.0-missing-host-header-validation-enables-dns-rebinding

### Enforcement

- https://github.com/fe3dback/go-arch-lint
- https://golangci-lint.run/docs/linters/configuration/

### Community sentiment

- https://dev.to/gregalc99/why-clean-architecture-struggles-in-golang-and-what-works-better-2n9i
- https://www.thoughtworks.com/insights/blog/programming-languages/mistakes-to-avoid-when-coming-from-an-object-oriented-language

**Not retrievable during research:** wails.io pages (HTTP 403; the GitHub docs source was used instead), the 2023 JetBrains interview with Kat Zien, and direct Hacker News and X threads.
