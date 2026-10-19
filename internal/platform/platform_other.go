//go:build !windows && !linux

package platform

import "github.com/AlexGalhardo/hacker-tools/internal/domain"

func Inventory() (Snapshot, error) {
	return Snapshot{}, domain.Unsupported("native route and counter provider is not available on this platform")
}
