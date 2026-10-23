package localdiscovery

import (
	"context"
	"io"
	"log"
	"net"
	"net/netip"
	"time"

	"github.com/hashicorp/mdns"
)

var mdnsQuery = mdns.QueryContext

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
