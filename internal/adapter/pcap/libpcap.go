package pcap

import (
	"context"
	"runtime"
	"strings"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const libpcapCapabilityDetail = "libpcap opens one interface; CAP_NET_RAW is required on the capture process. The UI and server are not elevated."

// Libpcap captures on Linux through libpcap. The UI and the web server stay
// unprivileged; opening an interface needs CAP_NET_RAW on this process.
type Libpcap struct{}

func NewLibpcap() *Libpcap { return &Libpcap{} }

func (l *Libpcap) Status(ctx context.Context) domain.Capability {
	c := domain.Capability{ID: "capture_libpcap", Provider: "libpcap"}
	if ctx.Err() != nil {
		c.State, c.Detail = domain.CapabilityFailed, "libpcap status cancelled"
		return c
	}
	if runtime.GOOS != "linux" {
		c.State, c.Detail = domain.CapabilityUnsupported, "libpcap capture is a Linux backend"
		return c
	}
	if !libpcapBuilt {
		c.State, c.Detail = domain.CapabilityUnsupported, "this build does not include libpcap"
		return c
	}
	c.State, c.Detail = domain.CapabilityAvailable, libpcapCapabilityDetail
	return c
}

func libpcapError(err error, fallback string) error {
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "permission denied") || strings.Contains(text, "operation not permitted") {
		return &domain.Error{Code: domain.CodePermissionRequired, Message: "libpcap could not open the device: CAP_NET_RAW is required on the capture process. The UI and server are not elevated."}
	}
	return &domain.Error{Code: domain.CodeFailed, Message: fallback}
}
