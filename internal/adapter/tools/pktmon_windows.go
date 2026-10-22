//go:build windows

package tools

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func detectPktmon() (PktmonStatus, error) {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		return PktmonStatus{}, err
	}
	status := PktmonStatus{Build: windows.RtlGetVersion().BuildNumber, Elevated: windows.GetCurrentProcessToken().IsElevated()}
	path := filepath.Join(dir, "PktMon.exe")
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
		status.Path = path
	}
	return status, nil
}

func pktmonDetectionFailure(error) domain.Capability {
	return domain.Capability{ID: "capture_pktmon", Provider: "pktmon", State: domain.CapabilityFailed, Detail: "Pktmon detection failed"}
}
