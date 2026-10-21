package app

import "time"

func (s *Service) RunMetricsForTest(tick <-chan time.Time) { s.runMetrics(tick) }
