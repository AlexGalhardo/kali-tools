package app

import (
	"context"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/job"
)

func (a *Application) RunMetricsForTest(tick <-chan time.Time) { a.runMetrics(tick) }
func (a *Application) PublishForTest(topic string, data any)   { a.jobs.Publish(topic, data) }
func (a *Application) WaitForTest(ctx context.Context, id job.ID) (job.Status, error) {
	return a.jobs.Wait(ctx, id)
}
