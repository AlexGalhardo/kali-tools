//go:build linux

package platform

import (
	"encoding/binary"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const (
	rtmNewRoute = 24
	rtmNewLink  = 16
	rtmsgLen    = 12
	ifinfoLen   = 16
	rtaOif      = 4
	rtaGateway  = 5
	iflaIfname  = 3
	iflaStats   = 7
	iflaStats64 = 23
	maxRoutes   = 4096
	maxLinks    = 4096
)

func Inventory() (Snapshot, error) {
	out := Snapshot{Adapters: []Adapter{}, Counters: []network.Counter{}}
	interfaces, err := net.Interfaces()
	if err != nil {
		return out, err
	}
	dns := resolvNameservers()
	byIndex, byName, err := routeTables()
	if err != nil {
		return out, err
	}
	for _, iface := range interfaces {
		gw := byIndex[uint32(iface.Index)]
		if len(gw) == 0 {
			gw = byName[iface.Name]
		}
		if gw == nil {
			gw = []string{}
		}
		out.Adapters = append(out.Adapters, Adapter{Index: uint32(iface.Index), Name: iface.Name, Gateways: append([]string{}, gw...), DNS: append([]string{}, dns...)})
	}
	counters, err := linkCounters()
	if err != nil {
		return out, err
	}
	out.Counters = counters
	return out, nil
}

func resolvNameservers() []string {
	raw, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return []string{}
	}
	dns := []string{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == "nameserver" {
			dns = append(dns, fields[1])
		}
	}
	return dns
}

func routeTables() (map[uint32][]string, map[string][]string, error) {
	raw, err := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_UNSPEC)
	if err == nil {
		byIndex, parseErr := parseNetlinkRoutes(raw)
		if parseErr == nil {
			return byIndex, map[string][]string{}, nil
		}
		err = parseErr
	}
	proc, procErr := os.ReadFile("/proc/net/route")
	if procErr != nil {
		return nil, nil, err
	}
	return map[uint32][]string{}, parseProcRoutes(string(proc)), nil
}

func linkCounters() ([]network.Counter, error) {
	raw, err := syscall.NetlinkRIB(syscall.RTM_GETLINK, syscall.AF_UNSPEC)
	if err == nil {
		counters, parseErr := parseNetlinkLinks(raw)
		if parseErr == nil {
			return counters, nil
		}
		err = parseErr
	}
	proc, procErr := os.ReadFile("/proc/net/dev")
	if procErr != nil {
		return nil, err
	}
	return parseProcCounters(string(proc))
}

func parseNetlinkRoutes(raw []byte) (map[uint32][]string, error) {
	gateways := map[uint32][]string{}
	if len(raw) > maxNetlinkBytes {
		return nil, domain.Invalid("route netlink dump exceeds limit")
	}
	for len(raw) > 0 {
		if len(raw) < nlmsgHeaderLen {
			return nil, domain.Invalid("truncated route netlink header")
		}
		size := int(binary.NativeEndian.Uint32(raw))
		if size < nlmsgHeaderLen || size > len(raw) {
			return nil, domain.Invalid("invalid route netlink message length")
		}
		kind := binary.NativeEndian.Uint16(raw[4:])
		if kind == nlmsgDone {
			return gateways, nil
		}
		if kind == nlmsgError {
			return nil, &domain.Error{Code: domain.CodeFailed, Message: "route netlink dump failed"}
		}
		if kind == rtmNewRoute {
			if err := addDefaultRoute(raw[nlmsgHeaderLen:size], gateways); err != nil {
				return nil, err
			}
			if len(gateways) > maxRoutes {
				return nil, domain.Invalid("route table exceeds limit")
			}
		}
		advance := align4(size)
		if advance > len(raw) {
			return nil, domain.Invalid("truncated route netlink padding")
		}
		raw = raw[advance:]
	}
	return gateways, nil
}

func addDefaultRoute(body []byte, gateways map[uint32][]string) error {
	if len(body) < rtmsgLen {
		return domain.Invalid("truncated route message")
	}
	if body[1] != 0 {
		return nil
	}
	family := body[0]
	var gateway net.IP
	var index uint32
	var hasIndex bool
	attrs := body[rtmsgLen:]
	for len(attrs) > 0 {
		if len(attrs) < rtattrHeaderLen {
			return domain.Invalid("truncated route attribute")
		}
		n := int(binary.NativeEndian.Uint16(attrs))
		kind := binary.NativeEndian.Uint16(attrs[2:])
		if n < rtattrHeaderLen || n > len(attrs) {
			return domain.Invalid("invalid route attribute length")
		}
		payload := attrs[4:n]
		switch kind {
		case rtaGateway:
			gateway = append(net.IP(nil), payload...)
		case rtaOif:
			if len(payload) < 4 {
				return domain.Invalid("truncated route interface")
			}
			index = binary.NativeEndian.Uint32(payload)
			hasIndex = true
		}
		advance := align4(n)
		if advance > len(attrs) {
			return domain.Invalid("truncated route attribute padding")
		}
		attrs = attrs[advance:]
	}
	if !hasIndex || len(gateway) == 0 {
		return nil
	}
	text := gatewayText(family, gateway)
	if text == "" {
		return nil
	}
	for _, existing := range gateways[index] {
		if existing == text {
			return nil
		}
	}
	gateways[index] = append(gateways[index], text)
	return nil
}

func gatewayText(family byte, gateway net.IP) string {
	switch family {
	case syscall.AF_INET:
		if len(gateway) < 4 {
			return ""
		}
		return net.IPv4(gateway[0], gateway[1], gateway[2], gateway[3]).String()
	case syscall.AF_INET6:
		if len(gateway) < 16 {
			return ""
		}
		return gateway[:16].String()
	default:
		return ""
	}
}

func parseNetlinkLinks(raw []byte) ([]network.Counter, error) {
	if len(raw) > maxNetlinkBytes {
		return nil, domain.Invalid("link netlink dump exceeds limit")
	}
	counters := []network.Counter{}
	for len(raw) > 0 {
		if len(raw) < nlmsgHeaderLen {
			return nil, domain.Invalid("truncated link netlink header")
		}
		size := int(binary.NativeEndian.Uint32(raw))
		if size < nlmsgHeaderLen || size > len(raw) {
			return nil, domain.Invalid("invalid link netlink message length")
		}
		kind := binary.NativeEndian.Uint16(raw[4:])
		if kind == nlmsgDone {
			return counters, nil
		}
		if kind == nlmsgError {
			return nil, &domain.Error{Code: domain.CodeFailed, Message: "link netlink dump failed"}
		}
		if kind == rtmNewLink {
			counter, ok, err := linkCounter(raw[nlmsgHeaderLen:size])
			if err != nil {
				return nil, err
			}
			if ok {
				counters = append(counters, counter)
				if len(counters) > maxLinks {
					return nil, domain.Invalid("link table exceeds limit")
				}
			}
		}
		advance := align4(size)
		if advance > len(raw) {
			return nil, domain.Invalid("truncated link netlink padding")
		}
		raw = raw[advance:]
	}
	return counters, nil
}

func linkCounter(body []byte) (network.Counter, bool, error) {
	if len(body) < ifinfoLen {
		return network.Counter{}, false, domain.Invalid("truncated link message")
	}
	var name string
	var received, sent uint64
	var haveStats bool
	attrs := body[ifinfoLen:]
	for len(attrs) > 0 {
		if len(attrs) < rtattrHeaderLen {
			return network.Counter{}, false, domain.Invalid("truncated link attribute")
		}
		n := int(binary.NativeEndian.Uint16(attrs))
		kind := binary.NativeEndian.Uint16(attrs[2:])
		if n < rtattrHeaderLen || n > len(attrs) {
			return network.Counter{}, false, domain.Invalid("invalid link attribute length")
		}
		payload := attrs[4:n]
		switch kind {
		case iflaIfname:
			name = strings.TrimRight(string(payload), "\x00")
		case iflaStats64:
			if len(payload) < 32 {
				return network.Counter{}, false, domain.Invalid("truncated link stats")
			}
			received = binary.NativeEndian.Uint64(payload[16:])
			sent = binary.NativeEndian.Uint64(payload[24:])
			haveStats = true
		case iflaStats:
			if !haveStats {
				if len(payload) < 16 {
					return network.Counter{}, false, domain.Invalid("truncated link stats")
				}
				received = uint64(binary.NativeEndian.Uint32(payload[8:]))
				sent = uint64(binary.NativeEndian.Uint32(payload[12:]))
				haveStats = true
			}
		}
		advance := align4(n)
		if advance > len(attrs) {
			return network.Counter{}, false, domain.Invalid("truncated link attribute padding")
		}
		attrs = attrs[advance:]
	}
	if name == "" || !haveStats {
		return network.Counter{}, false, nil
	}
	return network.Counter{ID: name, Received: received, Sent: sent}, true, nil
}

func parseProcRoutes(text string) map[string][]string {
	gateways := map[string][]string{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) <= 3 || fields[1] != "00000000" {
			continue
		}
		gateway, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil {
			continue
		}
		ip := net.IPv4(byte(gateway), byte(gateway>>8), byte(gateway>>16), byte(gateway>>24)).String()
		gateways[fields[0]] = append(gateways[fields[0]], ip)
	}
	return gateways
}

func parseProcCounters(text string) ([]network.Counter, error) {
	counters := []network.Counter{}
	for _, line := range strings.Split(text, "\n") {
		name, data, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(data)
		if len(fields) < 16 {
			continue
		}
		received, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, err
		}
		sent, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return nil, err
		}
		counters = append(counters, network.Counter{ID: strings.TrimSpace(name), Received: received, Sent: sent})
	}
	return counters, nil
}
