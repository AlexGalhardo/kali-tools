package tlsaudit_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/tlsaudit"
)

func TestValidateTLSAuditRequestBounds(t *testing.T) {
	valid := tlsaudit.Request{Host: "fixture.test", Port: 443, TimeoutMS: 5000, Lint: true}
	tests := []struct {
		name    string
		change  func(*tlsaudit.Request)
		invalid bool
	}{
		{name: "valid", change: func(*tlsaudit.Request) {}},
		{name: "empty host", change: func(r *tlsaudit.Request) { r.Host = "" }, invalid: true},
		{name: "slash in host", change: func(r *tlsaudit.Request) { r.Host = "fixture.test/path" }, invalid: true},
		{name: "host at limit", change: func(r *tlsaudit.Request) { r.Host = strings.Repeat("a.", 125) + "abc" }},
		{name: "host above limit", change: func(r *tlsaudit.Request) { r.Host = strings.Repeat("a.", 125) + "abcd" }, invalid: true},
		{name: "ipv4 host", change: func(r *tlsaudit.Request) { r.Host = "127.0.0.1" }},
		{name: "ipv6 host", change: func(r *tlsaudit.Request) { r.Host = "::1" }},
		{name: "zoned ipv6 host", change: func(r *tlsaudit.Request) { r.Host = "fe80::1%eth0" }},
		{name: "zero port", change: func(r *tlsaudit.Request) { r.Port = 0 }, invalid: true},
		{name: "negative port", change: func(r *tlsaudit.Request) { r.Port = -1 }, invalid: true},
		{name: "minimum port", change: func(r *tlsaudit.Request) { r.Port = 1 }},
		{name: "maximum port", change: func(r *tlsaudit.Request) { r.Port = 65535 }},
		{name: "port above limit", change: func(r *tlsaudit.Request) { r.Port = 65536 }, invalid: true},
		{name: "timeout below limit", change: func(r *tlsaudit.Request) { r.TimeoutMS = 499 }, invalid: true},
		{name: "minimum timeout", change: func(r *tlsaudit.Request) { r.TimeoutMS = 500 }},
		{name: "maximum timeout", change: func(r *tlsaudit.Request) { r.TimeoutMS = 15000 }},
		{name: "timeout above limit", change: func(r *tlsaudit.Request) { r.TimeoutMS = 15001 }, invalid: true},
		{name: "dns sni", change: func(r *tlsaudit.Request) { r.ServerName = "fixture.test" }},
		{name: "ipv4 sni", change: func(r *tlsaudit.Request) { r.ServerName = "127.0.0.1" }, invalid: true},
		{name: "ipv6 sni", change: func(r *tlsaudit.Request) { r.ServerName = "::1" }, invalid: true},
		{name: "zoned sni", change: func(r *tlsaudit.Request) { r.ServerName = "fe80::1%eth0" }, invalid: true},
		{name: "invalid sni", change: func(r *tlsaudit.Request) { r.ServerName = "fixture.test/path" }, invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := valid
			tt.change(&r)
			err := tlsaudit.Validate(r)
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
			if strings.Contains(de.Message, r.Host) && r.Host != "" {
				t.Fatalf("error includes host: %s", de.Message)
			}
		})
	}
}
