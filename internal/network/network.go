package network

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AlexGalhardo/hacker-tools/internal/domain"
	"github.com/AlexGalhardo/hacker-tools/internal/probe"
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
	Kind      string `json:"kind"`
	Target    string `json:"target"`
	Port      int    `json:"port"`
	TimeoutMS int    `json:"timeout_ms"`
}
type Result struct {
	Kind      string   `json:"kind"`
	Target    string   `json:"target"`
	Status    string   `json:"status"`
	ElapsedMS float64  `json:"elapsed_ms"`
	Values    []string `json:"values"`
	Provider  string   `json:"provider"`
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
			valid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-'
			if !valid {
				return domain.Invalid("invalid host")
			}
		}
	}
	return nil
}
func Validate(r Request) error {
	if r.TimeoutMS < 100 || r.TimeoutMS > 30000 {
		return domain.Invalid("timeout must be 100..30000 ms")
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
		u, e := url.Parse(r.Target)
		if e != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return domain.Invalid("expected HTTP(S) URL without credentials")
		}
		return ValidateHost(u.Hostname())
	default:
		return domain.Unsupported("diagnostic kind is not supported")
	}
	return nil
}
func Diagnose(ctx context.Context, r Request) (Result, error) {
	result := Result{Kind: r.Kind, Target: r.Target, Status: "completed", Values: []string{}, Provider: "go/net"}
	if err := Validate(r); err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	start := time.Now()
	var err error
	switch r.Kind {
	case "ping", "traceroute":
		result.Provider = probe.Provider
		result.Values, err = probe.Run(ctx, r.Target, r.Kind == "traceroute")
	case "dns":
		result.Values, err = net.DefaultResolver.LookupHost(ctx, r.Target)
		sort.Strings(result.Values)
	case "tcp":
		var c net.Conn
		c, err = (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(r.Target, strconv.Itoa(r.Port)))
		if c != nil {
			c.Close()
			result.Values = []string{"connected"}
		}
	case "tls":
		var c net.Conn
		c, err = (&tls.Dialer{NetDialer: &net.Dialer{}, Config: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: r.Target}}).DialContext(ctx, "tcp", net.JoinHostPort(r.Target, strconv.Itoa(r.Port)))
		if c != nil {
			defer c.Close()
			state := c.(*tls.Conn).ConnectionState()
			for _, cert := range state.PeerCertificates {
				result.Values = append(result.Values, fmt.Sprintf("subject=%s issuer=%s expires=%s", cert.Subject, cert.Issuer, cert.NotAfter.UTC().Format(time.RFC3339)))
			}
		}
	case "http":
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, r.Target, nil)
		if err == nil {
			transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSHandshakeTimeout: time.Duration(r.TimeoutMS) * time.Millisecond}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
			var resp *http.Response
			resp, err = client.Do(req)
			if resp != nil {
				defer resp.Body.Close()
				_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, 65536))
				result.Values = []string{resp.Status, resp.Proto}
			}
		}
	}
	result.ElapsedMS = float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		result.Status = "failed"
		return result, err
	}
	return result, nil
}

type PortResult struct {
	Host  string `json:"host"`
	Port  int    `json:"port"`
	State string `json:"state"`
}

func Discover(ctx context.Context, cidr string, ports []int, timeout time.Duration) ([]PortResult, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 24 {
		return nil, domain.Invalid("use an explicit IPv4 CIDR /24 or narrower")
	}
	if len(ports) == 0 || len(ports) > 32 || timeout < 100*time.Millisecond || timeout > 5*time.Second {
		return nil, domain.Invalid("1..32 ports and timeout 100..5000 ms required")
	}
	seen := map[int]bool{}
	for _, p := range ports {
		if p < 1 || p > 65535 || seen[p] {
			return nil, domain.Invalid("ports must be unique and 1..65535")
		}
		seen[p] = true
	}
	type task struct {
		host string
		port int
	}
	tasks := make(chan task)
	results := make([]PortResult, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				if ctx.Err() != nil {
					continue
				}
				probe, cancel := context.WithTimeout(ctx, timeout)
				c, e := (&net.Dialer{}).DialContext(probe, "tcp", net.JoinHostPort(t.host, strconv.Itoa(t.port)))
				cancel()
				state := "unreachable"
				if e == nil {
					state = "open"
					c.Close()
				}
				mu.Lock()
				results = append(results, PortResult{t.host, t.port, state})
				mu.Unlock()
			}
		}()
	}
outer:
	for ip := prefix.Masked().Addr(); prefix.Contains(ip); ip = ip.Next() {
		for _, p := range ports {
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
	return results, ctx.Err()
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
