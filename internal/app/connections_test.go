package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
)

func TestThroughputCancellation(t *testing.T) {
	application := app.New(fakeDeps())
	t.Cleanup(func() { _ = application.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := application.Throughput(ctx, time.Second, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
