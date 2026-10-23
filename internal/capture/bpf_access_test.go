package capture

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestBPFDevicePath(t *testing.T) {
	for _, test := range []struct {
		name string
		ok   bool
	}{
		{"/dev/bpf0", true},
		{"/dev/bpf12", true},
		{"/dev/bpf", false},
		{"/dev/bpf0a", false},
		{"/tmp/bpf0", false},
		{"/dev/bpf0/extra", false},
	} {
		if got := bpfDevicePath(test.name); got != test.ok {
			t.Fatalf("%s: %v", test.name, got)
		}
	}
}

func TestBPFAccessCapability(t *testing.T) {
	missing := bpfAccessCapability(0, false)
	if missing.State != domain.CapabilityUnsupported || !strings.Contains(missing.Detail, "No /dev/bpf") || strings.Contains(missing.Detail, "chmod") {
		t.Fatal(missing)
	}
	denied := bpfAccessCapability(2, false)
	if denied.ID != "capture_live" || denied.State != domain.CapabilityPermissionRequired || !strings.Contains(denied.Detail, "access_bpf") || strings.Contains(denied.Detail, "chmod") {
		t.Fatal(denied)
	}
	opened := bpfAccessCapability(1, true)
	if opened.State != domain.CapabilityUnsupported || !strings.Contains(opened.Detail, "can open /dev/bpf") || strings.Contains(opened.Detail, "chmod") {
		t.Fatal(opened)
	}
}

func TestLinuxCaptureNamesSetcap(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip()
	}
	got := (Native{}).Status(t.Context())
	if got.State != domain.CapabilityUnsupported || !strings.Contains(got.Detail, "sudo setcap cap_net_raw+eip") || strings.Contains(strings.ToLower(got.Detail), "helper") {
		t.Fatal(got)
	}
}

func TestBPFProbeDoesNotChangePermissions(t *testing.T) {
	raw, err := os.ReadFile("bpf_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"Chmod", "Chown", "Lchown", "Setuid", "Setgid", "syscall.Chmod"} {
		if strings.Contains(string(raw), word) {
			t.Fatalf("bpf probe contains %s", word)
		}
	}
}
