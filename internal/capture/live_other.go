//go:build !windows && !darwin

package capture

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func (Native) Status(context.Context) domain.Capability {
	return domain.Capability{ID: "capture_live", Provider: "raw_socket", State: domain.CapabilityUnsupported,
		Detail: "Live capture is not implemented in this build. For a pcap-tagged Linux binary, grant capture once with sudo setcap cap_net_raw+eip on that binary. Do not run the UI or the web server as root."}
}

func live(_ context.Context, _ LiveRequest, _ *Artifact) (LiveResult, error) {
	return LiveResult{Packets: []Packet{}, Limits: []string{}}, domain.Unsupported("live capture provider is not implemented on this platform")
}
