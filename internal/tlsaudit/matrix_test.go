package tlsaudit

import (
	"context"
	"crypto/tls"
	"testing"
)

func TestClientSupportsSelfTest(t *testing.T) {
	for _, v := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		first := clientSupports(v)
		second := clientSupports(v)
		if !first || first != second {
			t.Fatalf("local TLS %x unavailable", v)
		}
	}
}

func TestProtocolMatrixLegacyOnlyServer(t *testing.T) {
	for _, v := range []uint16{tls.VersionTLS10, tls.VersionTLS11} {
		t.Run(tls.VersionName(v), func(t *testing.T) {
			r, roots := auditFixtureServer(t, v, v, false)
			a := auditor{roots: roots}
			matrix := a.protocolMatrix(context.Background(), r)
			for _, p := range matrix {
				want := "rejected"
				if p.Version == tls.VersionName(v) {
					want = "accepted"
				}
				if p.Status == "client_unsupported" {
					continue
				}
				if p.Status != want {
					t.Fatalf("%+v want %s", p, want)
				}
			}
		})
	}
}
