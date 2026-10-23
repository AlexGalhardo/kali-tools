package network

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPRedirectTargetNotFollowed(t *testing.T) {
	var next atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { next.Add(1) }))
	t.Cleanup(other.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/landed", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL, TimeoutMS: 1000})
	if err != nil || result.HTTP == nil || result.HTTP.StatusCode != http.StatusFound || next.Load() != 0 {
		t.Fatalf("result=%+v err=%v hits=%d", result.HTTP, err, next.Load())
	}
	if result.HTTP.Redirect == nil || !result.HTTP.Redirect.Valid {
		t.Fatalf("redirect=%+v", result.HTTP.Redirect)
	}
}

func TestSecurityHeadersReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=1")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Add("X-Content-Type-Options", "nosniff")
		w.Header().Add("X-Content-Type-Options", "nosniff")
		w.Header().Set("Set-Cookie", "session=synthetic-token")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL, TimeoutMS: 1000})
	if err != nil || result.HTTP == nil || len(result.HTTP.SecurityHeaders) != len(securityHeaderNames) {
		t.Fatal(err, result.HTTP)
	}
	for i, name := range securityHeaderNames {
		if result.HTTP.SecurityHeaders[i].Name != name {
			t.Fatalf("order %d: %s", i, result.HTTP.SecurityHeaders[i].Name)
		}
	}
	hsts := result.HTTP.SecurityHeaders[0]
	if !hsts.Present || len(hsts.Notes) != 1 || hsts.Notes[0] != "ignored_over_http" {
		t.Fatalf("hsts=%+v", hsts)
	}
	xcto := result.HTTP.SecurityHeaders[3]
	if xcto.Value != "nosniff, nosniff" {
		t.Fatal(xcto.Value)
	}
	long := strings.Repeat("a", MaxHeaderValueBytes+1)
	if got := boundHeaderValue(long); len(got) <= MaxHeaderValueBytes || !strings.HasSuffix(got, " [truncated]") {
		t.Fatal(got)
	}
	if got := boundHeaderValue("a\x00b\t"); got != "a?b\t" {
		t.Fatal(got)
	}
}

func TestHTTPMetadataOmitsCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Set-Cookie", "session=synthetic-token")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL, TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil || strings.Contains(string(raw), "synthetic-token") || strings.Contains(string(raw), "Set-Cookie") {
		t.Fatal(err, string(raw))
	}
}

func TestHTTPPhases(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("x"))
	}))
	t.Cleanup(plain.Close)
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: plain.URL, TimeoutMS: 1000})
	if err != nil || result.HTTP == nil {
		t.Fatal(err)
	}
	p := result.HTTP.Phases
	if p.DNSMS != nil || p.TLSMS != nil || p.ConnectMS == nil || p.TTFBMS == nil || p.TotalMS < *p.TTFBMS {
		t.Fatalf("%+v", p)
	}
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("x"))
	}))
	t.Cleanup(tlsSrv.Close)
	t.Cleanup(func() { httpTransportHook = nil })
	httpTransportHook = func(timeout time.Duration) *http.Transport {
		base := tlsSrv.Client().Transport.(*http.Transport).Clone()
		base.MaxResponseHeaderBytes = 64 << 10
		base.Proxy = nil
		base.DisableKeepAlives = true
		base.TLSHandshakeTimeout = timeout
		return base
	}
	result, err = (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: tlsSrv.URL, TimeoutMS: 1000})
	if err != nil || result.HTTP == nil || result.HTTP.Phases.TLSMS == nil || result.HTTP.Phases.ConnectMS == nil {
		t.Fatalf("%+v %v", result.HTTP, err)
	}
}

func TestRedirectChainComplete(t *testing.T) {
	var hits atomic.Int32
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	mux.HandleFunc("/0", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, server.URL+"/1", http.StatusFound)
	})
	mux.HandleFunc("/1", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, server.URL+"/2", http.StatusFound)
	})
	mux.HandleFunc("/2", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, server.URL+"/done", http.StatusFound)
	})
	mux.HandleFunc("/done", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL + "/0", TimeoutMS: 1000, Redirects: 5})
	if err != nil || result.HTTP == nil || result.HTTP.ChainStatus != "complete" || len(result.HTTP.Chain) != 3 || hits.Load() != 4 {
		t.Fatalf("%+v hits=%d err=%v", result.HTTP, hits.Load(), err)
	}
	if result.HTTP.StatusCode != http.StatusNoContent || result.Values[0] != "204 No Content" {
		t.Fatal(result.Values, result.HTTP.StatusCode)
	}
}

func TestRedirectChainLoop(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, serverURL(r), http.StatusFound)
	}))
	t.Cleanup(server.Close)
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL, TimeoutMS: 1000, Redirects: 5})
	if err != nil || result.HTTP == nil || result.HTTP.ChainStatus != "loop" || hits.Load() != 1 {
		t.Fatalf("%+v hits=%d err=%v", result.HTTP, hits.Load(), err)
	}
}

func TestRedirectChainLimit(t *testing.T) {
	var hits atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		http.Redirect(w, r, server.URL+"/?n="+strconv.Itoa(int(n)), http.StatusFound)
	}))
	t.Cleanup(server.Close)
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL + "/", TimeoutMS: 1000, Redirects: 5})
	if err != nil || result.HTTP == nil || result.HTTP.ChainStatus != "limit" || len(result.HTTP.Chain) != 5 || hits.Load() != 6 {
		t.Fatalf("%+v hits=%d err=%v", result.HTTP, hits.Load(), err)
	}
}

func TestRedirectInvalidTarget(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Location", "javascript:alert(1)")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL, TimeoutMS: 1000, Redirects: 3})
	if err != nil || result.HTTP == nil || result.HTTP.ChainStatus != "invalid_target" || hits.Load() != 1 {
		t.Fatalf("%+v hits=%d err=%v", result.HTTP, hits.Load(), err)
	}
	if result.HTTP.Redirect == nil || result.HTTP.Redirect.Valid {
		t.Fatal(result.HTTP.Redirect)
	}
	base, parseErr := url.Parse(server.URL)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	got := redirectTarget(base, strings.Repeat("a", MaxLocationBytes+1))
	if got.Valid || !strings.Contains(strings.Join(got.Notes, " "), "location_truncated") || len(got.Location) != MaxLocationBytes {
		t.Fatalf("%+v", got)
	}
}

func TestRedirectsValidation(t *testing.T) {
	base := Request{Kind: "http", Target: "http://example.com", TimeoutMS: 100}
	cases := []struct {
		name string
		req  Request
		want string
	}{
		{"negative", func() Request { r := base; r.Redirects = -1; return r }(), "redirects must be 0..5"},
		{"zero", base, ""},
		{"max", func() Request { r := base; r.Redirects = 5; return r }(), ""},
		{"over", func() Request { r := base; r.Redirects = 6; return r }(), "redirects must be 0..5"},
		{"tcp", Request{Kind: "tcp", Target: "example.com", Port: 80, TimeoutMS: 100, Redirects: 1}, "redirects apply only to HTTP"},
	}
	for _, tc := range cases {
		err := Validate(tc.req)
		if tc.want == "" && err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func TestRedirectChainDeadline(t *testing.T) {
	var hits atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			http.Redirect(w, r, server.URL+"/hang", http.StatusFound)
			return
		}
		time.Sleep(2 * time.Second)
	}))
	t.Cleanup(server.Close)
	start := time.Now()
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: server.URL, TimeoutMS: 300, Redirects: 2})
	if time.Since(start) > time.Second || err == nil || result.HTTP == nil || result.HTTP.ChainStatus != "failed" || result.Status != "failed" {
		t.Fatalf("%+v err=%v elapsed=%s", result.HTTP, err, time.Since(start))
	}
	if len(result.Values) == 0 || result.Values[0] == "" {
		t.Fatal(result.Values)
	}
}

func TestRedirectFlags(t *testing.T) {
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "http://192.0.2.1/next")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(tlsSrv.Close)
	t.Cleanup(func() { httpTransportHook = nil })
	httpTransportHook = func(timeout time.Duration) *http.Transport {
		base := tlsSrv.Client().Transport.(*http.Transport).Clone()
		base.Proxy = nil
		base.DisableKeepAlives = true
		base.TLSHandshakeTimeout = timeout
		return base
	}
	result, err := (Diagnostics{}).Diagnose(t.Context(), Request{Kind: "http", Target: tlsSrv.URL, TimeoutMS: 1000})
	if err != nil || result.HTTP == nil || result.HTTP.Redirect == nil || !result.HTTP.Redirect.Downgrade || !result.HTTP.Redirect.CrossHost {
		t.Fatalf("%+v %v", result.HTTP, err)
	}
	if len(result.HTTP.Redirect.Notes) != 1 || result.HTTP.Redirect.Notes[0] != "downgrade" {
		t.Fatal(result.HTTP.Redirect.Notes)
	}
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host + r.URL.RequestURI()
}
