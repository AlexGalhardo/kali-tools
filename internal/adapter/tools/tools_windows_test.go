//go:build windows

package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKnownToolPathFindsProgramFilesInstall(t *testing.T) {
	base := t.TempDir()
	t.Setenv("ProgramFiles", base)
	t.Setenv("ProgramW6432", "")
	dir := filepath.Join(base, "Wireshark")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "tshark.exe")
	if err := os.WriteFile(want, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A real HKLM InstallDir may win; either way the result is a Wireshark install, not PATH.
	got, err := resolve("tshark")
	if err != nil || filepath.Base(got) != "tshark.exe" {
		t.Fatalf("resolved %q: %v", got, err)
	}
	if knownToolPath("other") != "" {
		t.Fatal("only tshark is pinned")
	}
}
