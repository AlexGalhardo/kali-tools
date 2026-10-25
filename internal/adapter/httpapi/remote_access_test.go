package httpapi

import (
	"log/slog"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestAllowFromRequiresRemoteAndValidCIDR(t *testing.T) {
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("fixture")}}
	for _, opts := range []Options{
		{Addr: "127.0.0.1:0", Assets: assets, AllowFrom: []string{"10.0.0.0/24"}},
		{Addr: "127.0.0.1:0", Assets: assets, AllowRemote: true, AllowFrom: []string{"not-a-cidr"}},
	} {
		if s, err := New(testApp(t), opts); err == nil {
			_ = s.listener.Close()
			t.Fatalf("accepted %+v", opts.AllowFrom)
		}
	}
}

func TestAllowListenerFiltersAtAccept(t *testing.T) {
	prefixes, err := parseAllowFrom([]string{"127.0.0.0/8", "10.1.2.3/24"})
	if err != nil {
		t.Fatal(err)
	}
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	l := &allowListener{Listener: inner, allow: prefixes[:1], log: slog.New(slog.DiscardHandler)}
	defer func() { _ = l.Close() }()
	go func() {
		if c, err := net.Dial("tcp", inner.Addr().String()); err == nil {
			_ = c.Close()
		}
	}()
	conn, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()

	blocked := &allowListener{allow: prefixes[1:]}
	for addr, want := range map[string]bool{"10.1.2.200:1": true, "10.1.3.1:1": false, "127.0.0.1:1": false, "[::ffff:10.1.2.9]:1": true} {
		tcp, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		if got := blocked.permitted(tcp); got != want {
			t.Fatalf("%s permitted=%v want %v", addr, got, want)
		}
	}
}

func TestSessionExchangeLocksOutAfterFailures(t *testing.T) {
	s, logs := securedServer(t)
	now := time.Unix(1000, 0)
	s.session.failures.now = func() time.Time { return now }
	exchange := func(token, remote string) int {
		r := httptest.NewRequest("POST", "/api/session", strings.NewReader(`{"token":"`+token+`"}`))
		r.Host = s.Addr().String()
		r.RemoteAddr = remote
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w.Code
	}
	for range maxFailures {
		if code := exchange("wrong-token", "192.0.2.7:4000"); code != 401 {
			t.Fatal(code)
		}
	}
	if code := exchange(s.session.launch, "192.0.2.7:4001"); code != 429 {
		t.Fatalf("locked peer got %d", code)
	}
	if code := exchange(s.session.launch, "192.0.2.8:4000"); code != 204 {
		t.Fatalf("other peer got %d", code)
	}
	now = now.Add(failureWindow + time.Second)
	if code := exchange(s.session.launch, "192.0.2.7:4002"); code != 204 {
		t.Fatalf("lockout never expired: %d", code)
	}
	out := logs.String()
	if !strings.Contains(out, "session exchange rejected") || !strings.Contains(out, "remote=192.0.2.7:4000") || strings.Contains(out, s.session.launch) || strings.Contains(out, "wrong-token") {
		t.Fatal(out)
	}
}

func TestFailureTrackerIsBounded(t *testing.T) {
	tracker := &failureTracker{now: time.Now, seen: map[string]*failure{}}
	for i := range 3 * maxTrackedPeer {
		tracker.record(net.IPv4(10, byte(i>>16), byte(i>>8), byte(i)).String())
	}
	if len(tracker.seen) > maxTrackedPeer {
		t.Fatal(len(tracker.seen))
	}
}
