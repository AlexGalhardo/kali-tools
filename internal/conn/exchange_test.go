package conn

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExchangeReportsEndpointDurationAndBytes(t *testing.T) {
	sample, err := Exchange(t.Context(), 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if sample.Source != "kali-tools" || sample.DurationMS != 50 || sample.ByteCount == 0 || !stringsHasPrefix(sample.Endpoint, "127.0.0.1:") {
		t.Fatalf("%+v", sample)
	}
}

func TestExchangeRejectsDurationAndCancellation(t *testing.T) {
	if _, err := Exchange(t.Context(), 0); err == nil {
		t.Fatal("accepted zero duration")
	}
	if _, err := Exchange(t.Context(), 11*time.Second); err == nil {
		t.Fatal("accepted long duration")
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := Exchange(ctx, 10*time.Second)
		done <- err
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not stop the exchange")
	}
}

func TestMeasureRequiresOneInput(t *testing.T) {
	if _, err := Measure(t.Context(), Request{}); err == nil {
		t.Fatal("accepted an empty request")
	}
	if _, err := Measure(t.Context(), Request{Duration: time.Second, IperfJSON: []byte(`{}`)}); err == nil {
		t.Fatal("accepted both inputs")
	}
}

func stringsHasPrefix(value, prefix string) bool {
	return len(value) >= len(prefix) && value[:len(prefix)] == prefix
}
