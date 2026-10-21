package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"runtime"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
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
	result, err := cli.AwaitCaptureForTest(t.Context(), events, func() { t.Error("unexpected stop") }, 200*time.Millisecond)
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
	_, err := cli.AwaitCaptureForTest(ctx, events, func() {
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
	_, err := cli.AwaitCaptureForTest(t.Context(), events, func() { calls.Add(1) }, 10*time.Millisecond)
	var typed contract.Error
	if calls.Load() != 1 || !errors.As(err, &typed) || typed.Code != "timeout" {
		t.Fatalf("stop calls=%d, err=%v", calls.Load(), err)
	}
}

func TestAwaitCaptureClosedChannel(t *testing.T) {
	events := make(chan job.Event)
	close(events)
	if _, err := cli.AwaitCaptureForTest(t.Context(), events, func() {}, 200*time.Millisecond); !errors.Is(err, job.ErrClosed) {
		t.Fatal(err)
	}
}

func TestCLICaptureRejectsLoopback(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := cli.Run(t.Context(), newApp(t), []string{"capture", "--target", "127.0.0.1"}, &out, &stderr); code != 2 || !bytes.Contains(out.Bytes(), []byte(`"invalid_input"`)) {
		t.Fatalf("exit=%d, out=%s, stderr=%s", code, out.String(), stderr.String())
	}
}

func TestParseCodedError(t *testing.T) {
	if err := cli.ParseCodedErrorForTest(""); err != nil {
		t.Fatal(err)
	}
	for _, code := range domain.ErrorCodes() {
		t.Run(code, func(t *testing.T) {
			var typed contract.Error
			err := cli.ParseCodedErrorForTest(code + ": synthetic")
			if !errors.As(err, &typed) || typed.Code != code || typed.Message != "synthetic" {
				t.Fatal(err)
			}
		})
	}
	for _, message := range []string{"unknown: synthetic", "synthetic failure"} {
		var typed contract.Error
		err := cli.ParseCodedErrorForTest(message)
		if !errors.As(err, &typed) || typed.Code != "failed" || typed.Message != message {
			t.Fatal(err)
		}
	}
}

func TestAwaitCaptureLegacyErrorAndUnexpectedData(t *testing.T) {
	events := make(chan job.Event, 2)
	events <- job.Event{Topic: job.TopicCapture, Data: "unexpected"}
	events <- job.Event{Topic: job.TopicCapture, Data: app.CaptureEvent{Error: "permission_required: synthetic denial"}}
	_, err := cli.AwaitCaptureForTest(t.Context(), events, func() { t.Error("unexpected stop") }, 200*time.Millisecond)
	var typed contract.Error
	if !errors.As(err, &typed) || typed.Code != "permission_required" || typed.Message != "synthetic denial" {
		t.Fatal(err)
	}
}

type captureProvider struct{ code string }

func (f captureProvider) Capture(context.Context, capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
	return capture.LiveResult{Packets: []capture.Packet{}, Provider: "synthetic"}, nil, &domain.Error{Code: f.code, Message: "synthetic provider state"}
}

func TestCLICaptureReportsProviderState(t *testing.T) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	var address string
	for _, candidate := range addresses {
		prefix, err := netip.ParsePrefix(candidate.String())
		if err == nil && prefix.Addr().Is4() && !prefix.Addr().IsLoopback() {
			address = prefix.Addr().String()
			break
		}
	}
	if address == "" {
		t.Skip("no local non-loopback IPv4 for validation")
	}
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	code := "unsupported"
	if runtime.GOOS == "windows" {
		code = "permission_required"
	}
	a := app.New(app.Deps{Jobs: store, Settings: store, Capture: captureProvider{code: code}})
	defer func() { _ = a.Close() }()
	var out, stderr bytes.Buffer
	start := time.Now()
	exitCode := cli.Run(t.Context(), a, []string{"capture", "--target", address, "--duration", "1", "--limit", "1"}, &out, &stderr)
	if time.Since(start) >= 11*time.Second {
		t.Fatal("capture exceeded bound")
	}
	var result envelope
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if exitCode != 1 || result.Error == nil || result.Error.Code != code {
		t.Fatalf("exit=%d, out=%s", exitCode, out.String())
	}
}

func TestCaptureCancellationGraceIsBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		events := make(chan job.Event)
		calls := 0
		before := time.Now()
		_, err := cli.AwaitCaptureForTest(ctx, events, func() { calls++ }, 200*time.Millisecond)
		if !errors.Is(err, context.Canceled) || calls != 1 || time.Since(before) != 5*time.Second {
			t.Fatalf("elapsed=%s, calls=%d, err=%v", time.Since(before), calls, err)
		}
	})
}
