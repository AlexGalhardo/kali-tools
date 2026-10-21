package main

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

func TestAwaitCapturePreservesTypedError(t *testing.T) {
	events := make(chan job.Event, 2)
	events <- job.Event{Topic: job.TopicMetrics}
	failure := &domain.Error{Code: "permission_required", Message: "synthetic denial"}
	events <- job.Event{Topic: job.TopicCapture, Data: app.CaptureEvent{Result: capture.LiveResult{Provider: "synthetic"}, Err: failure}}
	result, err := awaitCapture(t.Context(), events, func() { t.Error("unexpected stop") }, 200*time.Millisecond)
	var typed *domain.Error
	if result.Provider != "synthetic" || !errors.As(err, &typed) || typed.Code != "permission_required" {
		t.Fatalf("result=%+v, err=%v", result, err)
	}
}

func TestAwaitCaptureStopsOnCancel(t *testing.T) {
	events := make(chan job.Event, 1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var calls atomic.Int32
	_, err := awaitCapture(ctx, events, func() {
		calls.Add(1)
		events <- job.Event{Topic: job.TopicCapture, Data: app.CaptureEvent{Err: context.Canceled}}
	}, 200*time.Millisecond)
	if calls.Load() != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("stop calls=%d, err=%v", calls.Load(), err)
	}
}

func TestAwaitCaptureTimesOut(t *testing.T) {
	events := make(chan job.Event)
	var calls atomic.Int32
	_, err := awaitCapture(t.Context(), events, func() { calls.Add(1) }, 10*time.Millisecond)
	var typed *domain.Error
	if calls.Load() != 1 || !errors.As(err, &typed) || typed.Code != "timeout" {
		t.Fatalf("stop calls=%d, err=%v", calls.Load(), err)
	}
}

func TestAwaitCaptureClosedChannel(t *testing.T) {
	events := make(chan job.Event)
	close(events)
	if _, err := awaitCapture(t.Context(), events, func() {}, 200*time.Millisecond); !errors.Is(err, job.ErrClosed) {
		t.Fatal(err)
	}
}

func TestCLICaptureRejectsLoopback(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run(t.Context(), []string{"capture", "--target", "127.0.0.1"}, noEnv, &out, &stderr); code != 2 || !bytes.Contains(out.Bytes(), []byte(`"invalid_input"`)) {
		t.Fatalf("exit=%d, out=%s, stderr=%s", code, out.String(), stderr.String())
	}
}
