//go:build windows

package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsToolDescendantCancellation(t *testing.T) {
	t.Setenv("KALI_TOOLS_TEST_PROCESS", "1")
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	out, err := runCommand(ctx, os.Args[0], []string{"-test.run=^TestToolHelperProcess$", "--", "parent"}, 1024)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancel: %s %v", out, err)
	}
	var pid uint32
	if _, err := fmt.Sscanf(string(out), "child=%d", &pid); err != nil || pid == 0 {
		t.Fatalf("missing descendant PID: %q", out)
	}
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = windows.CloseHandle(process) }()
	status, err := windows.WaitForSingleObject(process, 1000)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("descendant survived: %d %v", status, err)
	}
}
