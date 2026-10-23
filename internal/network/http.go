package network

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	MaxRedirectHops     = 5
	MaxHeaderValueBytes = 256
	MaxLocationBytes    = 8192
	maxBodyBytes        = 65536
)

// httpTransportHook is test-only. Production leaves it nil.
var httpTransportHook func(timeout time.Duration) *http.Transport

type HTTPMetadata struct {
	StatusCode      int              `json:"status_code,omitempty"`
	FinalURL        string           `json:"final_url,omitempty"`
	Phases          HTTPPhases       `json:"phases"`
	SecurityHeaders []SecurityHeader `json:"security_headers"`
	Redirect        *RedirectTarget  `json:"redirect,omitempty"`
	Chain           []RedirectHop    `json:"chain,omitempty"`
	ChainStatus     string           `json:"chain_status,omitempty"`
}

type HTTPPhases struct {
	DNSMS     *float64 `json:"dns_ms"`
	ConnectMS *float64 `json:"connect_ms"`
	TLSMS     *float64 `json:"tls_ms"`
	TTFBMS    *float64 `json:"ttfb_ms"`
	TotalMS   float64  `json:"total_ms"`
}

type SecurityHeader struct {
	Name    string   `json:"name"`
	Present bool     `json:"present"`
	Value   string   `json:"value,omitempty"`
	Notes   []string `json:"notes,omitempty"`
}

type RedirectTarget struct {
	Location  string   `json:"location,omitempty"`
	Valid     bool     `json:"valid"`
	SelfLoop  bool     `json:"self_loop"`
	CrossHost bool     `json:"cross_host"`
	Downgrade bool     `json:"downgrade"`
	Notes     []string `json:"notes,omitempty"`
}

type RedirectHop struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Status     string `json:"status"`
}

var securityHeaderNames = []string{
	"Strict-Transport-Security",
	"Content-Security-Policy",
	"Content-Security-Policy-Report-Only",
	"X-Content-Type-Options",
	"X-Frame-Options",
	"Referrer-Policy",
	"Permissions-Policy",
	"Cross-Origin-Opener-Policy",
	"Cross-Origin-Embedder-Policy",
	"Cross-Origin-Resource-Policy",
}

type phaseRecorder struct {
	mu      sync.Mutex
	dns     *float64
	connect *float64
	tls     *float64
	ttfb    *float64
	start   time.Time
}

func (p *phaseRecorder) mark(dest **float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if *dest == nil {
		v := float64(time.Since(p.start).Microseconds()) / 1000
		*dest = &v
	}
}

func (p *phaseRecorder) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSDone:              func(httptrace.DNSDoneInfo) { p.mark(&p.dns) },
		ConnectDone:          func(string, string, error) { p.mark(&p.connect) },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { p.mark(&p.tls) },
		GotFirstResponseByte: func() { p.mark(&p.ttfb) },
	}
}

func (p *phaseRecorder) phases() HTTPPhases {
	p.mu.Lock()
	defer p.mu.Unlock()
	return HTTPPhases{DNSMS: p.dns, ConnectMS: p.connect, TLSMS: p.tls, TTFBMS: p.ttfb, TotalMS: float64(time.Since(p.start).Microseconds()) / 1000}
}

func diagnoseHTTP(ctx context.Context, r Request) (*HTTPMetadata, []string, error) {
	meta, values, err := fetchOnce(ctx, r.Target, time.Duration(r.TimeoutMS)*time.Millisecond)
	if meta == nil {
		return nil, values, err
	}
	hops := r.Redirects
	if hops > MaxRedirectHops {
		hops = MaxRedirectHops
	}
	if hops == 0 {
		meta.ChainStatus = "not_requested"
		return meta, values, err
	}
	meta.ChainStatus = "complete"
	if err != nil {
		return meta, values, err
	}
	seen := map[string]bool{normalizeURL(r.Target): true}
	for i := 0; i < hops; i++ {
		redir := meta.Redirect
		if redir == nil || redir.Location == "" || meta.StatusCode < 300 || meta.StatusCode > 399 {
			meta.ChainStatus = "complete"
			break
		}
		if !redir.Valid || redir.SelfLoop {
			status := "invalid"
			meta.ChainStatus = "invalid_target"
			if redir.SelfLoop {
				status = "loop"
				meta.ChainStatus = "loop"
			}
			meta.Chain = append(meta.Chain, RedirectHop{URL: redir.Location, Status: status})
			break
		}
		next := normalizeURL(redir.Location)
		if seen[next] {
			meta.Chain = append(meta.Chain, RedirectHop{URL: next, Status: "loop"})
			meta.ChainStatus = "loop"
			break
		}
		seen[next] = true
		hopMeta, hopValues, hopErr := fetchOnce(ctx, next, time.Duration(r.TimeoutMS)*time.Millisecond)
		hop := RedirectHop{URL: next, Status: "followed"}
		if hopMeta == nil || hopMeta.StatusCode == 0 {
			hop.Status = "failed"
			meta.Chain = append(meta.Chain, hop)
			meta.ChainStatus = "failed"
			if hopErr != nil {
				err = hopErr
			}
			break
		}
		hop.StatusCode = hopMeta.StatusCode
		meta.Chain = append(meta.Chain, hop)
		meta.StatusCode = hopMeta.StatusCode
		meta.FinalURL = hopMeta.FinalURL
		meta.SecurityHeaders = hopMeta.SecurityHeaders
		meta.Redirect = hopMeta.Redirect
		if hopValues != nil {
			values = hopValues
		}
		if hopErr != nil {
			meta.ChainStatus = "failed"
			err = hopErr
			break
		}
		if hopMeta.StatusCode < 300 || hopMeta.StatusCode > 399 || hopMeta.Redirect == nil || hopMeta.Redirect.Location == "" {
			meta.ChainStatus = "complete"
			break
		}
		if i == hops-1 {
			meta.ChainStatus = "limit"
		}
	}
	return meta, values, err
}

func fetchOnce(ctx context.Context, raw string, timeout time.Duration) (*HTTPMetadata, []string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, nil, err
	}
	rec := &phaseRecorder{start: time.Now()}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), rec.trace()))
	transport := &http.Transport{MaxResponseHeaderBytes: 64 << 10, Proxy: nil, DisableKeepAlives: true, TLSHandshakeTimeout: timeout}
	if httpTransportHook != nil {
		transport = httpTransportHook(timeout)
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	meta := &HTTPMetadata{FinalURL: req.URL.String(), Phases: rec.phases(), SecurityHeaders: securityHeaders(http.Header{}, req.URL.Scheme), ChainStatus: "not_requested"}
	if resp == nil {
		return meta, nil, err
	}
	defer resp.Body.Close()
	_, copyErr := io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyBytes))
	if err == nil {
		err = copyErr
	}
	meta.StatusCode = resp.StatusCode
	meta.SecurityHeaders = securityHeaders(resp.Header, req.URL.Scheme)
	meta.Redirect = redirectTarget(req.URL, resp.Header.Get("Location"))
	return meta, []string{resp.Status, resp.Proto}, err
}

func securityHeaders(h http.Header, scheme string) []SecurityHeader {
	out := make([]SecurityHeader, 0, len(securityHeaderNames))
	for _, name := range securityHeaderNames {
		values := h.Values(name)
		item := SecurityHeader{Name: name, Present: len(values) > 0}
		if item.Present {
			item.Value = boundHeaderValue(strings.Join(values, ", "))
		}
		if name == "Strict-Transport-Security" && scheme == "http" && item.Present {
			item.Notes = []string{"ignored_over_http"}
		}
		out = append(out, item)
	}
	return out
}

func boundHeaderValue(v string) string {
	v = strings.Map(func(r rune) rune {
		if r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return '?'
	}, v)
	if len(v) <= MaxHeaderValueBytes {
		return v
	}
	cut := MaxHeaderValueBytes
	for cut > 0 && !utf8.ValidString(v[:cut]) {
		cut--
	}
	return v[:cut] + " [truncated]"
}

func redirectTarget(base *url.URL, location string) *RedirectTarget {
	if location == "" {
		return &RedirectTarget{}
	}
	if len(location) > MaxLocationBytes {
		return &RedirectTarget{Location: location[:MaxLocationBytes], Notes: []string{"location_truncated"}}
	}
	loc, err := url.Parse(location)
	if err != nil || base == nil {
		return &RedirectTarget{Location: location}
	}
	resolved := base.ResolveReference(loc)
	if resolved.User != nil || (resolved.Scheme != "http" && resolved.Scheme != "https") || resolved.Hostname() == "" || ValidateHost(resolved.Hostname()) != nil {
		return &RedirectTarget{Location: location}
	}
	if resolved.Port() != "" {
		port, conv := strconv.Atoi(resolved.Port())
		if conv != nil || port < 1 || port > 65535 {
			return &RedirectTarget{Location: location}
		}
	}
	norm := normalizeURL(resolved.String())
	target := &RedirectTarget{Location: norm, Valid: true, SelfLoop: norm == normalizeURL(base.String())}
	if !strings.EqualFold(resolved.Hostname(), base.Hostname()) {
		target.CrossHost = true
	}
	if base.Scheme == "https" && resolved.Scheme == "http" {
		target.Downgrade = true
		target.Notes = []string{"downgrade"}
	}
	return target
}

func normalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	if port == "" {
		if strings.Contains(host, ":") {
			u.Host = "[" + host + "]"
		} else {
			u.Host = host
		}
	} else {
		u.Host = net.JoinHostPort(host, port)
	}
	u.Fragment = ""
	u.User = nil
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}
