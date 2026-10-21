# When to Fake

kali-tools uses hand-written fakes, not mocking frameworks. Fake at **system boundaries** only, which in this architecture are the driven ports:

- SQLite store (`JobStore`, `SettingsStore`) — though store tests use real SQLite
- External executables (`ToolRunner`/`ServiceInspector` for Nmap, TShark)
- OS networking (`Prober` for ICMP, `HostNetwork` for routes and counters)
- Capture and Wi-Fi providers
- Time (`Clock`) and randomness

Do not fake:

- Capability packages (`network`, `capture`, `wifi`) when testing `app`, unless they cross into the OS
- Internal collaborators in the same layer
- Anything you control without side effects

## Designing for fakes

### 1. Inject the dependency through a consumer-side port

```go
// Easy to fake: the use case receives the port
func NewService(store JobStore, runner ToolRunner, clock Clock) *Service

// Hard to fake: the use case reaches for the concrete adapter
func (s *Service) Inspect(ctx context.Context, scope Scope) (Inventory, error) {
    return tools.RunNmap(ctx, scope) // imports internal/adapter/tools: also a layer violation
}
```

### 2. Prefer specific methods over a generic one

```go
// GOOD: each method returns one specific shape
type ToolRunner interface {
    Inspect(ctx context.Context, scope network.Scope) (network.Inventory, error)
}

// BAD: the fake needs conditional logic to answer
type ToolRunner interface {
    Run(ctx context.Context, name string, args []string) ([]byte, error)
}
```

The specific approach means:

- Each fake returns one shape
- No conditional logic in test setup
- Tests show which operation they exercise
- The UI, HTTP and CLI never get a path to arbitrary command execution

### 3. Keep fakes honest

Run each fake through the same `internal/porttest` contract suite as its real adapter (PLAN H07).
