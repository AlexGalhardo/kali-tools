package fake

import (
	"context"
	"fmt"

	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

type WiFi struct {
	Network wifi.Network
	Secret  wifi.Secret
	Err     error
}

func (f WiFi) Current(ctx context.Context) (wifi.Network, error) {
	if err := ctx.Err(); err != nil {
		return wifi.Network{}, fmt.Errorf("fake Wi-Fi: %w", err)
	}
	v := f.Network
	if v.Channel != nil {
		channel := *v.Channel
		v.Channel = &channel
	}
	return v, f.Err
}
func (f WiFi) Reveal(ctx context.Context) (wifi.Secret, error) {
	if err := ctx.Err(); err != nil {
		return wifi.Secret{}, fmt.Errorf("fake reveal: %w", err)
	}
	return f.Secret, f.Err
}
