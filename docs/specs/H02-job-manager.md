# H02 Job manager

| Field | Value |
| --- | --- |
| PLAN tasks | H02 |
| Status | Implementation complete 2026-10-21; native AC-16 pending ([evidence](../verification/specs/H02.md)) |
| Depends on | H01 |
| Risk | High: concurrency rewrite of the job lifecycle, plus moving the metrics and capture loops out of the Wails adapter |

## 1. Goal

Extract the job lifecycle from `app.Service` into a new package, `internal/job`. It provides a `Manager` that:

- runs jobs under manager-owned contexts;
- bounds concurrency to 8;
- assigns stable random IDs;
- persists every state change through a hook;
- fans events out to any number of bounded subscribers, with a monotonic `Seq` and drop counting.

The metrics ticker and the capture session state move from `app.go` into `internal/app`. They publish through the same manager (debt D3). The Wails adapter keeps the same event names and payload shapes, so the frontend does not change. The web server (W03) and the CLI (C02) can stream the same events later.

## 2. Non-goals

- Do not declare driven ports (`JobStore`, `HostNetwork`, `CaptureProvider`, ...) or remove `internal/app`'s imports of `internal/adapter/*` (H03). H02 injects the metrics and capture sources as plain function fields that H03 replaces.
- Do not rename `app.Service` to `app.Application` or add facade methods for Wi-Fi, settings or history (H04).
- Do not change any Wails binding signature, generated file under `frontend/wailsjs`, or frontend code.
- Do not add a replay log, persistence of events, or an SSE endpoint (W03).
- Do not cap the number of subscribers in `job` (the HTTP adapter enforces its cap in W04).
- Do not track the live capture as a job or write it to history. Today it is neither, and that stays.
- Do not change CLI behavior. The CLI does not use the manager until H04.

## 3. Current state

Verified on 2026-10-21. Re-verify before starting.

- `internal/job/` contains only `.gitkeep` (docs/architecture.md:48).
- `internal/app/service.go` contains the following.
  - Types:
    - `Event{SchemaVersion, Job storage.Job, Result *network.Result}` with JSON tags `schema_version`, `job`, `result,omitempty` (lines 18-22);
    - `Clock` and `SystemClock` (lines 23-26);
    - `terminalEventWait = 2 * time.Second` (line 30).
  - `Service` fields `Store *storage.Store`, `Diagnostics`, `Clock`, `Events chan Event`, `mu`, `jobs map[string]context.CancelFunc`, `wg`, `closed` and `closeErr` (lines 32-42).
  - `New` creates `Events` with buffer 128 (lines 44-46).
  - `emit` drops when the channel is full (lines 48-53). `emitTerminal` waits up to 2 s (lines 56-63).
  - `Start`, for diagnostic jobs (lines 64-125):
    - validates with `network.Validate`;
    - rejects when closed (`"service is shutting down"`) or when `len(s.jobs) >= 8` (line 73);
    - generates a 16-byte `crypto/rand` hex ID (lines 76-79);
    - persists `queued` with summary `"Diagnostic queued"` using the caller's context (lines 80-81);
    - derives the job context from the caller's context (line 84);
    - persists `running` (line 95);
    - runs `Diagnostics.Diagnose`;
    - computes `cancelled` if `errors.Is(err, context.Canceled)` or `errors.Is(ctx.Err(), context.Canceled)` (line 109);
    - sets the summary to `"Diagnostic " + state` (line 115, "Never store targets, URLs, errors or secrets");
    - persists the terminal state with a fresh 5 s timeout (line 116);
    - on a write failure, sets the state to `failed` and the summary to `"History write failed"` (lines 118-121);
    - emits the terminal event with the result (line 122).
  - `Cancel(id string) bool` (lines 126-134).
  - `Close()` (lines 137-155) is idempotent. It cancels jobs, waits, then closes the store.
  - `Discover` and `InspectServices` (lines 158-189) run synchronously through `runOperation` (lines 191-243). `runOperation`:
    - starts directly in `running` with the summary `kind + " running"` (line 202);
    - labels terminal summaries `"Discovery"` or `"Service inspection"` (lines 223-229) and appends `": " + detail` when the detail is not empty;
    - joins the operation error with a history write error (line 239).
- `app.go` (Wails, tags `desktop || bindings`):
  - lines 59-68: forwarder goroutine that reads `a.service.Events` and calls `wruntime.EventsEmit(a.ctx, "job", event)`;
  - lines 69-84: metrics goroutine, a 2 s `time.NewTicker` that calls `platform.Inventory()`, `network.Tracker.Update(sample.Counters, now)` and `EventsEmit(a.ctx, "metrics", ...)`, and skips samples with errors;
  - lines 30-33 and 182-276: capture session state (`captureMu`, `captureCancel`, `captureWG`, `captureArtifact`) used by `ImportCapture`, `StartCapture`, `StopCapture` and `SaveCapture`. The capture goroutine emits `"capture"` with `map[string]any{"result": result, "error": message}` (line 240). `SaveCapture` writes with a 30 s timeout (line 270).
  - The exact user-visible messages are:
    - `"stop capture before importing a file"` (line 190);
    - `"capture started while importing"` (line 204);
    - `"capture already running"` (line 222);
    - `"capture format must be pcap or pcapng"` (line 257);
    - `"import or finish a capture before saving"` (line 264).
- Frontend consumers (`frontend/src/app/App.tsx`):
  - `job` events read `e.job` (lines 198-204) and `e.result` (line 278);
  - `capture` events read `e.result.packets`, `e.result.limited` and `e.error` (lines 209-218);
  - `metrics` events read `timestamp`, `receive_bps` and `send_bps` (lines 181-190).
- Tests in `internal/app/service_test.go` (package `app`):
  - `TestCancellationAndRedaction` (line 17) and `TestTerminalEventSurvivesFullEventBuffer` (line 154) read `s.Events` directly;
  - all five tests call `New(store, network.Diagnostics{})`.

## 4. Design

### 4.1 Package `internal/job`

`internal/job` may import only stdlib and `internal/domain`.

```go
package job

const (
    DefaultMaxConcurrent    = 8
    DefaultSubscriberBuffer = 256
    MaxConcurrentLimit      = 64              // Limits above this are clamped
    MaxSubscriberBuffer     = 4096            // Limits above this are clamped
    MaxFinished             = 256             // finished statuses retained for Snapshot/Wait
    TerminalWait            = 2 * time.Second // shared deadline per terminal publish

    TopicJob     = "job"
    TopicMetrics = "metrics"
    TopicCapture = "capture"
)

type ID string

type State string

const (
    Queued    State = "queued"
    Running   State = "running"
    Completed State = "completed"
    Failed    State = "failed"
    Cancelled State = "cancelled"
)

// Terminal reports whether s is completed, failed or cancelled.
func (s State) Terminal() bool

// Status mirrors the JSON of today's storage.Job byte for byte.
type Status struct {
    ID       ID         `json:"id"`
    Kind     string     `json:"kind"`
    State    State      `json:"state"`
    Created  time.Time  `json:"created"`
    Finished *time.Time `json:"finished,omitempty"`
    Summary  string     `json:"summary"`
}

type Event struct {
    SchemaVersion int       `json:"schema_version"` // domain.SchemaVersion
    Seq           uint64    `json:"seq"`            // monotonic per Manager, starts at 1
    Topic         string    `json:"topic"`          // "job" | "metrics" | "capture"
    Job           *Status   `json:"job,omitempty"`  // set only for Topic "job"
    Data          any       `json:"data,omitempty"` // job result, metrics sample or capture outcome
    At            time.Time `json:"at"`             // clock() at publish time
}

type Limits struct {
    MaxConcurrent    int // <= 0 means 8; clamped to MaxConcurrentLimit
    SubscriberBuffer int // <= 0 means 256; clamped to MaxSubscriberBuffer
}

var (
    // Both keep code invalid_input so CLI exit codes stay 2.
    ErrClosed = &domain.Error{Code: "invalid_input", Message: "service is shutting down"}
    ErrBusy   = &domain.Error{Code: "invalid_input", Message: "maximum concurrent operations reached"}
    // ErrNotFound never reaches a user in H02–H04; H06 maps it.
    ErrNotFound = errors.New("job: not found")
)

type Manager struct {
    // OnChange persists every state change (queued, running, terminal).
    // Set it before the first Start; nil disables persistence. A non-nil error
    // on queued aborts Start; on running it fails the job without calling run;
    // on the terminal write it turns the status into failed/"History write failed".
    OnChange func(Status) error
    // Summarize builds Status.Summary. Set it before the first Start; nil means
    // kind + " " + state. It must never include targets, URLs, errors or secrets.
    Summarize func(kind string, state State, detail string) string
    // unexported: mu, pubMu, clock, limits, seq, active map[ID]*entry,
    // finished map[ID]*entry, finishedOrder []ID, subs map[*subscriber]struct{},
    // wg sync.WaitGroup, closed bool
}

func NewManager(clock func() time.Time, limits Limits) *Manager // clock nil => time.Now().UTC()
func (m *Manager) Start(kind string, run func(ctx context.Context) (summary string, result any, err error)) (Status, error)
func (m *Manager) Cancel(id ID) bool
func (m *Manager) Snapshot(id ID) (Status, bool)
func (m *Manager) Wait(ctx context.Context, id ID) (Status, error) // addition, see 4.2
func (m *Manager) Subscribe(ctx context.Context) (<-chan Event, func() uint64)
func (m *Manager) Publish(topic string, data any)
func (m *Manager) Close()
```

The `summary` value returned by `run` is a **detail** passed to `Summarize`. For diagnostics it is always `""`.

### 4.2 Manager behavior

#### `Start(kind, run)`

1. If `kind == ""` or `run == nil`, return `domain.Invalid("job kind and run function are required")`.
2. Lock `mu`.
   - If closed, return `ErrClosed`.
   - If `len(active) >= limits.MaxConcurrent`, return `ErrBusy`.
3. Generate the ID from 16 `crypto/rand` bytes, hex-encoded (32 characters). If `rand` fails, return the error.
4. Build `Status{ID, Kind: kind, State: Queued, Created: clock(), Summary: summarize(kind, Queued, "")}`.
5. Create `ctx, cancel := context.WithCancel(context.Background())`. The job context is owned by the manager and is never derived from a request.
6. Register the entry in `active` (which reserves the slot), then call `wg.Add(1)` and unlock.
7. If `OnChange` is set and `OnChange(queued)` returns an error:
   - remove the entry;
   - call `cancel()` and `wg.Done()`;
   - return the zero `Status` and that error.
   - No event is published.
8. Publish a non-blocking `job` event for `queued`.
9. Start the job goroutine (below) and return the queued `Status`.

#### Job goroutine

1. Set the state to `Running` and the summary to `summarize(kind, Running, "")`, and store it in the entry.
   - If `OnChange(running)` fails, skip `run`; set `runErr` to that error, `state = stateFor(ctx, runErr)` and `result = nil`.
   - Otherwise, publish a non-blocking `running` event and call `detail, result, runErr := run(ctx)`, then set `state = stateFor(ctx, runErr)`.
2. `stateFor` is the following:

   ```go
   func stateFor(ctx context.Context, err error) State {
       switch {
       case errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled):
           return Cancelled
       case err != nil:
           return Failed
       default:
           return Completed
       }
   }
   ```

3. Set `finished := clock()`, `Finished = &finished`, `State = state` and `Summary = summarize(kind, state, detail)`. Note that `detail` is `""` when `run` was skipped.
4. If `OnChange(terminal)` fails, set `State = Failed`, `Summary = "History write failed"` and `entry.err = errors.Join(runErr, persistErr)`. Otherwise set `entry.err = runErr`.
5. Under `mu`:
   - move the entry from `active` to `finished`;
   - append the ID to `finishedOrder`;
   - while `len(finishedOrder) > MaxFinished`, evict the oldest entry.
6. Publish the terminal `job` event (`Data: result`) with terminal wait (below), then `close(entry.done)`, call `cancel()` and `wg.Done()`.

#### Other methods

- **`Cancel(id)`:** if `id` is active, call its cancel function and return true. Otherwise return false; unknown and already finished IDs both give false.
- **`Snapshot(id)`:** look up `active`, then `finished`. Return a copy of the `Status`.
- **`Wait(ctx, id)`:**
  - If the ID is neither active nor finished, return `ErrNotFound`.
  - Otherwise wait on `entry.done` or `ctx.Done()`. On `ctx.Done()`, return `ctx.Err()` and leave the job running.
  - After `done`, return the terminal `Status` and `entry.err`.
  - This method is an addition to the shared API. The synchronous use cases (`Discover`, `InspectServices`) need it to return the operation error and the history write error, as `runOperation` does today.
- **`Subscribe(ctx)`:**
  - Create a subscriber with `make(chan Event, limits.SubscriberBuffer)` and an `atomic.Uint64` drop counter.
  - Register it, and call `context.AfterFunc(ctx, unsubscribe)`.
  - `unsubscribe` takes the subscriber's own mutex, marks it closed, closes the channel and removes it.
  - The returned function reports the drops for this subscriber.
  - If the manager is closed, return an already closed channel and a function that returns 0.
- **`Publish(topic, data)`:**
  - `TopicMetrics` is non-blocking.
  - `TopicCapture` uses terminal wait, because it is the single completion event of a capture session.
  - Any other topic, including `TopicJob`, is ignored. Job events come only from the lifecycle.
  - After `Close`, it does nothing.

#### Delivery and ordering

- `pubMu` is held while `seq` is incremented and the event is delivered to every subscriber. Each subscriber therefore receives a strictly increasing `Seq`.
- **Non-blocking delivery:** for each subscriber, under its own mutex, if it is closed skip it; otherwise `select { case ch <- ev: default: drops.Add(1) }`.
- **Terminal wait:**
  - Make one shared timer of `TerminalWait` per publish.
  - For each subscriber, try a non-blocking send first. If that fails, `select { case ch <- ev: case <-timer.C: drops.Add(1) }`.
  - Once the timer fires, the remaining subscribers get non-blocking sends only.
  - Worst case, a terminal publish blocks for 2 s.
- **`Close()`:**
  1. Lock `mu`. If already closed, return.
  2. Set `closed = true`, cancel every active job and unlock.
  3. Call `wg.Wait()`. Terminal events of the cancelled jobs are still published.
  4. Unsubscribe and close every subscriber.
  - `Close` is idempotent, and `Start` returns `ErrClosed` after it.

#### Locking rules

- Never hold `mu` while calling `OnChange`, `run` or while delivering events.
- Never send on a subscriber channel without holding that subscriber's mutex and checking its `closed` flag.

### 4.3 `internal/app` changes

```go
package app

// persistTimeout bounds every job write (was the literal 5 s at service.go:116 and :233).
const persistTimeout = 5 * time.Second

// MetricsInterval is the sampling period of the metrics stream (was app.go:70).
const MetricsInterval = 2 * time.Second

// CaptureExportTimeout bounds packet file export (was app.go:270).
const CaptureExportTimeout = 30 * time.Second

type Service struct {
    Store       *storage.Store
    Diagnostics network.Diagnostics
    Jobs        *job.Manager
    // Counters samples interface counters for the metrics stream; nil disables
    // metrics. H03 replaces it with the HostNetwork port.
    Counters func(ctx context.Context) ([]network.Counter, error)
    // Live runs one bounded live capture. H03 replaces it with CaptureProvider.
    Live func(ctx context.Context, r capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error)
    // unexported: mu, closed, closeErr, done chan struct{}, bg sync.WaitGroup,
    // metricsStarted bool, capMu sync.Mutex, capCancel context.CancelFunc,
    // capArtifact *capture.Artifact
}

// CaptureEvent is the Data of a "capture" event. Its JSON equals the map
// emitted at app.go:240. Err keeps the typed error for in-process callers.
type CaptureEvent struct {
    Result capture.LiveResult `json:"result"`
    Error  string             `json:"error"`
    Err    error              `json:"-"`
}

func New(s *storage.Store, d network.Diagnostics, clock func() time.Time) *Service

func (s *Service) Start(parent context.Context, r network.Request) (storage.Job, error)
func (s *Service) Cancel(id string) bool
func (s *Service) Discover(parent context.Context, cidr string, ports []int, timeout time.Duration) ([]network.PortResult, error)
func (s *Service) InspectServices(parent context.Context, cidr string, ports []int) (tools.Nmap, error)
func (s *Service) StartMetrics()
func (s *Service) StartCapture(r capture.LiveRequest) error
func (s *Service) StopCapture()
func (s *Service) CanImportCapture() error
func (s *Service) ImportCapture(path string) (capture.ImportResult, error)
func (s *Service) CanExportCapture(format string) error
func (s *Service) ExportCapture(ctx context.Context, path, format string) error
func (s *Service) Close() error
```

Delete `Event`, `Clock`, `SystemClock`, `terminalEventWait`, `emit`, `emitTerminal`, the `Events`/`jobs`/`wg` fields and `runOperation`'s own ID and lifecycle code.

#### `New`

`New` sets the following:

- `Jobs = job.NewManager(clock, job.Limits{})`;
- `Live = capture.LiveSession`;
- `Jobs.Summarize = jobSummary`;
- `Jobs.OnChange`, which writes through `Store.PutJob` with `context.WithTimeout(context.Background(), persistTimeout)` and the conversion `storage.Job{ID: string(st.ID), Kind: st.Kind, State: string(st.State), Created: st.Created, Finished: st.Finished, Summary: st.Summary}`.

`jobSummary` preserves every terminal summary in history byte for byte:

```go
func jobSummary(kind string, state job.State, detail string) string {
    label := "Diagnostic"
    switch kind {
    case "discovery":
        label = "Discovery"
    case "nmap":
        label = "Service inspection"
    }
    s := label + " " + string(state)
    if detail != "" && state.Terminal() {
        s += ": " + detail
    }
    return s
}
```

#### Job use cases

- **`Start`** validates with `network.Validate(r)`, then calls `Jobs.Start(r.Kind, func(ctx) (string, any, error) { v, err := s.Diagnostics.Diagnose(ctx, r); return "", &v, err })` and returns the converted `storage.Job`. `parent` is no longer used for the job context. The parameter stays until H04.
- **`Cancel(id)`** returns `Jobs.Cancel(job.ID(id))`.
- **`Discover` / `InspectServices`** keep their validation and detail strings (`"%d endpoints checked, %d open"`, `"Nmap %s: %d hosts"`). They run through:

```go
func (s *Service) runOperation(parent context.Context, kind string, op func(context.Context) (string, error)) error {
    st, err := s.Jobs.Start(kind, func(ctx context.Context) (string, any, error) {
        detail, err := op(ctx)
        return detail, nil, err
    })
    if err != nil {
        return err
    }
    stop := context.AfterFunc(parent, func() { s.Jobs.Cancel(st.ID) })
    defer stop()
    _, err = s.Jobs.Wait(context.Background(), st.ID) // jobs are bounded by their own timeouts
    return err
}
```

#### Metrics

- `StartMetrics()` is idempotent. It is a no-op when `Counters == nil` or the service is closed.
- It starts `runMetrics(ticker.C)` in `s.bg`, using `time.NewTicker(MetricsInterval)`. The ticker stops when `done` closes.
- `runMetrics(tick <-chan time.Time)` keeps one `network.Tracker`. On each tick:
  - sample with `context.WithTimeout(context.Background(), MetricsInterval)`;
  - on success, call `Jobs.Publish(job.TopicMetrics, tracker.Update(counters, now))`;
  - on error, skip the tick (as `app.go:78-81` does).
- It returns when `done` closes.

#### Capture

The capture methods hold the state that used to live in `app.go`, and keep the messages listed in §3 unchanged.

- **`StartCapture(r)`:**
  1. `capture.ValidateLiveRequest(r)`.
  2. If closed, return `domain.Invalid("application is shutting down")`.
  3. Lock `capMu`. If a capture is running, return `"capture already running"`.
  4. Create `ctx, cancel := context.WithCancel(context.Background())` and store `cancel`. Set the artifact to nil.
  5. In `s.bg`, start a goroutine that:
     - calls `Live(ctx, r)`;
     - under `capMu`, stores the artifact and clears `capCancel`;
     - publishes `Jobs.Publish(job.TopicCapture, CaptureEvent{Result: result, Error: msg, Err: err})`, where `msg = err.Error()` when `err != nil`.
- **`StopCapture()`** cancels the running capture, if any.
- **`CanImportCapture()`** returns `"stop capture before importing a file"` while a capture is running, `"application is shutting down"` after `Close`, and nil otherwise.
- **`ImportCapture(path)`:**
  1. Run the `CanImportCapture` check.
  2. Call `capture.ReadArtifactFile(path)` without holding the lock.
  3. On success, lock. If a capture is now running, return `"capture started while importing"`. Otherwise store the artifact.
  4. On error, leave the stored artifact unchanged.
- **`CanExportCapture(format)`:**
  - A format other than `pcap` or `pcapng` returns `"capture format must be pcap or pcapng"`.
  - A running capture or a nil artifact returns `"import or finish a capture before saving"`.
- **`ExportCapture(ctx, path, format)`** runs the `CanExportCapture` check, then calls `artifact.Save` with `context.WithTimeout(ctx, CaptureExportTimeout)`.
- `CanImportCapture` and `CanExportCapture` exist because the Wails adapter must reject a request **before** it opens a native file dialog. `TestNativeBridge` relies on this through `SaveCapture("invalid")`.

#### `Close`

`Close()` is idempotent and returns the first close error on later calls. It runs these steps in order:

1. Mark the service closed.
2. `StopCapture()`.
3. `close(done)`.
4. `bg.Wait()`.
5. Set the artifact to nil.
6. `Jobs.Close()`.
7. `Store.Close()`.

### 4.4 Wiring and Wails adapter

- `internal/bootstrap.New`:
  - passes `cfg.Now` (or the default) as the clock to `app.New`;
  - sets `service.Counters = func(context.Context) ([]network.Counter, error) { snap, err := platform.Inventory(); return snap.Counters, err }`.
- `app.go` `startup`, after a successful bootstrap:

```go
events, _ := a.service.Jobs.Subscribe(a.ctx)
go func() {
    for ev := range events {
        name, payload := wailsEvent(ev)
        wruntime.EventsEmit(a.ctx, name, payload)
    }
}()
a.service.StartMetrics()
```

```go
// jobEventPayload keeps the pre-H02 "job" event JSON and adds seq.
type jobEventPayload struct {
    SchemaVersion int         `json:"schema_version"`
    Seq           uint64      `json:"seq"`
    Job           *job.Status `json:"job"`
    Result        any         `json:"result,omitempty"`
}

func wailsEvent(ev job.Event) (string, any) {
    if ev.Topic == job.TopicJob {
        return job.TopicJob, jobEventPayload{ev.SchemaVersion, ev.Seq, ev.Job, ev.Data}
    }
    return ev.Topic, ev.Data // "metrics": network.Metrics; "capture": app.CaptureEvent
}
```

- Delete `app.go:59-84` and the capture fields (lines 30-33). The binding methods keep their signatures:
  - `StartCapture` calls `a.service.StartCapture(r)` after the ready check.
  - `StopCapture` calls `a.service.StopCapture()` when ready.
  - `ImportCapture` calls `CanImportCapture()`, then opens the dialog. An empty path returns `capture.ImportResult{Packets: []capture.Packet{}}, nil` as today. Otherwise it calls `a.service.ImportCapture(path)`.
  - `SaveCapture(format)` calls `CanExportCapture(format)`, then opens the dialog (an empty path or a dialog error is returned as today), then calls `a.service.ExportCapture(a.ctx, path, format)` and returns `true`.
- `shutdown` keeps its order: mark closed, cancel `a.ctx` (which ends the forwarder subscription), then call `cleanup()`. The cleanup now stops capture and metrics inside `Service.Close`.

### 4.5 Preserved constraints

| Constraint | How H02 keeps it |
| --- | --- |
| Max 8 concurrent jobs | `job.DefaultMaxConcurrent = 8`, `Limits{}` in `app.New` |
| Terminal-event delivery | `TerminalWait = 2 s` per terminal publish, and capture completion uses it |
| No secrets or targets in history | `jobSummary` uses only label, state and count-only details; the diagnostic detail is always `""` |
| Wails event names | `job`, `metrics`, `capture` (`job.Topic*`) |
| Payload shapes | `job`: `{schema_version, seq, job, result?}`; `metrics`: `network.Metrics`; `capture`: `{result, error}` |
| CLI JSON envelope | CLI untouched |

The only intentional differences are the summaries of **non-terminal** states:

| Job | Before | After |
| --- | --- | --- |
| Diagnostic, running | `"Diagnostic queued"` | `"Diagnostic running"` |
| Discovery, running | `"discovery running"` | `"Discovery running"` |
| Service inspection, running | `"nmap running"` | `"Service inspection running"` |

Discovery and Nmap jobs now pass through `queued` before `running`. The frontend handles both (App.tsx:200-204).

## 5. Files

Create:

- `internal/job/job.go`: constants, `ID`, `State`, `Status`, `Event`, `Limits`, errors.
- `internal/job/manager.go`: `Manager` and its methods.
- `internal/job/manager_test.go`: package `job_test`.
- `internal/app/metrics.go`: `MetricsInterval`, `StartMetrics` and `runMetrics`.
- `internal/app/capture.go`: `CaptureEvent`, `CaptureExportTimeout` and the capture methods.
- `internal/app/export_test.go`: package `app`, `func (s *Service) RunMetricsForTest(tick <-chan time.Time) { s.runMetrics(tick) }`.
- `internal/app/capture_test.go` and `internal/app/metrics_test.go`: package `app_test`.

Modify:

- `internal/app/service.go`: lifecycle delegated to `job.Manager` (§4.3).
- `internal/app/service_test.go`: switch to package `app_test` (black-box), add the `nil` clock argument to `New`, and replace `s.Events` reads with `s.Jobs.Subscribe` (§8).
- `internal/bootstrap/bootstrap.go`: pass the clock and set `Counters`.
- `app.go`: forwarder through `Subscribe`, `StartMetrics`, capture methods delegated; remove the capture fields.
- `docs/architecture.md`: see §10.

Delete:

- `internal/job/.gitkeep`.

## 6. Implementation steps

Each step leaves `go vet ./... && go vet -tags desktop . && go test ./...` green.

1. Create `internal/job` (`job.go`, `manager.go`) and `manager_test.go`. Run `go test -race ./internal/job`.
2. Rewrite `internal/app/service.go` on top of the manager (§4.3, without metrics and capture). Update `service_test.go` as described in §8. Update the `app.New` call in `internal/bootstrap` (clock argument). Run `go test -race ./internal/app ./internal/bootstrap`.
3. In `app.go`, replace the `s.Events` forwarder with `Subscribe` + `wailsEvent`. Run `go vet -tags desktop .`.
4. Add `internal/app/metrics.go` and its test. Set `Counters` in bootstrap. In `app.go`, delete the ticker goroutine and call `StartMetrics()`.
5. Add `internal/app/capture.go` and its tests. Move the capture state out of `app.go` and delegate the four capture bindings.
6. Run `node scripts/task.mjs bindings` (expect no drift), `node scripts/task.mjs race`, and the frontend Playwright suite.
7. Update the docs (§10).
8. Write `docs/verification/specs/H02.md`, following README §5.3, with the real outputs of §9 and §12.

## 7. Acceptance criteria

- **AC-1:** Given a subscriber and a run that returns nil, when `Start` is called, then the subscriber receives `queued`, `running` and `completed` job events in this order with strictly increasing `Seq`. `OnChange` saw the same three states. The final `Snapshot` has `Finished != nil`.
- **AC-2:** Given a run that returns an error, the terminal state is `failed`. Given a run blocked on `ctx.Done()`, when `Cancel(id)` is called, then it returns true and the terminal state is `cancelled`. `Cancel` of an unknown ID returns false.
- **AC-3:** Given `Limits{}` and 8 blocked jobs, when a 9th `Start` is called, then it returns `ErrBusy`. After one job finishes and `Wait` returns, a new `Start` succeeds.
- **AC-4:** Given `Limits{SubscriberBuffer: 1}` and a subscriber that never reads, when `Publish(TopicMetrics, ...)` is called 10 times, then all calls return within 3 s, the drop counter reads 9, and a second, reading subscriber receives all 10 events with strictly increasing `Seq`.
- **AC-5:** Given a subscriber whose buffer is full and that starts draining right after `Start`, when the job finishes, then its terminal job event is received within 3 s (no silent loss).
- **AC-6:** Given a full subscriber that never reads, when a job finishes, then `Wait` returns within 3 s. The terminal wait is bounded by `TerminalWait`.
- **AC-7:** Given `OnChange` failing on `queued`, `Start` returns that error and no slot leaks: 8 further `Start` calls succeed. Given `OnChange` failing only on terminal states, the final `Status` is `failed` with summary `"History write failed"`, and `errors.Is(waitErr, persistErr)` is true.
- **AC-8:** Given running jobs and open subscribers, when `Close` is called, then every job context is cancelled, `Close` returns after all job goroutines end, every subscriber channel is closed, `Start` returns `ErrClosed`, `Subscribe` returns a closed channel, and a second `Close` is a no-op. The goroutine count returns to its baseline within 2 s, and the race detector is clean.
- **AC-9:** Given 100 started jobs (run in batches of at most 8), all IDs are 32 lowercase hex characters, are unique, and `Snapshot(id).ID == id`. After `MaxFinished + 1` finished jobs, the first ID is no longer found by `Snapshot`.
- **AC-10:** Given `Publish(TopicJob, x)` or an unknown topic, then no event is delivered.
- **AC-11:** Given `Limits{MaxConcurrent: 0, SubscriberBuffer: 0}`, the effective limits are 8 and 256. `Limits{MaxConcurrent: 1000, SubscriberBuffer: 1 << 20}` is clamped to 64 and 4096.
- **AC-12:** Given the `app.Service` tests, cancellation, redaction, the discovery summary after restart, the bound of 8 plus shutdown, idempotent `Close` and terminal delivery behave exactly as before (existing assertions unchanged).
- **AC-13:** Given a fake `Counters` and a manual tick channel, each tick publishes one `metrics` event whose `Data` is a `network.Metrics`. A sample error publishes nothing for that tick.
- **AC-14:** Given an imported synthetic PCAPNG, when `CanExportCapture("pcap")` and `ExportCapture` run, then the file round-trips with one packet. Without an artifact, `CanExportCapture("pcap")` returns `"import or finish a capture before saving"`. `CanExportCapture("txt")` returns `"capture format must be pcap or pcapng"`.
- **AC-15:** Given a fake `Live` that blocks until cancelled and a local non-loopback IPv4 address (skip the test when none exists):
  - while it runs, `StartCapture` returns `"capture already running"` and `CanImportCapture` returns `"stop capture before importing a file"`;
  - after `StopCapture`, exactly one `capture` event arrives whose `CaptureEvent.Err` satisfies `errors.Is(err, context.Canceled)`, and whose JSON has exactly the keys `result` and `error`.
- **AC-16 [native]:** Given the Windows desktop, the dashboard rates update every 2 s, a diagnostic job shows queued, running and terminal states, and a live capture shows packets. `TestNativeBridge` passes.

## 8. Tests

| Test | Package | Asserts | AC |
| --- | --- | --- | --- |
| `TestLifecycleEventsAndPersistence` | `internal/job` | Event order, `Seq`, `OnChange` states, `Finished` | AC-1 |
| `TestFailedAndCancelledStates` | `internal/job` | `failed`, `cancelled`, `Cancel(unknown) == false` | AC-2 |
| `TestConcurrencyLimitBoundary` | `internal/job` | 8 OK, 9th `ErrBusy`, slot reused after `Wait` | AC-3 |
| `TestSlowSubscriberDropsWithoutBlocking` | `internal/job` | Drops == 9, fast subscriber gets 10 ordered | AC-4 |
| `TestTerminalEventWaitsForSubscriber` | `internal/job` | Terminal event delivered after the buffer is filled | AC-5 |
| `TestTerminalWaitIsBounded` | `internal/job` | `Wait` returns in under 3 s with a stuck subscriber | AC-6 |
| `TestOnChangeFailures` | `internal/job` | Queued failure releases the slot; terminal failure gives `"History write failed"` | AC-7 |
| `TestCloseCancelsJobsAndClosesSubscribers` | `internal/job` | Shutdown contract and goroutine baseline (poll `runtime.NumGoroutine()` with `runtime.Gosched()` until 2 s; no `time.Sleep`) | AC-8 |
| `TestJobIDsAndSnapshotRetention` | `internal/job` | ID format and uniqueness; `MaxFinished` eviction at boundary + 1 | AC-9 |
| `TestPublishIgnoresJobAndUnknownTopics` | `internal/job` | No delivery | AC-10 |
| `TestLimitsDefaultsAndClamp` | `internal/job` | Defaults and clamps (expose the effective limits through an `export_test.go` accessor) | AC-11 |
| `TestCancellationAndRedaction` | `internal/app` (now `app_test`) | Unchanged assertions; reads `events, _ := s.Jobs.Subscribe(t.Context())` opened **before** `Start`; matches `ev.Job != nil && ev.Job.State == job.Cancelled` | AC-12 |
| `TestDiscoverySummarySurvivesRestartWithoutTargets` | `internal/app` | Unchanged except `New(store, network.Diagnostics{}, nil)`; still expects exactly `"Discovery completed: 1 endpoints checked, 1 open"` | AC-12 |
| `TestConcurrentDiagnosticsBoundAndShutdown` | `internal/app` | Unchanged except the `New` signature | AC-12 |
| `TestCloseIsIdempotent` | `internal/app` | Unchanged except the `New` signature | AC-12 |
| `TestTerminalEventSurvivesFullEventBuffer` | `internal/app` | Rewritten setup only: subscribe, then `Publish(job.TopicMetrics, nil)` `job.DefaultSubscriberBuffer` times, start the TCP job to a closed port, then drain; the terminal event for that ID with state `failed` must arrive within 3 s | AC-12 |
| `TestMetricsPublishesOnTick` | `internal/app` | Uses `RunMetricsForTest` with a manual `chan time.Time`; one `metrics` event per successful tick; none on error | AC-13 |
| `TestCaptureImportExportRoundTrip` | `internal/app` | Synthetic PCAPNG (build it as in `cmd/kali-tools/main_test.go:109-125`), `ImportCapture`, `CanExportCapture`, `ExportCapture`, `capture.ReadFile` | AC-14 |
| `TestCaptureSessionStates` | `internal/app` | Fake `Live`; running guards; one capture event; JSON keys | AC-15 |

All existing tests outside `internal/app` must pass unchanged. That includes `cmd/kali-tools` (all five), `internal/adapter/storage` (all five) and `internal/network`.

## 9. Verification

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt --diff ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run ./...
go vet ./... && go vet -tags desktop . && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
go test ./...
cd frontend && bun run typecheck && bun run lint && bun run build && bunx playwright test
```

Spec-specific:

```bash
go test -race -count=3 ./internal/job ./internal/app      # or: node scripts/task.mjs race
node scripts/task.mjs bindings                            # expect no drift
node scripts/task.mjs bridge                              # [native] Windows only
```

## 10. Documentation updates

- `docs/architecture.md`:
  - §1 and §2: `internal/job` is no longer "(plan)/(empty)".
  - §4.2: `app.Service` uses `job.Manager`.
  - §5.1: replace the `emit(Job)` flow with `job.Manager` → `Subscribe` → Wails forwarder.
  - §5.3: producers of `metrics` and `capture` are now `internal/app`; document the `seq` field and the 2 s terminal wait.
  - §8: mark D3 resolved (H02).
- `docs/architecture-research.md` §6.3: add one note that the implemented API is `Subscribe(ctx)` with `Limits.SubscriberBuffer`, `Cancel` returns `bool`, `Snapshot` returns `(Status, bool)`, and `Wait` exists.
- `PLAN.md`: tick the two H02 sub-items with the test evidence.

## 11. Risks and rollback

- **Deadlock between `mu`, `pubMu` and subscriber mutexes.** Mitigation: the lock order is always `pubMu` → subscriber mutex, `mu` is never held during delivery, and every test runs `-race -count=3`.
- **A stuck subscriber delays terminal publishes by up to 2 s each.** This is the same bound as today's single channel.
- **Desktop regressions in metrics or capture,** which unit tests cannot see. Mitigation: AC-16 [native], and the Playwright workflows with the unchanged payload shapes.
- **Rollback:** revert the H02 commits in reverse order (steps 5 → 1). Neither the storage schema nor the bindings change.

## 12. Quality and delivery assertions

### 12.1 Applicable gates

| Gate | Applies | Spec threshold |
| --- | --- | --- |
| QG-1 Formatting | yes | empty diff |
| QG-2 Lint | yes | `0 issues.` |
| QG-3 Vet on all targets | yes | exit 0, including `go vet -tags desktop .` |
| QG-4 Tests | yes | all pass; only the `internal/app` test changes listed in §8 |
| QG-5 Race | yes, tightened | `go test -race -count=3 ./internal/job ./internal/app` with no race reports |
| QG-6 Coverage | yes, tightened | `internal/job` ≥ 90 % (new, concurrency core); `internal/app` not below the recorded pre-spec value |
| QG-7 Vulnerabilities | yes | no new dependency |
| QG-8 Frontend | yes | all pass; Playwright workflow count unchanged |
| QG-9 Binding drift | yes (manual until H05) | `node scripts/task.mjs bindings` reports no drift |
| QG-10 Dependency rule | yes | DA-1 |
| QG-11 Secrets | yes | `TestCancellationAndRedaction` (`synthetic-psk` absent from history) and `TestDiscoverySummarySurvivesRestartWithoutTargets` pass |
| QG-12 Bounds | yes | `DefaultMaxConcurrent`, `DefaultSubscriberBuffer`, `MaxConcurrentLimit`, `MaxSubscriberBuffer`, `MaxFinished` and `TerminalWait` each tested at the limit and at limit + 1 (AC-3, AC-4, AC-6, AC-9, AC-11) |
| QG-13 Commit | yes | `refactor(job): extract job manager (H02)` plus one commit per step when split |
| QG-14 Docs | yes | architecture §1, §2, §4.2, §5.1, §5.3, §8 (D3); research §6.3 note; CHANGELOG entry |
| QG-15 Markdown | yes | `Markdown checks passed` |

### 12.2 Delivery assertions

| DA | Assertion | Check | Pass threshold | Verifier |
| --- | --- | --- | --- | --- |
| DA-1 | `internal/job` depends only on stdlib and `internal/domain` | `go list -f '{{join .Imports "\n"}}' ./internal/job \| grep kali-tools` | prints exactly `github.com/AlexGalhardo/kali-tools/internal/domain` | CI |
| DA-2 | `app.go` owns no metrics ticker or capture state | `grep -cE 'NewTicker\|captureMu\|captureArtifact\|captureCancel' app.go` | `0` | reviewer |
| DA-3 | The Wails event names are unchanged | `grep -cE 'Topic(Job\|Metrics\|Capture) *= *"(job\|metrics\|capture)"' internal/job/job.go` and `go test ./internal/job -run 'TestPublishIgnoresJobAndUnknownTopics'` | count `3`; test `PASS` | CI |
| DA-4 | The 8-job bound holds | `go test ./internal/job -run 'TestConcurrencyLimitBoundary' -v` and `go test ./internal/app -run 'TestConcurrentDiagnosticsBoundAndShutdown' -v` | both `PASS` | CI |
| DA-5 | Terminal events are not lost and the wait stays bounded | `go test ./internal/job -run 'TestTerminalEvent\|TestTerminalWaitIsBounded' -v` | `PASS`, with `TestTerminalWaitIsBounded` under 3 s | CI |
| DA-6 | A slow subscriber never blocks the producer | `go test ./internal/job -run 'TestSlowSubscriberDropsWithoutBlocking' -v` | `PASS`, drops == 9 | CI |
| DA-7 | Shutdown leaks no goroutines | `go test -race ./internal/job -run 'TestCloseCancelsJobsAndClosesSubscribers' -count=5` | `PASS` five times | CI |
| DA-8 | Desktop metrics, jobs and capture still update | Manual run of `wails dev -tags desktop` plus `node scripts/task.mjs bridge` | rates update every 2 s; job states visible; `TestNativeBridge` `PASS` | [native] |

### 12.3 Evidence record

`docs/verification/specs/H02.md`
