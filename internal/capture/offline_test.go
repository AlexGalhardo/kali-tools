package capture

import (
	"bytes"
	"encoding/binary"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
	"testing"
	"time"
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
