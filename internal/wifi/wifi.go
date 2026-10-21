package wifi

import "context"

type Network struct {
	Channel        *uint32 `json:"channel,omitempty"`
	Profile        string  `json:"profile"`
	SSID           string  `json:"ssid"`
	BSSID          string  `json:"bssid"`
	Signal         uint32  `json:"signal"`
	Authentication uint32  `json:"authentication"`
	State          string  `json:"state"`
}
type Secret struct {
	State string `json:"state"`
	Value string `json:"value"`
}

type Native struct{}

func (Native) Current(ctx context.Context) (Network, error) {
	if err := ctx.Err(); err != nil {
		return Network{}, err
	}
	return Current()
}

func (Native) Reveal(ctx context.Context) (Secret, error) {
	if err := ctx.Err(); err != nil {
		return Secret{}, err
	}
	return Reveal()
}
