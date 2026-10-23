package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
)

func TestCaptureBackendStatusIsOrderedAndDoesNotCapture(t *testing.T) {
	deps := fakeDeps()
	deps.CaptureBackends = map[string]app.CaptureBackend{
		capture.BackendPktmon: fake.LiveCapture{Capability: domain.Capability{ID: "capture_pktmon", State: "permission_required", Detail: "needs administrator"}, CaptureFunc: func(context.Context, capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
			t.Fatal("status started capture")
			return capture.LiveResult{}, nil, nil
		}},
	}
	a := serviceWithDeps(t, deps)
	got := a.CaptureBackendStatus(t.Context())
	if len(got) != 3 || got[0].ID != "capture_live" || got[1].ID != "capture_pktmon" || got[2].ID != "capture_npcap" || got[0].State != "unsupported" || got[1].State != "permission_required" {
		t.Fatal(got)
	}
	count := 0
	for _, c := range a.Capabilities(t.Context()) {
		if c.ID == "capture_live" || c.ID == "capture_pktmon" || c.ID == "capture_npcap" {
			count++
		}
	}
	if count != 3 {
		t.Fatal("missing or duplicate capabilities", count)
	}
}

func TestStartCaptureSelectsBackendAndRefusesUnavailable(t *testing.T) {
	for _, state := range []string{"available", "permission_required", "dependency_missing", "unsupported"} {
		t.Run(state, func(t *testing.T) {
			var calls atomic.Int32
			provider := fake.LiveCapture{
				Capability: domain.Capability{ID: "capture_pktmon", State: state, Detail: "synthetic state"},
				CaptureFunc: func(context.Context, capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
					calls.Add(1)
					dropped := uint64(2)
					return capture.LiveResult{Packets: []capture.Packet{{Number: 1}}, Dropped: &dropped, Limits: []string{capture.LimitAllNICs}}, nil, nil
				},
			}
			deps := fakeDeps()
			deps.CaptureBackends = map[string]app.CaptureBackend{capture.BackendPktmon: provider}
			a := serviceWithDeps(t, deps)
			clear(deps.CaptureBackends) // constructor owns its backend registry
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			events, _ := a.Subscribe(ctx)
			err := a.StartCapture(ctx, capture.LiveRequest{Backend: "pktmon", DurationSeconds: 1, PacketLimit: 1})
			if state != "available" {
				var failure *domain.Error
				if !errors.As(err, &failure) || failure.Code != state || calls.Load() != 0 {
					t.Fatalf("error=%v calls=%d", err, calls.Load())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for {
				select {
				case event := <-events:
					if event.Topic != job.TopicCapture {
						continue
					}
					rows, err := a.History(ctx)
					if err != nil || len(rows) != 1 || rows[0].Summary != "Capture pktmon completed: 1 packets, 2 dropped" {
						t.Fatalf("history=%v err=%v", rows, err)
					}
					raw, err := json.Marshal(rows)
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(string(raw), "synthetic state") || calls.Load() != 1 {
						t.Fatal("capture history leaked details or wrong backend called")
					}
					return
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
		})
	}
}

func newService(t *testing.T) *app.Application {
	return serviceWithDeps(t, fakeDeps())
}

func serviceWithDeps(t *testing.T, deps app.Deps) *app.Application {
	t.Helper()
	s := app.New(deps)
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
	expectMessage(t, s.CanExportCapture(t.Context(), "pcap"), "import or finish a capture before saving")
	expectMessage(t, s.CanExportCapture(t.Context(), "txt"), "capture format must be pcap or pcapng")
	source := captureFixture(t)
	result, err := s.ImportCapture(t.Context(), source)
	if err != nil || len(result.Packets) != 1 {
		t.Fatalf("import = %+v, err = %v", result, err)
	}
	if _, err := s.ImportCapture(t.Context(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing input accepted")
	}
	if err := s.CanExportCapture(t.Context(), "pcap"); err != nil {
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
	expectMessage(t, s.CanImportCapture(t.Context()), "application is shutting down")
}

func captureAddress(t *testing.T) string {
	t.Helper()
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
	return address
}

func TestCaptureInterfaceLossStaysOutOfHistory(t *testing.T) {
	deps := fakeDeps()
	deps.CaptureBackends = map[string]app.CaptureBackend{capture.BackendPktmon: fake.LiveCapture{
		Capability: domain.Capability{State: domain.CapabilityAvailable, Detail: "fixture"},
		Result:     capture.LiveResult{Limited: true, Limits: []string{capture.LimitTruncated}, Packets: []capture.Packet{{Info: "synthetic-psk"}}},
		Err:        &domain.Error{Code: domain.CodeFailed, Message: "capture interface disappeared"},
	}}
	s := serviceWithDeps(t, deps)
	events, _ := s.Subscribe(t.Context())
	if err := s.StartCapture(t.Context(), capture.LiveRequest{Backend: capture.BackendPktmon, DurationSeconds: 1, PacketLimit: 1}); err != nil {
		t.Fatal(err)
	}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	var event job.Event
	for event.Topic != job.TopicCapture {
		select {
		case event = <-events:
		case <-timer.C:
			t.Fatal("capture completion missing")
		}
	}
	outcome, ok := event.Data.(app.CaptureEvent)
	if !ok || outcome.Error != "failed: capture interface disappeared" || strings.Contains(outcome.Error, "synthetic-psk") {
		t.Fatalf("event = %+v", outcome)
	}
	history, err := s.History(t.Context())
	if err != nil || len(history) != 1 || history[0].State != job.Failed || strings.Contains(history[0].Summary, "synthetic-psk") {
		t.Fatalf("history = %+v %v", history, err)
	}
}

func TestCaptureSessionStates(t *testing.T) {
	address := captureAddress(t)
	deps := fakeDeps()
	started := make(chan struct{})
	deps.CaptureBackends = map[string]app.CaptureBackend{capture.BackendRawSocket: fakeCapture{CaptureFunc: func(ctx context.Context, r capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
		close(started)
		<-ctx.Done()
		return capture.LiveResult{Packets: []capture.Packet{}}, nil, ctx.Err()
	}}}
	s := serviceWithDeps(t, deps)
	events, _ := s.Subscribe(t.Context())
	request := capture.LiveRequest{Address: address, DurationSeconds: 1, PacketLimit: 1}
	if err := s.StartCapture(t.Context(), capture.LiveRequest{}); err == nil {
		t.Fatal("invalid request accepted")
	}
	if err := s.StartCapture(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("capture did not start")
	}
	expectMessage(t, s.StartCapture(t.Context(), request), "capture already running")
	expectMessage(t, s.CanImportCapture(t.Context()), "stop capture before importing a file")
	if _, err := s.ImportCapture(t.Context(), "unused"); err == nil {
		t.Fatal("import accepted during capture")
	}
	if err := s.CanExportCapture(t.Context(), "pcap"); err == nil {
		t.Fatal("export accepted during capture")
	}
	s.StopCapture(t.Context())
	var event job.Event
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for event.Topic != job.TopicCapture {
		select {
		case event = <-events:
		case <-timer.C:
			t.Fatal("capture completion missing")
		}
	}
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
	select {
	case extra := <-events:
		t.Fatalf("duplicate capture event: %+v", extra)
	default:
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	expectMessage(t, s.StartCapture(t.Context(), request), "application is shutting down")
}
