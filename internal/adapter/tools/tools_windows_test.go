//go:build windows

package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKnownToolPathOnlyPinsTShark(t *testing.T) {
	if knownToolPath("other") != "" {
		t.Fatal("only tshark is pinned")
	}
	got := knownToolPath("tshark")
	if got == "" {
		return
	}
	if filepath.Base(got) != "tshark.exe" {
		t.Fatalf("unexpected path %q", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("pinned path does not exist: %v", err)
	}
}
