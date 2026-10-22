package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
)

func testCertificate(t *testing.T) (string, string, string) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	pair := server.TLS.Certificates[0]
	key, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(pair.Certificate[0])
	parts := make([]string, len(sum))
	for i, value := range sum {
		parts[i] = fmt.Sprintf("%02X", value)
	}
	return certPath, keyPath, strings.Join(parts, ":")
}

func TestRemoteRequiresOptInAndTLS(t *testing.T) {
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("fixture")}}
	cert, key, _ := testCertificate(t)
	for _, test := range []struct {
		opts Options
		code string
	}{
		{Options{Addr: "invalid"}, "invalid_input"}, {Options{Addr: ":0"}, "invalid_input"}, {Options{Addr: "0.0.0.0:0"}, "invalid_input"}, {Options{Addr: "[::]:0"}, "invalid_input"},
		{Options{Addr: "127.0.0.1:-1"}, "invalid_input"}, {Options{Addr: "127.0.0.1:65536"}, "invalid_input"}, {Options{Addr: "127.0.0.1:abc"}, "invalid_input"},
		{Options{Addr: "192.0.2.10:0"}, "invalid_input"}, {Options{Addr: "192.0.2.10:0", AllowRemote: true}, "invalid_input"},
		{Options{Addr: "127.0.0.1:0", TLSCert: cert}, "invalid_input"}, {Options{Addr: "127.0.0.1:0", TLSKey: key}, "invalid_input"},
		{Options{Addr: "127.0.0.1:0", TLSCert: "missing", TLSKey: "missing"}, "invalid_input"},
		{Options{Addr: "192.0.2.10:0", AllowRemote: true, TLSCert: cert, TLSKey: key, Assets: assets}, "failed"},
		{Options{Addr: "127.0.0.1:0"}, "dependency_missing"}, {Options{Addr: "127.0.0.1:0", Assets: fstest.MapFS{}}, "dependency_missing"},
	} {
		server, err := New(testApp(t), test.opts)
		if err == nil {
			if server != nil {
				_ = server.listener.Close()
			}
			t.Fatal("invalid server options accepted", test.opts.Addr)
		}
		if got := contract.FromError(err).Code; got != test.code {
			t.Fatalf("%s: code=%s want=%s err=%v", test.opts.Addr, got, test.code, err)
		}
	}
}

func TestServerRunShutdownAndTLS(t *testing.T) {
	cert, key, fingerprint := testCertificate(t)
	for _, secure := range []bool{false, true} {
		opts := Options{Addr: "localhost:0", Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("fixture")}}}
		if secure {
			opts.TLSCert, opts.TLSKey = cert, key
		}
		s, err := New(testApp(t), opts)
		if err != nil {
			t.Fatal(err)
		}
		if secure && s.CertFingerprint() != fingerprint || !secure && s.CertFingerprint() != "" {
			t.Fatal(s.CertFingerprint())
		}
		launch, err := url.Parse(s.URL())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(launch.Fragment, "token=") || len(strings.TrimPrefix(launch.Fragment, "token=")) != 43 {
			t.Fatal("invalid launch URL")
		}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- s.Run(ctx) }()
		client := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		httpClient := client.Client()
		client.Close()
		httpClient.Timeout = time.Second
		response, err := httpClient.Get(launch.Scheme + "://" + launch.Host + "/healthz")
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != 200 {
			cancel()
			t.Fatal(response.StatusCode)
		}
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("server did not stop")
		}
		if err := s.Run(t.Context()); err == nil {
			t.Fatal("server ran twice")
		}
	}
}

func TestAssetsServedByteIdentical(t *testing.T) {
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>fixture</html>")}, "assets/app.js": &fstest.MapFile{Data: []byte("export const fixture=true;")}}
	s, err := New(testApp(t), Options{Addr: "127.0.0.1:0", Assets: assets})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.listener.Close() }()
	for path, value := range assets {
		w := serveRequest(s, "GET", "/"+path, "", false)
		if path == "index.html" {
			w = serveRequest(s, "GET", "/", "", false)
		}
		if w.Code != 200 || w.Body.String() != string(value.Data) {
			t.Fatalf("%s: status=%d body=%s", path, w.Code, w.Body)
		}
	}
}

func TestReadHeaderTimeout(t *testing.T) {
	s, _ := securedServer(t)
	s.http.ReadHeaderTimeout = 200 * time.Millisecond
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	connection, err := net.DialTimeout("tcp", s.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(connection, "G"); err != nil {
		t.Fatal(err)
	}
	response, err := io.ReadAll(io.LimitReader(connection, 4096))
	if len(response) > 0 && !strings.HasPrefix(string(response), "HTTP/1.1 400") && !strings.HasPrefix(string(response), "HTTP/1.1 408") {
		t.Fatal("incomplete headers accepted", string(response))
	}
	var failure net.Error
	if errors.As(err, &failure) && failure.Timeout() {
		t.Fatal("header timeout was not enforced")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}
