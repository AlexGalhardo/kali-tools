package tools

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const (
	inspectionScanner = "native"
	inspectionVersion = "1"
	bannerLimit       = 256
	responseLimit     = 512
	labelLimit        = 64
	bannerWait        = 200 * time.Millisecond
	probeWait         = 500 * time.Millisecond
)

var tsharkVersion = regexp.MustCompile(`^TShark \(Wireshark\) (4)\.([0-9]+)\.[0-9]+(?: |$)`)

func supportedVersion(name, line string) bool {
	if name != "tshark" {
		return false
	}
	match := tsharkVersion.FindStringSubmatch(line)
	if len(match) != 3 {
		return false
	}
	minor, err := strconv.Atoi(match[2])
	return err == nil && minor >= 2
}

// InspectServices identifies the protocol on explicitly scoped TCP ports.
// It reads a banner, then tries one TLS handshake and one HTTP/1.1 request.
// Raw probe bytes stay in this function. History records only the summary built by the application.
func InspectServices(parent context.Context, cidr string, ports []int) (app.ServiceInspection, error) {
	if err := network.ValidateDiscovery(cidr, ports, time.Second); err != nil {
		return app.ServiceInspection{}, err
	}
	ctx, cancel := context.WithTimeout(parent, network.DiscoveryDeadline)
	defer cancel()
	prefix, _ := netip.ParsePrefix(cidr)
	type task struct {
		host string
		port int
	}
	tasks := make(chan task)
	hits := map[string][]app.InspectionPort{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range network.DiscoveryWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range tasks {
				if ctx.Err() != nil {
					continue
				}
				port := inspectPort(ctx, item.host, item.port)
				if ctx.Err() != nil {
					continue
				}
				mu.Lock()
				hits[item.host] = append(hits[item.host], port)
				mu.Unlock()
			}
		}()
	}
	pace := time.NewTicker(network.DiscoveryPace)
	defer pace.Stop()
outer:
	for ip := prefix.Masked().Addr(); prefix.Contains(ip); ip = ip.Next() {
		for _, port := range ports {
			select {
			case <-pace.C:
			case <-ctx.Done():
				break outer
			}
			select {
			case tasks <- task{ip.String(), port}:
			case <-ctx.Done():
				break outer
			}
		}
	}
	close(tasks)
	wg.Wait()
	result := inspectionResult(hits)
	if err := parent.Err(); err != nil {
		result.Finished.Exit = ""
		return result, err
	}
	if ctx.Err() != nil {
		result.Finished.Exit = "deadline"
		result.Truncated = true
	}
	return result, nil
}

func inspectionResult(hits map[string][]app.InspectionPort) app.ServiceInspection {
	hosts := make([]app.InspectionHost, 0, len(hits))
	for host, ports := range hits {
		sort.Slice(ports, func(i, j int) bool { return ports[i].Port < ports[j].Port })
		state := "down"
		for _, port := range ports {
			if port.State.State == "open" || port.State.State == "closed" {
				state = "up"
				break
			}
		}
		hosts = append(hosts, app.InspectionHost{
			Status:    app.InspectionState{State: state},
			Addresses: []app.InspectionAddress{{Address: host, Type: "ipv4"}},
			Ports:     ports,
		})
	}
	sort.Slice(hosts, func(i, j int) bool {
		a, _ := netip.ParseAddr(hosts[i].Addresses[0].Address)
		b, _ := netip.ParseAddr(hosts[j].Addresses[0].Address)
		return a.Less(b)
	})
	return app.ServiceInspection{
		Scanner:  inspectionScanner,
		Version:  inspectionVersion,
		Finished: app.InspectionFinished{Exit: "success"},
		Hosts:    hosts,
	}
}

func inspectPort(ctx context.Context, host string, port int) app.InspectionPort {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := dial(ctx, address)
	if err != nil {
		return tcpPort(port, network.ClassifyDial(err), "", "", "", "")
	}
	banner := readLimited(conn, bannerLimit, bannerWait)
	_ = conn.Close()
	if name, product, version, ok := classifyBanner(banner); ok {
		return tcpPort(port, "open", name, product, version, "banner")
	}
	if len(banner) > 0 || ctx.Err() != nil {
		return tcpPort(port, "open", "", "", "", "")
	}
	if name, version, ok := identifyTLS(ctx, address); ok {
		return tcpPort(port, "open", name, "", version, "tls")
	}
	if product, version, ok := identifyHTTP(ctx, host, address); ok {
		return tcpPort(port, "open", "http", product, version, "http")
	}
	if name := identifyActive(ctx, address); name != "" {
		return tcpPort(port, "open", name, "", "", "probe")
	}
	return tcpPort(port, "open", "", "", "", "")
}

func tcpPort(port int, state, name, product, version, method string) app.InspectionPort {
	if method != "banner" && method != "tls" && method != "http" && method != "probe" {
		method = ""
	}
	return app.InspectionPort{
		Port:     port,
		Protocol: "tcp",
		State:    app.InspectionState{State: state},
		Service: app.InspectionService{
			Name:    label(name),
			Product: label(product),
			Version: label(version),
			Method:  method,
		},
	}
}

func dial(ctx context.Context, address string) (net.Conn, error) {
	probe, cancel := context.WithTimeout(ctx, probeWait)
	defer cancel()
	return (&net.Dialer{}).DialContext(probe, "tcp", address)
}

func readLimited(conn net.Conn, limit int, wait time.Duration) []byte {
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	buf := make([]byte, 0, limit)
	tmp := make([]byte, limit)
	for len(buf) < limit {
		n, err := conn.Read(tmp[:limit-len(buf)])
		buf = append(buf, tmp[:n]...)
		if err != nil || n == 0 {
			break
		}
	}
	return buf
}

func classifyBanner(raw []byte) (name, product, version string, ok bool) {
	if len(raw) >= 3 && raw[0] == 0xFF && raw[1] >= 0xFB && raw[1] <= 0xFE {
		return "telnet", "", "", true
	}
	if version, ok = mysqlVersion(raw); ok {
		return "mysql", "", label(version), true
	}
	text := string(raw)
	line, _, _ := strings.Cut(text, "\n")
	line = strings.TrimRight(line, "\r")
	switch {
	case strings.HasPrefix(line, "SSH-2.0-"), strings.HasPrefix(line, "SSH-1.99-"), strings.HasPrefix(line, "SSH-1.5-"):
		software := line
		for _, prefix := range []string{"SSH-2.0-", "SSH-1.99-", "SSH-1.5-"} {
			software = strings.TrimPrefix(software, prefix)
		}
		software, _, _ = strings.Cut(strings.TrimSpace(software), " ")
		product, version, _ = strings.Cut(software, "_")
		return "ssh", label(product), label(version), true
	case strings.HasPrefix(line, "RFB "):
		return "vnc", "", label(strings.TrimPrefix(line, "RFB ")), true
	case strings.HasPrefix(line, "* OK"):
		return "imap", "", "", true
	case strings.HasPrefix(line, "+OK"):
		return "pop3", "", "", true
	case strings.HasPrefix(line, "HTTP/"):
		product, version = serverProduct(raw)
		return "http", product, version, true
	}
	upper := strings.ToUpper(line)
	if strings.HasPrefix(line, "220") {
		if strings.Contains(upper, "SMTP") || strings.Contains(upper, "ESMTP") {
			return "smtp", "", "", true
		}
		if strings.Contains(upper, "FTP") {
			return "ftp", "", "", true
		}
	}
	return "", "", "", false
}

func mysqlVersion(raw []byte) (string, bool) {
	if len(raw) < 6 || raw[4] != 10 {
		return "", false
	}
	end := 5
	for end < len(raw) && raw[end] != 0 && end-5 < 32 {
		if raw[end] < 32 || raw[end] > 126 {
			return "", false
		}
		end++
	}
	if end == 5 || end >= len(raw) || raw[end] != 0 {
		return "", false
	}
	return string(raw[5:end]), true
}

func identifyTLS(ctx context.Context, address string) (name, version string, ok bool) {
	conn, err := dial(ctx, address)
	if err != nil {
		return "", "", false
	}
	defer conn.Close()
	probe, cancel := context.WithTimeout(ctx, probeWait)
	defer cancel()
	_ = conn.SetDeadline(time.Now().Add(probeWait))
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true, // identity only; the certificate is not treated as trusted
		MinVersion:         tls.VersionTLS10,
		NextProtos:         []string{"h2", "http/1.1"},
	})
	if err = tlsConn.HandshakeContext(probe); err != nil {
		return "", "", false
	}
	state := tlsConn.ConnectionState()
	name = "tls"
	if state.NegotiatedProtocol == "h2" || state.NegotiatedProtocol == "http/1.1" {
		name = "https"
	}
	return name, tls.VersionName(state.Version), true
}

func identifyHTTP(ctx context.Context, host, address string) (product, version string, ok bool) {
	conn, err := dial(ctx, address)
	if err != nil {
		return "", "", false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(probeWait))
	if _, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host); err != nil {
		return "", "", false
	}
	raw := readLimited(conn, responseLimit, probeWait)
	if !strings.HasPrefix(string(raw), "HTTP/") {
		return "", "", false
	}
	product, version = serverProduct(raw)
	return product, version, true
}

func serverProduct(raw []byte) (product, version string) {
	rest := string(raw)
	for rest != "" {
		var line string
		line, rest, _ = strings.Cut(rest, "\n")
		line = strings.TrimRight(line, "\r")
		if len(line) < 8 || !strings.EqualFold(line[:7], "server:") {
			continue
		}
		value := label(line[7:])
		product, version, _ = strings.Cut(value, "/")
		return label(product), label(version)
	}
	return "", ""
}

func label(value string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if r < 32 || r > 126 {
			continue
		}
		b.WriteRune(r)
		if b.Len() >= labelLimit {
			break
		}
	}
	return b.String()
}

// Probes for services that wait for the client to speak. Each is one
// connection, one write and one capped read.
var activeProbes = []struct {
	name    string
	payload []byte
	match   func([]byte) bool
}{
	{"redis", []byte("PING\r\n"), func(r []byte) bool {
		return bytes.HasPrefix(r, []byte("+PONG")) || bytes.HasPrefix(r, []byte("-NOAUTH"))
	}},
	{"rdp", []byte{0x03, 0x00, 0x00, 0x13, 0x0e, 0xe0, 0, 0, 0, 0, 0, 0x01, 0x00, 0x08, 0x00, 0, 0, 0, 0}, func(r []byte) bool {
		return len(r) >= 6 && r[0] == 0x03 && r[1] == 0x00 && r[5] == 0xD0
	}},
	{"smb", smbNegotiate(), func(r []byte) bool {
		return len(r) >= 8 && (bytes.Equal(r[4:8], []byte("\xFFSMB")) || bytes.Equal(r[4:8], []byte("\xFESMB")))
	}},
}

func smbNegotiate() []byte {
	header := []byte{0xFF, 'S', 'M', 'B', 0x72, 0, 0, 0, 0, 0x18, 0xC8, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF, 0xFE, 0xFF, 0, 0, 0, 0}
	body := append([]byte{0, 12, 0, 0x02}, "NT LM 0.12\x00"...)
	msg := append(header, body...)
	return append([]byte{0, 0, 0, byte(len(msg))}, msg...)
}

func identifyActive(ctx context.Context, address string) string {
	for _, p := range activeProbes {
		if ctx.Err() != nil {
			return ""
		}
		conn, err := dial(ctx, address)
		if err != nil {
			return ""
		}
		_ = conn.SetDeadline(time.Now().Add(probeWait))
		var reply []byte
		if _, err = conn.Write(p.payload); err == nil {
			reply = readLimited(conn, responseLimit, probeWait)
		}
		_ = conn.Close()
		if p.match(reply) {
			return p.name
		}
	}
	return ""
}
