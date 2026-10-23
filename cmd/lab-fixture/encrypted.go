package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/miekg/dns"
)

var labCAPEM []byte

func labCertificate() (pemBytes []byte, cert tls.Certificate) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "kali-tools lab CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		log.Fatal(err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "fixture"},
		NotBefore:    caTemplate.NotBefore,
		NotAfter:     caTemplate.NotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"fixture"},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caTemplate, &leafKey.PublicKey, caKey)
	if err != nil {
		log.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), tls.Certificate{
		Certificate: [][]byte{leafDER, caDER},
		PrivateKey:  leafKey,
	}
}

func startEncrypted(pemBytes []byte, cert tls.Certificate) {
	labCAPEM = pemBytes
	config := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
	server := &http.Server{
		Handler:           http.HandlerFunc(dohHandler),
		TLSConfig:         config,
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      2 * time.Second,
	}
	go func() {
		listener, err := tls.Listen("tcp", ":8443", config)
		if err != nil {
			log.Fatal(err)
		}
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	dot := &dns.Server{
		Addr:      ":8853",
		Net:       "tcp-tls",
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}},
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
			if err := w.WriteMsg(answer(req, roleRecursive)); err != nil {
				log.Print("DoT fixture write failed")
			}
		}),
	}
	go func() {
		if err := dot.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()
	go silentUDP()
}

func dohHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/dns-query" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 512))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req := new(dns.Msg)
	if err := req.Unpack(body); err != nil || len(req.Question) != 1 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	packed, err := answer(req, roleRecursive).Pack()
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/dns-message")
	_, _ = w.Write(packed)
}

func silentUDP() {
	conn, err := net.ListenPacket("udp", ":5357")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	buf := make([]byte, 512)
	for {
		if _, _, err := conn.ReadFrom(buf); err != nil {
			return
		}
	}
}
