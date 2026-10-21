package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
)

func TestExitCodeTable(t *testing.T) {
	for _, test := range []struct {
		code string
		want int
	}{
		{"", 0}, {"failed", 1}, {"no_connection", 1}, {"certificate_invalid", 1},
		{"invalid_input", 2}, {"unsupported", 3}, {"permission_required", 4},
		{"location_consent_required", 4}, {"dependency_missing", 5}, {"not_found", 6},
		{"timeout", 7}, {"conflict", 8}, {"unauthorized", 9}, {"forbidden", 9},
		{"cancelled", 130}, {"unknown", 1},
	} {
		t.Run(test.code, func(t *testing.T) {
			if got := cli.ExitCode(test.code); got != test.want {
				t.Fatalf("exit=%d, want %d", got, test.want)
			}
		})
	}
}

func TestCLIMapsUnknownErrorsGenerically(t *testing.T) {
	var out, stderr bytes.Buffer
	path := filepath.Join(t.TempDir(), "missing-capture.pcap")
	code := cli.Run(t.Context(), newApp(t), []string{"import", "--file", path}, &out, &stderr)
	var result envelope
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if code != 1 || result.Error == nil || result.Error.Code != "failed" || result.Error.Message != "Operation failed" || strings.Contains(out.String(), path) {
		t.Fatalf("exit=%d out=%s", code, out.String())
	}
}

func TestCLIContractCommandMappings(t *testing.T) {
	application := newApp(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, args := range [][]string{{"doctor"}, {"inventory"}, {"wifi"}, {"inspect", "--target", "192.0.2.0/24"}, {"discover", "--ports", "bad"}} {
		var out, stderr bytes.Buffer
		code := cli.Run(ctx, application, args, &out, &stderr)
		var response envelope
		if err := json.Unmarshal(out.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if args[0] == "doctor" {
			if code != 0 || response.Error != nil || !strings.Contains(out.String(), `"capabilities"`) {
				t.Fatal(out.String())
			}
			continue
		}
		if code == 0 || response.Error == nil {
			t.Fatalf("%v: %s", args, out.String())
		}
	}
}
