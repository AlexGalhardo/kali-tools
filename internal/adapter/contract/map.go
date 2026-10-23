package contract

import (
	"slices"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

func mapSlice[S, D any](values []S, convert func(S) D) []D {
	if values == nil {
		return nil
	}
	out := make([]D, len(values))
	for i, v := range values {
		out[i] = convert(v)
	}
	return out
}
func copyPointer[T any](v *T) *T {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}
func FromCapabilities(v []domain.Capability) []Capability {
	return mapSlice(v, func(c domain.Capability) Capability {
		return Capability{ID: c.ID, State: c.State, Detail: c.Detail, Provider: c.Provider}
	})
}

func FromNeighborTable(v network.NeighborTable) NeighborTable {
	return NeighborTable{Neighbors: mapSlice(v.Neighbors, func(row network.Neighbor) Neighbor {
		return Neighbor{Address: row.Address, MAC: row.MAC, Family: row.Family,
			InterfaceIndex: row.InterfaceIndex, InterfaceName: row.InterfaceName, State: row.State, Router: row.Router}
	}), Limited: v.Limited, Provider: v.Provider}
}
func FromInterfaces(v []network.InterfaceInfo) []Interface {
	return mapSlice(v, func(i network.InterfaceInfo) Interface {
		return Interface{Index: i.Index, Name: i.Name, MAC: i.MAC, MTU: i.MTU, Up: i.Up, Loopback: i.Loopback, Addresses: slices.Clone(i.Addresses)}
	})
}
func FromInventory(v app.HostSnapshot) Inventory {
	return Inventory{Adapters: mapSlice(v.Adapters, func(a app.HostAdapter) Adapter {
		return Adapter{Index: a.Index, Name: a.Name, Gateways: slices.Clone(a.Gateways), DNS: slices.Clone(a.DNS)}
	}), Counters: mapSlice(v.Counters, func(c network.Counter) Counter { return Counter{ID: c.ID, Received: c.Received, Sent: c.Sent} })}
}
func (r DiagnosticRequest) Request() network.Request {
	return network.Request{RecordType: r.RecordType, Kind: r.Kind, Target: r.Target, Port: r.Port, TimeoutMS: r.TimeoutMS, Redirects: r.Redirects}
}
func FromDiagnosticResult(v network.Result) DiagnosticResult {
	return DiagnosticResult{ErrorCode: v.ErrorCode, Kind: v.Kind, Target: v.Target, Status: v.Status, ElapsedMS: v.ElapsedMS, Values: slices.Clone(v.Values), Provider: v.Provider, HTTP: fromHTTP(v.HTTP)}
}
func fromHTTP(v *network.HTTPMetadata) *HTTPMetadata {
	if v == nil {
		return nil
	}
	out := &HTTPMetadata{StatusCode: v.StatusCode, FinalURL: v.FinalURL, Phases: clonePhases(v.Phases), SecurityHeaders: make([]SecurityHeader, len(v.SecurityHeaders)), Chain: make([]RedirectHop, len(v.Chain)), ChainStatus: v.ChainStatus}
	for i, h := range v.SecurityHeaders {
		out.SecurityHeaders[i] = SecurityHeader{Name: h.Name, Present: h.Present, Value: h.Value, Notes: slices.Clone(h.Notes)}
	}
	for i, hop := range v.Chain {
		out.Chain[i] = RedirectHop{URL: hop.URL, StatusCode: hop.StatusCode, Status: hop.Status}
	}
	if v.Redirect != nil {
		redir := *v.Redirect
		redir.Notes = slices.Clone(v.Redirect.Notes)
		out.Redirect = &RedirectTarget{Location: redir.Location, Valid: redir.Valid, SelfLoop: redir.SelfLoop, CrossHost: redir.CrossHost, Downgrade: redir.Downgrade, Notes: redir.Notes}
	}
	return out
}
func clonePhases(v network.HTTPPhases) HTTPPhases {
	return HTTPPhases{DNSMS: cloneMS(v.DNSMS), ConnectMS: cloneMS(v.ConnectMS), TLSMS: cloneMS(v.TLSMS), TTFBMS: cloneMS(v.TTFBMS), TotalMS: v.TotalMS}
}
func cloneMS(v *float64) *float64 {
	if v == nil {
		return nil
	}
	n := *v
	return &n
}
func FromStatus(v job.Status) Job {
	return Job{ID: string(v.ID), Kind: v.Kind, State: string(v.State), Created: v.Created, Finished: copyPointer(v.Finished), Summary: v.Summary}
}
func FromStatuses(v []job.Status) []Job { return mapSlice(v, FromStatus) }
func FromSettings(v app.Settings) Settings {
	return Settings{Language: v.Language, RetentionDays: v.RetentionDays}
}
func (s Settings) AppSettings() app.Settings {
	return app.Settings{Language: s.Language, RetentionDays: s.RetentionDays}
}
func FromPortResults(v []network.PortResult) []PortResult {
	return mapSlice(v, func(p network.PortResult) PortResult { return PortResult{Host: p.Host, Port: p.Port, State: p.State} })
}
func fromPacket(v capture.Packet) Packet {
	return Packet{Number: v.Number, Timestamp: v.Timestamp, Length: v.Length, Source: v.Source, Destination: v.Destination, Protocol: v.Protocol, Info: v.Info}
}
func FromImportResult(v capture.ImportResult) Capture {
	return Capture{Packets: mapSlice(v.Packets, fromPacket), Limited: v.Limited, Format: v.Format}
}
func (r LiveCaptureRequest) LiveRequest() capture.LiveRequest {
	return capture.LiveRequest{Backend: r.Backend, Address: r.Address, DurationSeconds: r.DurationSeconds, PacketLimit: r.PacketLimit}
}
func FromLiveResult(v capture.LiveResult) LiveCapture {
	return LiveCapture{Limited: v.Limited, Packets: mapSlice(v.Packets, fromPacket), Provider: v.Provider, Dropped: copyPointer(v.Dropped), Limits: mapSlice(v.Limits, func(s string) string { return s }), Started: v.Started, Finished: v.Finished}
}
func FromMetrics(v network.Metrics) Metrics {
	return Metrics{Timestamp: v.Timestamp, ReceiveBPS: v.ReceiveBPS, SendBPS: v.SendBPS, Interfaces: mapSlice(v.Interfaces, func(r network.Rate) Rate {
		return Rate{ID: r.ID, ReceiveBPS: r.ReceiveBPS, SendBPS: r.SendBPS, Timestamp: r.Timestamp}
	})}
}
func FromWiFi(v wifi.Network) WiFi {
	return WiFi{Channel: copyPointer(v.Channel), Profile: v.Profile, SSID: v.SSID, BSSID: v.BSSID, Signal: v.Signal, Authentication: v.Authentication, State: v.State}
}
func FromWiFiSecret(v wifi.Secret) WiFiSecret { return WiFiSecret{State: v.State, Value: v.Value} }
func FromTools(v []app.Tool) []Tool {
	return mapSlice(v, func(t app.Tool) Tool { return Tool{Name: t.Name, State: t.State, Version: t.Version, Path: t.Path} })
}
func FromServiceInspection(v app.Nmap) ServiceInspection {
	return ServiceInspection{Scanner: v.Scanner, Version: v.Version, Finished: NmapFinished{Exit: v.Finished.Exit}, Hosts: mapSlice(v.Hosts, func(h app.NmapHost) NmapHost {
		return NmapHost{Status: NmapState{State: h.Status.State}, Addresses: mapSlice(h.Addresses, func(a app.NmapAddress) NmapAddress { return NmapAddress{Address: a.Address, Type: a.Type} }), Ports: mapSlice(h.Ports, func(p app.NmapPort) NmapPort {
			return NmapPort{Service: NmapService{Name: p.Service.Name, Product: p.Service.Product, Version: p.Service.Version, Method: p.Service.Method}, Port: p.Port, Protocol: p.Protocol, State: NmapState{State: p.State.State}}
		})}
	})}
}
