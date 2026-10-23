package dnsinspect

import (
	"context"
	"errors"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (ins *inspector) compare(ctx context.Context, r Request, resolvers []string) *Comparison {
	type target struct{ id string }
	targets := make([]target, 0, 1+len(resolvers))
	targets = append(targets, target{id: "system"})
	for _, resolver := range resolvers {
		targets = append(targets, target{id: resolver})
	}
	results := make([]ResolverAnswer, len(targets))
	var wg sync.WaitGroup
	for i, item := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if item.id == "system" {
				results[i] = ins.systemView(ctx, r.Name, r.Type)
				return
			}
			results[i] = viewOf(item.id, ins.ask(ctx, item.id, r.Name, r.Type, true, false))
		}()
	}
	wg.Wait()
	return groupAnswers(results)
}

func viewOf(server string, got asked) ResolverAnswer {
	answers := got.answers
	if answers == nil {
		answers = []string{}
	}
	return ResolverAnswer{
		Resolver: server, Transport: got.transport, Status: got.status, RCode: got.rcode,
		AD: got.ad, Answers: answers, MinTTL: got.minTTL, ElapsedMS: got.elapsed,
	}
}

func groupAnswers(results []ResolverAnswer) *Comparison {
	buckets := map[string][]string{}
	var keys []string
	agree := true
	var first []string
	seenOK := false
	for _, result := range results {
		if result.Status != "ok" {
			continue
		}
		if !seenOK {
			first = result.Answers
			seenOK = true
		} else if !slices.Equal(first, result.Answers) {
			agree = false
		}
		key := strings.Join(result.Answers, "\n")
		if _, ok := buckets[key]; !ok {
			keys = append(keys, key)
		}
		buckets[key] = append(buckets[key], result.Resolver)
	}
	sort.Strings(keys)
	groups := make([][]string, 0, len(keys))
	for _, key := range keys {
		names := buckets[key]
		sort.Strings(names)
		groups = append(groups, names)
	}
	return &Comparison{Results: results, Agree: agree, Groups: groups}
}

func (ins *inspector) systemView(ctx context.Context, name, typ string) ResolverAnswer {
	out := ResolverAnswer{Resolver: "system", Transport: "os", Answers: []string{}}
	if !ins.spend() {
		out.Status = "skipped"
		return out
	}
	ins.spawn(ctx, func(ctx context.Context) {
		out = ins.doSystem(ctx, name, typ)
	})
	if out.Answers == nil {
		out.Answers = []string{}
	}
	return out
}

func (ins *inspector) doSystem(ctx context.Context, name, typ string) ResolverAnswer {
	out := ResolverAnswer{Resolver: "system", Transport: "os", Answers: []string{}}
	if typ == "" {
		typ = "A"
	}
	if ins.system != nil {
		answers, status, rcode, elapsed := ins.system(ctx, name, typ)
		if answers == nil {
			answers = []string{}
		}
		out.Answers, out.Status, out.RCode, out.ElapsedMS = answers, status, rcode, elapsed
		return out
	}
	switch typ {
	case "A", "AAAA", "CNAME", "MX", "NS", "TXT":
	default:
		out.Status = "unsupported_type"
		return out
	}
	qctx, cancel := context.WithTimeout(ctx, ins.timeout)
	defer cancel()
	start := time.Now()
	resolver := &net.Resolver{PreferGo: false}
	values, err := lookupSupported(qctx, resolver, name, typ)
	out.ElapsedMS = float64(time.Since(start).Microseconds()) / 1000
	sort.Strings(values)
	if len(values) > MaxAnswersReported {
		values = values[:MaxAnswersReported]
		ins.limited.Store(true)
	}
	for i, value := range values {
		if len(value) > MaxRDataText {
			values[i] = value[:MaxRDataText]
			ins.limited.Store(true)
		}
	}
	if values == nil {
		values = []string{}
	}
	out.Answers = values
	if err == nil {
		out.Status, out.RCode = "ok", "NOERROR"
		return out
	}
	var dnsErr *net.DNSError
	if errorsAsDNS(err, &dnsErr) && dnsErr.IsNotFound {
		out.Status, out.RCode = "nxdomain", "NXDOMAIN"
		return out
	}
	if isTimeout(err) || ctx.Err() != nil || qctx.Err() != nil {
		out.Status = "timeout"
		return out
	}
	out.Status = "servfail"
	return out
}

func lookupSupported(ctx context.Context, resolver *net.Resolver, host, typ string) ([]string, error) {
	switch typ {
	case "A", "AAAA":
		family := "ip4"
		if typ == "AAAA" {
			family = "ip6"
		}
		ips, err := resolver.LookupIP(ctx, family, host)
		values := make([]string, 0, len(ips))
		for _, ip := range ips {
			values = append(values, ip.String())
		}
		return values, err
	case "CNAME":
		name, err := resolver.LookupCNAME(ctx, host)
		if err != nil {
			return nil, err
		}
		return []string{name}, nil
	case "MX":
		records, err := resolver.LookupMX(ctx, host)
		values := make([]string, 0, len(records))
		for _, record := range records {
			values = append(values, strconv.Itoa(int(record.Pref))+" "+record.Host)
		}
		return values, err
	case "NS":
		records, err := resolver.LookupNS(ctx, host)
		values := make([]string, 0, len(records))
		for _, record := range records {
			values = append(values, record.Host)
		}
		return values, err
	case "TXT":
		return resolver.LookupTXT(ctx, host)
	default:
		return nil, nil
	}
}

func errorsAsDNS(err error, target **net.DNSError) bool {
	return errors.As(err, target)
}
