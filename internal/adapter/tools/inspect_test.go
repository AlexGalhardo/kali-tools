package tools

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSupportedToolVersions(t *testing.T) {
	for _, tt := range []struct {
		line string
		want bool
	}{
		{"TShark (Wireshark) 4.6.0", true},
		{"TShark (Wireshark) 4.2.0", true},
		{"TShark (Wireshark) 4.1.0", false},
		{"TShark (Wireshark) 5.0.0", false},
		{"arbitrary output", false},
	} {
		if got := supportedVersion("tshark", tt.line); got != tt.want {
			t.Fatalf("%q: %v", tt.line, got)
		}
	}
	if supportedVersion("nmap", "Nmap version 7.98") {
		t.Fatal("nmap is not an inventoried tool")
	}
}

func TestClassifyBanner(t *testing.T) {
	long := "SSH-2.0-OpenSSH_9.6 " + strings.Repeat("x", 300)
	for _, tt := range []struct {
		id, name, product, version, raw string
		ok                              bool
	}{
		{id: "ssh", name: "ssh", product: "OpenSSH", version: "9.6", raw: "SSH-2.0-OpenSSH_9.6\r\n", ok: true},
		{id: "ssh-tail", name: "ssh", product: "OpenSSH", version: "9.6", raw: long, ok: true},
		{id: "ftp", name: "ftp", raw: "220 (vsFTPd 3.0.5)\r\n", ok: true},
		{id: "smtp", name: "smtp", raw: "220 mail.example ESMTP ready\r\n", ok: true},
		{id: "imap", name: "imap", raw: "* OK Dovecot ready\r\n", ok: true},
		{id: "pop3", name: "pop3", raw: "+OK ready\r\n", ok: true},
		{id: "vnc", name: "vnc", version: "003.008", raw: "RFB 003.008\n", ok: true},
		{id: "http", name: "http", product: "nginx", version: "1.25.0", raw: "HTTP/1.1 200 OK\r\nServer: nginx/1.25.0\r\n\r\n<body>secret</body>", ok: true},
		{id: "ambiguous", raw: "220 ambiguous greeting\r\n"},
		{id: "binary", raw: "\x16\x03\x01garbage"},
	} {
		t.Run(tt.id, func(t *testing.T) {
			name, product, version, ok := classifyBanner([]byte(tt.raw))
			if name != tt.name || product != tt.product || version != tt.version || ok != tt.ok {
				t.Fatalf("got %q %q %q %v", name, product, version, ok)
			}
			if strings.Contains(product+version, "secret") || strings.Contains(product+version, "body") {
				t.Fatal("probe content retained")
			}
		})
	}
	mysql := []byte{0x00, 0x00, 0x00, 0x00, 10, '5', '.', '7', '.', '4', 0, 1}
	name, _, version, ok := classifyBanner(mysql)
	if !ok || name != "mysql" || version != "5.7.4" {
		t.Fatalf("mysql: %s %s %v", name, version, ok)
	}
}

func TestClassifyBannerCapsLabel(t *testing.T) {
	_, product, _, ok := classifyBanner([]byte("SSH-2.0-" + strings.Repeat("A", 80) + "\r\n"))
	if !ok || len(product) != labelLimit {
		t.Fatalf("product %q", product)
	}
}

func FuzzClassifyBanner(f *testing.F) {
	f.Add([]byte("SSH-2.0-OpenSSH_9.6\r\n"))
	f.Add([]byte("HTTP/1.0 200 OK\r\nServer: synthetic\r\n"))
	f.Add([]byte{0, 0, 0, 0, 10, '8', 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		name, product, version, _ := classifyBanner(data)
		if len(name) > labelLimit || len(product) > labelLimit || len(version) > labelLimit {
			t.Fatalf("label exceeded: %q %q %q", name, product, version)
		}
	})
}

func TestInspectServicesIdentifiesLoopback(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "synthetic/1")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(httpServer.Close)
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(tlsServer.Close)
	ssh, sshPort := serveBanner(t, "SSH-2.0-OpenSSH_9.6\r\n")
	t.Cleanup(func() { _ = ssh.Close() })
	unknown, unknownPort := serveBanner(t, "")
	t.Cleanup(func() { _ = unknown.Close() })
	closed := closedPort(t)
	var dials atomic.Int32
	decoy, decoyPort := serveCount(t, &dials)
	t.Cleanup(func() { _ = decoy.Close() })

	if _, err := InspectServices(t.Context(), "10.0.0.0/8", []int{decoyPort}); err == nil || dials.Load() != 0 {
		t.Fatalf("scope dials=%d err=%v", dials.Load(), err)
	}
	result, err := InspectServices(t.Context(), "127.0.0.1/32", []int{portOf(t, httpServer.URL), portOf(t, tlsServer.URL), sshPort, unknownPort, closed})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanner != "native" || result.Version != "1" || result.Finished.Exit != "success" || len(result.Hosts) != 1 {
		t.Fatalf("%+v", result)
	}
	got := map[int]appService{}
	for _, port := range result.Hosts[0].Ports {
		if result.Hosts[0].Addresses[0].Address != "127.0.0.1" {
			t.Fatal(result.Hosts[0].Addresses)
		}
		got[port.Port] = appService{port.State.State, port.Service.Name, port.Service.Product, port.Service.Version, port.Service.Method}
	}
	if got[portOf(t, httpServer.URL)] != (appService{"open", "http", "synthetic", "1", "http"}) {
		t.Fatalf("http %+v", got[portOf(t, httpServer.URL)])
	}
	tls := got[portOf(t, tlsServer.URL)]
	if tls.state != "open" || (tls.name != "https" && tls.name != "tls") || tls.method != "tls" || tls.version == "" {
		t.Fatalf("tls %+v", tls)
	}
	if got[sshPort] != (appService{"open", "ssh", "OpenSSH", "9.6", "banner"}) {
		t.Fatalf("ssh %+v", got[sshPort])
	}
	if got[closed].state != "unreachable" || got[closed].name != "" {
		t.Fatalf("closed %+v", got[closed])
	}
	if got[unknownPort] != (appService{"open", "", "", "", ""}) {
		t.Fatalf("unknown %+v", got[unknownPort])
	}
}

type appService struct {
	state, name, product, version, method string
}

func serveBanner(t *testing.T, banner string) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte(banner))
			_ = conn.Close()
		}
	}()
	return ln, portOf(t, "http://"+ln.Addr().String())
}

func serveCount(t *testing.T, dials *atomic.Int32) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			dials.Add(1)
			_ = conn.Close()
		}
	}()
	return ln, portOf(t, "http://"+ln.Addr().String())
}

func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := portOf(t, "http://"+ln.Addr().String())
	_ = ln.Close()
	return port
}

func portOf(t *testing.T, rawURL string) int {
	t.Helper()
	_, port, err := net.SplitHostPort(trimScheme(rawURL))
	if err != nil {
		t.Fatal(err)
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return number
}

func trimScheme(rawURL string) string {
	return strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://")
}
