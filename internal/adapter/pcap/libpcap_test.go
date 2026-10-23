package pcap

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestLibpcapStatus(t *testing.T) {
	l := NewLibpcap()
	got := l.Status(t.Context())
	if got.ID != "capture_libpcap" || got.Provider != "libpcap" {
		t.Fatal(got)
	}
	switch {
	case runtime.GOOS != "linux":
		if got.State != domain.CapabilityUnsupported || got.Detail != "libpcap capture is a Linux backend" {
			t.Fatal(got)
		}
	case !libpcapBuilt:
		if got.State != domain.CapabilityUnsupported || got.Detail != "this build does not include libpcap" {
			t.Fatal(got)
		}
	default:
		if got.State != domain.CapabilityAvailable || !strings.Contains(got.Detail, "CAP_NET_RAW") || strings.Contains(strings.ToLower(got.Detail), "sudo") {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if l.Status(ctx).State != domain.CapabilityFailed {
		t.Fatal("cancelled status probe ignored")
	}
}

func TestLibpcapOpenErrorRedactsAndClassifies(t *testing.T) {
	denied := libpcapError(errors.New("socket: permission denied synthetic-secret"), "libpcap could not open the device")
	var failure *domain.Error
	if !errors.As(denied, &failure) || failure.Code != domain.CodePermissionRequired || strings.Contains(failure.Message, "synthetic-secret") || !strings.Contains(failure.Message, "CAP_NET_RAW") {
		t.Fatal(denied)
	}
	if strings.Contains(strings.ToLower(failure.Message), "sudo") {
		t.Fatal(failure.Message)
	}
	notPermitted := libpcapError(errors.New("operation not permitted"), "libpcap could not open the device")
	if !errors.As(notPermitted, &failure) || failure.Code != domain.CodePermissionRequired {
		t.Fatal(notPermitted)
	}
	other := libpcapError(errors.New("no such device /tmp/synthetic-secret"), "libpcap device enumeration failed")
	if !errors.As(other, &failure) || failure.Code != domain.CodeFailed || failure.Message != "libpcap device enumeration failed" {
		t.Fatal(other)
	}
}
