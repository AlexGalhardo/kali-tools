package capture

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
)

func TestCommittedCaptureFixtures(t *testing.T) {
	for _, format := range []string{"pcap", "pcapng"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join("..", "..", "tests", "fixtures", "synthetic."+format)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "synthetic-secret") {
				t.Fatal("capture fixture contains a secret")
			}
			summary, retained, err := ReadArtifactFile(path)
			if err != nil || retained == nil || len(summary.Packets) != 1 || summary.Packets[0].Protocol != "DNS" || summary.Packets[0].Source != "192.0.2.1" {
				t.Fatalf("import %+v, %v", summary, err)
			}
			out := filepath.Join(t.TempDir(), "again."+format)
			if err = retained.Save(t.Context(), out, format); err != nil {
				t.Fatal(err)
			}
			again, _, err := ReadArtifactFile(out)
			if err != nil || len(again.Packets) != 1 || again.Packets[0].Protocol != "DNS" || again.Packets[0].Source != "192.0.2.1" {
				t.Fatalf("re-export %+v, %v", again, err)
			}
		})
	}
}

func TestArtifactRoundTripAndOwnedBytes(t *testing.T) {
	data := synthetic(t)
	original := append([]byte(nil), data...)
	ci := gopacket.CaptureInfo{Timestamp: time.Unix(100, 123456789), CaptureLength: len(data), Length: len(data), InterfaceIndex: 9}
	artifact := NewArtifact(layers.LinkTypeRaw)
	if err := artifact.Add(data, ci); err != nil {
		t.Fatal(err)
	}
	clear(data)
	for _, format := range []string{"pcap", "pcapng"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session."+format)
			if err := artifact.Save(t.Context(), path, format); err != nil {
				t.Fatal(err)
			}
			summary, retained, err := ReadArtifactFile(path)
			if err != nil || len(summary.Packets) != 1 || summary.Packets[0].Protocol != "DNS" || retained == nil {
				t.Fatalf("import %+v, %v", summary, err)
			}
			var b bytes.Buffer
			if err = retained.Write(t.Context(), &b, "pcap"); err != nil {
				t.Fatal(err)
			}
			reader, err := pcapgo.NewReader(&b)
			if err != nil {
				t.Fatal(err)
			}
			got, info, err := reader.ReadPacketData()
			if err != nil || !bytes.Equal(got, original) || !info.Timestamp.Equal(ci.Timestamp) || info.Length != ci.Length {
				t.Fatalf("packet changed: %+v, %v", info, err)
			}
		})
	}
}

func TestArtifactFailedSavePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.pcap")
	original := []byte("existing-user-file")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	a := NewArtifact(layers.LinkTypeRaw)
	data := synthetic(t)
	if err := a.Add(data, gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: len(data), Length: len(data)}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := a.Save(ctx, path, "pcap"); !errors.Is(err, context.Canceled) {
		t.Fatalf("save: %v", err)
	}
	if err := a.Save(t.Context(), path, "unsupported"); err == nil {
		t.Fatal("accepted invalid format")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("destination changed: %q %v", got, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary artifact leaked: %v %v", files, err)
	}
}

func TestArtifactBoundsAndWriterFailure(t *testing.T) {
	a := NewArtifact(layers.LinkTypeRaw)
	if err := a.Add([]byte{1}, gopacket.CaptureInfo{CaptureLength: 2, Length: 2}); err == nil {
		t.Fatal("invalid record accepted")
	}
	if err := a.Add(make([]byte, (1<<20)+1), gopacket.CaptureInfo{}); err == nil {
		t.Fatal("oversized record accepted")
	}
	ci := gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: 1, Length: 1}
	for range MaxPackets {
		if err := a.Add([]byte{0}, ci); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Add([]byte{0}, ci); err == nil {
		t.Fatal("unbounded records")
	}
	for _, format := range []string{"pcap", "pcapng"} {
		if err := a.Write(t.Context(), failingWriter{}, format); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("lost writer error: %v", err)
		}
		if err := a.Write(t.Context(), enospcWriter{}, format); !errors.Is(err, syscall.ENOSPC) {
			t.Fatalf("lost disk-full error: %v", err)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type enospcWriter struct{}

func (enospcWriter) Write([]byte) (int, error) { return 0, syscall.ENOSPC }
