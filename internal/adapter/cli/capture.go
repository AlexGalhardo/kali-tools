package cli

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

const captureGrace = 10 * time.Second
const captureCancelGrace = cancelGrace

func runCapture(ctx context.Context, application *app.Application, request capture.LiveRequest) (capture.LiveResult, error) {
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, _ := application.Subscribe(subCtx)
	if err := application.StartCapture(ctx, request); err != nil {
		return capture.LiveResult{}, err
	}
	return awaitCapture(ctx, events, func() { application.StopCapture(context.Background()) }, time.Duration(request.DurationSeconds)*time.Second+captureGrace)
}

func awaitCapture(ctx context.Context, events <-chan job.Event, stop func(), limit time.Duration) (capture.LiveResult, error) {
	timer := time.NewTimer(limit)
	defer timer.Stop()
	stopOnce := sync.OnceFunc(stop)
	cancelled := ctx.Done()
	wasCancelled := false
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return capture.LiveResult{}, job.ErrClosed
			}
			if event.Topic != job.TopicCapture {
				continue
			}
			outcome, ok := event.Data.(app.CaptureEvent)
			if ok {
				if wasCancelled || ctx.Err() != nil {
					return outcome.Result, ctx.Err()
				}
				err := outcome.Err
				if err == nil {
					err = parseCodedError(outcome.Error)
				}
				return outcome.Result, err
			}
		case <-cancelled:
			stopOnce()
			cancelled = nil
			wasCancelled = true
			timer.Reset(captureCancelGrace)
		case <-timer.C:
			stopOnce()
			if wasCancelled {
				return capture.LiveResult{}, ctx.Err()
			}
			return capture.LiveResult{}, contract.Error{Code: "timeout", Message: "capture did not finish in time"}
		}
	}
}

func parseCodedError(message string) error {
	if message == "" {
		return nil
	}
	code, detail, ok := strings.Cut(message, ": ")
	if ok {
		if domain.IsErrorCode(code) {
			return contract.Error{Code: code, Message: detail}
		}
	}
	return contract.Error{Code: "failed", Message: message}
}
