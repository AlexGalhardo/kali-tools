// Package job owns bounded job lifecycles and ordered event subscriptions.
package job

import (
	"errors"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const (
	DefaultMaxConcurrent    = 8
	DefaultSubscriberBuffer = 256
	MaxConcurrentLimit      = 64
	MaxSubscriberBuffer     = 4096
	MaxFinished             = 256
	TerminalWait            = 2 * time.Second
	TopicJob                = "job"
	TopicMetrics            = "metrics"
	TopicCapture            = "capture"
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

func (s State) Terminal() bool { return s == Completed || s == Failed || s == Cancelled }

type Status struct {
	ID       ID         `json:"id"`
	Kind     string     `json:"kind"`
	State    State      `json:"state"`
	Created  time.Time  `json:"created"`
	Finished *time.Time `json:"finished,omitempty"`
	Summary  string     `json:"summary"`
}

type Event struct {
	SchemaVersion int       `json:"schema_version"`
	Seq           uint64    `json:"seq"`
	Topic         string    `json:"topic"`
	Job           *Status   `json:"job,omitempty"`
	Data          any       `json:"data,omitempty"`
	At            time.Time `json:"at"`
}

type Limits struct {
	MaxConcurrent    int
	SubscriberBuffer int
}

var (
	ErrClosed   = &domain.Error{Code: domain.CodeUnavailable, Message: "service is shutting down"}
	ErrBusy     = &domain.Error{Code: domain.CodeConflict, Message: "maximum concurrent operations reached"}
	ErrNotFound = errors.New("job: not found")
)
