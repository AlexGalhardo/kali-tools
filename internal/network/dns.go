package network

import (
	"context"
	"fmt"
	"net"
	"sort"
)

func lookupDNS(ctx context.Context, resolver *net.Resolver, host, recordType string) ([]string, error) {
	values := []string{}
	var err error
	switch recordType {
	case "":
		values, err = resolver.LookupHost(ctx, host)
	case "A", "AAAA":
		family := "ip4"
		if recordType == "AAAA" {
			family = "ip6"
		}
		var ips []net.IP
		ips, err = resolver.LookupIP(ctx, family, host)
		for _, ip := range ips {
			values = append(values, ip.String())
		}
	case "CNAME":
		var name string
		name, err = resolver.LookupCNAME(ctx, host)
		if err == nil {
			values = append(values, name)
		}
	case "MX":
		var records []*net.MX
		records, err = resolver.LookupMX(ctx, host)
		for _, r := range records {
			values = append(values, fmt.Sprintf("%d %s", r.Pref, r.Host))
		}
	case "NS":
		var records []*net.NS
		records, err = resolver.LookupNS(ctx, host)
		for _, r := range records {
			values = append(values, r.Host)
		}
	case "TXT":
		values, err = resolver.LookupTXT(ctx, host)
	case "PTR":
		values, err = resolver.LookupAddr(ctx, host)
	}
	sort.Strings(values)
	limited := len(values) > 256
	values = values[:min(len(values), 256)]
	for i, value := range values {
		if len(value) > 1024 {
			values[i] = value[:1024] + " [truncated]"
			limited = true
		}
	}
	if limited {
		values = append(values, "Additional DNS data omitted by result limits")
	}
	return values, err
}
