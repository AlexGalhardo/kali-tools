//go:build !windows

package capture

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func (Native) Status(context.Context) domain.Capability {
	return domain.Capability{ID: "capture_live", Provider: "raw_socket", State: domain.CapabilityUnsupported,
		Detail: "live capture provider is not implemented on this platform"}
}

func live(_ context.Context, _ LiveRequest, _ *Artifact) (LiveResult, error) {
	return LiveResult{Packets: []Packet{}, Limits: []string{}}, domain.Unsupported("live capture provider is not implemented on this platform")
}
