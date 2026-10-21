package capture

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

type record struct {
	info gopacket.CaptureInfo
	data []byte
}

// Artifact is an in-memory session; payloads are written only by an explicit export.
type Artifact struct {
	mu      sync.Mutex
	link    layers.LinkType
	records []record
	size    int
}

func NewArtifact(link layers.LinkType) *Artifact { return &Artifact{link: link} }

func (a *Artifact) Add(data []byte, ci gopacket.CaptureInfo) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(data) > 1<<20 || ci.CaptureLength != len(data) || ci.Length < len(data) || ci.Length > 1<<20 {
		return domain.Invalid("invalid capture record length")
	}
	// Reserve enough space for PCAPNG headers and per-packet framing.
	if len(a.records) >= MaxPackets || a.size+len(data) > MaxFileSize-MaxPackets*64-4096 {
		return domain.Invalid("capture artifact reached its bounded memory limit")
	}
	a.records = append(a.records, record{ci, append([]byte(nil), data...)})
	a.size += len(data)
	return nil
}

func (a *Artifact) Write(ctx context.Context, out io.Writer, format string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	var write func(gopacket.CaptureInfo, []byte) error
	var flush func() error
	switch format {
	case "pcap":
		writer := pcapgo.NewWriterNanos(out)
		if err := writer.WriteFileHeader(1<<20, a.link); err != nil {
			return err
		}
		write = writer.WritePacket
	case "pcapng":
		writer, err := pcapgo.NewNgWriter(out, a.link)
		if err != nil {
			return err
		}
		write = writer.WritePacket
		flush = writer.Flush
	default:
		return domain.Invalid("capture format must be pcap or pcapng")
	}
	for _, r := range a.records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if r.info.Timestamp.Unix() < 0 || r.info.Timestamp.Unix() > 1<<32-1 {
			return domain.Invalid("capture timestamp is outside the supported export range")
		}
		ci := r.info
		ci.InterfaceIndex = 0
		if err := write(ci, r.data); err != nil {
			return err
		}
	}
	if flush != nil {
		return flush()
	}
	return ctx.Err()
}

// Save writes beside the destination first, preserving an existing file on failure.
func (a *Artifact) Save(ctx context.Context, path, format string) (err error) {
	if format != "pcap" && format != "pcapng" {
		return domain.Invalid("capture format must be pcap or pcapng")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".kali-tools-capture-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer func() { _ = f.Close(); _ = os.Remove(temp) }()
	if err = a.Write(ctx, f, format); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func ReadArtifactFile(path string) (ImportResult, *Artifact, error) {
	f, err := os.Open(path)
	if err != nil {
		return ImportResult{}, nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return ImportResult{}, nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > MaxFileSize {
		return ImportResult{}, nil, domain.Invalid("capture must be a regular file of at most 64 MiB")
	}
	var artifact *Artifact
	result, err := read(io.LimitReader(f, MaxFileSize+1), func(data []byte, ci gopacket.CaptureInfo, link layers.LinkType) error {
		if artifact == nil {
			artifact = NewArtifact(link)
		}
		return artifact.Add(data, ci)
	})
	if err != nil {
		return result, nil, err
	}
	return result, artifact, nil
}
