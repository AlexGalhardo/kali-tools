//go:build windows

package probe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestNativeLoopbackPingAndCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	values, e := Run(ctx, "127.0.0.1", false)
	var permission *domain.Error
	if errors.As(e, &permission) && permission.Code == "permission_required" {
		t.Skip("Native ICMP is denied by this host; acceptance remains pending")
	}
	if e != nil || len(values) != 5 {
		t.Fatal(values, e)
	}
	trace, err := Run(ctx, "127.0.0.1", true)
	if err != nil || len(trace) != 1 {
		t.Fatalf("native loopback trace: %v %v", trace, err)
	}
	cancelled, c := context.WithCancel(t.Context())
	c()
	_, e = Run(cancelled, "127.0.0.1", true)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestIPv6ICMPIsExplicitlyUnavailable(t *testing.T) {
	_, err := Run(t.Context(), "fe80::1%3", false)
	var typed *domain.Error
	if !errors.As(err, &typed) || typed.Code != "unsupported" {
		t.Fatalf("IPv6: %v", err)
	}
}
