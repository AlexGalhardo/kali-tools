package tlsaudit

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"io"
	"log"
	"math/big"
	"net"
	"testing"
	"time"
)

func fixtureCertificate(t *testing.T, now time.Time) *x509.Certificate {
	cert, _ := fixtureTLSCertificate(t, now)
	return cert
}

func discardLogger() *log.Logger { return log.New(io.Discard, "", 0) }

func fixtureTLSCertificate(t *testing.T, now time.Time) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: "fixture.test"},
		NotBefore: now.Add(-24 * time.Hour), NotAfter: now.Add(90 * 24 * time.Hour),
		DNSNames: []string{"fixture.test"}, IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key
}

func sctExtension(raw []byte) pkix.Extension {
	return pkix.Extension{Id: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 4, 2}, Value: raw}
}
