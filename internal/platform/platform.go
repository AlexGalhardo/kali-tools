package platform

import "github.com/AlexGalhardo/hacker-tools/internal/network"

type Adapter struct {
	Index    uint32   `json:"index"`
	Name     string   `json:"name"`
	Gateways []string `json:"gateways"`
	DNS      []string `json:"dns"`
}
type Snapshot struct {
	Adapters []Adapter         `json:"adapters"`
	Counters []network.Counter `json:"counters"`
}
