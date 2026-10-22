package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestPktmonArguments(t *testing.T) {
	if got := pktmonStartArgs("C:/private/capture.etl"); !reflect.DeepEqual(got, []string{"start", "--capture", "--comp", "nics", "--pkt-size", "512", "--file-name", "C:/private/capture.etl", "--file-size", "48", "--log-mode", "circular"}) {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(pktmonStopArgs(), []string{"stop"}) {
		t.Fatal("stop argv")
	}
	for _, drops := range []bool{false, true} {
		want := []string{"etl2pcap", "in", "--out", "out"}
		if drops {
			want = append(want, "--drop-only")
		}
		if got := pktmonConvertArgs("in", "out", drops); !reflect.DeepEqual(got, want) {
			t.Fatal(got)
		}
	}
}

func pktmonFixture(t *testing.T) (*Pktmon, *[][]string, *string) {
	t.Helper()
	var calls [][]string
	var dir string
	p := &Pktmon{status: func() (PktmonStatus, error) {
		return PktmonStatus{Path: filepath.Join(t.TempDir(), "PktMon.exe"), Build: 19041, Elevated: true}, nil
	},
		tmp: func() (string, error) {
			var err error
			dir, err = os.MkdirTemp(t.TempDir(), "session-")
			return dir, err
		},
		sleep: func(context.Context, time.Duration) {},
	}
	p.run = func(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
		if !filepath.IsAbs(path) || limit != PktmonOutputLimit {
			t.Fatal(path, limit)
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > PktmonConvertTimeout {
			t.Fatal("unbounded command")
		}
		calls = append(calls, append([]string(nil), args...))
		if args[0] == "etl2pcap" {
			count := 3
			if len(args) == 5 {
				count = 2
			}
			a := capture.NewArtifact(layers.LinkTypeEthernet)
			data := make([]byte, 60)
			for range count {
				if err := a.Add(data, gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: len(data), Length: len(data)}); err != nil {
					t.Fatal(err)
				}
			}
			if err := a.Save(ctx, args[3], "pcapng"); err != nil {
				t.Fatal(err)
			}
		}
		return nil, nil
	}
	return p, &calls, &dir
}

func TestPktmonSessionAndCleanup(t *testing.T) {
	p, calls, dir := pktmonFixture(t)
	result, artifact, err := p.Capture(t.Context(), capture.LiveRequest{Backend: capture.BackendPktmon, DurationSeconds: 1, PacketLimit: 2})
	if err != nil || artifact == nil || len(result.Packets) != 2 || !result.Limited || result.Dropped == nil || *result.Dropped != 2 {
		t.Fatalf("%+v artifact=%v err=%v", result, artifact != nil, err)
	}
	if len(*calls) != 4 || (*calls)[0][0] != "start" || (*calls)[1][0] != "stop" || !reflect.DeepEqual(result.Limits, []string{capture.LimitCircularBuffer, capture.LimitTruncated, capture.LimitComponentIdentity, capture.LimitAllNICs, capture.LimitDuplicates}) {
		t.Fatal(*calls, result)
	}
	if _, err := os.Stat(*dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("temporary capture retained", err)
	}
	if result.Finished.Before(result.Started) || !strings.Contains(result.Provider, "19041") {
		t.Fatal(result)
	}
}

func TestPktmonCancellationStopsExactlyOnce(t *testing.T) {
	p, calls, dir := pktmonFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	p.sleep = func(context.Context, time.Duration) { cancel() }
	run := p.run
	p.run = func(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
		if args[0] == "stop" && ctx.Err() != nil {
			t.Fatal("stop inherited cancellation")
		}
		return run(ctx, path, args, limit)
	}
	_, _, err := p.Capture(ctx, capture.LiveRequest{Backend: capture.BackendPktmon, DurationSeconds: 1, PacketLimit: 1})
	if !errors.Is(err, context.Canceled) || len(*calls) != 2 || (*calls)[1][0] != "stop" {
		t.Fatal(err, *calls)
	}
	if _, err := os.Stat(*dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancellation retained files")
	}
}

func TestPktmonFailureOwnershipAndRedaction(t *testing.T) {
	for _, step := range []string{"start", "stop", "convert", "drops", "import"} {
		t.Run(step, func(t *testing.T) {
			p, calls, dir := pktmonFixture(t)
			run := p.run
			p.run = func(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
				fail := args[0] == step || (step == "convert" && args[0] == "etl2pcap" && len(args) == 4) || (step == "drops" && len(args) == 5)
				if fail {
					*calls = append(*calls, args)
					return nil, errors.New("private-payload private-path")
				}
				if step == "import" && args[0] == "etl2pcap" {
					return nil, nil
				}
				return run(ctx, path, args, limit)
			}
			result, _, err := p.Capture(t.Context(), capture.LiveRequest{Backend: capture.BackendPktmon, DurationSeconds: 1, PacketLimit: 10})
			if step == "drops" {
				if err != nil || result.Dropped != nil || result.Limits[len(result.Limits)-1] != capture.LimitDropCountMissing {
					t.Fatal(result, err)
				}
			} else if err == nil || strings.Contains(err.Error(), "private-") {
				t.Fatal(err)
			}
			stops := 0
			for _, args := range *calls {
				if args[0] == "stop" {
					stops++
				}
			}
			want := 1
			if step == "start" {
				want = 0
			}
			if stops != want {
				t.Fatal("stopped unowned session or stopped twice", *calls)
			}
			if _, err := os.Stat(*dir); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("files retained")
			}
		})
	}
}

func TestPktmonConflictAndValidation(t *testing.T) {
	p, calls, _ := pktmonFixture(t)
	p.mu.Lock()
	_, _, err := p.Capture(t.Context(), capture.LiveRequest{Backend: capture.BackendPktmon, DurationSeconds: 1, PacketLimit: 1})
	p.mu.Unlock()
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeConflict {
		t.Fatal(err)
	}
	for _, request := range []capture.LiveRequest{{}, {Backend: capture.BackendPktmon, Address: "127.0.0.1", DurationSeconds: 1, PacketLimit: 1}} {
		if _, _, err := p.Capture(t.Context(), request); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if len(*calls) != 0 {
		t.Fatal("invalid input executed command")
	}
}

func TestPktmonStatusBoundaries(t *testing.T) {
	for _, test := range []struct {
		build    uint32
		path     string
		elevated bool
		want     string
	}{
		{19040, "absolute", true, "unsupported"}, {19041, "", true, "unsupported"}, {19041, "absolute", false, "permission_required"}, {19041, "absolute", true, "available"},
	} {
		got := pktmonCapability(PktmonStatus{Path: test.path, Build: test.build, Elevated: test.elevated})
		if got.State != test.want {
			t.Fatal(got)
		}
	}
}

func TestPktmonStatusAndEarlyFailures(t *testing.T) {
	native := NewPktmon()
	if got := native.Status(t.Context()); got.ID != "capture_pktmon" || got.State == "" {
		t.Fatal(got)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	native.sleep(ctx, time.Hour)
	native.sleep(t.Context(), time.Nanosecond)
	if native.Status(ctx).State != domain.CapabilityFailed {
		t.Fatal("cancelled status ignored")
	}
	if _, _, err := native.Capture(ctx, capture.LiveRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	dir, err := native.tmp()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	request := capture.LiveRequest{Backend: capture.BackendPktmon, DurationSeconds: 1, PacketLimit: 1}
	for _, failure := range []string{"status", "unavailable", "tmp", "relative", "chmod"} {
		t.Run(failure, func(t *testing.T) {
			p, calls, _ := pktmonFixture(t)
			switch failure {
			case "status":
				p.status = func() (PktmonStatus, error) { return PktmonStatus{}, errors.New("private-path") }
				if got := p.Status(t.Context()); strings.Contains(got.Detail, "private") {
					t.Fatal(got)
				}
			case "unavailable":
				p.status = func() (PktmonStatus, error) { return PktmonStatus{}, nil }
				if got := p.Status(t.Context()); got.State != domain.CapabilityUnsupported {
					t.Fatal(got)
				}
			case "tmp":
				p.tmp = func() (string, error) { return "", errors.New("private-path") }
			case "relative":
				p.tmp = func() (string, error) { return "relative-path", nil }
			case "chmod":
				p.tmp = func() (string, error) { return filepath.Join(t.TempDir(), "missing"), nil }
			}
			if _, _, err := p.Capture(t.Context(), request); err == nil || strings.Contains(err.Error(), "private-path") {
				t.Fatal(err)
			}
			if len(*calls) != 0 {
				t.Fatal("early failure started capture")
			}
		})
	}
	p, _, _ := pktmonFixture(t)
	if p.Status(t.Context()).State != domain.CapabilityAvailable {
		t.Fatal("available status missing")
	}
}
