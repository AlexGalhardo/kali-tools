package network

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDiagnosticsFixtures(t *testing.T) {
	var redirected atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Store(true) }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer server.Close()
	result, e := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL, TimeoutMS: 1000})
	if e != nil || len(result.Values) != 2 || result.Values[0] != "302 Found" {
		t.Fatalf("%+v %v", result, e)
	}
	if redirected.Load() {
		t.Fatal("followed redirect outside scope")
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if _, e = (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "tcp", Target: "127.0.0.1", Port: port, TimeoutMS: 500}); e != nil {
		t.Fatal(e)
	}
	listener.Close()
	if _, e = (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "tcp", Target: "127.0.0.1", Port: port, TimeoutMS: 500}); e == nil {
		t.Fatal("closed listener succeeded")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, e = (Diagnostics{}).Diagnose(ctx, Request{Kind: "tcp", Target: "127.0.0.1", Port: port, TimeoutMS: 500}); e == nil {
		t.Fatal("cancelled diagnostic succeeded")
	}
}
func TestHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	start := time.Now()
	if _, e := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL, TimeoutMS: 100}); e == nil {
		t.Fatal("expected timeout")
	}
	if time.Since(start) > time.Second {
		t.Fatal("timeout not bounded")
	}
}
func TestDiscoveryScopeAndLimits(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	items, e := Discover(t.Context(), "127.0.0.1/32", []int{port}, 100*time.Millisecond)
	if e != nil || len(items) != 1 || items[0].Host != "127.0.0.1" || items[0].State != "open" {
		t.Fatalf("%+v %v", items, e)
	}
	for _, cidr := range []string{"0.0.0.0/0", "192.168.0.0/16", "invalid", "::/128"} {
		if _, e := Discover(t.Context(), cidr, []int{port}, time.Second); e == nil {
			t.Fatal("accepted", cidr)
		}
	}
	if _, e := Discover(t.Context(), "127.0.0.1/32", []int{port, port}, time.Second); e == nil {
		t.Fatal("duplicate ports")
	}
}
func TestCounterResetAndSuspend(t *testing.T) {
	now := time.Now()
	for _, c := range []struct {
		before, after Counter
		duration      time.Duration
		rx, tx        float64
	}{{Counter{"a", 100, 50}, Counter{"a", 200, 100}, time.Second, 100, 50}, {Counter{"a", 100, 50}, Counter{"a", 1, 2}, time.Second, 0, 0}, {Counter{"a", 1, 2}, Counter{"b", 9, 10}, time.Second, 0, 0}, {Counter{"a", 1, 2}, Counter{"a", 10000, 20000}, time.Hour, 0, 0}} {
		got := Delta(c.before, c.after, c.duration, now)
		if got.ReceiveBPS != c.rx || got.SendBPS != c.tx {
			t.Fatal(got)
		}
	}
}
func TestTargetValidation(t *testing.T) {
	for _, host := range []string{"x; whoami", "-x", "http://x", "a\nb", "a..b", "user@example.com"} {
		if ValidateHost(host) == nil {
			t.Fatal("accepted", strconv.Quote(host))
		}
	}
	for _, host := range []string{"localhost", "example.com", "::1", "fe80::1%3", "127.0.0.1", "_dmarc.example.com"} {
		if e := ValidateHost(host); e != nil {
			t.Fatal(host, e)
		}
	}
}

func TestHTTPHeaderAndPortBounds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Fixture", strings.Repeat("x", 70<<10))
		w.WriteHeader(200)
	}))
	defer server.Close()
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL, TimeoutMS: 1000})
	if err == nil || result.Status != "failed" {
		t.Fatalf("oversized header accepted: %+v %v", result, err)
	}
	for _, target := range []string{"http://localhost:0", "http://localhost:65536", "http://localhost:99999999999999999999"} {
		if Validate(Request{Kind: "http", Target: target, TimeoutMS: 100}) == nil {
			t.Fatal("invalid HTTP port accepted")
		}
	}
}

func TestIPv6TCPFixture(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 loopback is unavailable on this host")
	}
	defer listener.Close()
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "tcp", Target: "::1", Port: listener.Addr().(*net.TCPAddr).Port, TimeoutMS: 1000})
	if err != nil || result.Status != "completed" || len(result.Values) != 1 {
		t.Fatalf("IPv6 TCP: %+v %v", result, err)
	}
}

func TestICMPWithoutProberIsUnsupported(t *testing.T) {
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "ping", Target: "127.0.0.1", TimeoutMS: 500})
	if err == nil || result.ErrorCode != "unsupported" || result.Status != "failed" {
		t.Fatal(result, err)
	}
}

type fakeProber struct{ trace bool }

func (fakeProber) Provider() string { return "fake" }
func (f *fakeProber) Run(_ context.Context, _ string, trace bool) ([]string, error) {
	f.trace = trace
	return []string{"probe=1"}, nil
}

func TestICMPUsesInjectedProber(t *testing.T) {
	prober := &fakeProber{}
	result, err := (Diagnostics{Prober: prober}).Diagnose(t.Context(), Request{Kind: "traceroute", Target: "127.0.0.1", TimeoutMS: 500})
	if err != nil || result.Provider != "fake" || !prober.trace || len(result.Values) != 1 {
		t.Fatal(result, err)
	}
}
