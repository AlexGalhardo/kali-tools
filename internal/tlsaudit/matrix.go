package tlsaudit

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"
)

func observationCiphers() []uint16 {
	out := []uint16{}
	for _, suite := range append(tls.CipherSuites(), tls.InsecureCipherSuites()...) {
		out = append(out, suite.ID)
	}
	return out
}

func (a auditor) protocolMatrix(ctx context.Context, r Request) []ProtocolResult {
	out := make([]ProtocolResult, 0, 4)
	for _, v := range []uint16{tls.VersionTLS10, tls.VersionTLS11, tls.VersionTLS12, tls.VersionTLS13} {
		p := ProtocolResult{Version: tls.VersionName(v), Status: "client_unsupported"}
		if !clientSupports(v) {
			out = append(out, p)
			continue
		}
		state, err := a.observe(ctx, r, v)
		p.Status = "error"
		switch {
		case err == nil:
			p.Status, p.Cipher = "accepted", tls.CipherSuiteName(state.CipherSuite)
		case errors.Is(err, context.DeadlineExceeded):
			p.Status = "timeout"
		case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF), strings.Contains(err.Error(), "remote error: tls:"):
			p.Status = "rejected"
		default:
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				p.Status = "timeout"
			}
		}
		out = append(out, p)
		if ctx.Err() != nil {
			break
		}
	}
	return out
}

var supportedVersions = sync.OnceValue(func() map[uint16]bool {
	result := map[uint16]bool{}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return result
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return result
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	for _, v := range []uint16{tls.VersionTLS10, tls.VersionTLS11, tls.VersionTLS12, tls.VersionTLS13} {
		result[v] = selfTestVersion(v, cert)
	}
	return result
})

func clientSupports(v uint16) bool { return supportedVersions()[v] }

func selfTestVersion(version uint16, cert tls.Certificate) bool {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	deadline := time.Now().Add(2 * time.Second)
	if err := left.SetDeadline(deadline); err != nil {
		return false
	}
	if err := right.SetDeadline(deadline); err != nil {
		return false
	}
	server := tls.Server(left, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: version, MaxVersion: version, CipherSuites: observationCiphers()})
	client := tls.Client(right, &tls.Config{MinVersion: version, MaxVersion: version, InsecureSkipVerify: true, CipherSuites: observationCiphers()}) //nolint:gosec // ephemeral in-process capability self-test (B08)
	done := make(chan error, 1)
	go func() { done <- server.Handshake() }()
	err := client.Handshake()
	if err != nil {
		right.Close()
	}
	serverErr := <-done
	return err == nil && serverErr == nil
}
