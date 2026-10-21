package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"path/filepath"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func newService(t *testing.T) *app.Service {
	t.Helper()
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	s := app.New(store, network.Diagnostics{}, nil)
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func expectMessage(t *testing.T, err error, message string) {
	t.Helper()
	var typed *domain.Error
	if !errors.As(err, &typed) || typed.Code != "invalid_input" || typed.Message != message {
		t.Fatalf("error = %v, want invalid_input with message %q", err, message)
	}
}

func captureFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.pcapng")
	artifact := capture.NewArtifact(layers.LinkTypeRaw)
	ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, SrcIP: []byte{192, 0, 2, 1}, DstIP: []byte{192, 0, 2, 2}, Protocol: layers.IPProtocolTCP}
	tcp := &layers.TCP{SrcPort: 12345, DstPort: 80, DataOffset: 5}
	buffer := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buffer, gopacket.SerializeOptions{FixLengths: true}, ip, tcp, gopacket.Payload("synthetic-private")); err != nil {
		t.Fatal(err)
	}
	if err := artifact.Add(buffer.Bytes(), gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: len(buffer.Bytes()), Length: len(buffer.Bytes())}); err != nil {
		t.Fatal(err)
	}
	if err := artifact.Save(t.Context(), path, "pcapng"); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCaptureImportExportRoundTrip(t *testing.T) {
	s := newService(t)
	expectMessage(t, s.CanExportCapture("pcap"), "import or finish a capture before saving")
	expectMessage(t, s.CanExportCapture("txt"), "capture format must be pcap or pcapng")
	source := captureFixture(t)
	result, err := s.ImportCapture(source)
	if err != nil || len(result.Packets) != 1 {
		t.Fatalf("import = %+v, err = %v", result, err)
	}
	if _, err := s.ImportCapture(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing input accepted")
	}
	if err := s.CanExportCapture("pcap"); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "output.pcap")
	if err := s.ExportCapture(t.Context(), destination, "pcap"); err != nil {
		t.Fatal(err)
	}
	imported, err := capture.ReadFile(destination)
	if err != nil || len(imported.Packets) != 1 || imported.Format != "pcap" {
		t.Fatalf("round trip = %+v, err = %v", imported, err)
	}
	if err := s.ExportCapture(t.Context(), destination, "txt"); err == nil {
		t.Fatal("invalid export accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	expectMessage(t, s.CanImportCapture(), "application is shutting down")
}

func TestCaptureSessionStates(t *testing.T) {
	s := newService(t)
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	var address string
	for _, candidate := range addresses {
		prefix, err := netip.ParsePrefix(candidate.String())
		if err == nil && prefix.Addr().Is4() && !prefix.Addr().IsLoopback() {
			address = prefix.Addr().String()
			break
		}
	}
	if address == "" {
		t.Skip("no local non-loopback IPv4 for capture validation")
	}
	started := make(chan struct{})
	s.Live = func(ctx context.Context, r capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
		close(started)
		<-ctx.Done()
		return capture.LiveResult{Packets: []capture.Packet{}}, nil, ctx.Err()
	}
	events, _ := s.Jobs.Subscribe(t.Context())
	request := capture.LiveRequest{Address: address, DurationSeconds: 1, PacketLimit: 1}
	if err := s.StartCapture(capture.LiveRequest{}); err == nil {
		t.Fatal("invalid request accepted")
	}
	if err := s.StartCapture(request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("capture did not start")
	}
	expectMessage(t, s.StartCapture(request), "capture already running")
	expectMessage(t, s.CanImportCapture(), "stop capture before importing a file")
	if _, err := s.ImportCapture("unused"); err == nil {
		t.Fatal("import accepted during capture")
	}
	if err := s.CanExportCapture("pcap"); err == nil {
		t.Fatal("export accepted during capture")
	}
	s.StopCapture()
	select {
	case event := <-events:
		outcome, ok := event.Data.(app.CaptureEvent)
		if event.Topic != job.TopicCapture || !ok || !errors.Is(outcome.Err, context.Canceled) {
			t.Fatalf("event = %+v", event)
		}
		raw, err := json.Marshal(outcome)
		if err != nil {
			t.Fatal(err)
		}
		var shape map[string]json.RawMessage
		if err := json.Unmarshal(raw, &shape); err != nil {
			t.Fatal(err)
		}
		if len(shape) != 2 || shape["result"] == nil || shape["error"] == nil {
			t.Fatalf("capture JSON = %s", raw)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("capture completion missing")
	}
	select {
	case extra := <-events:
		t.Fatalf("duplicate capture event: %+v", extra)
	default:
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	expectMessage(t, s.StartCapture(request), "application is shutting down")
}
