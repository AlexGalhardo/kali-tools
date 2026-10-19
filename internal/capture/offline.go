package capture

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/AlexGalhardo/hacker-tools/internal/domain"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
)

const MaxFileSize = 64 << 20
const MaxPackets = 10000

type Packet struct {
	Number      int       `json:"number"`
	Timestamp   time.Time `json:"timestamp"`
	Length      int       `json:"length"`
	Source      string    `json:"source"`
	Destination string    `json:"destination"`
	Protocol    string    `json:"protocol"`
	Info        string    `json:"info"`
}
type ImportResult struct {
	Packets []Packet `json:"packets"`
	Limited bool     `json:"limited"`
	Format  string   `json:"format"`
}

func Decode(data []byte, link layers.LinkType, ci gopacket.CaptureInfo, n int) (Packet, error) {
	if len(data) > 1<<20 {
		return Packet{}, domain.Invalid("packet exceeds 1 MiB")
	}
	p := gopacket.NewPacket(data, link, gopacket.DecodeOptions{Lazy: false, NoCopy: true})
	r := Packet{Number: n, Timestamp: ci.Timestamp, Length: ci.Length, Protocol: "unknown"}
	if l := p.NetworkLayer(); l != nil {
		r.Source = l.NetworkFlow().Src().String()
		r.Destination = l.NetworkFlow().Dst().String()
		r.Protocol = l.LayerType().String()
	}
	if l := p.TransportLayer(); l != nil {
		r.Protocol = l.LayerType().String()
		r.Info = l.TransportFlow().String()
	}
	if d := p.Layer(layers.LayerTypeDNS); d != nil {
		r.Protocol = "DNS"
		for _, q := range d.(*layers.DNS).Questions {
			if len(r.Info) > 512 {
				break
			}
			r.Info += fmt.Sprintf(" %s %s", q.Type, string(q.Name))
		}
	}
	if t := p.Layer(layers.LayerTypeTCP); t != nil {
		tcp := t.(*layers.TCP)
		payload := string(tcp.Payload)
		for _, method := range []string{"GET ", "POST ", "HEAD ", "PUT ", "DELETE ", "HTTP/"} {
			if strings.HasPrefix(payload, method) {
				r.Protocol = "HTTP"
				r.Info = "Plaintext HTTP message (payload omitted)"
				break
			}
		}
		if tcp.SrcPort == 443 || tcp.DstPort == 443 {
			r.Info += " TLS/encrypted application data"
		}
	}
	if p.ErrorLayer() != nil {
		r.Info = "Truncated or malformed packet"
	}
	return r, nil
}
func ReadFile(path string) (ImportResult, error) {
	f, e := os.Open(path)
	if e != nil {
		return ImportResult{}, e
	}
	defer f.Close()
	stat, e := f.Stat()
	if e != nil {
		return ImportResult{}, e
	}
	if !stat.Mode().IsRegular() || stat.Size() > MaxFileSize {
		return ImportResult{}, domain.Invalid("capture must be a regular file of at most 64 MiB")
	}
	return Read(io.LimitReader(f, MaxFileSize+1))
}
func Read(input io.Reader) (ImportResult, error) {
	raw, e := io.ReadAll(io.LimitReader(input, MaxFileSize+1))
	if e != nil {
		return ImportResult{}, e
	}
	if len(raw) > MaxFileSize {
		return ImportResult{}, domain.Invalid("capture exceeds 64 MiB")
	}
	if e = preflight(raw); e != nil {
		return ImportResult{}, e
	}
	r := bufio.NewReader(bytes.NewReader(raw))
	magic, e := r.Peek(4)
	if e != nil {
		return ImportResult{}, e
	}
	out := ImportResult{Packets: []Packet{}, Format: "pcap"}
	var read func() ([]byte, gopacket.CaptureInfo, error)
	var link layers.LinkType
	if binary.LittleEndian.Uint32(magic) == 0x0a0d0d0a {
		out.Format = "pcapng"
		reader, e := pcapgo.NewNgReader(r, pcapgo.NgReaderOptions{ErrorOnMismatchingLinkType: true})
		if e != nil {
			return out, e
		}
		read = reader.ReadPacketData
		link = reader.LinkType()
	} else {
		reader, e := pcapgo.NewReader(r)
		if e != nil {
			return out, e
		}
		if reader.Snaplen() > 1<<20 {
			return out, domain.Invalid("snaplen exceeds 1 MiB")
		}
		read = reader.ReadPacketData
		link = reader.LinkType()
	}
	for n := 1; n <= MaxPackets+1; n++ {
		data, ci, e := read()
		if e == io.EOF {
			return out, nil
		}
		if e != nil {
			return out, e
		}
		if n > MaxPackets {
			out.Limited = true
			break
		}
		packet, e := Decode(data, link, ci, n)
		if e != nil {
			return out, e
		}
		out.Packets = append(out.Packets, packet)
	}
	return out, nil
}
