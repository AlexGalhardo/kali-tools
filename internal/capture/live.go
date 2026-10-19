package capture

import (
	"context"
	"time"
)

type LiveRequest struct {
	Address         string `json:"address"`
	DurationSeconds int    `json:"duration_seconds"`
	PacketLimit     int    `json:"packet_limit"`
}
type LiveResult struct {
	Packets  []Packet  `json:"packets"`
	Provider string    `json:"provider"`
	Dropped  *uint64   `json:"dropped"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
}
type LiveProvider interface {
	Capture(context.Context, LiveRequest) (LiveResult, error)
}
