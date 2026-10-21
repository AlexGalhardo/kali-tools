package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func TestMetricsPublishesOnTick(t *testing.T) {
	s := newService(t)
	calls := 0
	s.Counters = func(context.Context) ([]network.Counter, error) {
		calls++
		if calls == 2 {
			return nil, errors.New("synthetic sample failure")
		}
		return []network.Counter{}, nil
	}
	events, _ := s.Jobs.Subscribe(t.Context())
	ticks := make(chan time.Time)
	stopped := make(chan struct{})
	go func() { defer close(stopped); s.RunMetricsForTest(ticks) }()
	first := time.Unix(100, 0)
	ticks <- first
	select {
	case event := <-events:
		metrics, ok := event.Data.(network.Metrics)
		if event.Topic != job.TopicMetrics || !ok || !metrics.Timestamp.Equal(first) {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("missing first metrics sample")
	}
	ticks <- first.Add(app.MetricsInterval)
	ticks <- first.Add(2 * app.MetricsInterval)
	select {
	case event := <-events:
		metrics, ok := event.Data.(network.Metrics)
		if !ok || !metrics.Timestamp.Equal(first.Add(2*app.MetricsInterval)) {
			t.Fatalf("failed tick published: %+v", event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("missing third metrics sample")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("metrics loop survived Close")
	}
}

func TestStartMetricsIsIdempotent(t *testing.T) {
	s := newService(t)
	s.StartMetrics() // A nil sampler must leave StartMetrics available for wiring.
	s.Counters = func(context.Context) ([]network.Counter, error) { return []network.Counter{}, nil }
	s.StartMetrics()
	s.StartMetrics()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s.StartMetrics()
}
