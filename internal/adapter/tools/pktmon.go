package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const (
	PktmonMinBuild       = 19041
	PktmonPacketBytes    = 512
	PktmonFileSizeMB     = 48
	PktmonStepTimeout    = 15 * time.Second
	PktmonConvertTimeout = 60 * time.Second
	PktmonOutputLimit    = 64 << 10
)

type PktmonStatus struct {
	Path     string
	Build    uint32
	Elevated bool
}

type commandRunner func(context.Context, string, []string, int) ([]byte, error)

type Pktmon struct {
	mu     sync.Mutex
	status func() (PktmonStatus, error)
	run    commandRunner
	tmp    func() (string, error)
	sleep  func(context.Context, time.Duration)
}

func NewPktmon() *Pktmon {
	status, err := detectPktmon()
	return &Pktmon{
		status: func() (PktmonStatus, error) { return status, err },
		run:    runCommand,
		tmp:    func() (string, error) { return os.MkdirTemp("", "kali-tools-pktmon-") },
		sleep: func(ctx context.Context, duration time.Duration) {
			timer := time.NewTimer(duration)
			defer timer.Stop()
			select {
			case <-ctx.Done():
			case <-timer.C:
			}
		},
	}
}

func pktmonStartArgs(etl string) []string {
	return []string{"start", "--capture", "--comp", "nics", "--pkt-size", "512", "--file-name", etl, "--file-size", "48", "--log-mode", "circular"}
}

func pktmonStopArgs() []string { return []string{"stop"} }

func pktmonConvertArgs(etl, out string, dropOnly bool) []string {
	args := []string{"etl2pcap", etl, "--out", out}
	if dropOnly {
		args = append(args, "--drop-only")
	}
	return args
}

func pktmonCapability(status PktmonStatus) domain.Capability {
	c := domain.Capability{ID: "capture_pktmon", Provider: "pktmon"}
	switch {
	case status.Build < PktmonMinBuild || status.Path == "":
		c.State, c.Detail = domain.CapabilityUnsupported, "Pktmon requires Windows 10 2004 (build 19041) or later"
	case !status.Elevated:
		c.State, c.Detail = domain.CapabilityPermissionRequired, "Pktmon requires an elevated (Administrator) kali-tools process; kali-tools never self-elevates"
	default:
		c.State, c.Detail = domain.CapabilityAvailable, "Built-in Pktmon; all NICs, packets truncated to 512 bytes"
	}
	return c
}

func (p *Pktmon) Status(ctx context.Context) domain.Capability {
	if ctx.Err() != nil {
		return domain.Capability{ID: "capture_pktmon", Provider: "pktmon", State: domain.CapabilityFailed, Detail: "Pktmon detection cancelled"}
	}
	status, err := p.status()
	if err != nil {
		return pktmonDetectionFailure(err)
	}
	return pktmonCapability(status)
}

func (p *Pktmon) command(ctx context.Context, path string, args []string, timeout time.Duration) error {
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := p.run(bounded, path, args, PktmonOutputLimit)
	return err
}

func (p *Pktmon) Capture(ctx context.Context, request capture.LiveRequest) (result capture.LiveResult, artifact *capture.Artifact, captureErr error) {
	result.Packets, result.Limits = []capture.Packet{}, []string{}
	if err := ctx.Err(); err != nil {
		return result, nil, err
	}
	if request.Backend != capture.BackendPktmon {
		return result, nil, domain.Invalid("Pktmon requires the pktmon backend")
	}
	if err := capture.ValidateLiveRequest(request); err != nil {
		return result, nil, err
	}
	if !p.mu.TryLock() {
		return result, nil, &domain.Error{Code: domain.CodeConflict, Message: "a Pktmon capture is already running"}
	}
	defer p.mu.Unlock()
	status, err := p.status()
	if err != nil {
		c := pktmonDetectionFailure(err)
		return result, nil, &domain.Error{Code: c.State, Message: c.Detail}
	}
	if c := pktmonCapability(status); c.State != domain.CapabilityAvailable {
		return result, nil, &domain.Error{Code: c.State, Message: c.Detail}
	}
	dir, err := p.tmp()
	if err != nil {
		return result, nil, pktmonFailed("Pktmon temporary directory could not be created")
	}
	if !filepath.IsAbs(dir) {
		return result, nil, pktmonFailed("Pktmon temporary directory must be absolute")
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.Chmod(dir, 0700); err != nil {
		return result, nil, pktmonFailed("Pktmon temporary directory could not be secured")
	}
	etl, out, drops := filepath.Join(dir, "capture.etl"), filepath.Join(dir, "capture.pcapng"), filepath.Join(dir, "drops.pcapng")
	result.Started = time.Now().UTC()
	if err := p.command(ctx, status.Path, pktmonStartArgs(etl), PktmonStepTimeout); err != nil {
		return result, nil, pktmonFailed("pktmon start failed; another Pktmon session may be active (check with 'pktmon status' in an elevated prompt)")
	}
	stopped := false
	stop := func() error {
		if stopped {
			return nil
		}
		stopped = true
		err := p.command(context.WithoutCancel(ctx), status.Path, pktmonStopArgs(), PktmonStepTimeout)
		result.Finished = time.Now().UTC()
		if err != nil {
			return pktmonFailed("pktmon stop failed; run 'pktmon stop' in an elevated prompt")
		}
		return nil
	}
	defer func() {
		if err := stop(); err != nil {
			captureErr = err
		}
	}()
	p.sleep(ctx, time.Duration(request.DurationSeconds)*time.Second)
	if err := stop(); err != nil {
		return result, nil, err
	}
	if err := ctx.Err(); err != nil {
		return result, nil, err
	}
	if err := p.command(ctx, status.Path, pktmonConvertArgs(etl, out, false), PktmonConvertTimeout); err != nil {
		return result, nil, pktmonFailed("Pktmon output conversion failed")
	}
	imported, retained, err := capture.ReadArtifactFile(out)
	if err != nil {
		return result, nil, pktmonFailed("Pktmon output could not be imported")
	}
	result.Packets, result.Limited = imported.Packets, imported.Limited
	if len(result.Packets) > request.PacketLimit {
		result.Packets = result.Packets[:request.PacketLimit]
		result.Limited = true
	}
	result.Provider = fmt.Sprintf("Windows Pktmon (build %d)", status.Build)
	result.Limits = []string{capture.LimitCircularBuffer, capture.LimitTruncated, capture.LimitComponentIdentity, capture.LimitAllNICs, capture.LimitDuplicates}
	if err := p.command(ctx, status.Path, pktmonConvertArgs(etl, drops, true), PktmonConvertTimeout); err == nil {
		if dropped, err := capture.ReadFile(drops); err == nil && !dropped.Limited {
			count := uint64(len(dropped.Packets))
			result.Dropped = &count
		}
	}
	if result.Dropped == nil {
		result.Limits = append(result.Limits, capture.LimitDropCountMissing)
	}
	if err := ctx.Err(); err != nil {
		return result, retained, err
	}
	return result, retained, nil
}

func pktmonFailed(message string) error {
	return &domain.Error{Code: domain.CodeFailed, Message: message}
}
