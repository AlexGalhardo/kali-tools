package localdiscovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/localdiscovery/snmpfake"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func labIfaces() []network.InterfaceInfo {
	return []network.InterfaceInfo{{Name: "lab0", Up: true, Addresses: []string{"127.0.0.1/8"}}}
}

func withIfaces(t *testing.T) {
	t.Helper()
	previous := validateInterfaces
	validateInterfaces = func() []network.InterfaceInfo { return labIfaces() }
	t.Cleanup(func() { validateInterfaces = previous })
}

func scope() Scope { return Scope{Interface: "lab0", CIDR: "127.0.0.1/32"} }

func TestValidateScope(t *testing.T) {
	ifaces := []network.InterfaceInfo{
		{Name: "lan", Up: true, Addresses: []string{"192.0.2.10/24", "2001:db8::1/64"}},
		{Name: "down", Addresses: []string{"192.0.2.10/24"}},
		{Name: "lo", Up: true, Loopback: true, Addresses: []string{"127.0.0.1/8"}},
	}
	valid := []Scope{{Interface: "lan", CIDR: "192.0.2.0/24"}, {Interface: "lan", CIDR: "192.0.2.10/32"}}
	for _, item := range valid {
		if err := ValidateScope(item, ifaces); err != nil {
			t.Fatal(item, err)
		}
	}
	invalid := []Scope{
		{Interface: "lan", CIDR: "192.0.2.0/23"},
		{Interface: "lan", CIDR: "2001:db8::/64"},
		{Interface: "down", CIDR: "192.0.2.0/24"},
		{Interface: "lo", CIDR: "127.0.0.1/32"},
		{Interface: "lan", CIDR: "198.51.100.0/24"},
		{Interface: "missing", CIDR: "192.0.2.0/24"},
	}
	for _, item := range invalid {
		if err := ValidateScope(item, ifaces); err == nil {
			t.Fatal("accepted", item)
		}
	}
	base := Request{Scope: valid[0], Protocols: []string{"mdns"}, DurationSeconds: 1, SSDPTarget: "ssdp:all"}
	if err := Validate(base, ifaces); err != nil {
		t.Fatal(err)
	}
	base.DurationSeconds = 0
	if err := Validate(base, ifaces); err == nil {
		t.Fatal("duration 0")
	}
	base.DurationSeconds = MaxDurationSeconds + 1
	if err := Validate(base, ifaces); err == nil {
		t.Fatal("duration 11")
	}
	base.DurationSeconds = MaxDurationSeconds
	if err := Validate(base, ifaces); err != nil {
		t.Fatal(err)
	}
	base.ServiceTypes = []string{"_a._tcp", "_b._tcp", "_c._tcp", "_d._udp", "_e._tcp"}
	if err := Validate(base, ifaces); err == nil {
		t.Fatal("five service types")
	}
	base.ServiceTypes = base.ServiceTypes[:MaxUserServiceTypes]
	if err := Validate(base, ifaces); err != nil {
		t.Fatal(err)
	}
	base.ServiceTypes = []string{"http._tcp"}
	if err := Validate(base, ifaces); err == nil {
		t.Fatal("bad service type")
	}
	secret, err := NewSecret(strings.Repeat("a", MaxSecretBytes))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSecret(strings.Repeat("a", MaxSecretBytes+1)); err == nil {
		t.Fatal("secret limit")
	}
	snmp := SNMPRequest{Scope: valid[0], Target: "192.0.2.10", Owned: true, Version: "v2c", Community: secret, TimeoutMS: SNMPMinTimeoutMS, Port: 1}
	if err := ValidateSNMP(snmp, ifaces); err != nil {
		t.Fatal(err)
	}
	snmp.TimeoutMS = SNMPMinTimeoutMS - 1
	if err := ValidateSNMP(snmp, ifaces); err == nil {
		t.Fatal("timeout")
	}
	snmp.TimeoutMS = SNMPMaxTimeoutMS + 1
	if err := ValidateSNMP(snmp, ifaces); err == nil {
		t.Fatal("timeout+1")
	}
}

func TestSSDPParseAndFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("location fetched") }))
	t.Cleanup(server.Close)
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	inside := netip.MustParseAddr("192.0.2.10")
	outside := netip.MustParseAddr("198.51.100.10")
	location := server.URL + "/description.xml"
	valid := []byte("HTTP/1.1 200 OK\r\nST: upnp:rootdevice\r\nUSN: uuid:one\r\nSERVER: lab\r\nLOCATION: " + location + "\r\n\r\n")
	dup := []byte("HTTP/1.1 200 OK\r\nUSN: uuid:one\r\nST: upnp:rootdevice\r\n\r\n")
	missing := []byte("HTTP/1.1 404 Not Found\r\nUSN: uuid:no\r\n\r\n")
	out := Result{}
	seen := map[string]struct{}{}
	ingestSSDP(&out, prefix, inside, valid, seen)
	ingestSSDP(&out, prefix, inside, dup, seen)
	ingestSSDP(&out, prefix, inside, missing, seen)
	ingestSSDP(&out, prefix, inside, []byte("not http"), seen)
	ingestSSDP(&out, prefix, inside, make([]byte, MaxDatagramBytes+1), seen)
	ingestSSDP(&out, prefix, outside, valid, seen)
	if len(out.Responders) != 1 || out.Malformed != 3 || out.OutOfScope != 1 || out.Responders[0].Location != location {
		t.Fatalf("%+v", out)
	}
	long := "HTTP/1.1 200 OK\r\nUSN: " + strings.Repeat("u", MaxHeaderValue+1) + "\r\n\r\n"
	ingestSSDP(&out, prefix, inside, []byte(long), seen)
	if len(out.Responders[1].Name) != MaxHeaderValue {
		t.Fatal(len(out.Responders[1].Name))
	}
}

func TestSSDPLoopbackRoundTrip(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	previous := ssdpDest
	ssdpDest = conn.LocalAddr().(*net.UDPAddr)
	t.Cleanup(func() { ssdpDest = previous })
	sender, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sender.Close() })
	payload := ssdpRequest(Request{DurationSeconds: 3, SSDPTarget: "ssdp:all"})
	for i := range SSDPSends {
		if _, err := sender.WriteToUDP(payload, ssdpDest); err != nil {
			t.Fatal(err)
		}
		if i < SSDPSends-1 {
			time.Sleep(SSDPSendGap)
		}
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 2048)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != string(payload) || !strings.Contains(string(payload), "MX: 3\r\n") || !strings.Contains(string(payload), "USER-AGENT: kali-tools/"+domain.Version+" UPnP/1.1\r\n") {
		t.Fatalf("%q", buf[:n])
	}
}

func TestMDNSEntryConversion(t *testing.T) {
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	fields := make([]string, MaxTXTFields+8)
	for i := range fields {
		fields[i] = strings.Repeat("t", MaxTXTBytes+5)
	}
	item, ok := convertEntry("_http._tcp", &mdns.ServiceEntry{
		Name: "Printer", Host: "printer.local", AddrV4: net.IPv4(192, 0, 2, 20), Port: 80, InfoFields: fields,
	}, prefix)
	if !ok || len(item.TXT) != MaxTXTFields || len(item.TXT[0]) != MaxTXTBytes {
		t.Fatalf("%v %+v", ok, item)
	}
	if _, ok := convertEntry("_http._tcp", &mdns.ServiceEntry{Name: "out", AddrV4: net.IPv4(198, 51, 100, 9), Port: 9}, prefix); ok {
		t.Fatal("out of scope kept")
	}
}

func TestResponderBound(t *testing.T) {
	rows := make([]Responder, MaxResponders)
	for i := range rows {
		rows[i] = Responder{Protocol: "ssdp", Address: "192.0.2.10", Name: fmt.Sprintf("n%d", i)}
	}
	exact := finish(Result{Responders: rows})
	if exact.Limited || len(exact.Responders) != MaxResponders {
		t.Fatalf("%d %v", len(exact.Responders), exact.Limited)
	}
	extra := make([]Responder, 0, MaxResponders+1)
	extra = append(extra, rows...)
	extra = append(extra, Responder{Protocol: "ssdp", Address: "192.0.2.11", Name: "extra"})
	over := finish(Result{Responders: extra})
	if !over.Limited || len(over.Responders) != MaxResponders {
		t.Fatalf("%d %v", len(over.Responders), over.Limited)
	}
}

func TestSecretRedaction(t *testing.T) {
	secret, err := NewSecret("synthetic-community")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(secret)
	if err != nil {
		t.Fatal(err)
	}
	text := fmt.Sprintf("%v %+v %#v %s", secret, secret, secret, raw)
	if strings.Contains(text, "synthetic-community") || !strings.Contains(text, "[REDACTED]") {
		t.Fatal(text)
	}
}

func TestSNMPSystemAndInterfaces(t *testing.T) {
	withIfaces(t)
	agent, err := snmpfake.Start("lab-community", SNMPMaxRows+44)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(agent.Close)
	host, port := splitHostPort(t, agent.Addr())
	secret, err := NewSecret("lab-community")
	if err != nil {
		t.Fatal(err)
	}
	report, err := QuerySNMP(t.Context(), SNMPRequest{
		Scope: scope(), Target: host, Port: port, Owned: true, Version: "v2c", Community: secret,
		TimeoutMS: 1000, Interfaces: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range systemKeys {
		if report.System[key] == "" {
			t.Fatal(report.System)
		}
	}
	if report.System["sysName"] != snmpfake.SysName || !report.Limited || len(report.Interfaces) != SNMPMaxRows {
		t.Fatalf("%s limited=%v rows=%d", report.System["sysName"], report.Limited, len(report.Interfaces))
	}
	if secret.reveal() == "" {
		t.Fatal("caller secret was cleared")
	}
}

func TestSNMPErrorsAndRedaction(t *testing.T) {
	withIfaces(t)
	agent, err := snmpfake.Start("lab-community", 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(agent.Close)
	host, port := splitHostPort(t, agent.Addr())
	secret, _ := NewSecret("synthetic-community")
	_, err = QuerySNMP(t.Context(), SNMPRequest{
		Scope: scope(), Target: host, Port: port, Owned: true, Version: "v2c", Community: secret, TimeoutMS: SNMPMinTimeoutMS,
	})
	assertSNMP(t, err, domain.CodeTimeout, "synthetic-community")
	pass, _ := NewSecret("synthetic-pass")
	_, err = QuerySNMP(t.Context(), SNMPRequest{
		Scope: scope(), Target: host, Port: port, Owned: true, Version: "v3", TimeoutMS: SNMPMinTimeoutMS,
		V3: &SNMPv3Creds{User: "owner", AuthProtocol: "SHA256", AuthPass: pass, PrivProtocol: "AES", PrivPass: pass},
	})
	assertSNMP(t, err, domain.CodeUnauthorized, "synthetic-pass")
}

func TestSNMPRequiresOwnership(t *testing.T) {
	withIfaces(t)
	agent, err := snmpfake.Start("lab-community", 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(agent.Close)
	host, port := splitHostPort(t, agent.Addr())
	secret, _ := NewSecret("lab-community")
	_, err = QuerySNMP(t.Context(), SNMPRequest{
		Scope: scope(), Target: host, Port: port, Owned: false, Version: "v2c", Community: secret, TimeoutMS: 1000,
	})
	if err == nil || !strings.Contains(err.Error(), "confirm that you own or administer this device") {
		t.Fatal(err)
	}
	if agent.Requests.Load() != 0 {
		t.Fatal(agent.Requests.Load())
	}
}

func TestReadOnlyGuard(t *testing.T) {
	root := "."
	for _, name := range []string{"snmp.go", "ssdp.go"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if name == "snmp.go" {
			for _, banned := range []string{".Set(", "SendTrap", "WalkAll", ".Walk("} {
				if strings.Contains(text, banned) {
					t.Fatal(banned)
				}
			}
		}
		if name == "ssdp.go" && (strings.Contains(text, "http.Get") || strings.Contains(text, "http.NewRequest")) {
			t.Fatal(text)
		}
	}
}

func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return host, n
}

func ipv4LoopbackName(t *testing.T) string {
	t.Helper()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, ifi := range ifaces {
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if ok && ipnet.IP.To4() != nil && ipnet.IP.IsLoopback() {
				return ifi.Name
			}
		}
	}
	t.Fatal("no IPv4 loopback interface")
	return ""
}

func TestDiscoverOnLoopback(t *testing.T) {
	name := ipv4LoopbackName(t)
	previous := validateInterfaces
	validateInterfaces = func() []network.InterfaceInfo {
		return []network.InterfaceInfo{{Name: name, Up: true, Addresses: []string{"127.0.0.1/8"}}}
	}
	t.Cleanup(func() { validateInterfaces = previous })
	socket, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = socket.Close() })
	dest := ssdpDest
	ssdpDest = socket.LocalAddr().(*net.UDPAddr)
	t.Cleanup(func() { ssdpDest = dest })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := socket.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if !bytes.Contains(buf[:n], []byte("M-SEARCH")) {
				continue
			}
			_, _ = socket.WriteToUDP([]byte("HTTP/1.1 200 OK\r\nUSN: uuid:lab\r\nST: ssdp:all\r\nLOCATION: http://127.0.0.1/desc.xml\r\n\r\n"), addr)
		}
	}()
	query := mdnsQuery
	calls := 0
	mdnsQuery = func(_ context.Context, params *mdns.QueryParam) error {
		if params.Logger == nil {
			t.Error("mdns logger is nil")
		}
		calls++
		if calls > 1 {
			return errors.New("failed to bind")
		}
		params.Entries <- &mdns.ServiceEntry{Name: "outside", AddrV4: net.ParseIP("192.0.2.9")}
		params.Entries <- &mdns.ServiceEntry{Name: "printer", Host: "printer.local.", AddrV4: net.IPv4(127, 0, 0, 1), Port: 9, InfoFields: []string{"ty=lab"}}
		return nil
	}
	t.Cleanup(func() { mdnsQuery = query })
	result, err := Discover(t.Context(), Request{
		Scope: Scope{Interface: name, CIDR: "127.0.0.1/32"}, Protocols: []string{"mdns", "ssdp"}, DurationSeconds: 1, SSDPTarget: "ssdp:all",
	})
	if err != nil || !hasName(result, "printer") || !hasName(result, "uuid:lab") || result.OutOfScope == 0 || !slices.Contains(result.Errors, "mdns_bind_failed") {
		t.Fatalf("%v %+v", err, result)
	}
	if _, code := queryMDNS(t.Context(), Request{Scope: Scope{Interface: "missing", CIDR: "192.0.2.0/24"}, DurationSeconds: 1}); code != "mdns_bind_failed" {
		t.Fatal(code)
	}
	if _, code := querySSDP(t.Context(), Request{Scope: Scope{Interface: "missing", CIDR: "192.0.2.0/24"}, DurationSeconds: 1, SSDPTarget: "ssdp:all"}); code != "ssdp_bind_failed" {
		t.Fatal(code)
	}
	if currentInterfaces() == nil && len(currentInterfaces()) != 0 {
		t.Fatal("interface inventory")
	}
	if !bytes.Contains(ssdpRequest(Request{DurationSeconds: 10, SSDPTarget: "upnp:rootdevice"}), []byte("MX: 5\r\n")) || itoa(0) != "0" {
		t.Fatal("ssdp request")
	}
	if _, err := sourceIPv4(Scope{Interface: name, CIDR: "192.0.2.0/24"}); err == nil {
		t.Fatal("accepted an address outside the scope")
	}
}

func TestMDNSUnicastFallback(t *testing.T) {
	ifi, err := net.InterfaceByName(ipv4LoopbackName(t))
	if err != nil {
		t.Fatal(err)
	}
	socket, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = socket.Close() })
	previousDest := mdnsDest
	mdnsDest = socket.LocalAddr().(*net.UDPAddr)
	t.Cleanup(func() { mdnsDest = previousDest })
	go func() {
		buf := make([]byte, 1500)
		n, addr, err := socket.ReadFromUDP(buf)
		if err != nil {
			return
		}
		req := new(dns.Msg)
		if req.Unpack(buf[:n]) != nil || len(req.Question) != 1 {
			return
		}
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Authoritative = true
		resp.Answer = []dns.RR{
			&dns.PTR{Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 120}, Ptr: "fixture._kali-lab._tcp.local."},
			&dns.SRV{Hdr: dns.RR_Header{Name: "fixture._kali-lab._tcp.local.", Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: 120}, Priority: 0, Weight: 0, Port: 8080, Target: "fixture.local."},
			&dns.A{Hdr: dns.RR_Header{Name: "fixture.local.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120}, A: net.IPv4(127, 0, 0, 1)},
			&dns.TXT{Hdr: dns.RR_Header{Name: "fixture._kali-lab._tcp.local.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 120}, Txt: []string{"ty=lab"}},
		}
		raw, err := resp.Pack()
		if err == nil {
			_, _ = socket.WriteToUDP(raw, addr)
		}
	}()
	previous := mdnsLibrary
	mdnsLibrary = func(context.Context, *mdns.QueryParam) error {
		return errors.New("failed to bind to any multicast udp port")
	}
	t.Cleanup(func() { mdnsLibrary = previous })
	entries := make(chan *mdns.ServiceEntry, 4)
	if err := queryLibrary(t.Context(), &mdns.QueryParam{
		Service: "_kali-lab._tcp", Domain: "local", Timeout: time.Second,
		Interface: ifi, Entries: entries, WantUnicastResponse: true, DisableIPv6: true,
		Logger: nil,
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case entry := <-entries:
		if entry.Name != "fixture._kali-lab._tcp.local." || entry.Port != 8080 || !entry.AddrV4.Equal(net.IPv4(127, 0, 0, 1)) || len(entry.InfoFields) != 1 {
			t.Fatalf("%+v", entry)
		}
	default:
		t.Fatal("no entry")
	}
	if serviceEntry(&dns.Msg{}) != nil {
		t.Fatal("empty response")
	}
	if err := queryLibrary(t.Context(), &mdns.QueryParam{
		Service: "_kali-lab._tcp", Domain: "local", Timeout: time.Millisecond,
		Interface: &net.Interface{}, Entries: entries,
	}); err == nil {
		t.Fatal("expected the library error")
	}
}

func TestSNMPBranches(t *testing.T) {
	withIfaces(t)
	secret, err := NewSecret("lab-community")
	if err != nil {
		t.Fatal(err)
	}
	base := SNMPRequest{Scope: scope(), Target: "127.0.0.1", Owned: true, Version: "v2c", Community: secret, TimeoutMS: 500}
	base.Version = "v1"
	if err := ValidateSNMP(base, labIfaces()); err == nil {
		t.Fatal("version")
	}
	base.Version = "v2c"
	base.Community = Secret{}
	if err := ValidateSNMP(base, labIfaces()); err == nil {
		t.Fatal("community")
	}
	base.Community = secret
	base.Target = "192.0.2.1"
	if err := ValidateSNMP(base, labIfaces()); err == nil {
		t.Fatal("target")
	}
	base.Target = "127.0.0.1"
	base.Port = 70000
	if err := ValidateSNMP(base, labIfaces()); err == nil {
		t.Fatal("port")
	}
	pass, err := NewSecret("synthetic-pass")
	if err != nil {
		t.Fatal(err)
	}
	v3 := base
	v3.Port = 161
	v3.Version = "v3"
	v3.V3 = &SNMPv3Creds{User: "owner", AuthProtocol: "nope", AuthPass: pass}
	if err := ValidateSNMP(v3, labIfaces()); err == nil {
		t.Fatal("auth")
	}
	v3.V3.AuthProtocol = "SHA512"
	v3.V3.PrivPass = pass
	v3.V3.PrivProtocol = "nope"
	if err := ValidateSNMP(v3, labIfaces()); err == nil {
		t.Fatal("priv")
	}
	if authProtocol("SHA") == 0 && authProtocol("SHA512") == 0 && authProtocol("SHA256") == 0 {
		t.Fatal("auth protocol")
	}
	if privacyProtocol("AES256") == privacyProtocol("AES") && privacyProtocol("AES") == 0 {
		t.Fatal("privacy protocol")
	}
	for n := 1; n <= 8; n++ {
		if operStatus(n) == "" {
			t.Fatal(n)
		}
	}
	if snmpValue(gosnmp.SnmpPDU{Type: gosnmp.TimeTicks, Value: 100}) == "" || snmpValue(gosnmp.SnmpPDU{Value: []byte{0, 1}}) == "" || snmpValue(gosnmp.SnmpPDU{Value: 7}) == "" {
		t.Fatal("value")
	}
	long := strings.Repeat("a", SNMPMaxValueBytes+8)
	if len(snmpValue(gosnmp.SnmpPDU{Value: long})) != SNMPMaxValueBytes || macString("ab") == "" || macString(1) != "" {
		t.Fatal("truncate")
	}
	if snmpErr(nil) == nil || snmpErr(errors.New("authorization")) == nil || snmpErr(errors.New("deadline")) == nil || snmpErr(errors.New("other")) == nil {
		t.Fatal("snmp error mapping")
	}
	empty := Request{Scope: scope(), DurationSeconds: 1}
	if err := Validate(empty, labIfaces()); err == nil {
		t.Fatal("protocols")
	}
	empty.Protocols = []string{"bootp"}
	if err := Validate(empty, labIfaces()); err == nil {
		t.Fatal("protocol")
	}
	empty.Protocols = []string{"ssdp"}
	empty.SSDPTarget = "upnp:device"
	if err := Validate(empty, labIfaces()); err == nil {
		t.Fatal("ssdp target")
	}
}

func hasName(result Result, name string) bool {
	for _, item := range result.Responders {
		if item.Name == name {
			return true
		}
	}
	return false
}

func assertSNMP(t *testing.T, err error, code, secret string) {
	t.Helper()
	var got *domain.Error
	if !errors.As(err, &got) || got.Code != code {
		t.Fatal(err)
	}
	text := fmt.Sprintf("%v %+v %s", err, err, got.Message)
	if strings.Contains(text, secret) {
		t.Fatal(text)
	}
}
