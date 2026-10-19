package network

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestDNSRecordFixtures(t *testing.T) {
	socket, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		b := make([]byte, 4096)
		for {
			n, peer, err := socket.ReadFrom(b)
			if err != nil {
				return
			}
			var query dnsmessage.Message
			if query.Unpack(b[:n]) != nil || len(query.Questions) != 1 {
				continue
			}
			q := query.Questions[0]
			if q.Name.String() == "silent.test." {
				continue
			}
			answer := dnsmessage.Message{Header: dnsmessage.Header{ID: query.ID, Response: true, RecursionDesired: true, RecursionAvailable: true}, Questions: query.Questions}
			name := dnsmessage.MustNewName("canonical.test.")
			header := dnsmessage.ResourceHeader{Name: q.Name, Class: dnsmessage.ClassINET, Type: q.Type, TTL: 1}
			if q.Name.String() == "missing.test." {
				answer.RCode = dnsmessage.RCodeNameError
			} else {
				var body dnsmessage.ResourceBody
				if q.Name.String() == "alias.test." && (q.Type == dnsmessage.TypeA || q.Type == dnsmessage.TypeAAAA) {
					answer.Answers = append(answer.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: q.Name, Class: dnsmessage.ClassINET, Type: dnsmessage.TypeCNAME, TTL: 1}, Body: &dnsmessage.CNAMEResource{CNAME: name}})
					header.Name = name
				}
				switch q.Type {
				case dnsmessage.TypeA:
					body = &dnsmessage.AResource{A: [4]byte{192, 0, 2, 42}}
				case dnsmessage.TypeAAAA:
					body = &dnsmessage.AAAAResource{AAAA: [16]byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x42}}
				case dnsmessage.TypeMX:
					body = &dnsmessage.MXResource{Pref: 10, MX: name}
				case dnsmessage.TypeNS:
					body = &dnsmessage.NSResource{NS: name}
				case dnsmessage.TypeTXT:
					body = &dnsmessage.TXTResource{TXT: []string{"synthetic-value"}}
				case dnsmessage.TypePTR:
					body = &dnsmessage.PTRResource{PTR: name}
				}
				if body != nil {
					answer.Answers = append(answer.Answers, dnsmessage.Resource{Header: header, Body: body})
				}
			}
			raw, err := answer.Pack()
			if err == nil {
				_, _ = socket.WriteTo(raw, peer)
			}
		}
	}()
	t.Cleanup(func() { _ = socket.Close(); <-done })
	resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", socket.LocalAddr().String())
	}}
	for _, tt := range []struct{ kind, host, want string }{
		{"A", "fixture.test", "192.0.2.42"}, {"AAAA", "fixture.test", "2001:db8::42"}, {"CNAME", "alias.test", "canonical.test."},
		{"MX", "fixture.test", "10 canonical.test."}, {"NS", "fixture.test", "canonical.test."}, {"TXT", "fixture.test", "synthetic-value"}, {"PTR", "192.0.2.42", "canonical.test."},
	} {
		t.Run(tt.kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			values, err := lookupDNS(ctx, resolver, tt.host, tt.kind)
			if err != nil || len(values) != 1 || values[0] != tt.want {
				t.Fatalf("%v, %v", values, err)
			}
		})
	}
	if _, err := lookupDNS(t.Context(), resolver, "missing.test", "A"); err == nil {
		t.Fatal("NXDOMAIN accepted")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if _, err := lookupDNS(ctx, resolver, "silent.test", "TXT"); err == nil {
		t.Fatal("timeout accepted")
	}
	cancelled, stop := context.WithCancel(t.Context())
	stop()
	if _, err := lookupDNS(cancelled, resolver, "fixture.test", "A"); err == nil {
		t.Fatal("cancelled query accepted")
	}
}

func TestDNSRequestValidation(t *testing.T) {
	for _, r := range []Request{
		{Kind: "dns", Target: "example.test", RecordType: "ANY", TimeoutMS: 100},
		{Kind: "dns", Target: "example.test", RecordType: "PTR", TimeoutMS: 100},
		{Kind: "dns", Target: "fe80::1%3", RecordType: "PTR", TimeoutMS: 100},
		{Kind: "tcp", Target: "localhost", RecordType: "A", Port: 80, TimeoutMS: 100},
		{Kind: "http", Target: "http://example.test/" + strings.Repeat("x", 8192), TimeoutMS: 100},
	} {
		if Validate(r) == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
}
