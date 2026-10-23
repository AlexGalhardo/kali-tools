//go:build !linux

package conn

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func Read(ctx context.Context) (Table, error) {
	if err := ctx.Err(); err != nil {
		return emptyTable(), err
	}
	return emptyTable(), domain.Unsupported("socket-to-process mapping is not implemented on this platform")
}
