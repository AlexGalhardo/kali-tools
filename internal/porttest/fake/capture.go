package fake

import (
	"context"
	"fmt"
	"slices"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
)

type LiveCapture struct {
	Result      capture.LiveResult
	Err         error
	CaptureFunc func(context.Context, capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error)
}

func (f LiveCapture) Capture(ctx context.Context, r capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return capture.LiveResult{}, nil, fmt.Errorf("fake capture: %w", err)
	}
	if err := capture.ValidateLiveRequest(r); err != nil {
		return capture.LiveResult{}, nil, err
	}
	v, err := f.Result, f.Err
	var artifact *capture.Artifact
	if f.CaptureFunc != nil {
		v, artifact, err = f.CaptureFunc(ctx, r)
	}
	v.Packets = slices.Clone(v.Packets)
	if v.Dropped != nil {
		dropped := *v.Dropped
		v.Dropped = &dropped
	}
	return v, artifact, err
}
