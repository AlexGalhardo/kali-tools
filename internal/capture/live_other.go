//go:build !windows

package capture

import (
	"context"
	"github.com/AlexGalhardo/hacker-tools/internal/domain"
)

func Live(_ context.Context, _ LiveRequest) (LiveResult, error) {
	return LiveResult{}, domain.Unsupported("live capture provider is not implemented on this platform")
}
