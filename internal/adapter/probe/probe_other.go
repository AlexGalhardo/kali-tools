//go:build !windows

package probe

import (
	"context"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const Provider = "unavailable"

func Run(context.Context, string, bool) ([]string, error) {
	return nil, domain.Unsupported("native ICMP provider is not implemented on this platform")
}
