package dnsinspect

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"strconv"
)

func (ins *inspector) soaNS(ctx context.Context, r Request, resolvers []string) *Consistency {
	out := &Consistency{Servers: []AuthServer{}, Verdict: "no_servers"}
	if len(resolvers) == 0 {
		return out
	}
	resolver := resolvers[0]
	zone := ins.discoverZone(ctx, r.Name, resolver)
	out.Zone = zone
	if zone == "" {
		return out
	}
	names := nsNames(ins.ask(ctx, resolver, zone, "NS", true, false).msg)
	for _, name := range names {
		for _, ip := range ins.resolveNS(ctx, resolver, name) {
			out.Servers = append(out.Servers, ins.readAuth(ctx, name, ip, zone))
		}
	}
	out.Verdict, out.SerialsAgree, out.NSSetsAgree = judgeAuth(out.Servers)
	return out
}

func (ins *inspector) discoverZone(ctx context.Context, name, resolver string) string {
	cur := apex(name)
	for range MaxChainDepth {
		if _, ok := matchedSOA(ins.ask(ctx, resolver, cur, "SOA", true, false), cur); ok {
			return cur
		}
		next := parentZone(cur)
		if next == cur {
			break
		}
		cur = next
	}
	return ""
}

func (ins *inspector) resolveNS(ctx context.Context, resolver, host string) []string {
	found := answerIPs(ins.ask(ctx, resolver, host, "A", true, false).msg, "A")
	if len(found) < MaxAddrsPerNS {
		found = append(found, answerIPs(ins.ask(ctx, resolver, host, "AAAA", true, false).msg, "AAAA")...)
	}
	if len(found) > MaxAddrsPerNS {
		found = found[:MaxAddrsPerNS]
	}
	return found
}

func (ins *inspector) readAuth(ctx context.Context, host, ip, zone string) AuthServer {
	addr, err := netip.ParseAddr(ip)
	port := uint16(53)
	if err == nil {
		port = ins.authPort(addr)
	}
	if p, ok := ins.nsPort[apex(host)]; ok {
		port = p
	}
	target := net.JoinHostPort(ip, strconv.Itoa(int(port)))
	server := AuthServer{NS: host, Address: ip, NSSet: []string{}}
	soa := ins.ask(ctx, target, zone, "SOA", false, false)
	switch soa.status {
	case "skipped":
		server.Status = "skipped"
		return server
	case "timeout", "unreachable", "port53_unreachable":
		server.Status = "unreachable"
		if soa.status == "timeout" {
			server.Status = "timeout"
		}
		return server
	case "refused":
		server.Status = "refused"
		return server
	}
	if !soa.aa {
		server.Status = "lame"
		return server
	}
	ns := ins.ask(ctx, target, zone, "NS", false, false)
	server.AA = true
	server.Status = "ok"
	if serial, ok := matchedSOA(soa, zone); ok {
		value := serial
		server.Serial = &value
	}
	server.NSSet = nsNames(ns.msg)
	return server
}

func judgeAuth(servers []AuthServer) (verdict string, serials, sets bool) {
	lame := false
	var okServers []AuthServer
	for _, server := range servers {
		switch server.Status {
		case "lame":
			lame = true
		case "ok":
			okServers = append(okServers, server)
		}
	}
	if !lame && len(okServers) == 0 {
		return "no_servers", false, false
	}
	serials, sets = sameSerials(okServers), sameNS(okServers)
	if lame && serials && sets {
		return "lame_servers", serials, sets
	}
	if len(okServers) == 0 {
		return "lame_servers", false, false
	}
	if !serials {
		return "serial_mismatch", false, sets
	}
	if !sets {
		return "ns_mismatch", serials, false
	}
	return "consistent", true, true
}

func sameSerials(servers []AuthServer) bool {
	if len(servers) == 0 {
		return true
	}
	base := servers[0].Serial
	for _, server := range servers[1:] {
		if base == nil || server.Serial == nil || *base != *server.Serial {
			return false
		}
	}
	return base != nil || len(servers) == 1
}

func sameNS(servers []AuthServer) bool {
	if len(servers) == 0 {
		return true
	}
	for _, server := range servers[1:] {
		if !slices.Equal(servers[0].NSSet, server.NSSet) {
			return false
		}
	}
	return true
}
