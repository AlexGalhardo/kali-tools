package network

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTLSTrustAndSummary(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	address := server.Listener.Addr().(*net.TCPAddr)
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	values, err := diagnoseTLS(t.Context(), address.IP.String(), address.Port, roots)
	if err != nil || len(values) != 1 || !strings.Contains(values[0], "issuer=") || !strings.Contains(values[0], server.Certificate().NotAfter.UTC().Format(time.RFC3339)) {
		t.Fatalf("summary %v, error %v", values, err)
	}
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "tls", Target: address.IP.String(), Port: address.Port, TimeoutMS: 1000})
	if err == nil || result.Status != "failed" || result.ErrorCode != "certificate_invalid" || len(result.Values) != 0 {
		t.Fatalf("untrusted certificate accepted: %+v, %v", result, err)
	}
}

func TestTLSHandshakeDeadlineAndCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := listener.Accept()
		if err == nil {
			accepted <- c
		}
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = diagnoseTLS(ctx, "127.0.0.1", listener.Addr().(*net.TCPAddr).Port, nil)
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("unbounded handshake: %v", err)
	}
	select {
	case c := <-accepted:
		c.Close()
	case <-time.After(time.Second):
		t.Fatal("server did not accept")
	}
	ctx2, cancel2 := context.WithCancel(t.Context())
	cancel2()
	_, err = diagnoseTLS(ctx2, "127.0.0.1", listener.Addr().(*net.TCPAddr).Port, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestTLSRejectsExpiredTrustedCertificate(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Expired fixture"}, NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: time.Now().Add(-time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{{Certificate: [][]byte{raw}, PrivateKey: key}}}
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	address := server.Listener.Addr().(*net.TCPAddr)
	_, err = diagnoseTLS(t.Context(), address.IP.String(), address.Port, roots)
	var invalid x509.CertificateInvalidError
	if !errors.As(err, &invalid) || invalid.Reason != x509.Expired || diagnosticErrorCode(err) != "certificate_invalid" {
		t.Fatalf("expected expiration failure: %v", err)
	}
}
