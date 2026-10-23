package localdiscovery

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"

	"golang.org/x/net/ipv4"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

var errNoSource = errors.New("no source address")

var ssdpDest = &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}

func multicastSearch() bool {
	return ssdpDest.Port == 1900 && ssdpDest.IP.Equal(net.IPv4(239, 255, 255, 250))
}

func querySSDP(ctx context.Context, r Request) (Result, string) {
	prefix := scopePrefix(r.Scope.CIDR)
	src, err := sourceIPv4(r.Scope)
	if err != nil {
		return Result{}, "ssdp_bind_failed"
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: src, Port: 0})
	if err != nil {
		return Result{}, "ssdp_bind_failed"
	}
	defer func() { _ = conn.Close() }()
	packet := ipv4.NewPacketConn(conn)
	ifi, err := net.InterfaceByName(r.Scope.Interface)
	if err != nil {
		return Result{}, "ssdp_bind_failed"
	}
	if multicastSearch() && (packet.SetMulticastInterface(ifi) != nil || packet.SetMulticastTTL(SSDPTTL) != nil) {
		return Result{}, "ssdp_bind_failed"
	}
	payload := ssdpRequest(r)
	for i := range SSDPSends {
		if _, err := conn.WriteToUDP(payload, ssdpDest); err != nil {
			return Result{}, "ssdp_bind_failed"
		}
		if i == SSDPSends-1 {
			break
		}
		timer := time.NewTimer(SSDPSendGap)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Result{}, ""
		case <-timer.C:
		}
	}
	deadline := time.Now().Add(time.Duration(r.DurationSeconds) * time.Second)
	out := Result{}
	seen := map[string]struct{}{}
	buf := make([]byte, MaxDatagramBytes+1)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		srcAddr, _ := netip.AddrFromSlice(addr.IP.To4())
		ingestSSDP(&out, prefix, srcAddr, buf[:n], seen)
	}
	return out, ""
}

func ssdpRequest(r Request) []byte {
	mx := r.DurationSeconds
	if mx > 5 {
		mx = 5
	}
	return []byte("M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: " + itoa(mx) + "\r\n" +
		"ST: " + r.SSDPTarget + "\r\n" +
		"USER-AGENT: kali-tools/" + domain.Version + " UPnP/1.1\r\n" +
		"\r\n")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [4]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

func ingestSSDP(out *Result, prefix netip.Prefix, src netip.Addr, payload []byte, seen map[string]struct{}) {
	if len(payload) > MaxDatagramBytes {
		out.Malformed++
		return
	}
	if !inScope(prefix, src) {
		out.OutOfScope++
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(payload)), nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		out.Malformed++
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return
	}
	_ = resp.Body.Close()
	usn := clip(resp.Header.Get("USN"), MaxHeaderValue)
	if usn == "" {
		out.Malformed++
		return
	}
	if _, ok := seen[usn]; ok {
		return
	}
	seen[usn] = struct{}{}
	item := Responder{
		Protocol: "ssdp", Address: src.String(), Name: usn,
		ServiceType: clip(resp.Header.Get("ST"), MaxHeaderValue),
		Server:      clip(resp.Header.Get("SERVER"), MaxHeaderValue),
	}
	if raw := resp.Header.Get("LOCATION"); raw != "" {
		parsed, err := url.Parse(raw)
		if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			item.Location = clip(raw, MaxHeaderValue)
		}
	}
	out.Responders = append(out.Responders, item)
}

func sourceIPv4(scope Scope) (net.IP, error) {
	prefix := scopePrefix(scope.CIDR)
	ifi, err := net.InterfaceByName(scope.Interface)
	if err != nil {
		return nil, err
	}
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil {
			continue
		}
		parsed, err := netip.ParseAddr(ip.String())
		if err != nil || !prefix.Contains(parsed) {
			continue
		}
		return ip, nil
	}
	return nil, errNoSource
}
