package network

import "time"

type Metrics struct {
	Timestamp  time.Time `json:"timestamp"`
	ReceiveBPS float64   `json:"receive_bps"`
	SendBPS    float64   `json:"send_bps"`
	Interfaces []Rate    `json:"interfaces"`
}
type Tracker struct {
	previous map[string]Counter
	time     time.Time
}

func (t *Tracker) Update(counters []Counter, now time.Time) Metrics {
	result := Metrics{Timestamp: now.UTC(), Interfaces: []Rate{}}
	next := make(map[string]Counter, len(counters))
	for _, c := range counters {
		if p, ok := t.previous[c.ID]; ok {
			rate := Delta(p, c, now.Sub(t.time), now.UTC())
			result.Interfaces = append(result.Interfaces, rate)
			result.ReceiveBPS += rate.ReceiveBPS
			result.SendBPS += rate.SendBPS
		}
		next[c.ID] = c
	}
	t.previous = next
	t.time = now
	return result
}
