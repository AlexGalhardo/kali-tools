package app

import (
	"context"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const MetricsInterval = 2 * time.Second

func (s *Service) StartMetrics() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deps.Host == nil || s.closed || s.metricsStarted {
		return
	}
	s.metricsStarted = true
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		ticker := time.NewTicker(MetricsInterval)
		defer ticker.Stop()
		s.runMetrics(ticker.C)
	}()
}

func (s *Service) runMetrics(tick <-chan time.Time) {
	var tracker network.Tracker
	for {
		select {
		case <-s.done:
			return
		case now, ok := <-tick:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), MetricsInterval)
			snapshot, err := s.deps.Host.Inventory(ctx)
			cancel()
			if err == nil {
				s.Jobs.Publish(job.TopicMetrics, tracker.Update(snapshot.Counters, now))
			}
		}
	}
}
