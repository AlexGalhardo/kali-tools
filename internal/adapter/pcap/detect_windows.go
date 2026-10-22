//go:build windows

package pcap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func isElevated() bool { return windows.GetCurrentProcessToken().IsElevated() }

func DetectNpcap() (NpcapStatus, error) {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		return NpcapStatus{}, err
	}
	p := npcapProbe{WpcapDLL: regularFile(filepath.Join(dir, "Npcap", "wpcap.dll")),
		PacketDLL: regularFile(filepath.Join(dir, "Npcap", "Packet.dll")), LegacyWpcap: regularFile(filepath.Join(dir, "wpcap.dll"))}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\npcap`, registry.QUERY_VALUE)
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return NpcapStatus{}, err
	}
	if err == nil {
		p.ServiceKey = true
		defer func() { _ = key.Close() }()
		parameters, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\npcap\Parameters`, registry.QUERY_VALUE)
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			return NpcapStatus{}, err
		}
		if err == nil {
			defer func() { _ = parameters.Close() }()
			if value, _, err := parameters.GetIntegerValue("AdminOnly"); err == nil && value == 1 {
				p.AdminOnly = 1
			}
			if value, _, err := parameters.GetIntegerValue("LoopbackSupport"); err == nil && value == 1 {
				p.LoopbackSupport = 1
			}
		}
	}
	if p.PacketDLL {
		p.Version = fileVersion(filepath.Join(dir, "Npcap", "Packet.dll"))
	}
	return statusFromProbe(p), nil
}

func regularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func fileVersion(path string) string {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil || !validVersionInfoSize(size) {
		return ""
	}
	buffer := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buffer[0])); err != nil {
		return ""
	}
	var value *windows.VS_FIXEDFILEINFO
	var length uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buffer[0]), `\`, unsafe.Pointer(&value), &length); err != nil {
		return ""
	}
	if value == nil || length < uint32(unsafe.Sizeof(windows.VS_FIXEDFILEINFO{})) {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d.%d", value.FileVersionMS>>16, value.FileVersionMS&0xffff, value.FileVersionLS>>16, value.FileVersionLS&0xffff)
}
