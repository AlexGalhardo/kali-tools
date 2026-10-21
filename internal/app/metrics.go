package app

import (
	"context"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const MetricsInterval = 2 * time.Second

func (a *Application) StartMetrics(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.deps.Host == nil || a.closed || a.metricsStarted {
		return
	}
	a.metricsStarted = true
	a.bg.Add(1)
	go func() {
		defer a.bg.Done()
		ticker := time.NewTicker(MetricsInterval)
		defer ticker.Stop()
		a.runMetrics(ticker.C)
	}()
}

func (a *Application) runMetrics(tick <-chan time.Time) {
	var tracker network.Tracker
	for {
		select {
		case <-a.done:
			return
		case now, ok := <-tick:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), MetricsInterval)
			snapshot, err := a.deps.Host.Inventory(ctx)
			cancel()
			if err == nil {
				a.jobs.Publish(job.TopicMetrics, tracker.Update(snapshot.Counters, now))
			}
		}
	}
}
