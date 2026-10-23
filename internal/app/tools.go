package app

type Tool struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// Nmap is the service-inspection result. The name matches the generated Wails models.
// Scanner is "native". This value is not an Nmap document.
type Nmap struct {
	Scanner  string       `json:"scanner"`
	Version  string       `json:"version"`
	Finished NmapFinished `json:"finished"`
	Hosts    []NmapHost   `json:"hosts"`
}
type NmapFinished struct {
	Exit string `json:"exit"`
}
type NmapState struct {
	State string `json:"state"`
}
type NmapAddress struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}
type NmapService struct {
	Name    string `json:"name"`
	Product string `json:"product"`
	Version string `json:"version"`
	Method  string `json:"method"`
}
type NmapPort struct {
	Service  NmapService `json:"service"`
	Port     int         `json:"port"`
	Protocol string      `json:"protocol"`
	State    NmapState   `json:"state"`
}
type NmapHost struct {
	Status    NmapState     `json:"status"`
	Addresses []NmapAddress `json:"addresses"`
	Ports     []NmapPort    `json:"ports"`
}
