package contract

import (
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const (
	EventJob     = "job"
	EventMetrics = "metrics"
	EventCapture = "capture"
)

func EventPayload(ev job.Event) (string, any, bool) {
	switch ev.Topic {
	case job.TopicJob:
		if ev.Job == nil {
			return "", nil, false
		}
		var result *DiagnosticResult
		if ev.Data != nil {
			r, ok := ev.Data.(*network.Result)
			if !ok {
				return "", nil, false
			}
			if r != nil {
				mapped := FromDiagnosticResult(*r)
				result = &mapped
			}
		}
		return EventJob, JobEvent{SchemaVersion: ev.SchemaVersion, Seq: ev.Seq, Job: FromStatus(*ev.Job), Result: result}, true
	case job.TopicMetrics:
		v, ok := ev.Data.(network.Metrics)
		if !ok {
			return "", nil, false
		}
		return EventMetrics, FromMetrics(v), true
	case job.TopicCapture:
		v, ok := ev.Data.(app.CaptureEvent)
		if !ok {
			return "", nil, false
		}
		message := v.Error
		if v.Err != nil {
			message = FromError(v.Err).Error()
		}
		return EventCapture, CaptureEvent{Result: FromLiveResult(v.Result), Error: message}, true
	default:
		return "", nil, false
	}
}
