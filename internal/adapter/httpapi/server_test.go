package httpapi

import (
	"bufio"
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
	"sync"
	"testing"
	"testing/fstest"
	"testing/synctest"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
)

type pipeListener struct {
	connections chan net.Conn
	closed      chan struct{}
	once        sync.Once
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connections:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}
func (l *pipeListener) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *pipeListener) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345} }

func TestServerForcedShutdownBoundary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _ := securedServer(t)
		_ = s.listener.Close()
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()
		defer clientConn.Close()
		listener := &pipeListener{connections: make(chan net.Conn, 1), closed: make(chan struct{})}
		listener.connections <- serverConn
		s.listener = listener
		entered, release := make(chan struct{}), make(chan struct{})
		s.http.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			close(entered)
			<-release // model a handler that ignores cancellation
		})
		defer close(release)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- s.Run(ctx) }()
		go func() { _, _ = io.WriteString(clientConn, "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n") }()
		<-entered
		cancel()
		synctest.Wait()
		time.Sleep(5*time.Second - time.Nanosecond)
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("shutdown returned before five-second deadline: %v", err)
		default:
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		select {
		case err := <-done:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("shutdown error=%v", err)
			}
		default:
			t.Fatal("shutdown exceeded five-second deadline")
		}
		time.Sleep(time.Nanosecond)
		if _, err := clientConn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
			t.Fatalf("forced shutdown did not close connection: %v", err)
		}
	})
}

func TestServerProtocolLimits(t *testing.T) {
	s, _ := securedServer(t)
	if s.http.ReadHeaderTimeout != 5*time.Second || s.http.IdleTimeout != 120*time.Second {
		t.Fatal("header/idle timeout configuration differs from W04")
	}
	if s.http.MaxHeaderBytes != 16<<10 || s.http.WriteTimeout != 0 {
		t.Fatal("header limit or SSE-compatible write timeout differs from W04")
	}
}

func TestHeaderSizeOnWire(t *testing.T) {
	s, _ := securedServer(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})
	// net/http reserves another 4096 bytes for buffered header read-ahead.
	for _, size := range []int{maxHeaderBytes, maxHeaderBytes + 4096, maxHeaderBytes + 4096 + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			connection, err := net.DialTimeout("tcp", s.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			if err := connection.SetDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			prefix := "GET /healthz HTTP/1.1\r\nHost: " + s.Addr().String() + "\r\nX-Padding: "
			raw := prefix + strings.Repeat("a", size-len(prefix)-4) + "\r\n\r\n"
			if _, err := io.WriteString(connection, raw); err != nil {
				t.Fatal(err)
			}
			response, err := http.ReadResponse(bufio.NewReader(connection), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			want := 200
			if size > maxHeaderBytes+4096 {
				want = 431
			}
			if response.StatusCode != want {
				t.Fatalf("header bytes=%d: status=%d want=%d", size, response.StatusCode, want)
			}
		})
	}
}

func TestIdleConnectionTimeout(t *testing.T) {
	s, _ := securedServer(t)
	s.http.IdleTimeout = 200 * time.Millisecond
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})
	connection, err := net.DialTimeout("tcp", s.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(connection, "GET /healthz HTTP/1.1\r\nHost: %s\r\n\r\n", s.Addr()); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(connection)
	response, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if err != nil || response.StatusCode != 200 {
		t.Fatal(response.StatusCode, err)
	}
	if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
		t.Fatalf("idle connection was not closed: %v", err)
	}
}

func TestServerShutdownClosesSSE(t *testing.T) {
	s, err := New(testApp(t), Options{
		Addr:   "127.0.0.1:0",
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("fixture")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	requestCtx, stop := context.WithTimeout(t.Context(), 3*time.Second)
	defer stop()
	r, err := http.NewRequestWithContext(requestCtx, "GET", "http://"+s.Addr().String()+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+s.session.launch)
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	if err != nil || line != "retry: 3000\n" {
		t.Fatalf("SSE not established: %q %v", line, err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-requestCtx.Done():
		t.Fatal("server did not stop with an active SSE client")
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatal(err)
	}
	if len(s.subscribers) != 0 {
		t.Fatal("shutdown leaked SSE subscriber")
	}
}

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
