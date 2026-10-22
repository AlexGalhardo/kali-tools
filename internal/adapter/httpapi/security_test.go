package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func securedServer(t *testing.T) (*Server, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	s, err := New(testApp(t), Options{Addr: "127.0.0.1:0", Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>fixture</html>")}}, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.listener.Close() })
	return s, &logs
}

func serveRequest(s *Server, method, path, body string, authorize bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = s.Addr().String()
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if authorize {
		r.Header.Set("Authorization", "Bearer "+s.session.launch)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestForeignHostRejected(t *testing.T) {
	s, _ := securedServer(t)
	_, port, err := net.SplitHostPort(s.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"evil.example:" + port, "127.0.0.1:1", "", "localhost"} {
		for _, path := range []string{"/", "/healthz", "/api/capabilities"} {
			r := httptest.NewRequest("GET", path, nil)
			r.Host = host
			r.Header.Set("Authorization", "Bearer "+s.session.launch)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("host=%q path=%s status=%d", host, path, w.Code)
			}
		}
	}
}

func TestAllowedHosts(t *testing.T) {
	s, _ := securedServer(t)
	_, port, err := net.SplitHostPort(s.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"127.0.0.1:" + port, "localhost:" + port, "[::1]:" + port, "LOCALHOST:" + port} {
		r := httptest.NewRequest("GET", "/healthz", nil)
		r.Host = host
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(host, w.Code, w.Body)
		}
	}
}

func TestCrossOriginPostRejected(t *testing.T) {
	s, _ := securedServer(t)
	for _, headers := range []map[string]string{{"Sec-Fetch-Site": "cross-site"}, {"Origin": "https://evil.example"}, {"Sec-Fetch-Site": "same-site"}} {
		r := httptest.NewRequest("POST", "/api/diagnostics", strings.NewReader(`{"kind":"tcp","target":"127.0.0.1","port":9,"timeout_ms":100}`))
		r.Host = s.Addr().String()
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+s.session.launch)
		for key, value := range headers {
			r.Header.Set(key, value)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal(w.Code, w.Body)
		}
	}
	history, err := s.app.History(t.Context())
	if err != nil || len(history) != 0 {
		t.Fatal(history, err)
	}
}

func TestCrossSiteFetchRejected(t *testing.T) {
	s, _ := securedServer(t)
	r := httptest.NewRequest("GET", "/api/history", nil)
	r.Host = s.Addr().String()
	r.Header.Set("Authorization", "Bearer "+s.session.launch)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
}

func routePatterns(t *testing.T) []string {
	t.Helper()
	source, err := parser.ParseFile(token.NewFileSet(), "routes.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	patterns := []string{}
	ast.Inspect(source, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Handle" {
			return true
		}
		value, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return true
		}
		pattern, err := strconv.Unquote(value.Value)
		if err != nil {
			t.Fatal(err)
		}
		patterns = append(patterns, pattern)
		return true
	})
	return patterns
}

func TestMissingOrInvalidTokenRejected(t *testing.T) {
	s, _ := securedServer(t)
	for _, pattern := range routePatterns(t) {
		parts := strings.SplitN(pattern, " ", 2)
		if !strings.HasPrefix(parts[1], "/api/") || pattern == "POST /api/session" {
			continue
		}
		for _, credential := range []string{"none", "cookie", "bearer"} {
			r := httptest.NewRequest(parts[0], strings.ReplaceAll(parts[1], "{id}", "0123456789abcdef0123456789abcdef"), nil)
			r.Host = s.Addr().String()
			if credential == "cookie" {
				r.AddCookie(&http.Cookie{Name: s.session.name, Value: "invalid"})
			}
			if credential == "bearer" {
				r.Header.Set("Authorization", "Bearer invalid")
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != 401 {
				t.Fatalf("%s %s: %d %s", pattern, credential, w.Code, w.Body)
			}
		}
	}
	if len(s.subscribers) != 0 {
		t.Fatal("unauthorized caller subscribed")
	}
}

func TestEventsRequireSession(t *testing.T) {
	s, _ := securedServer(t)
	w := serveRequest(s, "GET", "/api/events", "", false)
	if w.Code != 401 || len(s.subscribers) != 0 {
		t.Fatal(w.Code, w.Body)
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	s, _ := securedServer(t)
	for _, test := range []struct {
		method, path, body string
		auth               bool
		status             int
	}{
		{"GET", "/healthz", "", false, 200}, {"GET", "/api/history", "", false, 401}, {"GET", "/missing", "", false, 404},
		{"PUT", "/api/settings", `{}` + strings.Repeat(" ", maxJSONBody), true, 413}, {"OPTIONS", "/api/settings", "", true, 405},
	} {
		w := serveRequest(s, test.method, test.path, test.body, test.auth)
		if w.Code != test.status {
			t.Fatal(w.Code, w.Body)
		}
		for key, want := range map[string]string{"Content-Security-Policy": contentSecurityPolicy, "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer", "X-Frame-Options": "DENY", "Cross-Origin-Opener-Policy": "same-origin", "Cross-Origin-Resource-Policy": "same-origin", "Permissions-Policy": "camera=(), microphone=(), geolocation=()"} {
			if w.Header().Get(key) != want {
				t.Fatalf("status=%d header %s=%q", w.Code, key, w.Header().Get(key))
			}
		}
		for key := range w.Header() {
			if strings.HasPrefix(key, "Access-Control-") {
				t.Fatal("unexpected CORS", w.Header())
			}
		}
	}
}

func TestSSESubscriberCap(t *testing.T) {
	s, _ := securedServer(t)
	for range maxSSESubscribers {
		s.subscribers <- struct{}{}
	}
	w := serveRequest(s, "GET", "/api/events", "", true)
	if w.Code != 429 {
		t.Fatal(w.Code, w.Body)
	}
	<-s.subscribers
	w = serveRequest(s, "GET", "/api/events", "", true)
	if w.Code != 200 || len(s.subscribers) != maxSSESubscribers-1 {
		t.Fatal(w.Code, len(s.subscribers))
	}
}

func TestInflightCap(t *testing.T) {
	s, _ := securedServer(t)
	for range maxInflight {
		s.inflight <- struct{}{}
	}
	w := serveRequest(s, "GET", "/api/history", "", true)
	if w.Code != 429 {
		t.Fatal(w.Code, w.Body)
	}
	<-s.inflight
	w = serveRequest(s, "GET", "/api/history", "", true)
	if w.Code != 200 || len(s.inflight) != maxInflight-1 {
		t.Fatal(w.Code, len(s.inflight))
	}
}

func TestSSEConcurrentClientsAtLimit(t *testing.T) {
	s := &Server{app: testApp(t), subscribers: make(chan struct{}, maxSSESubscribers), keepAlive: sseKeepAlive}
	finished := make(chan struct{}, maxSSESubscribers+2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { finished <- struct{}{} }()
		s.handleEvents()(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	open := func() (*http.Response, context.CancelFunc) {
		t.Helper()
		stream, stop := context.WithCancel(ctx)
		r, err := http.NewRequestWithContext(stream, "GET", server.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := server.Client().Do(r)
		if err != nil {
			stop()
			t.Fatal(err)
		}
		t.Cleanup(func() { stop(); _ = response.Body.Close() })
		return response, stop
	}
	var first context.CancelFunc
	for i := range maxSSESubscribers {
		response, stop := open()
		defer response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatalf("stream %d: %d", i, response.StatusCode)
		}
		line, err := bufio.NewReader(response.Body).ReadString('\n')
		if err != nil || line != "retry: 3000\n" {
			t.Fatalf("stream %d not established: %q %v", i, line, err)
		}
		if i == 0 {
			first = stop
		}
	}
	response, stop := open()
	defer response.Body.Close()
	if response.StatusCode != 429 {
		t.Fatalf("ninth stream: %d", response.StatusCode)
	}
	stop()
	select {
	case <-finished: // rejected ninth stream
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	first()
	select {
	case <-finished: // cancelled first stream released its slot
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	response, stop = open()
	defer response.Body.Close()
	defer stop()
	if response.StatusCode != 200 {
		t.Fatalf("replacement stream: %d", response.StatusCode)
	}
}

func TestInflightConcurrentRequestsAtLimit(t *testing.T) {
	s := &Server{inflight: make(chan struct{}, maxInflight)}
	entered := make(chan struct{}, maxInflight+1)
	release := make(chan struct{})
	handler := s.limitRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	done := make(chan int, maxInflight)
	for range maxInflight {
		go func() {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequestWithContext(ctx, "GET", "/api/history", nil))
			done <- w.Code
		}()
	}
	for range maxInflight {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("requests did not reach handler")
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/history", nil))
	if w.Code != 429 {
		t.Fatalf("seventeenth request: %d", w.Code)
	}
	close(release)
	for range maxInflight {
		select {
		case code := <-done:
			if code != 204 {
				t.Errorf("admitted request: %d", code)
			}
		case <-ctx.Done():
			t.Fatal("requests did not stop")
		}
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/history", nil))
	if w.Code != 204 || len(s.inflight) != 0 {
		t.Fatal("request slots were not released")
	}
}
