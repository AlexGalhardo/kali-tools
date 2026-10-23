package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
)

type hintFailureWriter struct{ calls int }

type closedOutput struct{}

func (closedOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWiFiOutputWriteFailures(t *testing.T) {
	for _, test := range []struct {
		name         string
		json, denied bool
	}{
		{"JSON consent envelope", true, true},
		{"human consent message", false, true},
		{"human metadata", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, provider := revealApplication(t)
			if test.denied {
				provider.Err = &domain.Error{Code: domain.CodeLocationConsentRequired, Message: "Windows location access is off"}
			}
			args := []string{"wifi", "show"}
			if test.json {
				args = append([]string{"--json"}, args...)
			}
			if code := cli.Run(t.Context(), a, args, closedOutput{}, closedOutput{}); code != cli.ExitFailed {
				t.Fatalf("exit=%d", code)
			}
		})
	}
}

func (w *hintFailureWriter) Write(value []byte) (int, error) {
	w.calls++
	if w.calls == 2 {
		return 0, io.ErrClosedPipe
	}
	return len(value), nil
}

func TestConsentHintWriteFailure(t *testing.T) {
	a, provider := revealApplication(t)
	provider.Err = &domain.Error{Code: domain.CodeLocationConsentRequired, Message: "Windows location access is off"}
	var stderr hintFailureWriter
	if code := cli.Run(t.Context(), a, []string{"wifi", "show"}, io.Discard, &stderr); code != cli.ExitFailed || stderr.calls != 2 {
		t.Fatalf("exit=%d writes=%d", code, stderr.calls)
	}
}

func TestCLIWiFiConsentEnvelope(t *testing.T) {
	a, provider := revealApplication(t)
	provider.Network.SSID = "synthetic-ssid"
	provider.Network.BSSID = "synthetic-bssid"
	provider.Network.Profile = "synthetic-profile"
	provider.Err = &domain.Error{Code: domain.CodeLocationConsentRequired, Message: "Windows location access is off; open " + domain.LocationSettingsURI}
	for _, jsonMode := range []bool{true, false} {
		var out, stderr bytes.Buffer
		args := []string{"wifi", "show"}
		if jsonMode {
			args = append([]string{"--json"}, args...)
		}
		code := cli.Run(t.Context(), a, args, &out, &stderr)
		if code != cli.ExitPermission {
			t.Fatalf("exit=%d", code)
		}
		if jsonMode {
			var body struct {
				Error struct {
					Code        string `json:"code"`
					SettingsURI string `json:"settings_uri"`
				} `json:"error"`
			}
			if err := json.Unmarshal(out.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != "location_consent_required" || body.Error.SettingsURI != domain.LocationSettingsURI {
				t.Fatal(out.String())
			}
		} else if !strings.Contains(stderr.String(), "Open: "+domain.LocationSettingsURI) {
			t.Fatal("missing settings hint", stderr.String())
		}
		for _, secret := range []string{"synthetic-ssid", "synthetic-bssid", "synthetic-profile", "synthetic-test-key"} {
			if strings.Contains(out.String()+stderr.String(), secret) {
				t.Fatal("consent output contains Wi-Fi data")
			}
		}
	}
}

func TestCLIWiFiConsentRevealDoesNotPrintProviderKey(t *testing.T) {
	a, provider := revealApplication(t)
	provider.Err = &domain.Error{Code: domain.CodeLocationConsentRequired, Message: "Windows location access is off"}
	for _, args := range [][]string{{"--json", "wifi", "reveal", "--confirm"}, {"--lang", "pt-BR", "wifi", "reveal", "--confirm"}} {
		var out, stderr bytes.Buffer
		if code := cli.Run(t.Context(), a, args, &out, &stderr); code != cli.ExitPermission {
			t.Fatal(code)
		}
		if strings.Contains(out.String()+stderr.String(), "synthetic-test-key") {
			t.Fatal("error response leaked provider key")
		}
		if args[0] == "--lang" && !strings.Contains(stderr.String(), "Abrir: "+domain.LocationSettingsURI) {
			t.Fatal("missing translated hint")
		}
	}
}

func TestExitCodePerErrorCode(t *testing.T) {
	for _, code := range domain.ErrorCodes() {
		t.Run(code, func(t *testing.T) {
			store := fake.NewStore()
			a := app.New(app.Deps{Jobs: store, Settings: store, WiFi: fake.WiFi{Err: &domain.Error{Code: code, Message: "synthetic provider state"}}})
			defer func() { _ = a.Close() }()
			var out, stderr bytes.Buffer
			got := runJSON(t.Context(), a, []string{"wifi", "show"}, &out, &stderr)
			var env envelope
			if err := json.Unmarshal(out.Bytes(), &env); err != nil {
				t.Fatal(err)
			}
			if got != cli.ExitCode(code) || env.Error == nil || env.Error.Code != code {
				t.Fatalf("exit=%d stdout=%s stderr=%s", got, &out, &stderr)
			}
		})
	}
}

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

func TestCLIDocCoversCommands(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "cli.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range cli.CommandPathsForTest() {
		if !strings.Contains(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n## "+command+"\n") {
			t.Error("missing command heading", command)
		}
	}
}

func TestNoColorWithoutTerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	var stderr bytes.Buffer
	code := cli.Run(t.Context(), newTestApp(t), []string{"wifi", "show"}, w, &stderr)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	if code != 0 || strings.Contains(out.String()+stderr.String(), "\x1b") {
		t.Fatalf("exit=%d out=%s stderr=%s", code, &out, &stderr)
	}
}

func TestFlagErrorsDoNotEchoValues(t *testing.T) {
	for _, flag := range []string{"--port", "--timeout"} {
		t.Run(flag, func(t *testing.T) {
			var out, stderr bytes.Buffer
			code := runJSON(t.Context(), nil, []string{"--json", "diagnose", flag, "synthetic-secret"}, &out, &stderr)
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
	for _, args := range [][]string{{"discover", "--ports", "synthetic-secret"}, {"diagnose", "http", "--target", "http://user:synthetic-secret@localhost/"}} {
		var out, stderr bytes.Buffer
		code := runJSON(t.Context(), newTestApp(t), args, &out, &stderr)
		if code != 2 || strings.Contains(out.String()+stderr.String(), "synthetic-secret") {
			t.Fatalf("exit=%d stdout=%s stderr=%s", code, &out, &stderr)
		}
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
		if got := runJSON(t.Context(), nil, test.args, &out, &stderr); got != test.want {
			t.Fatalf("%v: exit=%d, want=%d; stderr=%s", test.args, got, test.want, stderr.String())
		}
	}
}

func TestDetachRejected(t *testing.T) {
	for _, command := range []string{"version", "doctor", "capabilities", "interfaces", "inventory", "neighbors", "connections", "throughput", "diagnose tcp", "discover", "inspect", "wifi show", "wifi reveal", "capture live", "capture import", "capture export", "jobs list", "jobs show", "jobs cancel", "history list", "history clear", "settings get", "settings set", "tools", "serve"} {
		t.Run(command, func(t *testing.T) {
			var out, stderr bytes.Buffer
			args := append([]string{"--json"}, strings.Fields(command)...)
			args = append(args, "--detach")
			code := cli.Run(t.Context(), nil, args, &out, &stderr)
			if code != 2 || !strings.Contains(out.String(), "no daemon mode") {
				t.Fatalf("exit=%d out=%s stderr=%s", code, &out, &stderr)
			}
		})
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	var list, stderr bytes.Buffer
	if code := runJSON(t.Context(), nil, []string{"help"}, &list, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	for _, name := range []string{"version", "doctor", "interfaces", "inventory", "diagnose", "discover", "inspect", "wifi", "capture", "import", "capabilities", "tools", "neighbors", "connections", "throughput", "wifi show", "jobs list", "jobs show", "history list", "settings get", "jobs cancel", "history clear", "settings set", "wifi reveal", "capture live", "capture import", "capture export", "serve"} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(list.String(), name) {
				t.Fatal("missing command", name)
			}
			for _, args := range [][]string{append(strings.Fields(name), "--help"), append([]string{"help"}, strings.Fields(name)...)} {
				var out bytes.Buffer
				if cli.NeedsApp(args) || runJSON(t.Context(), nil, args, &out, &stderr) != 0 || !strings.Contains(out.String(), name) {
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
			if got := runJSON(t.Context(), nil, []string{"version", "--timeout", test.value}, &out, &stderr); got != test.want {
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
		if got := runJSON(t.Context(), newTestApp(t), fields[2:], &out, &stderr); got != want {
			t.Fatalf("%s: exit=%d, want=%d", line, got, want)
		}
		var response envelope
		if err := json.Unmarshal(out.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if fields[1] != "-" && (response.Error == nil || response.Error.Code != fields[1]) {
			t.Fatal("legacy error code changed", line)
		}
		warnings := 0
		if fields[2] == "wifi" || fields[2] == "capture" || fields[2] == "import" ||
			strings.Contains(line, "--kind") || strings.Contains(line, "--format json") {
			warnings = 1
		}
		if strings.Count(stderr.String(), "is deprecated") != warnings {
			t.Fatalf("%s: expected %d deprecation warnings; stderr=%s", line, warnings, stderr.String())
		}
	}
}

func TestJSONModeAlwaysEnvelope(t *testing.T) {
	for _, args := range [][]string{
		{"capabilities"}, {"tools"}, {"neighbors"}, {"wifi", "show"},
		{"jobs", "list"}, {"jobs", "list", "--all"}, {"jobs", "show", fixtureJobID},
		{"history", "list"}, {"settings", "get"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var out, stderr bytes.Buffer
			got := runJSON(t.Context(), newTestApp(t), append([]string{"--json"}, args...), &out, &stderr)
			var response envelope
			if err := json.Unmarshal(out.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			want := 0
			if got != want || response.SchemaVersion != 1 || strings.Contains(stderr.String(), "schema_version") {
				t.Fatalf("exit=%d, stdout=%s, stderr=%s", got, out.String(), stderr.String())
			}
		})
	}
}

func TestJobIDValidation(t *testing.T) {
	for _, test := range []struct {
		id   string
		want int
	}{
		{fixtureJobID, 0}, {strings.Repeat("0", 32), 6},
		{strings.Repeat("0", 31), 2}, {strings.Repeat("0", 33), 2},
		{strings.ToUpper(fixtureJobID), 2},
	} {
		t.Run(test.id, func(t *testing.T) {
			var out, stderr bytes.Buffer
			if got := runJSON(t.Context(), newTestApp(t), []string{"--json", "jobs", "show", test.id}, &out, &stderr); got != test.want {
				t.Fatalf("exit=%d, want %d; out=%s", got, test.want, out.String())
			}
		})
	}
}

func TestJobsListFiltersTerminalHistory(t *testing.T) {
	a := newTestApp(t)
	for _, test := range []struct {
		args []string
		want int
	}{
		{[]string{"jobs", "list"}, 0},
		{[]string{"jobs", "list", "--all"}, 1},
		{[]string{"history", "list"}, 1},
	} {
		var out, stderr bytes.Buffer
		if runJSON(t.Context(), a, append([]string{"--json"}, test.args...), &out, &stderr) != 0 {
			t.Fatal(out.String(), stderr.String())
		}
		var response struct{ Data []struct{ ID string } }
		if err := json.Unmarshal(out.Bytes(), &response); err != nil || len(response.Data) != test.want {
			t.Fatalf("result=%s, error=%v", out.String(), err)
		}
	}
}

func TestWiFiRevealRequiresConfirm(t *testing.T) {
	a, provider := revealApplication(t)
	var out, stderr bytes.Buffer
	code := runJSON(t.Context(), a, []string{"--json", "wifi", "reveal"}, &out, &stderr)
	if code != 2 || provider.reveals.Load() != 0 || strings.Contains(out.String()+stderr.String(), "synthetic-test-key") {
		t.Fatalf("exit=%d, reveals=%d", code, provider.reveals.Load())
	}
}

func TestWiFiRevealWarnsAndNeverPersists(t *testing.T) {
	a, provider := revealApplication(t)
	var out, stderr bytes.Buffer
	code := runJSON(t.Context(), a, []string{"--json", "wifi", "reveal", "--confirm"}, &out, &stderr)
	rows, err := a.History(t.Context())
	if code != 0 || provider.reveals.Load() != 1 || !strings.Contains(out.String(), "synthetic-test-key") ||
		!strings.Contains(stderr.String(), "Terminal scrollback") || strings.Contains(stderr.String(), "synthetic-test-key") || err != nil || len(rows) != 0 {
		t.Fatalf("exit=%d, reveals=%d, history_rows=%d, error=%v", code, provider.reveals.Load(), len(rows), err)
	}
}

func TestSettingsAndHistoryMutations(t *testing.T) {
	a := newTestApp(t)
	for _, args := range [][]string{
		{"settings", "set", "--language", "pt-BR", "--retention-days", "365"},
		{"history", "clear"},
	} {
		var out, stderr bytes.Buffer
		if code := runJSON(t.Context(), a, append([]string{"--json"}, args...), &out, &stderr); code != 0 {
			t.Fatalf("%v: exit=%d, out=%s", args, code, out.String())
		}
	}
	settings, err := a.Settings(t.Context())
	if err != nil || settings.Language != "pt-BR" || settings.RetentionDays != 365 {
		t.Fatal(settings, err)
	}
	rows, err := a.History(t.Context())
	if err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}

func TestCaptureOutputRequiresFormatFirst(t *testing.T) {
	for _, path := range []string{"capture live", "capture import", "capture export"} {
		t.Run(path, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "must-not-exist.pcap")
			args := append([]string{"--json"}, strings.Fields(path)...)
			args = append(args, "--output", destination)
			var out, stderr bytes.Buffer
			if code := runJSON(t.Context(), newTestApp(t), args, &out, &stderr); code != 2 {
				t.Fatalf("exit=%d, out=%s", code, out.String())
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("destination created", err)
			}
		})
	}
}

func TestCLIMapsUnknownErrorsGenerically(t *testing.T) {
	var out, stderr bytes.Buffer
	path := filepath.Join(t.TempDir(), "missing-capture.pcap")
	code := runJSON(t.Context(), newApp(t), []string{"import", "--file", path}, &out, &stderr)
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
		code := runJSON(ctx, application, args, &out, &stderr)
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
