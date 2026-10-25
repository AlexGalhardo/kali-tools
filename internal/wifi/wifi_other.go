//go:build !windows && !linux && !darwin

package wifi

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func Current() (Network, error) {
	return Network{}, domain.Unsupported("Wi-Fi provider is not implemented for this platform")
}
func Reveal(context.Context) (Secret, error) {
	return Secret{}, domain.Unsupported("Wi-Fi key provider is not implemented for this platform")
}
