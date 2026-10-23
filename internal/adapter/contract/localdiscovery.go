package contract

import (
	"encoding/json"
	"slices"

	"github.com/AlexGalhardo/kali-tools/internal/localdiscovery"
)

func (r LocalDiscoveryRequest) Request() localdiscovery.Request {
	return localdiscovery.Request{
		Scope:           localdiscovery.Scope{Interface: r.Scope.Interface, CIDR: r.Scope.CIDR},
		Protocols:       slices.Clone(r.Protocols),
		DurationSeconds: r.DurationSeconds,
		ServiceTypes:    slices.Clone(r.ServiceTypes),
		SSDPTarget:      r.SSDPTarget,
	}
}

func FromLocalDiscovery(v localdiscovery.Result) LocalDiscoveryResult {
	return LocalDiscoveryResult{
		Responders: mapSlice(v.Responders, func(item localdiscovery.Responder) LocalResponder {
			return LocalResponder{
				Protocol: item.Protocol, Address: item.Address, Port: item.Port, Name: item.Name, Host: item.Host,
				ServiceType: item.ServiceType, TXT: slices.Clone(item.TXT), Server: item.Server, Location: item.Location,
			}
		}),
		Limited: v.Limited, OutOfScope: v.OutOfScope, Malformed: v.Malformed, Errors: slices.Clone(v.Errors),
	}
}

func (r SNMPRequest) Request() (localdiscovery.SNMPRequest, error) {
	out := localdiscovery.SNMPRequest{
		Scope:  localdiscovery.Scope{Interface: r.Scope.Interface, CIDR: r.Scope.CIDR},
		Target: r.Target, Port: r.Port, Owned: r.Owned, Version: r.Version, TimeoutMS: r.TimeoutMS, Interfaces: r.Interfaces,
	}
	if r.Community != "" {
		secret, err := localdiscovery.NewSecret(r.Community)
		if err != nil {
			return localdiscovery.SNMPRequest{}, err
		}
		out.Community = secret
	}
	if r.V3 != nil {
		creds := &localdiscovery.SNMPv3Creds{User: r.V3.User, AuthProtocol: r.V3.AuthProtocol, PrivProtocol: r.V3.PrivProtocol}
		if r.V3.AuthPass != "" {
			secret, err := localdiscovery.NewSecret(r.V3.AuthPass)
			if err != nil {
				return localdiscovery.SNMPRequest{}, err
			}
			creds.AuthPass = secret
		}
		if r.V3.PrivPass != "" {
			secret, err := localdiscovery.NewSecret(r.V3.PrivPass)
			if err != nil {
				return localdiscovery.SNMPRequest{}, err
			}
			creds.PrivPass = secret
		}
		out.V3 = creds
	}
	return out, nil
}

func (r SNMPRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Scope      LocalScope `json:"scope"`
		Target     string     `json:"target"`
		Port       int        `json:"port"`
		Owned      bool       `json:"owned"`
		Version    string     `json:"version"`
		TimeoutMS  int        `json:"timeout_ms"`
		Interfaces bool       `json:"interfaces"`
	}{r.Scope, r.Target, r.Port, r.Owned, r.Version, r.TimeoutMS, r.Interfaces})
}

func FromSNMP(v localdiscovery.SNMPResult) SNMPResult {
	system := map[string]string(nil)
	if v.System != nil {
		system = make(map[string]string, len(v.System))
		for key, value := range v.System {
			system[key] = value
		}
	}
	return SNMPResult{
		Target: v.Target, System: system, Limited: v.Limited,
		Interfaces: mapSlice(v.Interfaces, func(row localdiscovery.SNMPInterface) SNMPInterface {
			return SNMPInterface{Index: row.Index, Descr: row.Descr, OperStatus: row.OperStatus, SpeedBPS: row.SpeedBPS, MAC: row.MAC}
		}),
	}
}

type LocalScope struct {
	Interface string `json:"interface"`
	CIDR      string `json:"cidr"`
}

type LocalDiscoveryRequest struct {
	Scope           LocalScope `json:"scope"`
	Protocols       []string   `json:"protocols"`
	DurationSeconds int        `json:"duration_seconds"`
	ServiceTypes    []string   `json:"service_types,omitempty"`
	SSDPTarget      string     `json:"ssdp_target,omitempty"`
}

type LocalResponder struct {
	Protocol    string   `json:"protocol"`
	Address     string   `json:"address"`
	Port        int      `json:"port,omitempty"`
	Name        string   `json:"name"`
	Host        string   `json:"host,omitempty"`
	ServiceType string   `json:"service_type,omitempty"`
	TXT         []string `json:"txt,omitempty"`
	Server      string   `json:"server,omitempty"`
	Location    string   `json:"location,omitempty"`
}

type LocalDiscoveryResult struct {
	Responders []LocalResponder `json:"responders"`
	Limited    bool             `json:"limited"`
	OutOfScope int              `json:"out_of_scope"`
	Malformed  int              `json:"malformed"`
	Errors     []string         `json:"errors"`
}

type SNMPRequest struct {
	Scope      LocalScope     `json:"scope"`
	Target     string         `json:"target"`
	Port       int            `json:"port"`
	Owned      bool           `json:"owned"`
	Version    string         `json:"version"`
	Community  string         `json:"community,omitempty"`
	V3         *SNMPv3Request `json:"v3,omitempty"`
	TimeoutMS  int            `json:"timeout_ms"`
	Interfaces bool           `json:"interfaces"`
}

type SNMPv3Request struct {
	User         string `json:"user"`
	AuthProtocol string `json:"auth_protocol"`
	AuthPass     string `json:"auth_pass,omitempty"`
	PrivProtocol string `json:"priv_protocol,omitempty"`
	PrivPass     string `json:"priv_pass,omitempty"`
}

type SNMPResult struct {
	Target     string            `json:"target"`
	System     map[string]string `json:"system"`
	Interfaces []SNMPInterface   `json:"interfaces"`
	Limited    bool              `json:"limited"`
}

type SNMPInterface struct {
	Index      int    `json:"index"`
	Descr      string `json:"descr"`
	OperStatus string `json:"oper_status"`
	SpeedBPS   uint64 `json:"speed_bps"`
	MAC        string `json:"mac"`
}
