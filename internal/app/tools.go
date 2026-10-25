package app

type Tool struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// ServiceInspection is the native TCP service-inspection result.
// Scanner is "native" and Version is the probe-set revision.
// JSON fields stay scanner, version, finished and hosts.
type ServiceInspection struct {
	Scanner  string             `json:"scanner"`
	Version  string             `json:"version"`
	Finished InspectionFinished `json:"finished"`
	Hosts    []InspectionHost   `json:"hosts"`
	// Truncated is set when the deadline cut the scan; Hosts holds what finished.
	Truncated bool `json:"truncated"`
}
type InspectionFinished struct {
	Exit string `json:"exit"`
}
type InspectionState struct {
	State string `json:"state"`
}
type InspectionAddress struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}
type InspectionService struct {
	Name    string `json:"name"`
	Product string `json:"product"`
	Version string `json:"version"`
	Method  string `json:"method"`
}
type InspectionPort struct {
	Service  InspectionService `json:"service"`
	Port     int               `json:"port"`
	Protocol string            `json:"protocol"`
	State    InspectionState   `json:"state"`
}
type InspectionHost struct {
	Status    InspectionState     `json:"status"`
	Addresses []InspectionAddress `json:"addresses"`
	Ports     []InspectionPort    `json:"ports"`
}
