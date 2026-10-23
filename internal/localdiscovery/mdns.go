package localdiscovery

import (
	"context"
	"io"
	"log"
	"net"
	"net/netip"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"
)

// mdnsLibrary is the hashicorp query. Docker returns "setsockopt: no such device"
// for its nil-interface multicast bind, so a unicast-response query follows.
var mdnsLibrary = mdns.QueryContext

var mdnsDest = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

var mdnsQuery = queryLibrary

func queryLibrary(ctx context.Context, params *mdns.QueryParam) error {
	err := mdnsLibrary(ctx, params)
	if err == nil || params == nil || params.Interface == nil {
		return err
	}
	if queryUnicast(ctx, params) == nil {
		return nil
	}
	return err
}

func queryUnicast(ctx context.Context, params *mdns.QueryParam) error {
	ip := interfaceIPv4(params.Interface)
	if ip == nil {
		return errNoSource
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ip, Port: 0})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(params.Service+"."+params.Domain), dns.TypePTR)
	msg.Question[0].Qclass |= 1 << 15
	msg.RecursionDesired = false
	raw, err := msg.Pack()
	if err != nil {
		return err
	}
	if _, err := conn.WriteToUDP(raw, mdnsDest); err != nil {
		return err
	}
	deadline := time.Now().Add(params.Timeout)
	buf := make([]byte, MaxDatagramBytes)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return nil
		}
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil || n == 0 {
			continue
		}
		var resp dns.Msg
		if resp.Unpack(buf[:n]) != nil || resp.Id != msg.Id {
			continue
		}
		entry := serviceEntry(&resp)
		if entry == nil {
			continue
		}
		select {
		case params.Entries <- entry:
		default:
		}
	}
	return nil
}

func interfaceIPv4(ifi *net.Interface) net.IP {
	if ifi == nil {
		return nil
	}
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ip := ipnet.IP.To4(); ip != nil {
			return ip
		}
	}
	return nil
}

func serviceEntry(resp *dns.Msg) *mdns.ServiceEntry {
	entry := &mdns.ServiceEntry{}
	for _, rr := range append(resp.Answer, resp.Extra...) {
		switch rec := rr.(type) {
		case *dns.PTR:
			entry.Name = rec.Ptr
		case *dns.SRV:
			if entry.Name == "" {
				entry.Name = rec.Hdr.Name
			}
			entry.Host = rec.Target
			entry.Port = int(rec.Port)
		case *dns.TXT:
			entry.InfoFields = rec.Txt
		case *dns.A:
			entry.AddrV4 = rec.A
			entry.Addr = rec.A
		}
	}
	if entry.AddrV4 == nil || entry.Port == 0 || entry.Name == "" {
		return nil
	}
	return entry
}

func queryMDNS(ctx context.Context, r Request) (Result, string) {
	ifi, err := net.InterfaceByName(r.Scope.Interface)
	if err != nil {
		return Result{}, "mdns_bind_failed"
	}
	types := append([]string{}, DefaultServiceTypes...)
	types = append(types, r.ServiceTypes...)
	per := time.Duration(r.DurationSeconds) * time.Second / time.Duration(len(types))
	if per < 200*time.Millisecond {
		per = 200 * time.Millisecond
	}
	prefix := scopePrefix(r.Scope.CIDR)
	out := Result{}
	seen := map[string]struct{}{}
	for _, kind := range types {
		if ctx.Err() != nil {
			break
		}
		entries := make(chan *mdns.ServiceEntry, MaxResponders)
		params := &mdns.QueryParam{
			Service: kind, Domain: "local", Timeout: per, Interface: ifi, Entries: entries,
			WantUnicastResponse: true, DisableIPv6: true, Logger: log.New(io.Discard, "", 0),
		}
		err := mdnsQuery(ctx, params)
		for {
			select {
			case entry := <-entries:
				item, ok := convertEntry(kind, entry, prefix)
				if !ok {
					if entry != nil && entry.AddrV4 != nil {
						out.OutOfScope++
					}
					continue
				}
				key := item.Protocol + "|" + item.Address + "|" + item.Name
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
				if len(out.Responders) >= MaxResponders {
					out.Limited = true
					continue
				}
				out.Responders = append(out.Responders, item)
			default:
				goto drained
			}
		}
	drained:
		if err != nil {
			return out, "mdns_bind_failed"
		}
	}
	return out, ""
}

func convertEntry(kind string, entry *mdns.ServiceEntry, prefix netip.Prefix) (Responder, bool) {
	if entry == nil || entry.AddrV4 == nil {
		return Responder{}, false
	}
	ip, ok := netip.AddrFromSlice(entry.AddrV4.To4())
	if !ok || !inScope(prefix, ip) {
		return Responder{}, false
	}
	fields := entry.InfoFields
	if len(fields) > MaxTXTFields {
		fields = fields[:MaxTXTFields]
	}
	txt := make([]string, 0, len(fields))
	for _, field := range fields {
		txt = append(txt, clip(field, MaxTXTBytes))
	}
	return Responder{
		Protocol: "mdns", Address: ip.String(), Port: entry.Port,
		Name: clip(entry.Name, 255), Host: clip(entry.Host, 255), ServiceType: kind, TXT: txt,
	}, true
}
