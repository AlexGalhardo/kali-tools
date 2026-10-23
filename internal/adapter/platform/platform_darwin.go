//go:build darwin

package platform

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"strings"
	"syscall"

	"golang.org/x/net/route"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func Inventory() (Snapshot, error) {
	out := Snapshot{Adapters: []Adapter{}, Counters: []network.Counter{}}
	interfaces, err := net.Interfaces()
	if err != nil {
		return out, domain.Unsupported("macOS interface list is not available")
	}
	if len(interfaces) > maxDarwinMessages {
		return out, domain.Invalid("interface list exceeds limit")
	}
	gateways, err := defaultGateways()
	if err != nil {
		return out, err
	}
	dns := darwinNameservers()
	names := map[int]string{}
	for _, iface := range interfaces {
		names[iface.Index] = iface.Name
		gw := gateways[iface.Index]
		if gw == nil {
			gw = []string{}
		}
		out.Adapters = append(out.Adapters, Adapter{Index: uint32(iface.Index), Name: iface.Name, Gateways: append([]string{}, gw...), DNS: append([]string{}, dns...)})
	}
	counters, err := interfaceCounters(names)
	if err != nil {
		return out, err
	}
	out.Counters = counters
	return out, nil
}

func defaultGateways() (map[int][]string, error) {
	rib, err := route.FetchRIB(syscall.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return nil, routeReadError(err)
	}
	if len(rib) > maxNetlinkBytes {
		return nil, domain.Invalid("route dump exceeds limit")
	}
	if len(rib) == 0 {
		return map[int][]string{}, nil
	}
	msgs, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil, domain.Unsupported("macOS route provider is not available")
	}
	if len(msgs) > maxDarwinMessages {
		return nil, domain.Invalid("route dump exceeds limit")
	}
	out := map[int][]string{}
	for _, msg := range msgs {
		rm, ok := msg.(*route.RouteMessage)
		if !ok || rm.Flags&syscall.RTF_GATEWAY == 0 || rm.Index <= 0 || len(out[rm.Index]) >= 8 {
			continue
		}
		if !unspecifiedDestination(rm.Addrs) {
			continue
		}
		gw, ok := gatewayString(rm.Addrs)
		if !ok {
			continue
		}
		out[rm.Index] = append(out[rm.Index], gw)
	}
	return out, nil
}

func interfaceCounters(names map[int]string) ([]network.Counter, error) {
	rib, err := route.FetchRIB(syscall.AF_UNSPEC, route.RIBTypeInterface, 0)
	if err != nil {
		return nil, routeReadError(err)
	}
	parsed, err := parseDarwinCounters(rib)
	if err != nil {
		return nil, err
	}
	out := []network.Counter{}
	for _, row := range parsed {
		name := names[int(row.Index)]
		if name == "" {
			continue
		}
		out = append(out, network.Counter{ID: name, Received: row.Received, Sent: row.Sent})
	}
	return out, nil
}

func routeReadError(err error) error {
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		return &domain.Error{Code: domain.CodePermissionRequired, Message: "macOS denied route access"}
	}
	return domain.Unsupported("macOS route provider is not available")
}

func unspecifiedDestination(addrs []route.Addr) bool {
	if len(addrs) <= syscall.RTAX_DST || addrs[syscall.RTAX_DST] == nil {
		return true
	}
	switch addr := addrs[syscall.RTAX_DST].(type) {
	case *route.Inet4Addr:
		return addr.IP == [4]byte{}
	case *route.Inet6Addr:
		return netip.AddrFrom16(addr.IP).IsUnspecified()
	default:
		return false
	}
}

func gatewayString(addrs []route.Addr) (string, bool) {
	if len(addrs) <= syscall.RTAX_GATEWAY {
		return "", false
	}
	switch addr := addrs[syscall.RTAX_GATEWAY].(type) {
	case *route.Inet4Addr:
		if addr.IP == ([4]byte{}) {
			return "", false
		}
		return netip.AddrFrom4(addr.IP).String(), true
	case *route.Inet6Addr:
		ip := netip.AddrFrom16(addr.IP)
		if !ip.IsValid() || ip.IsUnspecified() {
			return "", false
		}
		return ip.String(), true
	default:
		return "", false
	}
}

func darwinNameservers() []string {
	raw, err := os.ReadFile("/etc/resolv.conf")
	if err != nil || len(raw) > 1<<16 {
		return []string{}
	}
	dns := []string{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == "nameserver" && len(dns) < 8 {
			dns = append(dns, fields[1])
		}
	}
	return dns
}
