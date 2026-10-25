//go:build windows

package tools

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
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
	// Known-folder lookup instead of os.Getenv: env reads are confined to composition roots.
	if base, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, 0); err == nil {
		dirs = append(dirs, filepath.Join(base, "Wireshark"))
	}
	for _, dir := range dirs {
		candidate := filepath.Join(dir, "tshark.exe")
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}
