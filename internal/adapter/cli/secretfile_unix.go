//go:build !windows

package cli

import (
	"os"
	"syscall"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func secretFileOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) {
		return domain.Invalid("secret file must be owned by the user")
	}
	return nil
}
