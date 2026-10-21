package cli

import (
	"context"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

func AwaitCaptureForTest(ctx context.Context, events <-chan job.Event, stop func(), limit time.Duration) (capture.LiveResult, error) {
	return awaitCapture(ctx, events, stop, limit)
}
func ParseCodedErrorForTest(message string) error { return parseCodedError(message) }
