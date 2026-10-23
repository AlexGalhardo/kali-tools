package conn

import "time"

const (
	maxFileBytes = 1 << 20
	maxRows      = 4096
	maxPIDs      = 4096
	maxFDs       = 1024
	minDuration  = time.Millisecond
	maxDuration  = 10 * time.Second
)

// Connection is one socket whose owning process was still visible.
type Connection struct {
	Protocol string `json:"protocol"`
	Local    string `json:"local"`
	Remote   string `json:"remote"`
	State    string `json:"state"`
	PID      int    `json:"pid"`
	Process  string `json:"process"`
}

// Table is a socket inventory. It has no interface counters and no rate.
type Table struct {
	Connections []Connection `json:"connections"`
	Limited     bool         `json:"limited"`
	Provider    string       `json:"provider"`
}

// Sample is a measured or imported transfer. It is not a bandwidth estimate.
type Sample struct {
	Endpoint   string `json:"endpoint"`
	DurationMS int64  `json:"duration_ms"`
	ByteCount  uint64 `json:"byte_count"`
	Source     string `json:"source"`
}

// Request selects a loopback measurement or an iperf3 -J document, not both.
type Request struct {
	Duration  time.Duration
	IperfJSON []byte
}

func emptyTable() Table {
	return Table{Connections: []Connection{}}
}
