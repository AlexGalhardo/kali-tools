package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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

func TestFlagErrorsDoNotEchoValues(t *testing.T) {
	for _, flag := range []string{"--port", "--timeout"} {
		t.Run(flag, func(t *testing.T) {
			var out, stderr bytes.Buffer
			code := cli.Run(t.Context(), nil, []string{"--json", "diagnose", flag, "synthetic-secret"}, &out, &stderr)
			var result envelope
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if code != 2 || result.Error == nil || result.Error.Code != "invalid_input" ||
				strings.Contains(out.String()+stderr.String(), "synthetic-secret") {
				t.Fatalf("exit=%d, stdout=%s, stderr=%s", code, out.String(), stderr.String())
			}
		})
	}
}

func TestPerCommandFlagsAndHelp(t *testing.T) {
	for _, test := range []struct {
		args []string
		want int
	}{
		{[]string{"version", "--port", "1"}, 2},
		{[]string{"version", "--help"}, 0},
		{[]string{"--help"}, 0},
		{[]string{"--json", "version"}, 0},
		{[]string{"version", "--lang", "fr"}, 2},
		{[]string{"diagnose", "--detach"}, 2},
	} {
		var out, stderr bytes.Buffer
		if got := cli.Run(t.Context(), nil, test.args, &out, &stderr); got != test.want {
			t.Fatalf("%v: exit=%d, want=%d; stderr=%s", test.args, got, test.want, stderr.String())
		}
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	var list, stderr bytes.Buffer
	if code := cli.Run(t.Context(), nil, []string{"help"}, &list, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	for _, name := range []string{"version", "doctor", "interfaces", "inventory", "diagnose", "discover", "inspect", "wifi", "capture", "import"} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(list.String(), name) {
				t.Fatal("missing command", name)
			}
			for _, args := range [][]string{{name, "--help"}, {"help", name}} {
				var out bytes.Buffer
				if cli.NeedsApp(args) || cli.Run(t.Context(), nil, args, &out, &stderr) != 0 || !strings.Contains(out.String(), name) {
					t.Fatal("invalid command help", args)
				}
			}
		})
	}
}

func TestTimeoutParsing(t *testing.T) {
	for _, test := range []struct {
		value string
		want  int
	}{
		{"100", 0}, {"100ms", 0}, {"600000", 0}, {"10m", 0},
		{"99", 2}, {"600001", 2}, {"10m1s", 2}, {"NaN", 2},
		{"9223372036854775807", 2}, {"-1", 2},
	} {
		t.Run(test.value, func(t *testing.T) {
			var out, stderr bytes.Buffer
			if got := cli.Run(t.Context(), nil, []string{"version", "--timeout", test.value}, &out, &stderr); got != test.want {
				t.Fatalf("exit=%d, want %d", got, test.want)
			}
		})
	}
}

func TestLegacyInvocationsStillParse(t *testing.T) {
	content, err := os.ReadFile("testdata/legacy_invocations.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		fields := strings.Fields(line)
		want, err := strconv.Atoi(fields[0])
		if err != nil {
			t.Fatal(err)
		}
		var out, stderr bytes.Buffer
		if got := cli.Run(t.Context(), newApp(t), fields[2:], &out, &stderr); got != want {
			t.Fatalf("%s: exit=%d, want=%d", line, got, want)
		}
		var response envelope
		if err := json.Unmarshal(out.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if fields[1] != "-" && (response.Error == nil || response.Error.Code != fields[1]) {
			t.Fatal("legacy error code changed", line)
		}
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
