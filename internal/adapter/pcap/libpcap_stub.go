//go:build !(linux && pcap)

package pcap

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const libpcapBuilt = false

func (l *Libpcap) Capture(ctx context.Context, _ capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return capture.LiveResult{}, nil, err
	}
	return capture.LiveResult{}, nil, domain.Unsupported("this build does not include the libpcap capture backend")
}
