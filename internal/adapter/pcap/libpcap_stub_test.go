//go:build !(linux && pcap)

package pcap

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestLibpcapStubDoesNotCapture(t *testing.T) {
	l := NewLibpcap()
	_, _, err := l.Capture(t.Context(), capture.LiveRequest{Backend: capture.BackendLibpcap, Address: "127.0.0.1", DurationSeconds: 1, PacketLimit: 1})
	var failure *domain.Error
	if !errors.As(err, &failure) || failure.Code != domain.CodeUnsupported || failure.Message != "this build does not include the libpcap capture backend" {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := l.Capture(ctx, capture.LiveRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
