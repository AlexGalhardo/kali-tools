package contract

import "time"

// Error is the only error shape that leaves a driving adapter.
type Error struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	SettingsURI string `json:"settings_uri,omitempty"`
}

// Error keeps the "<code>: <message>" form that the frontend parses (App.tsx errorText).
func (e Error) Error() string { return e.Code + ": " + e.Message }

type Capability struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Detail   string `json:"detail"`
	Provider string `json:"provider"`
}

type Neighbor struct {
	Address        string `json:"address"`
	MAC            string `json:"mac"`
	Family         string `json:"family"`
	InterfaceIndex uint32 `json:"interface_index"`
	InterfaceName  string `json:"interface_name"`
	State          string `json:"state"`
	Router         bool   `json:"router"`
}

type NeighborTable struct {
	Neighbors []Neighbor `json:"neighbors"`
	Limited   bool       `json:"limited"`
	Provider  string     `json:"provider"`
}

type Connection struct {
	Protocol string `json:"protocol"`
	Local    string `json:"local"`
	Remote   string `json:"remote"`
	State    string `json:"state"`
	PID      int    `json:"pid"`
	Process  string `json:"process"`
}

type ConnectionTable struct {
	Connections []Connection `json:"connections"`
	Limited     bool         `json:"limited"`
	Provider    string       `json:"provider"`
}

type ThroughputSample struct {
	Endpoint   string `json:"endpoint"`
	DurationMS int64  `json:"duration_ms"`
	ByteCount  uint64 `json:"byte_count"`
	Source     string `json:"source"`
}

type Interface struct {
	Index     int      `json:"index"`
	Name      string   `json:"name"`
	MAC       string   `json:"mac"`
	MTU       int      `json:"mtu"`
	Up        bool     `json:"up"`
	Loopback  bool     `json:"loopback"`
	Addresses []string `json:"addresses"`
}

type Counter struct {
	ID       string `json:"id"`
	Received uint64 `json:"received"`
	Sent     uint64 `json:"sent"`
}

type Adapter struct {
	Index    uint32   `json:"index"`
	Name     string   `json:"name"`
	Gateways []string `json:"gateways"`
	DNS      []string `json:"dns"`
}

type Inventory struct {
	Adapters []Adapter `json:"adapters"`
	Counters []Counter `json:"counters"`
}

type DiagnosticRequest struct {
	RecordType string `json:"record_type,omitempty"`
	Kind       string `json:"kind"`
	Target     string `json:"target"`
	Port       int    `json:"port"`
	TimeoutMS  int    `json:"timeout_ms"`
	Redirects  int    `json:"redirects,omitempty"`
}

type DiagnosticResult struct {
	ErrorCode string        `json:"error_code,omitempty"`
	Kind      string        `json:"kind"`
	Target    string        `json:"target"`
	Status    string        `json:"status"`
	ElapsedMS float64       `json:"elapsed_ms"`
	Values    []string      `json:"values"`
	Provider  string        `json:"provider"`
	HTTP      *HTTPMetadata `json:"http,omitempty"`
}

type HTTPMetadata struct {
	StatusCode      int              `json:"status_code,omitempty"`
	FinalURL        string           `json:"final_url,omitempty"`
	Phases          HTTPPhases       `json:"phases"`
	SecurityHeaders []SecurityHeader `json:"security_headers"`
	Redirect        *RedirectTarget  `json:"redirect,omitempty"`
	Chain           []RedirectHop    `json:"chain,omitempty"`
	ChainStatus     string           `json:"chain_status,omitempty"`
}

type HTTPPhases struct {
	DNSMS     *float64 `json:"dns_ms"`
	ConnectMS *float64 `json:"connect_ms"`
	TLSMS     *float64 `json:"tls_ms"`
	TTFBMS    *float64 `json:"ttfb_ms"`
	TotalMS   float64  `json:"total_ms"`
}

type SecurityHeader struct {
	Name    string   `json:"name"`
	Present bool     `json:"present"`
	Value   string   `json:"value,omitempty"`
	Notes   []string `json:"notes,omitempty"`
}

type RedirectTarget struct {
	Location  string   `json:"location,omitempty"`
	Valid     bool     `json:"valid"`
	SelfLoop  bool     `json:"self_loop"`
	CrossHost bool     `json:"cross_host"`
	Downgrade bool     `json:"downgrade"`
	Notes     []string `json:"notes,omitempty"`
}

type RedirectHop struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Status     string `json:"status"`
}

type Job struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	State    string     `json:"state"`
	Created  time.Time  `json:"created"`
	Finished *time.Time `json:"finished,omitempty"`
	Summary  string     `json:"summary"`
}

// JobEvent is the payload of the "job" event.
type JobEvent struct {
	SchemaVersion int               `json:"schema_version"`
	Seq           uint64            `json:"seq"`
	Job           Job               `json:"job"`
	Result        *DiagnosticResult `json:"result,omitempty"`
}

type Settings struct {
	Language      string `json:"language"`
	RetentionDays int    `json:"retention_days"`
}

type PortResult struct {
	Host  string `json:"host"`
	Port  int    `json:"port"`
	State string `json:"state"`
}

type Packet struct {
	Number      int       `json:"number"`
	Timestamp   time.Time `json:"timestamp"`
	Length      int       `json:"length"`
	Source      string    `json:"source"`
	Destination string    `json:"destination"`
	Protocol    string    `json:"protocol"`
	Info        string    `json:"info"`
}

// Capture is an imported packet file summary (bridge.ts Capture).
type Capture struct {
	Packets []Packet `json:"packets"`
	Limited bool     `json:"limited"`
	Format  string   `json:"format"`
}

type LiveCaptureRequest struct {
	Backend         string `json:"backend,omitempty"`
	Address         string `json:"address"`
	DurationSeconds int    `json:"duration_seconds"`
	PacketLimit     int    `json:"packet_limit"`
}

type LiveCapture struct {
	Limited  bool      `json:"limited"`
	Packets  []Packet  `json:"packets"`
	Provider string    `json:"provider"`
	Dropped  *uint64   `json:"dropped"`
	Limits   []string  `json:"limits"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
}

// CaptureEvent is the payload of the "capture" event. Error is "" on success,
// otherwise FromError(err).Error().
type CaptureEvent struct {
	Result LiveCapture `json:"result"`
	Error  string      `json:"error"`
}

type Rate struct {
	ID         string    `json:"id"`
	ReceiveBPS float64   `json:"receive_bps"`
	SendBPS    float64   `json:"send_bps"`
	Timestamp  time.Time `json:"timestamp"`
}

// Metrics is the payload of the "metrics" event.
type Metrics struct {
	Timestamp  time.Time `json:"timestamp"`
	ReceiveBPS float64   `json:"receive_bps"`
	SendBPS    float64   `json:"send_bps"`
	Interfaces []Rate    `json:"interfaces"`
}

type WiFi struct {
	Channel        *uint32 `json:"channel,omitempty"`
	Profile        string  `json:"profile"`
	SSID           string  `json:"ssid"`
	BSSID          string  `json:"bssid"`
	Signal         uint32  `json:"signal"`
	Authentication uint32  `json:"authentication"`
	State          string  `json:"state"`
}

// WiFiSecret is the only DTO allowed to carry secret material, and only in Value.
type WiFiSecret struct {
	State string `json:"state"`
	Value string `json:"value"`
}

// String and GoString never print Value, so %v, %+v and %#v cannot leak it into logs.
func (s WiFiSecret) String() string   { return "{state:" + s.State + " value:[redacted]}" }
func (s WiFiSecret) GoString() string { return s.String() }

type Tool struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

type ServiceInspection struct {
	Scanner  string             `json:"scanner"`
	Version  string             `json:"version"`
	Finished InspectionFinished `json:"finished"`
	Hosts    []InspectionHost   `json:"hosts"`
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
