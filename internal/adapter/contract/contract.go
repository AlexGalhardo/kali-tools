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
}

type DiagnosticResult struct {
	ErrorCode string   `json:"error_code,omitempty"`
	Kind      string   `json:"kind"`
	Target    string   `json:"target"`
	Status    string   `json:"status"`
	ElapsedMS float64  `json:"elapsed_ms"`
	Values    []string `json:"values"`
	Provider  string   `json:"provider"`
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
