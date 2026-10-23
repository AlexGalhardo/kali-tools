package main

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestPinnedSignedDS(t *testing.T) {
	if got := signedDSText(); got != pinnedSignedDS {
		t.Fatalf("got %s", got)
	}
	pack := buildSigned(time.Now())
	keys := []dns.RR{pack.dnskey[0], pack.dnskey[1]}
	sig := pack.dnskey[2].(*dns.RRSIG)
	if err := sig.Verify(pack.dnskey[0].(*dns.DNSKEY), keys); err != nil {
		t.Fatal(err)
	}
	if err := pack.www[1].(*dns.RRSIG).Verify(pack.dnskey[1].(*dns.DNSKEY), []dns.RR{pack.www[0]}); err != nil {
		t.Fatal(err)
	}
}
