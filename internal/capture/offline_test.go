package capture

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
)

func synthetic(t *testing.T) []byte {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, SrcIP: []byte{192, 0, 2, 1}, DstIP: []byte{192, 0, 2, 2}, Protocol: layers.IPProtocolUDP}
	udp := &layers.UDP{SrcPort: 12345, DstPort: 53}
	if e := udp.SetNetworkLayerForChecksum(ip); e != nil {
		t.Fatal(e)
	}
	dns := &layers.DNS{ID: 1, Questions: []layers.DNSQuestion{{Name: []byte("example.test"), Type: layers.DNSTypeA, Class: layers.DNSClassIN}}}
	if e := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, ip, udp, dns); e != nil {
		t.Fatal(e)
	}
	return buf.Bytes()
}
func TestPCAPAndPCAPNG(t *testing.T) {
	data := synthetic(t)
	ci := gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: len(data), Length: len(data)}
	for _, format := range []string{"pcap", "pcapng"} {
		var b bytes.Buffer
		if format == "pcap" {
			writer := pcapgo.NewWriter(&b)
			if e := writer.WriteFileHeader(65536, layers.LinkTypeRaw); e != nil {
				t.Fatal(e)
			}
			if e := writer.WritePacket(ci, data); e != nil {
				t.Fatal(e)
			}
		} else {
			writer, e := pcapgo.NewNgWriter(&b, layers.LinkTypeRaw)
			if e != nil {
				t.Fatal(e)
			}
			if e = writer.WritePacket(ci, data); e != nil {
				t.Fatal(e)
			}
			if e = writer.Flush(); e != nil {
				t.Fatal(e)
			}
		}
		result, e := Read(&b)
		if e != nil || len(result.Packets) != 1 || result.Packets[0].Protocol != "DNS" || result.Packets[0].Source != "192.0.2.1" {
			t.Fatalf("%s %+v %v", format, result, e)
		}
	}
}
func TestOversizedDeclaredRecord(t *testing.T) {
	var b bytes.Buffer
	writer := pcapgo.NewWriter(&b)
	if e := writer.WriteFileHeader(65536, layers.LinkTypeRaw); e != nil {
		t.Fatal(e)
	}
	raw := append(b.Bytes(), make([]byte, 16)...)
	binary.LittleEndian.PutUint32(raw[32:36], 0x7fffffff)
	if _, e := Read(bytes.NewReader(raw)); e == nil {
		t.Fatal("oversized packet accepted")
	}
}
func FuzzCapture(f *testing.F) {
	f.Add([]byte{0xd4, 0xc3, 0xb2, 0xa1})
	f.Add([]byte{0x0a, 0x0d, 0x0d, 0x0a})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2<<20 {
			t.Skip()
		}
		_, _ = Read(bytes.NewReader(data))
	})
}

func TestLinkAndICMPSummaries(t *testing.T) {
	ethernet := &layers.Ethernet{SrcMAC: []byte{2, 0, 0, 0, 0, 1}, DstMAC: []byte{2, 0, 0, 0, 0, 2}, EthernetType: layers.EthernetTypeARP}
	cases := []struct {
		name                                string
		link                                layers.LinkType
		layers                              []gopacket.SerializableLayer
		protocol, source, destination, info string
	}{
		{"ethernet", layers.LinkTypeEthernet, []gopacket.SerializableLayer{ethernet, &layers.ARP{AddrType: layers.LinkTypeEthernet, Protocol: layers.EthernetTypeIPv4, HwAddressSize: 6, ProtAddressSize: 4, Operation: layers.ARPRequest, SourceHwAddress: ethernet.SrcMAC, SourceProtAddress: []byte{192, 0, 2, 1}, DstHwAddress: ethernet.DstMAC, DstProtAddress: []byte{192, 0, 2, 2}}}, "Ethernet", "02:00:00:00:00:01", "02:00:00:00:00:02", ""},
		{"icmp4", layers.LinkTypeRaw, []gopacket.SerializableLayer{
			&layers.IPv4{Version: 4, IHL: 5, TTL: 64, SrcIP: []byte{192, 0, 2, 1}, DstIP: []byte{192, 0, 2, 2}, Protocol: layers.IPProtocolICMPv4},
			&layers.ICMPv4{TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0)},
		}, "ICMPv4", "192.0.2.1", "192.0.2.2", "EchoRequest"},
		{"icmp6", layers.LinkTypeRaw, []gopacket.SerializableLayer{
			&layers.IPv6{Version: 6, HopLimit: 64, SrcIP: []byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, DstIP: []byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}, NextHeader: layers.IPProtocolICMPv6},
			&layers.ICMPv6{TypeCode: layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0)},
			&layers.ICMPv6Echo{},
		}, "ICMPv6", "2001:db8::1", "2001:db8::2", "EchoRequest"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			b := gopacket.NewSerializeBuffer()
			if err := gopacket.SerializeLayers(b, gopacket.SerializeOptions{FixLengths: true}, tt.layers...); err != nil {
				t.Fatal(err)
			}
			got, err := Decode(b.Bytes(), tt.link, gopacket.CaptureInfo{Length: len(b.Bytes())}, 1)
			if err != nil || got.Protocol != tt.protocol || got.Source != tt.source || got.Destination != tt.destination || got.Info != tt.info {
				t.Fatalf("got %+v, err %v", got, err)
			}
		})
	}
}

func TestTruncationIPv6ExtensionsAndNoReassembly(t *testing.T) {
	full := synthetic(t)
	short := full[:len(full)-4]
	got, err := Decode(short, layers.LinkTypeRaw, gopacket.CaptureInfo{CaptureLength: len(short), Length: len(full)}, 1)
	if err != nil || (!strings.Contains(got.Info, "truncated") && !strings.Contains(got.Info, "Truncated")) || strings.Contains(got.Info, "example.test") {
		t.Fatalf("truncated decode: %+v %v", got, err)
	}
	hop := &layers.IPv6HopByHop{NextHeader: layers.IPProtocolICMPv6, Options: []*layers.IPv6HopByHopOption{{OptionType: 1, OptionData: []byte{0, 0, 0, 0}}}}
	icmp := []gopacket.SerializableLayer{
		&layers.IPv6{Version: 6, HopLimit: 64, SrcIP: []byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, DstIP: []byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}, NextHeader: layers.IPProtocolIPv6HopByHop},
		hop,
		&layers.ICMPv6{TypeCode: layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0)},
		&layers.ICMPv6Echo{},
	}
	buf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true}, icmp...); err != nil {
		t.Fatal(err)
	}
	got, err = Decode(buf.Bytes(), layers.LinkTypeRaw, gopacket.CaptureInfo{CaptureLength: len(buf.Bytes()), Length: len(buf.Bytes())}, 2)
	if err != nil || got.Protocol != "ICMPv6" || got.Source != "2001:db8::1" || !strings.Contains(got.Info, "EchoRequest") {
		t.Fatalf("hop-by-hop: %+v %v", got, err)
	}
	secret := []byte("synthetic-secret-payload")
	frag := []gopacket.SerializableLayer{
		&layers.IPv6{Version: 6, HopLimit: 64, SrcIP: []byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, DstIP: []byte{0x20, 1, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}, NextHeader: layers.IPProtocolIPv6Fragment},
		&layers.IPv6Fragment{NextHeader: layers.IPProtocolUDP, FragmentOffset: 1, MoreFragments: true, Identification: 7},
		gopacket.Payload(secret),
	}
	buf = gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true}, frag...); err != nil {
		t.Fatal(err)
	}
	got, err = Decode(buf.Bytes(), layers.LinkTypeRaw, gopacket.CaptureInfo{CaptureLength: len(buf.Bytes()), Length: len(buf.Bytes()) + 8}, 3)
	if err != nil || strings.Contains(got.Info, "synthetic-secret") || got.Protocol == "UDP" {
		t.Fatalf("fragment was reassembled: %+v %v", got, err)
	}
	parts := []string{"GET /syn", "thetic-secret"}
	for _, part := range parts {
		ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, SrcIP: []byte{192, 0, 2, 1}, DstIP: []byte{192, 0, 2, 2}, Protocol: layers.IPProtocolTCP}
		tcp := &layers.TCP{SrcPort: 12345, DstPort: 80, DataOffset: 5}
		one := gopacket.NewSerializeBuffer()
		if err := gopacket.SerializeLayers(one, gopacket.SerializeOptions{FixLengths: true}, ip, tcp, gopacket.Payload([]byte(part))); err != nil {
			t.Fatal(err)
		}
		got, err = Decode(one.Bytes(), layers.LinkTypeRaw, gopacket.CaptureInfo{Length: len(one.Bytes())}, 4)
		if err != nil || strings.Contains(got.Info, "thetic") || strings.Contains(got.Info, "secret") {
			t.Fatalf("segment %q leaked payload: %+v %v", part, got, err)
		}
		if part == "thetic-secret" && got.Protocol == "HTTP" {
			t.Fatalf("second segment reassembled into HTTP: %+v", got)
		}
	}
}

func TestCorruptedCaptureRejected(t *testing.T) {
	header := []byte{0xd4, 0xc3, 0xb2, 0xa1, 2, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0}
	packet := make([]byte, 16)
	binary.LittleEndian.PutUint32(packet[8:12], 4)
	binary.LittleEndian.PutUint32(packet[12:16], 2)
	for _, raw := range [][]byte{nil, {1, 2, 3}, header[:20], append(header, packet...)} {
		if _, err := Read(bytes.NewReader(raw)); err == nil {
			t.Fatalf("accepted %d bytes", len(raw))
		}
	}
}

func TestHTTPAndPortMetadata(t *testing.T) {
	for _, tt := range []struct{ payload, protocol, info string }{
		{"PATCH /private HTTP/1.1\r\n", "HTTP", "Plaintext HTTP message (payload omitted) Port 443 (encryption not verified)"},
		{"OPTIONS /private HTTP/1.1\r\n", "HTTP", "Plaintext HTTP message (payload omitted) Port 443 (encryption not verified)"},
		{"private-payload", "TCP", "Port 443 (encryption not verified)"},
	} {
		ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, SrcIP: []byte{192, 0, 2, 1}, DstIP: []byte{192, 0, 2, 2}, Protocol: layers.IPProtocolTCP}
		tcp := &layers.TCP{SrcPort: 12345, DstPort: 443, DataOffset: 5}
		b := gopacket.NewSerializeBuffer()
		if err := gopacket.SerializeLayers(b, gopacket.SerializeOptions{FixLengths: true}, ip, tcp, gopacket.Payload(tt.payload)); err != nil {
			t.Fatal(err)
		}
		got, err := Decode(b.Bytes(), layers.LinkTypeRaw, gopacket.CaptureInfo{Length: len(b.Bytes())}, 1)
		if err != nil || got.Protocol != tt.protocol || !strings.Contains(got.Info, tt.info) || strings.Contains(got.Info, "private") {
			t.Fatalf("got %+v, err %v", got, err)
		}
	}
}
