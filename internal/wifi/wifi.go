package wifi

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
