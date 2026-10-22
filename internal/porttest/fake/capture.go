package fake

import (
	"context"
	"fmt"
	"slices"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

type LiveCapture struct {
	Capability  domain.Capability
	Result      capture.LiveResult
	Err         error
	CaptureFunc func(context.Context, capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error)
}

func (f LiveCapture) Status(context.Context) domain.Capability {
	if f.Capability.State != "" {
		return f.Capability
	}
	return domain.Capability{ID: "capture_live", State: "available", Provider: "fake", Detail: "synthetic capture"}
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
	v.Limits = slices.Clone(v.Limits)
	if v.Dropped != nil {
		dropped := *v.Dropped
		v.Dropped = &dropped
	}
	return v, artifact, err
}
