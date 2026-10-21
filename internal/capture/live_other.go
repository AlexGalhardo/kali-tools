//go:build !windows

package capture

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func live(_ context.Context, _ LiveRequest, _ *Artifact) (LiveResult, error) {
	return LiveResult{}, domain.Unsupported("live capture provider is not implemented on this platform")
}
