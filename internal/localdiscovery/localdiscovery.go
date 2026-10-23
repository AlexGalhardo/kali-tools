package localdiscovery

import (
	"context"
	"net/netip"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const (
	MinDurationSeconds  = 1
	MaxDurationSeconds  = 10
	MaxResponders       = 256
	MaxServiceTypes     = 16
	MaxUserServiceTypes = 4
	MaxDatagramBytes    = 8192
	MaxHeaderValue      = 1024
	MaxTXTFields        = 32
	MaxTXTBytes         = 1024
	SSDPSends           = 2
	SSDPSendGap         = 250 * time.Millisecond
	SSDPTTL             = 2
	SNMPPort            = 161
	SNMPMinTimeoutMS    = 500
	SNMPMaxTimeoutMS    = 5000
	SNMPRetries         = 1
	SNMPMaxRepetitions  = 16
	SNMPMaxRows         = 256
	SNMPMaxValueBytes   = 512
	MaxSecretBytes      = 256
)

// DefaultServiceTypes is the fixed allowlist queried by mDNS.
var DefaultServiceTypes = []string{
	"_http._tcp", "_https._tcp", "_ipp._tcp", "_printer._tcp", "_airplay._tcp", "_raop._tcp",
	"_googlecast._tcp", "_ssh._tcp", "_smb._tcp", "_workstation._tcp", "_device-info._tcp", "_hap._tcp",
}

var serviceTypePattern = regexp.MustCompile(`^_[a-z0-9-]{1,15}\._(tcp|udp)$`)

type Scope struct {
	Interface string `json:"interface"`
	CIDR      string `json:"cidr"`
}

type Request struct {
	Scope           Scope    `json:"scope"`
	Protocols       []string `json:"protocols"`
	DurationSeconds int      `json:"duration_seconds"`
	ServiceTypes    []string `json:"service_types,omitempty"`
	SSDPTarget      string   `json:"ssdp_target,omitempty"`
}

type Responder struct {
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

type Result struct {
	Responders []Responder `json:"responders"`
	Limited    bool        `json:"limited"`
	OutOfScope int         `json:"out_of_scope"`
	Malformed  int         `json:"malformed"`
	Errors     []string    `json:"errors"`
}

func ValidateScope(s Scope, ifaces []network.InterfaceInfo) error {
	iface, ok := findIface(s.Interface, ifaces)
	if !ok || !iface.Up || iface.Loopback || len(ipv4Prefixes(iface)) == 0 {
		return domain.Invalid("invalid interface")
	}
	if err := network.ValidateDiscovery(s.CIDR, []int{80}, time.Second); err != nil {
		return err
	}
	prefix, _ := netip.ParsePrefix(s.CIDR)
	for _, have := range ipv4Prefixes(iface) {
		if prefix.Overlaps(have) {
			return nil
		}
	}
	return domain.Invalid("scope must be on the selected interface's link")
}

func Validate(r Request, ifaces []network.InterfaceInfo) error {
	if err := ValidateScope(r.Scope, ifaces); err != nil {
		return err
	}
	if r.DurationSeconds < MinDurationSeconds || r.DurationSeconds > MaxDurationSeconds {
		return domain.Invalid("duration must be 1..10 seconds")
	}
	if len(r.Protocols) == 0 {
		return domain.Invalid("at least one protocol is required")
	}
	for _, protocol := range r.Protocols {
		if protocol != "mdns" && protocol != "ssdp" {
			return domain.Invalid("unknown protocol")
		}
	}
	if len(r.ServiceTypes) > MaxUserServiceTypes {
		return domain.Invalid("at most 4 extra service types")
	}
	if len(DefaultServiceTypes)+len(r.ServiceTypes) > MaxServiceTypes {
		return domain.Invalid("too many service types")
	}
	for _, kind := range r.ServiceTypes {
		if !serviceTypePattern.MatchString(kind) {
			return domain.Invalid("invalid service type")
		}
	}
	target := r.SSDPTarget
	if target == "" {
		target = "ssdp:all"
	}
	if target != "ssdp:all" && target != "upnp:rootdevice" {
		return domain.Invalid("invalid SSDP target")
	}
	return nil
}

var validateInterfaces = currentInterfaces

func Discover(ctx context.Context, r Request) (Result, error) {
	if err := Validate(r, validateInterfaces()); err != nil {
		return Result{}, err
	}
	if r.SSDPTarget == "" {
		r.SSDPTarget = "ssdp:all"
	}
	out := Result{Responders: []Responder{}, Errors: []string{}}
	if slices.Contains(r.Protocols, "mdns") {
		found, code := queryMDNS(ctx, r)
		out.Responders = append(out.Responders, found.Responders...)
		out.OutOfScope += found.OutOfScope
		out.Limited = out.Limited || found.Limited
		if code != "" {
			out.Errors = append(out.Errors, code)
		}
	}
	if slices.Contains(r.Protocols, "ssdp") {
		found, code := querySSDP(ctx, r)
		out.Responders = append(out.Responders, found.Responders...)
		out.OutOfScope += found.OutOfScope
		out.Malformed += found.Malformed
		out.Limited = out.Limited || found.Limited
		if code != "" {
			out.Errors = append(out.Errors, code)
		}
	}
	return finish(out), nil
}

func finish(out Result) Result {
	if out.Responders == nil {
		out.Responders = []Responder{}
	}
	if out.Errors == nil {
		out.Errors = []string{}
	}
	sort.Slice(out.Responders, func(i, j int) bool {
		if out.Responders[i].Address != out.Responders[j].Address {
			return out.Responders[i].Address < out.Responders[j].Address
		}
		if out.Responders[i].Protocol != out.Responders[j].Protocol {
			return out.Responders[i].Protocol < out.Responders[j].Protocol
		}
		return out.Responders[i].Name < out.Responders[j].Name
	})
	if len(out.Responders) > MaxResponders {
		out.Responders = out.Responders[:MaxResponders]
		out.Limited = true
	}
	return out
}

func currentInterfaces() []network.InterfaceInfo {
	ifaces, err := network.Interfaces()
	if err != nil {
		return nil
	}
	return ifaces
}

func findIface(name string, ifaces []network.InterfaceInfo) (network.InterfaceInfo, bool) {
	for _, iface := range ifaces {
		if iface.Name == name {
			return iface, true
		}
	}
	return network.InterfaceInfo{}, false
}

func ipv4Prefixes(iface network.InterfaceInfo) []netip.Prefix {
	var out []netip.Prefix
	for _, raw := range iface.Addresses {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || !prefix.Addr().Is4() {
			continue
		}
		out = append(out, prefix)
	}
	return out
}

func scopePrefix(cidr string) netip.Prefix {
	prefix, _ := netip.ParsePrefix(cidr)
	return prefix
}

func inScope(prefix netip.Prefix, ip netip.Addr) bool {
	return ip.Is4() && prefix.Contains(ip)
}

func clip(s string, n int) string {
	s = strings.ToValidUTF8(s, "")
	if len(s) > n {
		return s[:n]
	}
	return s
}
