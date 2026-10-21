package wailsui

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const (
	eventJob     = "job"
	eventMetrics = "metrics"
	eventCapture = "capture"
)

type jobPayload struct {
	SchemaVersion int             `json:"schema_version"`
	Seq           uint64          `json:"seq"`
	Job           job.Status      `json:"job"`
	Result        *network.Result `json:"result,omitempty"`
}
type capturePayload struct {
	Result capture.LiveResult `json:"result"`
	Error  string             `json:"error"`
}

func (b *Bindings) forward(ctx, fctx context.Context, events <-chan job.Event) {
	defer close(b.done)
	for {
		select {
		case <-fctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			name, payload, ok := eventPayload(event)
			if ok {
				b.emit(ctx, name, payload)
			}
		}
	}
}

func eventPayload(event job.Event) (string, any, bool) {
	switch event.Topic {
	case job.TopicJob:
		if event.Job == nil {
			return "", nil, false
		}
		var result *network.Result
		if event.Data != nil {
			var ok bool
			result, ok = event.Data.(*network.Result)
			if !ok {
				return "", nil, false
			}
		}
		return eventJob, jobPayload{SchemaVersion: event.SchemaVersion, Seq: event.Seq, Job: *event.Job, Result: result}, true
	case job.TopicMetrics:
		metrics, ok := event.Data.(network.Metrics)
		if !ok {
			return "", nil, false
		}
		return eventMetrics, metrics, true
	case job.TopicCapture:
		outcome, ok := event.Data.(app.CaptureEvent)
		if !ok {
			return "", nil, false
		}
		return eventCapture, capturePayload{Result: outcome.Result, Error: outcome.Error}, true
	default:
		return "", nil, false
	}
}
