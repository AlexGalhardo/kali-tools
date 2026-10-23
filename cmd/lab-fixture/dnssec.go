package main

import (
	"crypto"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/miekg/dns"
)

// Seeds are fixed so the lab DS does not change between starts.
var (
	labKSKSeed = bytesRepeat(0x11)
	labZSKSeed = bytesRepeat(0x22)
)

// pinnedSignedDS is the DS the integration test sends as a trust anchor.
const pinnedSignedDS = "signed.test. IN DS 38590 15 2 b3d09e28ce56fd36df2da483c42ee8de3987d26747f553a6f15baa5a760485e4"

type signedPack struct {
	soa    *dns.SOA
	ns     []dns.RR
	dnskey []dns.RR
	www    []dns.RR
}

func bytesRepeat(b byte) []byte {
	out := make([]byte, ed25519.SeedSize)
	for i := range out {
		out[i] = b
	}
	return out
}

func keyFromSeed(name string, flags uint16, seed []byte) (*dns.DNSKEY, crypto.Signer) {
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	key := &dns.DNSKEY{
		Hdr:       dns.RR_Header{Name: name, Rrtype: dns.TypeDNSKEY, Class: dns.ClassINET, Ttl: 3600},
		Flags:     flags,
		Protocol:  3,
		Algorithm: dns.ED25519,
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}
	return key, priv
}

func signedDSText() string {
	ksk, _ := keyFromSeed(dns.Fqdn("signed.test"), 257, labKSKSeed)
	ds := ksk.ToDS(dns.SHA256)
	return fmt.Sprintf("%s IN DS %d %d %d %s", ds.Hdr.Name, ds.KeyTag, ds.Algorithm, ds.DigestType, ds.Digest)
}

func buildSigned(now time.Time) signedPack {
	name := dns.Fqdn("signed.test")
	if got := signedDSText(); got != pinnedSignedDS {
		log.Fatalf("lab DS %s does not match the pinned anchor", got)
	}
	ksk, kskPriv := keyFromSeed(name, 257, labKSKSeed)
	zsk, zskPriv := keyFromSeed(name, 256, labZSKSeed)
	inception := uint32(now.Add(-time.Hour).Unix())
	expiration := uint32(now.Add(24 * time.Hour).Unix())
	dnskeySig := signRR(name, kskPriv, ksk.KeyTag(), inception, expiration, []dns.RR{ksk, zsk})
	address := &dns.A{
		Hdr: dns.RR_Header{Name: dns.Fqdn("www.signed.test"), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   []byte{192, 0, 2, 10},
	}
	answerSig := signRR(name, zskPriv, zsk.KeyTag(), inception, expiration, []dns.RR{address})
	ns1 := dns.Fqdn("ns1.signed.test")
	ns2 := dns.Fqdn("ns2.signed.test")
	return signedPack{
		soa:    soaRR(name, ns1, 100),
		ns:     []dns.RR{nsRR(name, ns1), nsRR(name, ns2)},
		dnskey: []dns.RR{ksk, zsk, dnskeySig},
		www:    []dns.RR{address, answerSig},
	}
}

func signRR(signerName string, key crypto.Signer, tag uint16, inception, expiration uint32, rrset []dns.RR) *dns.RRSIG {
	sig := &dns.RRSIG{
		Hdr:         dns.RR_Header{Name: rrset[0].Header().Name, Rrtype: dns.TypeRRSIG, Class: dns.ClassINET, Ttl: 3600},
		TypeCovered: rrset[0].Header().Rrtype,
		Algorithm:   dns.ED25519,
		Labels:      uint8(dns.CountLabel(rrset[0].Header().Name)),
		OrigTtl:     rrset[0].Header().Ttl,
		Expiration:  expiration,
		Inception:   inception,
		KeyTag:      tag,
		SignerName:  signerName,
	}
	if err := sig.Sign(key, rrset); err != nil {
		log.Fatal(err)
	}
	return sig
}
