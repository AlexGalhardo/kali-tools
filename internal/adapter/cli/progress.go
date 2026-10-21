package cli

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const cancelGrace = 3 * time.Second
const maxProgressLines = 1000

type progressPrinter struct {
	out   io.Writer
	lang  string
	lines int
	err   error
}

func (p *progressPrinter) print(status job.Status) {
	if p.lines >= maxProgressLines || p.err != nil {
		return
	}
	id := string(status.ID)
	if len(id) > 8 {
		id = id[:8]
	}
	_, p.err = fmt.Fprintf(p.out, "job %s %s: %s\n", id, status.Kind, label(p.lang, string(status.State)))
	p.lines++
}

func startProgress(ctx context.Context, a *app.Application, call invocation, stderr io.Writer) func() error {
	if call.g.JSON {
		return func() error { return nil }
	}
	switch call.command.path {
	case "diagnose", "discover", "inspect", "capture", "capture live", "capture import", "capture export", "import":
	default:
		return func() error { return nil }
	}
	subCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	events, _ := a.Subscribe(subCtx)
	stop := make(chan struct{})
	done := make(chan struct{})
	printer := progressPrinter{out: stderr, lang: call.g.Lang}
	go func() {
		defer close(done)
		active := make(map[job.ID]struct{})
		interrupted := ctx.Done()
		observe := func(event job.Event) {
			if event.Topic != job.TopicJob || event.Job == nil {
				return
			}
			status := *event.Job
			if status.State.Terminal() {
				delete(active, status.ID)
			} else if len(active) < job.MaxConcurrentLimit {
				active[status.ID] = struct{}{}
			}
			printer.print(status)
		}
		for {
			select {
			case event, ok := <-events:
				if !ok {
					return
				}
				observe(event)
			case <-interrupted:
				for id := range active {
					a.CancelJob(subCtx, id)
				}
				interrupted = nil
			case <-stop:
				for {
					select {
					case event, ok := <-events:
						if !ok {
							return
						}
						observe(event)
					default:
						for id := range active {
							if snapshot, ok := a.JobSnapshot(subCtx, id); ok && snapshot.State.Terminal() {
								printer.print(snapshot)
							}
						}
						return
					}
				}
			}
		}
	}()
	return sync.OnceValue(func() error { close(stop); <-done; cancel(); return printer.err })
}

func runDiagnostic(ctx context.Context, a *app.Application, request network.Request) (network.Result, error) {
	if network.Validate(request) != nil {
		// Keep the failed-result shape without reflecting rejected flag values or credentials.
		result, err := a.Diagnose(ctx, request)
		result.Kind, result.Target = "", ""
		return result, err
	}
	subCtx, unsubscribe := context.WithCancel(context.WithoutCancel(ctx))
	defer unsubscribe()
	events, dropped := a.Subscribe(subCtx)
	status, err := a.StartDiagnostic(ctx, request)
	if err != nil {
		return network.Result{}, err
	}
	// Recover a missed terminal event without introducing another goroutine.
	poll := time.NewTicker(100 * time.Millisecond)
	defer poll.Stop()
	var grace <-chan time.Time
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	cancelled := ctx.Done()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return network.Result{}, job.ErrClosed
			}
			if event.Topic != job.TopicJob || event.Job == nil || event.Job.ID != status.ID || !event.Job.State.Terminal() {
				continue
			}
			result, _ := event.Data.(*network.Result)
			if result != nil {
				return *result, diagnosticOutcome(*event.Job, result, ctx.Err())
			}
			return network.Result{}, diagnosticOutcome(*event.Job, nil, ctx.Err())
		case <-cancelled:
			a.CancelJob(subCtx, status.ID)
			cancelled = nil
			timer = time.NewTimer(cancelGrace)
			grace = timer.C
		case <-grace:
			return network.Result{}, ctx.Err()
		case <-poll.C:
			if dropped() > 0 {
				if snapshot, ok := a.JobSnapshot(subCtx, status.ID); ok && snapshot.State.Terminal() {
					return network.Result{}, diagnosticOutcome(snapshot, nil, ctx.Err())
				}
			}
		}
	}
}

func diagnosticOutcome(status job.Status, result *network.Result, interrupted error) error {
	if interrupted != nil {
		return interrupted
	}
	if result != nil && result.ErrorCode != "" {
		return &domain.Error{Code: result.ErrorCode, Message: status.Summary}
	}
	if status.State == job.Cancelled {
		return context.Canceled
	}
	if status.State == job.Failed {
		return &domain.Error{Code: domain.CodeFailed, Message: status.Summary}
	}
	return nil
}
