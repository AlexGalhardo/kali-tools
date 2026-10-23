//go:build darwin && !cgo

package wifi

import "github.com/AlexGalhardo/kali-tools/internal/domain"

func Current() (Network, error) {
	return Network{}, domain.Unsupported("CoreWLAN requires cgo")
}

func Reveal() (Secret, error) {
	return Secret{}, domain.Unsupported("Keychain requires cgo")
}
