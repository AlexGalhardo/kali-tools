package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestToolHelperProcess(t *testing.T) {
	if os.Getenv("KALI_TOOLS_TEST_PROCESS") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	switch mode {
	case "ok":
		fmt.Print("synthetic-output")
		os.Exit(0)
	case "fail":
		fmt.Fprint(os.Stderr, "synthetic-private-output")
		os.Exit(7)
	case "overflow":
		fmt.Print(strings.Repeat("x", 65536))
		time.Sleep(10 * time.Second)
	case "sleep":
		time.Sleep(10 * time.Second)
	case "parent":
		cmd := exec.Command(os.Args[0], "-test.run=^TestToolHelperProcess$", "--", "sleep")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			os.Exit(8)
		}
		fmt.Printf("child=%d\n", cmd.Process.Pid)
		_ = cmd.Wait()
	}
	os.Exit(0)
}

func TestCommandBoundsAndErrors(t *testing.T) {
	t.Setenv("KALI_TOOLS_TEST_PROCESS", "1")
	for _, tt := range []struct {
		mode    string
		limit   int
		wantErr bool
	}{
		{"ok", 1024, false}, {"fail", 1024, true}, {"overflow", 1024, true},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			out, err := runCommand(ctx, os.Args[0], []string{"-test.run=^TestToolHelperProcess$", "--", tt.mode}, tt.limit)
			if (err != nil) != tt.wantErr || len(out) > tt.limit {
				t.Fatalf("output bytes=%d err=%v", len(out), err)
			}
			if tt.mode == "ok" && string(out) != "synthetic-output" {
				t.Fatalf("%q", out)
			}
			if err != nil && strings.Contains(err.Error(), "synthetic-private") {
				t.Fatal("stderr leaked")
			}
			if tt.mode == "overflow" && (err == nil || err.Error() != "tool output limit exceeded") {
				t.Fatalf("overflow: %v", err)
			}
		})
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	start := time.Now()
	_, err := runCommand(ctx, os.Args[0], []string{"-test.run=^TestToolHelperProcess$", "--", "sleep"}, 1024)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 3*time.Second {
		t.Fatalf("deadline: %v", err)
	}
}
