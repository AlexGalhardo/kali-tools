//go:build !windows

package tools

import (
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func detectPktmon() (PktmonStatus, error) {
	return PktmonStatus{}, domain.Unsupported("Pktmon exists only on Windows")
}

func pktmonDetectionFailure(error) domain.Capability {
	return domain.Capability{ID: "capture_pktmon", Provider: "pktmon", State: domain.CapabilityUnsupported, Detail: "Pktmon exists only on Windows"}
}
