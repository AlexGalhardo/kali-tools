package network

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"time"
)

func diagnoseTLS(ctx context.Context, host string, port int, roots *x509.CertPool) ([]string, error) {
	serverName := host
	if ip, err := netip.ParseAddr(host); err == nil {
		serverName = ip.WithZone("").String()
	}
	c, err := (&tls.Dialer{NetDialer: &net.Dialer{}, Config: &tls.Config{
		MinVersion: tls.VersionTLS12, ServerName: serverName, RootCAs: roots,
	}}).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	defer c.Close()
	state := c.(*tls.Conn).ConnectionState()
	values := []string{}
	for _, cert := range state.PeerCertificates[:min(len(state.PeerCertificates), 16)] {
		subject, issuer := cert.Subject.String(), cert.Issuer.String()
		values = append(values, fmt.Sprintf("subject=%.1024s issuer=%.1024s expires=%s", subject, issuer, cert.NotAfter.UTC().Format(time.RFC3339)))
	}
	return values, nil
}
