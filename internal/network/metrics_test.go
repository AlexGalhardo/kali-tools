package network

import (
	"testing"
	"time"
)

func TestTrackerRemovesDisappearedInterfaces(t *testing.T) {
	var tracker Tracker
	now := time.Now()
	tracker.Update([]Counter{{"a", 10, 20}}, now)
	got := tracker.Update([]Counter{{"b", 10000, 10000}}, now.Add(time.Second))
	if got.ReceiveBPS != 0 || len(got.Interfaces) != 0 {
		t.Fatal(got)
	}
	got = tracker.Update([]Counter{{"a", 20000, 20000}}, now.Add(2*time.Second))
	if got.ReceiveBPS != 0 {
		t.Fatal("stale counter reused", got)
	}
	got = tracker.Update([]Counter{{"a", 20100, 20200}}, now.Add(3*time.Second))
	if got.ReceiveBPS != 100 || got.SendBPS != 200 {
		t.Fatal(got)
	}
}
