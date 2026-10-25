//go:build windows

package tools

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

func knownToolPath(name string) string {
	if name != "tshark" {
		return ""
	}
	var dirs []string
	if key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Wireshark`, registry.QUERY_VALUE); err == nil {
		if dir, _, err := key.GetStringValue("InstallDir"); err == nil {
			dirs = append(dirs, dir)
		}
		_ = key.Close()
	}
	for _, env := range []string{"ProgramFiles", "ProgramW6432"} {
		if base := os.Getenv(env); base != "" {
			dirs = append(dirs, filepath.Join(base, "Wireshark"))
		}
	}
	for _, dir := range dirs {
		candidate := filepath.Join(dir, "tshark.exe")
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}
