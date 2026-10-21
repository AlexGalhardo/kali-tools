package cli

import (
	"context"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const cancelGrace = 3 * time.Second

func runDiagnostic(ctx context.Context, a *app.Application, request network.Request) (network.Result, error) {
	if network.Validate(request) != nil {
		// Keep the legacy failed-result shape for rejected requests; no job is started.
		return a.Diagnose(ctx, request)
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
