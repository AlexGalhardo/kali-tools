package dnsinspect

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

// inspectHook is set by tests and by the integration-only lab helper.
var inspectHook func(*inspector)

type exchangeFunc func(context.Context, string, *dns.Msg) (*dns.Msg, time.Duration, error)

type inspector struct {
	timeout  time.Duration
	deadline time.Duration
	roots    *x509.CertPool
	now      func() time.Time
	authPort func(netip.Addr) uint16
	nsPort   map[string]uint16
	track    *sync.WaitGroup
	onWorker func()
	udp      exchangeFunc
	tcp      exchangeFunc
	system   func(context.Context, string, string) ([]string, string, string, float64)
	sent     atomic.Int32
	limited  atomic.Bool
	jobs     chan func(context.Context)
}

func newInspector(r Request) *inspector {
	ins := &inspector{
		timeout:  time.Duration(r.TimeoutMS) * time.Millisecond,
		now:      time.Now,
		authPort: func(netip.Addr) uint16 { return 53 },
	}
	if inspectHook != nil {
		inspectHook(ins)
	}
	return ins
}

func (ins *inspector) start(ctx context.Context) func() {
	ins.jobs = make(chan func(context.Context), Workers)
	var wg sync.WaitGroup
	for range Workers {
		wg.Add(1)
		if ins.track != nil {
			ins.track.Add(1)
		}
		go func() {
			defer wg.Done()
			if ins.track != nil {
				defer ins.track.Done()
			}
			if ins.onWorker != nil {
				ins.onWorker()
			}
			for fn := range ins.jobs {
				fn(ctx)
			}
		}()
	}
	return func() {
		close(ins.jobs)
		wg.Wait()
	}
}

func (ins *inspector) finish(report Report) Report {
	report.Queries = int(ins.sent.Load())
	report.Limited = ins.limited.Load()
	return report
}

func (ins *inspector) spend() bool {
	for {
		cur := ins.sent.Load()
		if cur >= MaxQueries {
			ins.limited.Store(true)
			return false
		}
		if ins.sent.CompareAndSwap(cur, cur+1) {
			return true
		}
	}
}

func (ins *inspector) spawn(ctx context.Context, fn func(context.Context)) {
	if ins.jobs == nil {
		fn(ctx)
		return
	}
	done := make(chan struct{})
	select {
	case <-ctx.Done():
		return
	case ins.jobs <- func(ctx context.Context) {
		defer close(done)
		fn(ctx)
	}:
	}
	<-done
}

type asked struct {
	status    string
	rcode     string
	transport string
	elapsed   float64
	answers   []string
	minTTL    *uint32
	ad        bool
	aa        bool
	cut       bool
	msg       *dns.Msg
}

func question(name, typ string, recursion, checkingDisabled bool) *dns.Msg {
	if typ == "" {
		typ = "A"
	}
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), dns.StringToType[typ])
	msg.RecursionDesired = recursion
	msg.CheckingDisabled = checkingDisabled
	msg.SetEdns0(EDNSBufferSize, true)
	return msg
}

func (ins *inspector) ask(ctx context.Context, server, name, typ string, recursion, checkingDisabled bool) asked {
	out := asked{status: "skipped", answers: []string{}}
	if ctx.Err() != nil {
		return out
	}
	q := question(name, typ, recursion, checkingDisabled)
	ins.spawn(ctx, func(ctx context.Context) {
		out = ins.exchange(ctx, server, q)
	})
	if out.answers == nil {
		out.answers = []string{}
	}
	return out
}

func (ins *inspector) exchange(ctx context.Context, server string, q *dns.Msg) asked {
	res := asked{answers: []string{}}
	if !ins.spend() {
		res.status = "skipped"
		return res
	}
	udp := ins.udp
	if udp == nil {
		udp = ins.defaultExchange("udp")
	}
	start := time.Now()
	msg, rtt, err := udp(ctx, server, q.Copy())
	transport := "udp"
	if err == nil && msg != nil && msg.Truncated {
		if !ins.spend() {
			res.status = "skipped"
			return res
		}
		tcp := ins.tcp
		if tcp == nil {
			tcp = ins.defaultExchange("tcp")
		}
		msg, rtt, err = tcp(ctx, server, q.Copy())
		transport = "tcp"
	}
	res.elapsed = float64(time.Since(start).Microseconds()) / 1000
	if rtt > 0 {
		res.elapsed = float64(rtt.Microseconds()) / 1000
	}
	if err != nil {
		res.status = ins.failStatus(ctx, server, err)
		res.transport = transport
		return res
	}
	res.msg = msg
	res.transport = transport
	res.ad = msg.AuthenticatedData
	res.aa = msg.Authoritative
	res.status, res.rcode = rcodeStatus(msg.Rcode)
	res.answers, res.minTTL, res.cut = normalizeAnswers(msg)
	if res.cut {
		ins.limited.Store(true)
	}
	return res
}

func (ins *inspector) defaultExchange(network string) exchangeFunc {
	return func(ctx context.Context, addr string, msg *dns.Msg) (*dns.Msg, time.Duration, error) {
		client := &dns.Client{Net: network, UDPSize: EDNSBufferSize, Timeout: ins.timeout}
		qctx, cancel := context.WithTimeout(ctx, ins.timeout)
		defer cancel()
		return client.ExchangeContext(qctx, msg, addr)
	}
}

func (ins *inspector) failStatus(ctx context.Context, server string, udpErr error) string {
	if errors.Is(udpErr, context.Canceled) {
		return "timeout"
	}
	timedOut := isTimeout(udpErr) || errors.Is(udpErr, context.DeadlineExceeded)
	if !timedOut {
		return "unreachable"
	}
	if !isTimeout(udpErr) {
		udpErr = netTimeout{}
	}
	pctx, cancel := context.WithTimeout(ctx, ins.timeout)
	defer cancel()
	tcpErr := ins.probe(pctx, server)
	_, portText, _ := net.SplitHostPort(server)
	port, _ := strconv.Atoi(portText)
	return classifyTransport(port, udpErr, tcpErr)
}

func (ins *inspector) probe(ctx context.Context, addr string) error {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if conn != nil {
		_ = conn.Close()
	}
	return err
}

func rcodeStatus(code int) (string, string) {
	name := dns.RcodeToString[code]
	if name == "" {
		name = strconv.Itoa(code)
	}
	switch code {
	case dns.RcodeSuccess:
		return "ok", name
	case dns.RcodeNameError:
		return "nxdomain", name
	case dns.RcodeServerFailure:
		return "servfail", name
	case dns.RcodeRefused:
		return "refused", name
	default:
		return "rcode_other", name
	}
}

func normalizeAnswers(msg *dns.Msg) ([]string, *uint32, bool) {
	out := []string{}
	if msg == nil {
		return out, nil, false
	}
	var minTTL *uint32
	cut := false
	for _, rr := range msg.Answer {
		copied := dns.Copy(rr)
		ttl := copied.Header().Ttl
		if minTTL == nil || ttl < *minTTL {
			value := ttl
			minTTL = &value
		}
		copied.Header().Ttl = 0
		text := strings.ToLower(copied.String())
		if len(text) > MaxRDataText {
			text = text[:MaxRDataText]
			cut = true
		}
		out = append(out, text)
	}
	sort.Strings(out)
	if len(out) > MaxAnswersReported {
		out = out[:MaxAnswersReported]
		cut = true
	}
	return out, minTTL, cut
}

func matchedSOA(a asked, suffix string) (uint32, bool) {
	if a.msg == nil {
		return 0, false
	}
	want := strings.ToLower(dns.Fqdn(suffix))
	for _, rr := range a.msg.Answer {
		soa, ok := rr.(*dns.SOA)
		if ok && strings.ToLower(soa.Hdr.Name) == want {
			return soa.Serial, true
		}
	}
	return 0, false
}

func nsNames(msg *dns.Msg) []string {
	out := []string{}
	if msg == nil {
		return out
	}
	for _, rr := range msg.Answer {
		ns, ok := rr.(*dns.NS)
		if !ok {
			continue
		}
		out = append(out, strings.ToLower(dns.Fqdn(ns.Ns)))
	}
	sort.Strings(out)
	if len(out) > MaxNameservers {
		out = out[:MaxNameservers]
		return out
	}
	return out
}

func answerIPs(msg *dns.Msg, kind string) []string {
	out := []string{}
	if msg == nil {
		return out
	}
	for _, rr := range msg.Answer {
		switch record := rr.(type) {
		case *dns.A:
			if kind == "A" && record.A != nil {
				out = append(out, record.A.String())
			}
		case *dns.AAAA:
			if kind == "AAAA" && record.AAAA != nil {
				out = append(out, record.AAAA.String())
			}
		}
		if len(out) >= MaxAddrsPerNS {
			return out[:MaxAddrsPerNS]
		}
	}
	return out
}

func (ins *inspector) doh(ctx context.Context, endpoint string, r Request) EncryptedResult {
	out := EncryptedResult{Endpoint: endpoint, Protocol: "doh"}
	if !ins.spend() {
		out.Status = "skipped"
		return out
	}
	packed, err := question(r.Name, r.Type, true, false).Pack()
	if err != nil || len(packed) > 512 {
		out.Status = "bad_response"
		return out
	}
	reqCtx, cancel := context.WithTimeout(ctx, ins.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(packed))
	if err != nil {
		out.Status = "bad_response"
		return out
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	start := time.Now()
	resp, err := ins.httpClient().Do(req)
	out.ElapsedMS = float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		out.Status = clientStatus(err)
		return out
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		out.Status = "bad_response"
		return out
	}
	if len(body) > MaxResponseBytes {
		ins.limited.Store(true)
		out.Status = "bad_response"
		return out
	}
	if resp.StatusCode != http.StatusOK {
		out.Status = "http_status"
		out.HTTPCode = resp.StatusCode
		return out
	}
	msg := new(dns.Msg)
	if err := msg.Unpack(body); err != nil {
		out.Status = "bad_response"
		return out
	}
	out.Status = "ok"
	out.RCode = dns.RcodeToString[msg.Rcode]
	return out
}

func (ins *inspector) dot(ctx context.Context, endpoint string, r Request) EncryptedResult {
	out := EncryptedResult{Endpoint: endpoint, Protocol: "dot"}
	if !ins.spend() {
		out.Status = "skipped"
		return out
	}
	addr, serverName := dotEndpoint(endpoint)
	client := &dns.Client{
		Net:     "tcp-tls",
		Timeout: ins.timeout,
		TLSConfig: &tls.Config{
			ServerName: serverName,
			MinVersion: tls.VersionTLS12,
			RootCAs:    ins.roots,
		},
	}
	qctx, cancel := context.WithTimeout(ctx, ins.timeout)
	defer cancel()
	start := time.Now()
	msg, rtt, err := client.ExchangeContext(qctx, question(r.Name, r.Type, true, false), addr)
	out.ElapsedMS = float64(time.Since(start).Microseconds()) / 1000
	if rtt > 0 {
		out.ElapsedMS = float64(rtt.Microseconds()) / 1000
	}
	if err != nil {
		out.Status = clientStatus(err)
		return out
	}
	out.Status = "ok"
	out.RCode = dns.RcodeToString[msg.Rcode]
	return out
}

func (ins *inspector) httpClient() *http.Client {
	return &http.Client{
		Timeout: ins.timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			Proxy:                  nil,
			DisableKeepAlives:      true,
			MaxResponseHeaderBytes: 16 << 10,
			TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: ins.roots},
		},
	}
}

func dotEndpoint(raw string) (addr, serverName string) {
	if addr, err := netip.ParseAddr(raw); err == nil {
		return net.JoinHostPort(addr.String(), "853"), addr.String()
	}
	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		return net.JoinHostPort(raw, "853"), raw
	}
	return raw, host
}

func clientStatus(err error) string {
	if isTLS(err) {
		return "tls_error"
	}
	if isTimeout(err) || errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "unreachable"
}

func isTLS(err error) bool {
	var unknown x509.UnknownAuthorityError
	var host x509.HostnameError
	var invalid x509.CertificateInvalidError
	var verify *tls.CertificateVerificationError
	return errors.As(err, &unknown) || errors.As(err, &host) || errors.As(err, &invalid) || errors.As(err, &verify)
}
