package network

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

type InterfaceInfo struct {
	Index     int      `json:"index"`
	Name      string   `json:"name"`
	MAC       string   `json:"mac"`
	MTU       int      `json:"mtu"`
	Up        bool     `json:"up"`
	Loopback  bool     `json:"loopback"`
	Addresses []string `json:"addresses"`
}

func Interfaces() ([]InterfaceInfo, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := make([]InterfaceInfo, 0, len(all))
	for _, i := range all {
		addresses, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		item := InterfaceInfo{Index: i.Index, Name: i.Name, MAC: i.HardwareAddr.String(), MTU: i.MTU, Up: i.Flags&net.FlagUp != 0, Loopback: i.Flags&net.FlagLoopback != 0, Addresses: []string{}}
		for _, a := range addresses {
			item.Addresses = append(item.Addresses, a.String())
		}
		sort.Strings(item.Addresses)
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Index < result[j].Index })
	return result, nil
}

type Request struct {
	RecordType string `json:"record_type,omitempty"`
	Kind       string `json:"kind"`
	Target     string `json:"target"`
	Port       int    `json:"port"`
	TimeoutMS  int    `json:"timeout_ms"`
	Redirects  int    `json:"redirects,omitempty"`
}
type Result struct {
	ErrorCode string        `json:"error_code,omitempty"`
	Kind      string        `json:"kind"`
	Target    string        `json:"target"`
	Status    string        `json:"status"`
	ElapsedMS float64       `json:"elapsed_ms"`
	Values    []string      `json:"values"`
	Provider  string        `json:"provider"`
	HTTP      *HTTPMetadata `json:"http,omitempty"`
}

func ValidateHost(host string) error {
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "/\\\r\n\t @?#") {
		return domain.Invalid("invalid host")
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return nil
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return domain.Invalid("invalid host")
		}
		for _, r := range label {
			valid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_'
			if !valid {
				return domain.Invalid("invalid host")
			}
		}
	}
	return nil
}
func Validate(r Request) error {
	if r.Kind != "http" && r.Redirects != 0 {
		return domain.Invalid("redirects apply only to HTTP diagnostics")
	}
	if r.Kind == "http" && (r.Redirects < 0 || r.Redirects > MaxRedirectHops) {
		return domain.Invalid("redirects must be 0..5")
	}
	if r.TimeoutMS < 100 || r.TimeoutMS > 30000 {
		return domain.Invalid("timeout must be 100..30000 ms")
	}
	if r.RecordType != "" && r.Kind != "dns" {
		return domain.Invalid("record type applies only to DNS diagnostics")
	}
	if r.Kind == "dns" {
		switch r.RecordType {
		case "", "A", "AAAA", "CNAME", "MX", "NS", "TXT", "PTR":
		default:
			return domain.Invalid("DNS record type must be A, AAAA, CNAME, MX, NS, TXT or PTR")
		}
		if r.RecordType == "PTR" {
			if ip, e := netip.ParseAddr(r.Target); e != nil || ip.Zone() != "" {
				return domain.Invalid("PTR target must be an IP address without a zone")
			}
		}
	}
	switch r.Kind {
	case "dns", "tcp", "tls", "ping", "traceroute":
		if err := ValidateHost(r.Target); err != nil {
			return err
		}
		if (r.Kind == "tcp" || r.Kind == "tls") && (r.Port < 1 || r.Port > 65535) {
			return domain.Invalid("port must be 1..65535")
		}
	case "http":
		if len(r.Target) > 8192 {
			return domain.Invalid("HTTP URL exceeds 8192 bytes")
		}
		u, e := url.Parse(r.Target)
		if e != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return domain.Invalid("expected HTTP(S) URL without credentials")
		}
		if u.Port() != "" {
			port, err := strconv.Atoi(u.Port())
			if err != nil || port < 1 || port > 65535 {
				return domain.Invalid("HTTP port must be 1..65535")
			}
		}
		return ValidateHost(u.Hostname())
	default:
		return domain.Unsupported("diagnostic kind is not supported")
	}
	return nil
}

// Prober runs native ICMP echo or traceroute probes. Composition roots inject
// an OS implementation (internal/adapter/probe); nil means unsupported.
type Prober interface {
	Provider() string
	Run(ctx context.Context, target string, trace bool) ([]string, error)
}

// Diagnostics runs bounded single-target diagnostics with injected OS providers.
type Diagnostics struct {
	Prober Prober
}

func (d Diagnostics) Diagnose(ctx context.Context, r Request) (Result, error) {
	result := Result{Kind: r.Kind, Target: r.Target, Status: "completed", Values: []string{}, Provider: "go/net"}
	if err := Validate(r); err != nil {
		result.Status = "failed"
		result.ErrorCode = diagnosticErrorCode(err)
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	start := time.Now()
	var err error
	switch r.Kind {
	case "ping", "traceroute":
		if d.Prober == nil {
			err = domain.Unsupported("native ICMP provider is not available")
			break
		}
		result.Provider = d.Prober.Provider()
		result.Values, err = d.Prober.Run(ctx, r.Target, r.Kind == "traceroute")
	case "dns":
		result.Values, err = lookupDNS(ctx, net.DefaultResolver, r.Target, r.RecordType)
	case "tcp":
		var c net.Conn
		c, err = (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(r.Target, strconv.Itoa(r.Port)))
		if c != nil {
			c.Close()
			result.Values = []string{"connected"}
		}
	case "tls":
		result.Values, err = diagnoseTLS(ctx, r.Target, r.Port, nil)
	case "http":
		var meta *HTTPMetadata
		var values []string
		meta, values, err = diagnoseHTTP(ctx, r)
		if values != nil {
			result.Values = values
		}
		result.HTTP = meta
	}
	result.ElapsedMS = float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		result.Status = "failed"
		result.ErrorCode = diagnosticErrorCode(err)
		if errors.Is(err, context.Canceled) {
			result.Status = "cancelled"
		}
		return result, err
	}
	return result, nil
}

// Discovery limits. The scope validator and the scanners share them so a
// request that cannot finish inside DiscoveryDeadline is refused up front.
const (
	DiscoveryDeadline = 2 * time.Minute
	DiscoveryWorkers  = 32
	DiscoveryPace     = 10 * time.Millisecond
)

// scanDeadline is a variable only so tests can shorten it.
var scanDeadline = DiscoveryDeadline

type PortResult struct {
	Host      string  `json:"host"`
	Port      int     `json:"port"`
	State     string  `json:"state"`
	RTTMS     float64 `json:"rtt_ms"`
	HostState string  `json:"host_state"`
}

// Winsock error numbers; Go reports them as syscall.Errno on Windows and they
// do not match the portable syscall.E* values there.
const (
	wsaeConnRefused syscall.Errno = 10061
	wsaeHostUnreach syscall.Errno = 10065
	wsaeNetUnreach  syscall.Errno = 10051
)

// ClassifyDial maps a TCP dial error to open, closed, filtered or unreachable.
func ClassifyDial(err error) string {
	var netErr net.Error
	switch {
	case err == nil:
		return "open"
	case errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, wsaeConnRefused):
		return "closed"
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH), errors.Is(err, wsaeHostUnreach), errors.Is(err, wsaeNetUnreach):
		return "unreachable"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "filtered"
	}
	return "unreachable"
}

// HostState is up when any port answered (open or closed), down when every
// probe failed at the network layer, and unknown when probes only timed out.
func HostState(states []string) string {
	down := len(states) > 0
	for _, s := range states {
		if s == "open" || s == "closed" {
			return "up"
		}
		if s != "unreachable" {
			down = false
		}
	}
	if down {
		return "down"
	}
	return "unknown"
}

func ValidateDiscovery(cidr string, ports []int, timeout time.Duration) error {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 24 {
		return domain.Invalid("use an explicit IPv4 CIDR /24 or narrower")
	}
	if len(ports) == 0 || len(ports) > 32 || timeout < 100*time.Millisecond || timeout > 5*time.Second {
		return domain.Invalid("1..32 ports and timeout 100..5000 ms required")
	}
	seen := map[int]bool{}
	for _, p := range ports {
		if p < 1 || p > 65535 || seen[p] {
			return domain.Invalid("ports must be unique and 1..65535")
		}
		seen[p] = true
	}
	tasks := int64(1<<(32-prefix.Bits())) * int64(len(ports))
	worst := time.Duration(tasks * int64(timeout) / DiscoveryWorkers)
	if pace := time.Duration(tasks * int64(DiscoveryPace)); pace > worst {
		worst = pace
	}
	if worst > DiscoveryDeadline {
		return domain.Invalid("scope needs about %d s in the worst case, limit is %d s; reduce ports, timeout or the range", int(worst.Seconds()), int(DiscoveryDeadline.Seconds()))
	}
	return nil
}

// Discover probes every host and port of the scope. When the internal
// deadline cuts the scan it returns the results gathered so far with
// truncated set; a cancelled parent context is still an error.
func Discover(ctx context.Context, cidr string, ports []int, timeout time.Duration) (results []PortResult, truncated bool, err error) {
	if err := ValidateDiscovery(cidr, ports, timeout); err != nil {
		return nil, false, err
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, scanDeadline)
	defer cancel()
	prefix, _ := netip.ParsePrefix(cidr)
	type task struct {
		host string
		port int
	}
	tasks := make(chan task)
	results = make([]PortResult, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range DiscoveryWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				if ctx.Err() != nil {
					continue
				}
				probe, cancel := context.WithTimeout(ctx, timeout)
				start := time.Now()
				c, e := (&net.Dialer{}).DialContext(probe, "tcp", net.JoinHostPort(t.host, strconv.Itoa(t.port)))
				rtt := float64(time.Since(start).Microseconds()) / 1000
				cancel()
				if e == nil {
					c.Close()
				} else if ctx.Err() != nil {
					continue
				}
				mu.Lock()
				results = append(results, PortResult{Host: t.host, Port: t.port, State: ClassifyDial(e), RTTMS: rtt})
				mu.Unlock()
			}
		}()
	}
	pace := time.NewTicker(DiscoveryPace)
	defer pace.Stop()
outer:
	for ip := prefix.Masked().Addr(); prefix.Contains(ip); ip = ip.Next() {
		for _, p := range ports {
			select {
			case <-pace.C:
			case <-ctx.Done():
				break outer
			}
			select {
			case tasks <- task{ip.String(), p}:
			case <-ctx.Done():
				break outer
			}
		}
	}
	close(tasks)
	wg.Wait()
	sort.Slice(results, func(i, j int) bool {
		if results[i].Host == results[j].Host {
			return results[i].Port < results[j].Port
		}
		a, _ := netip.ParseAddr(results[i].Host)
		b, _ := netip.ParseAddr(results[j].Host)
		return a.Less(b)
	})
	byHost := map[string][]string{}
	for _, r := range results {
		byHost[r.Host] = append(byHost[r.Host], r.State)
	}
	for i := range results {
		results[i].HostState = HostState(byHost[results[i].Host])
	}
	if parent.Err() != nil {
		return results, false, parent.Err()
	}
	return results, ctx.Err() != nil, nil
}

type Counter struct {
	ID       string `json:"id"`
	Received uint64 `json:"received"`
	Sent     uint64 `json:"sent"`
}
type Rate struct {
	ID         string    `json:"id"`
	ReceiveBPS float64   `json:"receive_bps"`
	SendBPS    float64   `json:"send_bps"`
	Timestamp  time.Time `json:"timestamp"`
}

func Delta(previous, current Counter, elapsed time.Duration, now time.Time) Rate {
	r := Rate{ID: current.ID, Timestamp: now}
	if elapsed <= 0 || elapsed > 30*time.Second || previous.ID != current.ID {
		return r
	}
	if current.Received >= previous.Received {
		r.ReceiveBPS = float64(current.Received-previous.Received) / elapsed.Seconds()
	}
	if current.Sent >= previous.Sent {
		r.SendBPS = float64(current.Sent-previous.Sent) / elapsed.Seconds()
	}
	return r
}

func diagnosticErrorCode(err error) string {
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var typed *domain.Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	var dns *net.DNSError
	if errors.As(err, &dns) && dns.IsNotFound {
		return "not_found"
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout"
	}
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &authority) || errors.As(err, &hostname) || errors.As(err, &invalid) {
		return "certificate_invalid"
	}
	return "failed"
}
