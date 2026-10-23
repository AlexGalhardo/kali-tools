//go:build darwin

package capture

import (
	"context"
	"os"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const maxBPFDevices = 256

func (Native) Status(ctx context.Context) domain.Capability {
	if ctx.Err() != nil {
		return domain.Capability{ID: "capture_live", Provider: "bpf", State: domain.CapabilityFailed, Detail: "BPF access check cancelled"}
	}
	count, readable, err := probeBPF()
	if err != nil {
		return domain.Capability{ID: "capture_live", Provider: "bpf", State: domain.CapabilityFailed, Detail: "BPF device list could not be read"}
	}
	return bpfAccessCapability(count, readable)
}

func live(_ context.Context, _ LiveRequest, _ *Artifact) (LiveResult, error) {
	return LiveResult{Packets: []Packet{}, Limits: []string{}}, domain.Unsupported("live capture provider is not implemented on this platform")
}

func probeBPF() (int, bool, error) {
	entries, err := os.ReadDir("/dev")
	if err != nil {
		return 0, false, err
	}
	devices := 0
	readable := false
	for _, entry := range entries {
		if devices >= maxBPFDevices {
			break
		}
		name := "/dev/" + entry.Name()
		if !bpfDevicePath(name) {
			continue
		}
		info, err := os.Lstat(name)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeDevice == 0 {
			continue
		}
		devices++
		if readable {
			continue
		}
		file, err := os.OpenFile(name, os.O_RDONLY, 0)
		if err != nil {
			continue
		}
		_ = file.Close()
		readable = true
	}
	return devices, readable, nil
}
