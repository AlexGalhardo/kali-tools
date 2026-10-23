package dnsinspect

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func withHook(t *testing.T, fn func(*inspector)) {
	t.Helper()
	previous := inspectHook
	inspectHook = fn
	t.Cleanup(func() { inspectHook = previous })
}

func nxSystem(context.Context, string, string) ([]string, string, string, float64) {
	return nil, "nxdomain", "NXDOMAIN", 0
}

func listenDNS(t *testing.T, addr string, handler dns.Handler) string {
	t.Helper()
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	return serveDNS(t, conn, nil, handler)
}

func listenBoth(t *testing.T, addr string, handler dns.Handler) string {
	t.Helper()
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	tcp, err := net.Listen("tcp", conn.LocalAddr().String())
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	return serveDNS(t, conn, tcp, handler)
}

func serveDNS(t *testing.T, conn net.PacketConn, tcp net.Listener, handler dns.Handler) string {
	t.Helper()
	ready := make(chan struct{}, 2)
	notify := func() { ready <- struct{}{} }
	udp := &dns.Server{PacketConn: conn, Handler: handler, NotifyStartedFunc: notify}
	var tcpServer *dns.Server
	go func() { _ = udp.ActivateAndServe() }()
	if tcp != nil {
		tcpServer = &dns.Server{Listener: tcp, Handler: handler, NotifyStartedFunc: notify}
		go func() { _ = tcpServer.ActivateAndServe() }()
	}
	expect := 1
	if tcp != nil {
		expect = 2
	}
	for range expect {
		select {
		case <-ready:
		case <-time.After(2 * time.Second):
			t.Fatal("dns server did not start")
		}
	}
	t.Cleanup(func() {
		_ = udp.Shutdown()
		if tcpServer != nil {
			_ = tcpServer.Shutdown()
		}
	})
	return conn.LocalAddr().String()
}

func reply(w dns.ResponseWriter, req *dns.Msg, aa bool, answer []dns.RR) {
	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Authoritative = aa
	msg.RecursionAvailable = !aa
	msg.Answer = answer
	_ = w.WriteMsg(msg)
}

func hdr(name string, typ uint16, ttl uint32) dns.RR_Header {
	return dns.RR_Header{Name: dns.Fqdn(name), Rrtype: typ, Class: dns.ClassINET, Ttl: ttl}
}

func aRR(name, ip string, ttl uint32) *dns.A {
	return &dns.A{Hdr: hdr(name, dns.TypeA, ttl), A: net.ParseIP(ip)}
}

func soaRR(name string, serial uint32) *dns.SOA {
	fqdn := dns.Fqdn(name)
	return &dns.SOA{Hdr: hdr(name, dns.TypeSOA, 60), Ns: "ns1." + fqdn, Mbox: "hostmaster." + fqdn, Serial: serial, Refresh: 100, Retry: 100, Expire: 100, Minttl: 100}
}

func nsRR(zone, host string) *dns.NS {
	return &dns.NS{Hdr: hdr(zone, dns.TypeNS, 60), Ns: dns.Fqdn(host)}
}

func TestCompareResolversDisagree(t *testing.T) {
	left := listenDNS(t, "127.0.0.1:0", dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		reply(w, req, false, []dns.RR{aRR(req.Question[0].Name, "192.0.2.10", 12345)})
	}))
	right := listenDNS(t, "127.0.0.1:0", dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		reply(w, req, false, []dns.RR{aRR(req.Question[0].Name, "192.0.2.20", 99)})
	}))
	withHook(t, func(ins *inspector) { ins.system = nxSystem })
	report, err := Inspect(t.Context(), Request{
		Name: "example.com", Type: "A", Resolvers: []string{left, right}, Checks: []string{"compare"}, TimeoutMS: 500,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Compare == nil || report.Compare.Agree || len(report.Compare.Groups) != 2 {
		t.Fatalf("%+v", report.Compare)
	}
	got := map[string]string{}
	for _, result := range report.Compare.Results {
		if result.Resolver == "system" {
			if result.Status != "nxdomain" {
				t.Fatalf("%+v", result)
			}
			continue
		}
		if result.Status != "ok" || result.Transport != "udp" || len(result.Answers) != 1 {
			t.Fatalf("%+v", result)
		}
		if strings.Contains(result.Answers[0], "12345") || result.MinTTL == nil || *result.MinTTL != 12345 && *result.MinTTL != 99 {
			t.Fatalf("%+v", result)
		}
		got[result.Resolver] = result.Answers[0]
	}
	if !strings.Contains(got[left], "192.0.2.10") || !strings.Contains(got[right], "192.0.2.20") {
		t.Fatal(got)
	}
}

func TestTruncatedAnswerRetriesTCP(t *testing.T) {
	var sawEDNS atomic.Bool
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		if opt := req.IsEdns0(); opt != nil && opt.UDPSize() == EDNSBufferSize && opt.Do() {
			sawEDNS.Store(true)
		}
		if w.RemoteAddr().Network() == "udp" {
			msg := new(dns.Msg)
			msg.SetReply(req)
			msg.Truncated = true
			msg.Answer = []dns.RR{aRR(req.Question[0].Name, "192.0.2.1", 60)}
			_ = w.WriteMsg(msg)
			return
		}
		reply(w, req, false, []dns.RR{aRR(req.Question[0].Name, "192.0.2.99", 60)})
	})
	addr := listenBoth(t, "127.0.0.1:0", handler)
	withHook(t, func(ins *inspector) { ins.system = nxSystem })
	report, err := Inspect(t.Context(), Request{
		Name: "example.com", Type: "A", Resolvers: []string{addr}, Checks: []string{"compare"}, TimeoutMS: 500,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, result := range report.Compare.Results {
		if result.Resolver != addr {
			continue
		}
		found = true
		if result.Transport != "tcp" || result.Status != "ok" || !strings.Contains(strings.Join(result.Answers, " "), "192.0.2.99") {
			t.Fatalf("%+v", result)
		}
	}
	if !found || !sawEDNS.Load() {
		t.Fatalf("found=%v edns=%v %+v", found, sawEDNS.Load(), report.Compare)
	}
}

func TestSOANSConsistency(t *testing.T) {
	const ns1, ns2 = "ns1.lab.test.", "ns2.lab.test."
	authA := map[string]struct {
		serial uint32
		aa     bool
	}{
		"consistent.test.": {100, true},
		"mismatch.test.":   {100, true},
		"lame.test.":       {1, false},
	}
	authB := map[string]struct {
		serial uint32
		aa     bool
	}{
		"consistent.test.": {100, true},
		"mismatch.test.":   {101, true},
	}
	serveAuth := func(zones map[string]struct {
		serial uint32
		aa     bool
	}) dns.Handler {
		return dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
			q := req.Question[0]
			zone, ok := zones[strings.ToLower(q.Name)]
			if !ok {
				reply(w, req, false, nil)
				return
			}
			var answer []dns.RR
			switch q.Qtype {
			case dns.TypeSOA:
				answer = []dns.RR{soaRR(q.Name, zone.serial)}
			case dns.TypeNS:
				answer = []dns.RR{nsRR(q.Name, ns1), nsRR(q.Name, ns2)}
			}
			reply(w, req, zone.aa, answer)
		})
	}
	aAddr := listenBoth(t, "127.0.0.1:0", serveAuth(authA))
	bAddr := listenBoth(t, "127.0.0.2:0", serveAuth(authB))
	resolver := listenBoth(t, "127.0.0.1:0", dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		q := req.Question[0]
		name := strings.ToLower(q.Name)
		switch q.Qtype {
		case dns.TypeSOA:
			if strings.HasSuffix(name, ".test.") || strings.HasSuffix(name, "test.") {
				reply(w, req, false, []dns.RR{soaRR(name, 1)})
				return
			}
		case dns.TypeNS:
			if name == "empty.test." {
				reply(w, req, false, nil)
				return
			}
			if name == "lame.test." {
				reply(w, req, false, []dns.RR{nsRR(name, ns1)})
				return
			}
			reply(w, req, false, []dns.RR{nsRR(name, ns1), nsRR(name, ns2)})
			return
		case dns.TypeA:
			switch name {
			case ns1:
				reply(w, req, false, []dns.RR{aRR(name, "127.0.0.1", 60)})
				return
			case ns2:
				reply(w, req, false, []dns.RR{aRR(name, "127.0.0.2", 60)})
				return
			}
		}
		reply(w, req, false, nil)
	}))
	ports := map[string]uint16{}
	for _, addr := range []string{aAddr, bAddr} {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatal(err)
		}
		n, err := strconv.Atoi(port)
		if err != nil {
			t.Fatal(err)
		}
		ports[host] = uint16(n)
	}
	withHook(t, func(ins *inspector) {
		ins.authPort = func(ip netip.Addr) uint16 { return ports[ip.String()] }
	})
	cases := []struct {
		name, want string
	}{
		{"consistent.test", "consistent"},
		{"mismatch.test", "serial_mismatch"},
		{"lame.test", "lame_servers"},
		{"empty.test", "no_servers"},
	}
	for _, tt := range cases {
		t.Run(tt.want, func(t *testing.T) {
			report, err := Inspect(t.Context(), Request{
				Name: tt.name, Type: "A", Resolvers: []string{resolver}, Checks: []string{"soa_ns"}, TimeoutMS: 500,
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if report.SOANS == nil || report.SOANS.Verdict != tt.want {
				t.Fatalf("%+v", report.SOANS)
			}
		})
	}
}

func TestDNSSECChainVerdicts(t *testing.T) {
	now := time.Now()
	zone := newZone(t, "signed.test", now)
	secureAddr := listenDNS(t, "127.0.0.1:0", zone.handler(false))
	tampered := newZone(t, "signed.test", now)
	tamperedAddr := listenDNS(t, "127.0.0.1:0", tampered.handler(true))
	parent := newZone(t, "example.test", now)
	insecureAddr := listenDNS(t, "127.0.0.1:0", parent.childHandler())
	base := Request{Type: "A", Checks: []string{"dnssec"}, TimeoutMS: 500}
	t.Run("secure", func(t *testing.T) {
		req := base
		req.Name = "www.signed.test"
		req.Resolvers = []string{secureAddr}
		req.TrustAnchors = []string{zone.dsText()}
		report, err := Inspect(t.Context(), req, nil)
		if err != nil || report.DNSSEC == nil || report.DNSSEC.Verdict != "secure" || !report.DNSSEC.AnswerSig {
			t.Fatalf("%v %+v", err, report.DNSSEC)
		}
		for _, link := range report.DNSSEC.Links {
			if link.Status != "secure" {
				t.Fatalf("%+v", report.DNSSEC)
			}
		}
	})
	t.Run("bogus", func(t *testing.T) {
		req := base
		req.Name = "www.signed.test"
		req.Resolvers = []string{tamperedAddr}
		req.TrustAnchors = []string{tampered.dsText()}
		report, err := Inspect(t.Context(), req, nil)
		if err != nil || report.DNSSEC == nil || report.DNSSEC.Verdict != "bogus" {
			t.Fatalf("%v %+v", err, report.DNSSEC)
		}
	})
	t.Run("expired", func(t *testing.T) {
		withHook(t, func(ins *inspector) { ins.now = func() time.Time { return now.Add(48 * time.Hour) } })
		req := base
		req.Name = "www.signed.test"
		req.Resolvers = []string{secureAddr}
		req.TrustAnchors = []string{zone.dsText()}
		report, err := Inspect(t.Context(), req, nil)
		if err != nil || report.DNSSEC == nil || report.DNSSEC.Verdict != "bogus" {
			t.Fatalf("%v %+v", err, report.DNSSEC)
		}
	})
	t.Run("insecure", func(t *testing.T) {
		req := base
		req.Name = "www.child.example.test"
		req.Resolvers = []string{insecureAddr}
		req.TrustAnchors = []string{parent.dsText()}
		report, err := Inspect(t.Context(), req, nil)
		if err != nil || report.DNSSEC == nil || report.DNSSEC.Verdict != "insecure" {
			t.Fatalf("%v %+v", err, report.DNSSEC)
		}
	})
}

type signedZone struct {
	name       string
	ksk, zsk   *dns.DNSKEY
	kskPriv    crypto.Signer
	zskPriv    crypto.Signer
	dnskeySig  *dns.RRSIG
	answerSig  *dns.RRSIG
	address    *dns.A
	soa        *dns.SOA
	inception  uint32
	expiration uint32
}

func newZone(t *testing.T, name string, now time.Time) signedZone {
	t.Helper()
	fqdn := dns.Fqdn(name)
	z := signedZone{
		name:       fqdn,
		inception:  uint32(now.Add(-time.Hour).Unix()),
		expiration: uint32(now.Add(24 * time.Hour).Unix()),
	}
	z.ksk, z.kskPriv = keyFromSeed(fqdn, 257, bytesRepeat(0x11))
	z.zsk, z.zskPriv = keyFromSeed(fqdn, 256, bytesRepeat(0x22))
	z.dnskeySig = z.sign(t, z.kskPriv, z.ksk.KeyTag(), []dns.RR{z.ksk, z.zsk})
	if err := z.dnskeySig.Verify(z.ksk, []dns.RR{z.ksk, z.zsk}); err != nil {
		t.Fatal(err)
	}
	z.address = aRR("www."+fqdn, "192.0.2.10", 300)
	z.answerSig = z.sign(t, z.zskPriv, z.zsk.KeyTag(), []dns.RR{z.address})
	z.soa = soaRR(fqdn, 1)
	return z
}

func (z signedZone) dsText() string {
	ds := z.ksk.ToDS(dns.SHA256)
	return fmt.Sprintf("%s IN DS %d %d %d %s", ds.Hdr.Name, ds.KeyTag, ds.Algorithm, ds.DigestType, ds.Digest)
}

func (z signedZone) sign(t *testing.T, key crypto.Signer, tag uint16, rrset []dns.RR) *dns.RRSIG {
	t.Helper()
	sig := &dns.RRSIG{
		Hdr:        hdr(rrset[0].Header().Name, dns.TypeRRSIG, 3600),
		Algorithm:  dns.ED25519,
		KeyTag:     tag,
		SignerName: z.name,
		Inception:  z.inception,
		Expiration: z.expiration,
	}
	if err := sig.Sign(key, rrset); err != nil {
		t.Fatal(err)
	}
	return sig
}

func (z signedZone) handler(tamper bool) dns.Handler {
	sig := z.dnskeySig
	if tamper {
		cloned := *z.dnskeySig
		raw := []byte(cloned.Signature)
		raw[0] = 'A'
		if string(raw) == cloned.Signature {
			raw[0] = 'B'
		}
		cloned.Signature = string(raw)
		sig = &cloned
	}
	return dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		q := req.Question[0]
		name := strings.ToLower(q.Name)
		switch q.Qtype {
		case dns.TypeSOA:
			if name == z.name {
				reply(w, req, true, []dns.RR{z.soa})
				return
			}
		case dns.TypeDNSKEY:
			if name == z.name {
				reply(w, req, true, []dns.RR{z.ksk, z.zsk, sig})
				return
			}
		case dns.TypeA:
			if name == strings.ToLower(z.address.Hdr.Name) {
				reply(w, req, true, []dns.RR{z.address, z.answerSig})
				return
			}
		}
		reply(w, req, true, nil)
	})
}

func (z signedZone) childHandler() dns.Handler {
	child := dns.Fqdn("child." + z.name)
	return dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		q := req.Question[0]
		name := strings.ToLower(q.Name)
		switch q.Qtype {
		case dns.TypeSOA:
			if name == child {
				reply(w, req, true, []dns.RR{soaRR(child, 1)})
				return
			}
		case dns.TypeDNSKEY:
			if name == z.name {
				reply(w, req, true, []dns.RR{z.ksk, z.zsk, z.dnskeySig})
				return
			}
		case dns.TypeDS:
			if name == child {
				reply(w, req, true, nil)
				return
			}
		}
		reply(w, req, true, nil)
	})
}

func keyFromSeed(name string, flags uint16, seed []byte) (*dns.DNSKEY, crypto.Signer) {
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	key := &dns.DNSKEY{
		Hdr:       hdr(name, dns.TypeDNSKEY, 3600),
		Flags:     flags,
		Protocol:  3,
		Algorithm: dns.ED25519,
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}
	return key, priv
}

func bytesRepeat(b byte) []byte {
	out := make([]byte, ed25519.SeedSize)
	for i := range out {
		out[i] = b
	}
	return out
}

func TestEncryptedEndpoints(t *testing.T) {
	okServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/dns-message" || r.Header.Get("Accept") != "application/dns-message" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 600))
		req := new(dns.Msg)
		if err := req.Unpack(body); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		msg := new(dns.Msg)
		msg.SetReply(req)
		packed, _ := msg.Pack()
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(packed)
	}))
	t.Cleanup(okServer.Close)
	statusServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(statusServer.Close)
	bigServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, MaxResponseBytes+1))
	}))
	t.Cleanup(bigServer.Close)
	pool := x509.NewCertPool()
	pool.AddCert(okServer.Certificate())
	pool.AddCert(statusServer.Certificate())
	pool.AddCert(bigServer.Certificate())
	cert, dotPool := localCert(t)
	dotAddr := listenDoT(t, cert)
	withHook(t, func(ins *inspector) { ins.roots = pool })
	report, err := Inspect(t.Context(), Request{
		Name: "example.com", Type: "A", Checks: []string{"encrypted"}, TimeoutMS: 1000,
		DoH: []string{okServer.URL + "/dns-query"},
	}, nil)
	if err != nil || len(report.Encrypted) != 1 || report.Encrypted[0].Status != "ok" || report.Encrypted[0].RCode != "NOERROR" {
		t.Fatalf("%v %+v", err, report.Encrypted)
	}
	withHook(t, func(ins *inspector) { ins.roots = pool })
	report, err = Inspect(t.Context(), Request{
		Name: "example.com", Type: "A", Checks: []string{"encrypted"}, TimeoutMS: 1000,
		DoH: []string{statusServer.URL + "/dns-query"},
	}, nil)
	if err != nil || report.Encrypted[0].Status != "http_status" || report.Encrypted[0].HTTPCode != 500 {
		t.Fatalf("%v %+v", err, report.Encrypted)
	}
	withHook(t, func(ins *inspector) { ins.roots = nil })
	report, err = Inspect(t.Context(), Request{
		Name: "example.com", Type: "A", Checks: []string{"encrypted"}, TimeoutMS: 1000,
		DoH: []string{okServer.URL + "/dns-query"},
	}, nil)
	if err != nil || report.Encrypted[0].Status != "tls_error" {
		t.Fatalf("%v %+v", err, report.Encrypted)
	}
	withHook(t, func(ins *inspector) { ins.roots = pool })
	report, err = Inspect(t.Context(), Request{
		Name: "example.com", Type: "A", Checks: []string{"encrypted"}, TimeoutMS: 1000,
		DoH: []string{bigServer.URL + "/dns-query"},
	}, nil)
	if err != nil || report.Encrypted[0].Status != "bad_response" {
		t.Fatalf("%v %+v", err, report.Encrypted)
	}
	withHook(t, func(ins *inspector) { ins.roots = dotPool })
	report, err = Inspect(t.Context(), Request{
		Name: "example.com", Type: "A", Checks: []string{"encrypted"}, TimeoutMS: 1000,
		DoT: []string{dotAddr},
	}, nil)
	if err != nil || report.Encrypted[0].Protocol != "dot" || report.Encrypted[0].Status != "ok" {
		t.Fatalf("%v %+v", err, report.Encrypted)
	}
}

func localCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

func listenDoT(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	srv := &dns.Server{Listener: ln, NotifyStartedFunc: func() { close(ready) }, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		reply(w, req, false, []dns.RR{aRR(req.Question[0].Name, "192.0.2.10", 60)})
	})}
	go func() { _ = srv.ActivateAndServe() }()
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("dot server did not start")
	}
	t.Cleanup(func() { _ = srv.Shutdown() })
	return ln.Addr().String()
}

func TestQueryBudget(t *testing.T) {
	var sent atomic.Int32
	withHook(t, func(ins *inspector) {
		ins.udp = func(_ context.Context, _ string, msg *dns.Msg) (*dns.Msg, time.Duration, error) {
			sent.Add(1)
			return budgetReply(msg), time.Millisecond, nil
		}
	})
	report, err := Inspect(t.Context(), Request{
		Name: "a.b.c.d.e.f.g.test", Type: "A", Resolvers: []string{"192.0.2.53"},
		Checks: []string{"soa_ns", "dnssec"}, TimeoutMS: 500,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Limited || report.Queries > MaxQueries || sent.Load() > MaxQueries || sent.Load() != int32(report.Queries) {
		t.Fatalf("limited=%v queries=%d sent=%d", report.Limited, report.Queries, sent.Load())
	}
}

func budgetReply(req *dns.Msg) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetReply(req)
	msg.Authoritative = true
	if len(req.Question) == 0 {
		return msg
	}
	q := req.Question[0]
	name := strings.ToLower(q.Name)
	switch q.Qtype {
	case dns.TypeSOA:
		if name == "test." {
			msg.Answer = []dns.RR{soaRR(name, 1)}
		}
	case dns.TypeNS:
		if name == "test." {
			for i := 1; i <= MaxNameservers+1; i++ {
				msg.Answer = append(msg.Answer, nsRR(name, fmt.Sprintf("ns%d.test", i)))
			}
		}
	case dns.TypeA:
		msg.Answer = []dns.RR{aRR(name, "192.0.2.1", 60)}
	case dns.TypeAAAA:
		msg.Answer = []dns.RR{&dns.AAAA{Hdr: hdr(name, dns.TypeAAAA, 60), AAAA: net.ParseIP("2001:db8::1")}}
	}
	return msg
}

func TestInspectCancellation(t *testing.T) {
	var track sync.WaitGroup
	started := make(chan struct{})
	var once sync.Once
	withHook(t, func(ins *inspector) {
		ins.track = &track
		ins.system = nxSystem
		ins.udp = func(ctx context.Context, _ string, _ *dns.Msg) (*dns.Msg, time.Duration, error) {
			once.Do(func() { close(started) })
			<-ctx.Done()
			return nil, 0, ctx.Err()
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	errc := make(chan error, 1)
	go func() {
		_, err := Inspect(ctx, Request{
			Name: "example.com", Type: "A", Resolvers: []string{"192.0.2.1"}, Checks: []string{"compare"}, TimeoutMS: 1000,
		}, nil)
		errc <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("inspection did not start")
	}
	cancel()
	err := <-errc
	var target *domain.Error
	if !errors.As(err, &target) || target.Code != domain.CodeCancelled {
		t.Fatal(err)
	}
	track.Wait()
}

func TestRootAnchorsParse(t *testing.T) {
	set, err := parseAnchors(nil)
	if err != nil || len(set) != 2 {
		t.Fatal(err, len(set))
	}
	tags := map[uint16]bool{}
	for _, ds := range set {
		if ds.Hdr.Name != "." || ds.Algorithm != 8 || ds.DigestType != 2 {
			t.Fatalf("%+v", ds)
		}
		tags[ds.KeyTag] = true
	}
	if !tags[20326] || !tags[38696] {
		t.Fatal(tags)
	}
}

func TestRuntimeLimits(t *testing.T) {
	t.Run("answers", func(t *testing.T) {
		withHook(t, func(ins *inspector) {
			ins.system = nxSystem
			ins.udp = func(_ context.Context, _ string, req *dns.Msg) (*dns.Msg, time.Duration, error) {
				msg := new(dns.Msg)
				msg.SetReply(req)
				for i := range MaxAnswersReported + 1 {
					msg.Answer = append(msg.Answer, aRR(req.Question[0].Name, fmt.Sprintf("192.0.2.%d", i+1), 60))
				}
				return msg, time.Millisecond, nil
			}
		})
		report, err := Inspect(t.Context(), Request{Name: "example.com", Type: "A", Resolvers: []string{"192.0.2.1"}, Checks: []string{"compare"}, TimeoutMS: 200}, nil)
		if err != nil || !report.Limited {
			t.Fatal(err, report.Limited)
		}
		for _, result := range report.Compare.Results {
			if result.Resolver == "system" {
				continue
			}
			if len(result.Answers) != MaxAnswersReported {
				t.Fatal(len(result.Answers))
			}
		}
	})
	t.Run("rdata", func(t *testing.T) {
		withHook(t, func(ins *inspector) {
			ins.system = nxSystem
			ins.udp = func(_ context.Context, _ string, req *dns.Msg) (*dns.Msg, time.Duration, error) {
				msg := new(dns.Msg)
				msg.SetReply(req)
				msg.Answer = []dns.RR{&dns.TXT{Hdr: hdr(req.Question[0].Name, dns.TypeTXT, 60), Txt: []string{strings.Repeat("a", MaxRDataText+1)}}}
				return msg, time.Millisecond, nil
			}
		})
		report, err := Inspect(t.Context(), Request{Name: "example.com", Type: "TXT", Resolvers: []string{"192.0.2.1"}, Checks: []string{"compare"}, TimeoutMS: 200}, nil)
		if err != nil || !report.Limited {
			t.Fatal(err, report.Limited)
		}
		for _, result := range report.Compare.Results {
			if result.Status == "ok" && len(result.Answers[0]) != MaxRDataText {
				t.Fatal(len(result.Answers[0]))
			}
		}
	})
	t.Run("workers", func(t *testing.T) {
		var started atomic.Int32
		withHook(t, func(ins *inspector) {
			ins.system = nxSystem
			ins.onWorker = func() { started.Add(1) }
			ins.udp = func(_ context.Context, _ string, req *dns.Msg) (*dns.Msg, time.Duration, error) {
				msg := new(dns.Msg)
				msg.SetReply(req)
				return msg, time.Millisecond, nil
			}
		})
		if _, err := Inspect(t.Context(), Request{Name: "example.com", Type: "A", Resolvers: []string{"192.0.2.1"}, Checks: []string{"compare"}, TimeoutMS: 200}, nil); err != nil {
			t.Fatal(err)
		}
		if started.Load() != Workers {
			t.Fatal(started.Load())
		}
	})
	if OverallDeadline != 30*time.Second {
		t.Fatal(OverallDeadline)
	}
}

func TestLookupAndStatusBranches(t *testing.T) {
	addr := listenDNS(t, "127.0.0.1:0", dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		q := req.Question[0]
		var answer []dns.RR
		switch q.Qtype {
		case dns.TypeA:
			answer = []dns.RR{aRR(q.Name, "192.0.2.9", 30)}
		case dns.TypeAAAA:
			answer = []dns.RR{&dns.AAAA{Hdr: hdr(q.Name, dns.TypeAAAA, 30), AAAA: net.ParseIP("2001:db8::9")}}
		case dns.TypeCNAME:
			answer = []dns.RR{&dns.CNAME{Hdr: hdr(q.Name, dns.TypeCNAME, 30), Target: "alias.example.com."}}
		case dns.TypeMX:
			answer = []dns.RR{&dns.MX{Hdr: hdr(q.Name, dns.TypeMX, 30), Preference: 10, Mx: "mail.example.com."}}
		case dns.TypeNS:
			answer = []dns.RR{nsRR(q.Name, "ns.example.com")}
		case dns.TypeTXT:
			answer = []dns.RR{&dns.TXT{Hdr: hdr(q.Name, dns.TypeTXT, 30), Txt: []string{"v=spf1"}}}
		default:
			reply(w, req, true, nil)
			return
		}
		reply(w, req, true, answer)
	}))
	resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", addr)
	}}
	for _, typ := range []string{"A", "AAAA", "CNAME", "MX", "NS", "TXT"} {
		values, err := lookupSupported(t.Context(), resolver, "example.com", typ)
		if err != nil || len(values) == 0 {
			t.Fatalf("%s: %v %v", typ, err, values)
		}
	}
	if values, err := lookupSupported(t.Context(), resolver, "example.com", "SOA"); err != nil || values != nil {
		t.Fatalf("default: %v %v", err, values)
	}
	broken := &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("down")
	}}
	if _, err := lookupSupported(t.Context(), broken, "example.com", "MX"); err == nil {
		t.Fatal("expected lookup error")
	}
	ins := &inspector{timeout: 200 * time.Millisecond, authPort: func(netip.Addr) uint16 { return 53 }}
	if got := ins.doSystem(t.Context(), "example.com", "SOA"); got.Status != "unsupported_type" {
		t.Fatal(got.Status)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if got := ins.doSystem(cancelled, "example.com", "A"); got.Status != "timeout" {
		t.Fatal(got.Status)
	}
	for _, code := range []int{dns.RcodeSuccess, dns.RcodeNameError, dns.RcodeServerFailure, dns.RcodeRefused, 99} {
		if status, name := rcodeStatus(code); status == "" || name == "" {
			t.Fatal(code, status, name)
		}
	}
	if clientStatus(netTimeout{}) != "timeout" || clientStatus(context.DeadlineExceeded) != "timeout" || clientStatus(errors.New("down")) != "unreachable" {
		t.Fatal("client status")
	}
	if clientStatus(x509.UnknownAuthorityError{}) != "tls_error" || clientStatus(x509.HostnameError{}) != "tls_error" || clientStatus(x509.CertificateInvalidError{}) != "tls_error" || clientStatus(&tls.CertificateVerificationError{}) != "tls_error" {
		t.Fatal("tls status")
	}
	if err := ins.httpClient().CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatal(err)
	}
	for _, raw := range []string{"192.0.2.53", "example.com", "example.com:853"} {
		endpoint, name := dotEndpoint(raw)
		if endpoint == "" || name == "" {
			t.Fatal(raw)
		}
	}
	var timeoutErr net.Error = netTimeout{}
	if timeoutErr.Error() == "" || !timeoutErr.Timeout() {
		t.Fatal("net timeout")
	}
	silent, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = silent.Close() })
	go func() {
		buf := make([]byte, 512)
		for {
			if _, _, err := silent.ReadFrom(buf); err != nil {
				return
			}
		}
	}()
	withHook(t, func(ins *inspector) { ins.system = nxSystem })
	report, err := Inspect(t.Context(), Request{
		Name: "example.com", Type: "A", Resolvers: []string{silent.LocalAddr().String(), "127.0.0.1:1"},
		Checks: []string{"compare"}, TimeoutMS: 200,
	}, nil)
	if err != nil || report.Compare == nil {
		t.Fatal(err)
	}
	sawTimeoutPath := false
	for _, result := range report.Compare.Results {
		if result.Resolver == "system" {
			continue
		}
		if result.Status != "unreachable" && result.Status != "timeout" {
			t.Fatalf("%+v", result)
		}
		sawTimeoutPath = true
	}
	if !sawTimeoutPath {
		t.Fatal("no explicit resolver result")
	}
	report, err = Inspect(t.Context(), Request{
		Name: "example.com", Type: "A", Checks: []string{"encrypted"}, TimeoutMS: 300,
		DoT: []string{"127.0.0.1:1"}, DoH: []string{"https://127.0.0.1:1/dns-query"},
	}, nil)
	if err != nil || len(report.Encrypted) != 2 {
		t.Fatalf("%v %+v", err, report.Encrypted)
	}
	for _, item := range report.Encrypted {
		if item.Status != "unreachable" && item.Status != "timeout" && item.Status != "tls_error" {
			t.Fatalf("%+v", item)
		}
	}
}
