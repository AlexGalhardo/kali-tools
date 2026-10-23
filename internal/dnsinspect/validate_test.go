package dnsinspect_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/dnsinspect"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const validDS = ". IN DS 20326 8 2 E06D44B80B8F1D39A95C0B0D7C65D08458E880409BBC683457104237C7F8EC8D"

func TestValidateDNSInspectRequest(t *testing.T) {
	valid := dnsinspect.Request{
		Name:      "example.com",
		Type:      "A",
		Checks:    []string{"compare"},
		TimeoutMS: 2000,
	}
	tests := []struct {
		name    string
		change  func(*dnsinspect.Request)
		invalid bool
	}{
		{name: "valid", change: func(*dnsinspect.Request) {}},
		{name: "empty type means A", change: func(r *dnsinspect.Request) { r.Type = "" }},
		{name: "accepted types", change: func(r *dnsinspect.Request) { r.Type = "DNSKEY" }},
		{name: "ip name", change: func(r *dnsinspect.Request) { r.Name = "192.0.2.1" }, invalid: true},
		{name: "ipv6 name", change: func(r *dnsinspect.Request) { r.Name = "2001:db8::1" }, invalid: true},
		{name: "empty name", change: func(r *dnsinspect.Request) { r.Name = "" }, invalid: true},
		{name: "name at limit", change: func(r *dnsinspect.Request) { r.Name = strings.Repeat("a.", 125) + "abc" }},
		{name: "name above limit", change: func(r *dnsinspect.Request) { r.Name = strings.Repeat("a.", 125) + "abcd" }, invalid: true},
		{name: "type any", change: func(r *dnsinspect.Request) { r.Type = "ANY" }, invalid: true},
		{name: "four resolvers", change: func(r *dnsinspect.Request) { r.Resolvers = numberedIPs(dnsinspect.MaxResolvers) }},
		{name: "five resolvers", change: func(r *dnsinspect.Request) { r.Resolvers = numberedIPs(dnsinspect.MaxResolvers + 1) }, invalid: true},
		{name: "duplicate resolver", change: func(r *dnsinspect.Request) { r.Resolvers = []string{"192.0.2.1", "192.0.2.1:53"} }, invalid: true},
		{name: "hostname resolver", change: func(r *dnsinspect.Request) { r.Resolvers = []string{"dns.example"} }, invalid: true},
		{name: "zoned unicast resolver", change: func(r *dnsinspect.Request) { r.Resolvers = []string{"192.0.2.1%eth0"} }, invalid: true},
		{name: "link local resolver", change: func(r *dnsinspect.Request) { r.Resolvers = []string{"fe80::1%eth0"} }},
		{name: "ipv6 resolver", change: func(r *dnsinspect.Request) { r.Resolvers = []string{"2001:db8::1"} }},
		{name: "two doh", change: func(r *dnsinspect.Request) { r.DoH = numberedDoH(dnsinspect.MaxDoH) }},
		{name: "three doh", change: func(r *dnsinspect.Request) { r.DoH = numberedDoH(dnsinspect.MaxDoH + 1) }, invalid: true},
		{name: "http doh", change: func(r *dnsinspect.Request) { r.DoH = []string{"http://dns.example/dns-query"} }, invalid: true},
		{name: "doh userinfo", change: func(r *dnsinspect.Request) { r.DoH = []string{"https://user:pass@dns.example/dns-query"} }, invalid: true},
		{name: "doh at length limit", change: func(r *dnsinspect.Request) { r.DoH = []string{dohOfLength(2048)} }},
		{name: "doh above length limit", change: func(r *dnsinspect.Request) { r.DoH = []string{dohOfLength(2049)} }, invalid: true},
		{name: "two dot", change: func(r *dnsinspect.Request) { r.DoT = numberedDoT(dnsinspect.MaxDoT) }},
		{name: "three dot", change: func(r *dnsinspect.Request) { r.DoT = numberedDoT(dnsinspect.MaxDoT + 1) }, invalid: true},
		{name: "dot port bounds", change: func(r *dnsinspect.Request) { r.DoT = []string{"dns.example:1", "dns.example:65535"} }},
		{name: "dot port zero", change: func(r *dnsinspect.Request) { r.DoT = []string{"dns.example:0"} }, invalid: true},
		{name: "dot port above limit", change: func(r *dnsinspect.Request) { r.DoT = []string{"dns.example:65536"} }, invalid: true},
		{name: "empty checks", change: func(r *dnsinspect.Request) { r.Checks = nil }, invalid: true},
		{name: "duplicate check", change: func(r *dnsinspect.Request) { r.Checks = []string{"compare", "compare"} }, invalid: true},
		{name: "unknown check", change: func(r *dnsinspect.Request) { r.Checks = []string{"axfr"} }, invalid: true},
		{name: "encrypted without endpoint", change: func(r *dnsinspect.Request) { r.Checks = []string{"encrypted"} }, invalid: true},
		{name: "encrypted with doh", change: func(r *dnsinspect.Request) {
			r.Checks = []string{"encrypted"}
			r.DoH = []string{"https://dns.example/dns-query"}
		}},
		{name: "five anchors", change: func(r *dnsinspect.Request) { r.TrustAnchors = repeat(validDS, dnsinspect.MaxTrustAnchors+1) }, invalid: true},
		{name: "four anchors", change: func(r *dnsinspect.Request) { r.TrustAnchors = repeat(validDS, dnsinspect.MaxTrustAnchors) }},
		{name: "address is not a ds", change: func(r *dnsinspect.Request) { r.TrustAnchors = []string{"example.com. IN A 192.0.2.1"} }, invalid: true},
		{name: "timeout 199", change: func(r *dnsinspect.Request) { r.TimeoutMS = dnsinspect.MinTimeoutMS - 1 }, invalid: true},
		{name: "timeout 200", change: func(r *dnsinspect.Request) { r.TimeoutMS = dnsinspect.MinTimeoutMS }},
		{name: "timeout 5000", change: func(r *dnsinspect.Request) { r.TimeoutMS = dnsinspect.MaxTimeoutMS }},
		{name: "timeout 5001", change: func(r *dnsinspect.Request) { r.TimeoutMS = dnsinspect.MaxTimeoutMS + 1 }, invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := valid
			tt.change(&r)
			err := dnsinspect.Validate(r)
			if !tt.invalid {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			de, ok := errors.AsType[*domain.Error](err)
			if !ok || de.Code != domain.CodeInvalidInput {
				t.Fatalf("got %v, want invalid_input", err)
			}
			for _, secret := range submitted(r) {
				if secret != "" && strings.Contains(de.Message, secret) {
					t.Fatalf("error includes submitted value %q: %s", secret, de.Message)
				}
			}
		})
	}
}

func TestValidateDNSInspectRecordTypes(t *testing.T) {
	for _, typ := range []string{"", "A", "AAAA", "CNAME", "MX", "NS", "TXT", "SOA", "CAA", "DS", "DNSKEY"} {
		err := dnsinspect.Validate(dnsinspect.Request{Name: "example.com", Type: typ, Checks: []string{"compare"}, TimeoutMS: 2000})
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
	}
}

func submitted(r dnsinspect.Request) []string {
	out := []string{r.Name}
	out = append(out, r.Resolvers...)
	out = append(out, r.DoH...)
	out = append(out, r.DoT...)
	out = append(out, r.TrustAnchors...)
	return out
}

func numberedIPs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("192.0.2.%d", i+1)
	}
	return out
}

func numberedDoH(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("https://dns.example/dns-query-%d", i)
	}
	return out
}

func numberedDoT(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("dns%d.example", i)
	}
	return out
}

func dohOfLength(n int) string {
	prefix := "https://example.com/"
	return prefix + strings.Repeat("a", n-len(prefix))
}

func repeat(value string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = value
	}
	return out
}
