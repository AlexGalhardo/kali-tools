package main

import (
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const (
	roleA         = "a"
	roleB         = "b"
	roleRecursive = "recursive"
)

type labZone struct {
	name   string
	soa    *dns.SOA
	ns     []dns.RR
	dnskey []dns.RR
	www    []dns.RR
}

var (
	zonesA []labZone
	zonesB []labZone
	glue   net.IP
)

func startNameServers(now time.Time) {
	glue = fixtureIPv4()
	signed := buildSigned(now)
	zonesA = []labZone{
		plainZone("consistent.test", 100),
		plainZone("mismatch.test", 100),
		{name: dns.Fqdn("signed.test"), soa: signed.soa, ns: signed.ns, dnskey: signed.dnskey, www: signed.www},
	}
	zonesB = []labZone{
		plainZone("consistent.test", 100),
		plainZone("mismatch.test", 101),
	}
	serveBoth(":5354", roleA)
	serveBoth(":5355", roleB)
	serveBoth(":5356", roleRecursive)
}

func plainZone(name string, serial uint32) labZone {
	fqdn := dns.Fqdn(name)
	ns1 := dns.Fqdn("ns1." + strings.TrimSuffix(name, "."))
	ns2 := dns.Fqdn("ns2." + strings.TrimSuffix(name, "."))
	return labZone{name: fqdn, soa: soaRR(fqdn, ns1, serial), ns: []dns.RR{nsRR(fqdn, ns1), nsRR(fqdn, ns2)}}
}

func soaRR(owner, primary string, serial uint32) *dns.SOA {
	return &dns.SOA{
		Hdr:     dns.RR_Header{Name: owner, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 60},
		Ns:      primary,
		Mbox:    dns.Fqdn("hostmaster." + strings.TrimSuffix(owner, ".")),
		Serial:  serial,
		Refresh: 3600,
		Retry:   600,
		Expire:  86400,
		Minttl:  60,
	}
}

func nsRR(owner, target string) *dns.NS {
	return &dns.NS{Hdr: dns.RR_Header{Name: owner, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 60}, Ns: target}
}

func aRR(owner string, ip net.IP) *dns.A {
	return &dns.A{Hdr: dns.RR_Header{Name: dns.Fqdn(owner), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: ip}
}

func fixtureIPv4() net.IP {
	conn, err := net.Dial("udp", "192.0.2.1:9")
	if err != nil {
		return net.IPv4(127, 0, 0, 1)
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP.To4() == nil {
		return net.IPv4(127, 0, 0, 1)
	}
	return addr.IP.To4()
}

func serveBoth(addr, role string) {
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		if err := w.WriteMsg(answer(req, role)); err != nil {
			log.Print("DNS fixture write failed")
		}
	})
	for _, network := range []string{"udp", "tcp"} {
		server := &dns.Server{Addr: addr, Net: network, Handler: handler, UDPSize: 1232}
		go func() {
			if err := server.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		}()
	}
}

func answer(req *dns.Msg, role string) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Authoritative = role != roleRecursive
	msg.RecursionAvailable = role == roleRecursive
	if len(req.Question) != 1 {
		msg.Rcode = dns.RcodeRefused
		return msg
	}
	q := req.Question[0]
	name := strings.ToLower(q.Name)
	if role == roleRecursive {
		if q.Qtype == dns.TypeA && name == "fixture.test." {
			msg.Answer = []dns.RR{aRR("fixture.test.", net.IPv4(192, 0, 2, 42))}
			return msg
		}
		if q.Qtype == dns.TypeA && name == "differs.test." {
			msg.Answer = []dns.RR{aRR("differs.test.", net.IPv4(192, 0, 2, 43))}
			return msg
		}
		if q.Qtype == dns.TypeA && glueName(name) {
			msg.Answer = []dns.RR{aRR(name, glue)}
			return msg
		}
	}
	rrs, ok := zoneRecords(role, name, q.Qtype)
	if !ok {
		msg.Rcode = dns.RcodeNameError
		return msg
	}
	msg.Answer = rrs
	return msg
}

func glueName(name string) bool {
	return (strings.HasPrefix(name, "ns1.") || strings.HasPrefix(name, "ns2.")) && strings.HasSuffix(name, ".test.")
}

func zoneRecords(role, name string, qtype uint16) ([]dns.RR, bool) {
	set := zonesA
	if role == roleB {
		set = zonesB
	}
	for _, zone := range set {
		if name == "www."+zone.name && qtype == dns.TypeA && zone.www != nil {
			return zone.www, true
		}
		if name != zone.name {
			continue
		}
		switch qtype {
		case dns.TypeSOA:
			return []dns.RR{zone.soa}, true
		case dns.TypeNS:
			return zone.ns, true
		case dns.TypeDNSKEY:
			if zone.dnskey != nil {
				return zone.dnskey, true
			}
		}
	}
	return nil, false
}

func probeLabDNS() bool {
	return probeSOA("127.0.0.1:5354", "consistent.test.", 100) && probeA("127.0.0.1:5356", "fixture.test.", "192.0.2.42")
}

func probeSOA(addr, name string, serial uint32) bool {
	msg, ok := exchange(addr, name, dns.TypeSOA)
	if !ok {
		return false
	}
	for _, rr := range msg.Answer {
		soa, isSOA := rr.(*dns.SOA)
		if isSOA && soa.Serial == serial {
			return true
		}
	}
	return false
}

func probeA(addr, name, want string) bool {
	msg, ok := exchange(addr, name, dns.TypeA)
	if !ok {
		return false
	}
	for _, rr := range msg.Answer {
		if a, isA := rr.(*dns.A); isA && a.A.String() == want {
			return true
		}
	}
	return false
}

func exchange(addr, name string, qtype uint16) (*dns.Msg, bool) {
	query := new(dns.Msg)
	query.SetQuestion(dns.Fqdn(name), qtype)
	client := &dns.Client{Net: "udp", Timeout: 300 * time.Millisecond}
	msg, _, err := client.Exchange(query, addr)
	return msg, err == nil && msg != nil && msg.Rcode == dns.RcodeSuccess
}
