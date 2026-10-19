//go:build !windows

package wifi

import "github.com/AlexGalhardo/hacker-tools/internal/domain"

func Current() (Network, error) {
	return Network{}, domain.Unsupported("Wi-Fi provider is not implemented for this platform")
}
func Reveal() (Secret, error) {
	return Secret{}, domain.Unsupported("Wi-Fi key provider is not implemented for this platform")
}
