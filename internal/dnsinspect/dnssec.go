package dnsinspect

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func (ins *inspector) dnssecCheck(ctx context.Context, r Request, resolvers []string) (*DNSSECChain, error) {
	anchors, err := parseAnchors(r.TrustAnchors)
	if err != nil {
		return nil, err
	}
	chain := &DNSSECChain{Links: []DNSSECLink{}, Verdict: "indeterminate"}
	if len(resolvers) == 0 || len(anchors) == 0 {
		return chain, nil
	}
	chain.Resolver = resolvers[0]
	zone := ins.discoverZone(ctx, r.Name, chain.Resolver)
	if zone == "" {
		chain.Links = []DNSSECLink{{Zone: apex(r.Name), Status: "indeterminate"}}
		return chain, nil
	}
	zones := chainZones(anchors[0].Hdr.Name, zone)
	var lastKeys []*dns.DNSKEY
	for i, name := range zones {
		if ctx.Err() != nil {
			break
		}
		link, keys := ins.dnssecLink(ctx, chain.Resolver, name, anchors, i == 0)
		chain.Links = append(chain.Links, link)
		lastKeys = keys
		if link.Status != "secure" {
			break
		}
	}
	if allStatus(chain.Links, "secure") {
		answer := ins.ask(ctx, chain.Resolver, r.Name, r.Type, true, true)
		chain.AnswerSig = signedAnswer(answer.msg, dns.StringToType[r.Type], lastKeys, ins.now())
	}
	chain.Verdict = chainVerdict(chain.Links, chain.AnswerSig)
	return chain, nil
}

func chainZones(anchor, zone string) []string {
	anchor, zone = apex(anchor), apex(zone)
	up := []string{}
	cur := zone
	for range MaxChainDepth {
		up = append(up, cur)
		if cur == anchor {
			break
		}
		next := parentZone(cur)
		if next == cur {
			break
		}
		cur = next
	}
	slices.Reverse(up)
	if len(up) == 0 || up[0] != anchor {
		up = append([]string{anchor}, up...)
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, name := range up {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
		if len(out) == MaxChainDepth {
			break
		}
	}
	return out
}

func (ins *inspector) dnssecLink(ctx context.Context, resolver, zone string, anchors []*dns.DS, anchorZone bool) (DNSSECLink, []*dns.DNSKEY) {
	link := DNSSECLink{Zone: zone, Status: "indeterminate"}
	var dsSet []*dns.DS
	if anchorZone {
		dsSet = anchors
	} else {
		parent := ins.ask(ctx, resolver, zone, "DS", true, true)
		switch parent.status {
		case "timeout", "servfail", "skipped":
			return link, nil
		}
		dsSet = dsRecords(parent.msg)
		if parent.status == "ok" && len(dsSet) == 0 {
			link.Status = "insecure"
			return link, nil
		}
	}
	got := ins.ask(ctx, resolver, zone, "DNSKEY", true, true)
	switch got.status {
	case "timeout", "servfail", "skipped":
		return link, nil
	}
	keys := dnskeys(got.msg)
	matched := matchDS(keys, dsSet)
	if matched == nil {
		link.Status = "bogus"
		return link, keys
	}
	link.KeyTag = matched.KeyTag()
	link.Algorithm = algorithmName(matched.Algorithm)
	link.DSMatches = true
	if !supportedAlg(matched.Algorithm) {
		link.Status = "indeterminate"
		link.Detail = fmt.Sprintf("unsupported algorithm %d", matched.Algorithm)
		return link, keys
	}
	link.SigValid = signatureValid(matched, rrsigs(got.msg, dns.TypeDNSKEY), dnskeyRRSet(keys), ins.now())
	if link.SigValid {
		link.Status = "secure"
		return link, keys
	}
	link.Status = "bogus"
	return link, keys
}

func chainVerdict(links []DNSSECLink, answerSig bool) string {
	if len(links) == 0 {
		return "indeterminate"
	}
	bogus, insecure, indeterminate := false, false, false
	for _, link := range links {
		switch link.Status {
		case "bogus":
			bogus = true
		case "insecure":
			insecure = true
		case "indeterminate":
			indeterminate = true
		case "secure":
		default:
			indeterminate = true
		}
	}
	if bogus {
		return "bogus"
	}
	if insecure {
		return "insecure"
	}
	if indeterminate {
		return "indeterminate"
	}
	if answerSig {
		return "secure"
	}
	return "bogus"
}

func allStatus(links []DNSSECLink, status string) bool {
	if len(links) == 0 {
		return false
	}
	for _, link := range links {
		if link.Status != status {
			return false
		}
	}
	return true
}

func matchDS(keys []*dns.DNSKEY, set []*dns.DS) *dns.DNSKEY {
	for _, key := range keys {
		if key.Flags&1 != 1 {
			continue
		}
		for _, ds := range set {
			if ds.KeyTag != key.KeyTag() || ds.Algorithm != key.Algorithm {
				continue
			}
			digest := key.ToDS(ds.DigestType)
			if digest != nil && strings.EqualFold(digest.Digest, ds.Digest) {
				return key
			}
		}
	}
	return nil
}

func signatureValid(key *dns.DNSKEY, sigs []*dns.RRSIG, rrset []dns.RR, now time.Time) bool {
	for _, sig := range sigs {
		if sig.KeyTag != key.KeyTag() || sig.Algorithm != key.Algorithm {
			continue
		}
		if sig.Verify(key, rrset) == nil && sig.ValidityPeriod(now) {
			return true
		}
	}
	return false
}

func signedAnswer(msg *dns.Msg, qtype uint16, keys []*dns.DNSKEY, now time.Time) bool {
	if msg == nil || len(keys) == 0 {
		return false
	}
	var rrset []dns.RR
	for _, rr := range msg.Answer {
		if _, ok := rr.(*dns.RRSIG); ok {
			continue
		}
		if rr.Header().Rrtype == qtype {
			rrset = append(rrset, rr)
		}
	}
	if len(rrset) == 0 {
		return false
	}
	sigs := rrsigs(msg, qtype)
	for _, key := range keys {
		if !supportedAlg(key.Algorithm) {
			continue
		}
		if signatureValid(key, sigs, rrset, now) {
			return true
		}
	}
	return false
}

func dnskeys(msg *dns.Msg) []*dns.DNSKEY {
	var out []*dns.DNSKEY
	if msg == nil {
		return nil
	}
	for _, rr := range msg.Answer {
		if key, ok := rr.(*dns.DNSKEY); ok {
			out = append(out, key)
		}
	}
	return out
}

func dnskeyRRSet(keys []*dns.DNSKEY) []dns.RR {
	out := make([]dns.RR, 0, len(keys))
	for _, key := range keys {
		out = append(out, key)
	}
	return out
}

func rrsigs(msg *dns.Msg, covered uint16) []*dns.RRSIG {
	var out []*dns.RRSIG
	if msg == nil {
		return nil
	}
	for _, rr := range msg.Answer {
		sig, ok := rr.(*dns.RRSIG)
		if ok && sig.TypeCovered == covered {
			out = append(out, sig)
		}
	}
	return out
}

func dsRecords(msg *dns.Msg) []*dns.DS {
	var out []*dns.DS
	if msg == nil {
		return nil
	}
	for _, rr := range msg.Answer {
		if ds, ok := rr.(*dns.DS); ok {
			out = append(out, ds)
		}
	}
	return out
}

func supportedAlg(alg uint8) bool {
	switch alg {
	case dns.RSASHA256, dns.RSASHA512, dns.ECDSAP256SHA256, dns.ECDSAP384SHA384, dns.ED25519:
		return true
	default:
		return false
	}
}

func algorithmName(alg uint8) string {
	if name := dns.AlgorithmToString[alg]; name != "" {
		return name
	}
	return strconv.Itoa(int(alg))
}
