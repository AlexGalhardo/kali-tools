package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

type goldenCase struct {
	name string
	args []string
}

func goldenCases(t *testing.T) []goldenCase {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "input.pcap")
	a := capture.NewArtifact(layers.LinkTypeRaw)
	packet := []byte{0x45, 0, 0, 20, 0, 0, 0, 0, 64, 1, 0, 0, 192, 0, 2, 1, 192, 0, 2, 2}
	if err := a.Add(packet, gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: len(packet), Length: len(packet)}); err != nil {
		t.Fatal(err)
	}
	if err := a.Save(t.Context(), file, "pcap"); err != nil {
		t.Fatal(err)
	}
	return []goldenCase{
		{"version", []string{"version"}}, {"doctor", []string{"doctor"}}, {"capabilities", []string{"capabilities"}}, {"interfaces", []string{"interfaces"}},
		{"inventory", []string{"inventory"}}, {"neighbors", []string{"neighbors"}}, {"diagnose", []string{"diagnose", "ping", "--target", "fixture.invalid"}},
		{"discover", []string{"discover", "--target", "127.0.0.1/32", "--ports", port}}, {"inspect", []string{"inspect", "--target", "127.0.0.1/32"}},
		{"wifi-show", []string{"wifi", "show"}}, {"wifi-reveal", []string{"wifi", "reveal", "--confirm"}},
		{"capture-live", []string{"capture", "live"}}, {"capture-import", []string{"capture", "import", "--file", file}},
		{"capture-export", []string{"capture", "export", "--file", file, "--output", file + ".pcapng", "--format", "pcapng"}},
		{"jobs-list", []string{"jobs", "list", "--all"}}, {"jobs-show", []string{"jobs", "show", fixtureJobID}}, {"jobs-cancel", []string{"jobs", "cancel", fixtureJobID}},
		{"history-list", []string{"history", "list"}}, {"history-clear", []string{"history", "clear"}},
		{"settings-get", []string{"settings", "get"}}, {"settings-set", []string{"settings", "set", "--language", "pt-BR", "--retention-days", "7"}},
		{"tools", []string{"tools"}}, {"serve", []string{"serve"}},
		{"wifi", []string{"wifi"}}, {"capture", []string{"capture"}}, {"import", []string{"import", "--file", file}},
	}
}

var elapsed = regexp.MustCompile(`\([0-9.]+ ms,`)
var progressID = regexp.MustCompile(`job [0-9a-f]{8} `)
var randomJobID = regexp.MustCompile(`[0-9a-f]{32}`)

func goldenArgs(t *testing.T, a *app.Application, test goldenCase) []string {
	t.Helper()
	if test.name != "jobs-cancel" {
		return slices.Clone(test.args)
	}
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	t.Cleanup(server.Close)
	status, err := a.StartDiagnostic(t.Context(), network.Request{Kind: "http", Target: server.URL, TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not start")
	}
	return []string{"jobs", "cancel", string(status.ID)}
}

func normalizeOutput(t *testing.T, test goldenCase, output string, jsonMode bool) string {
	t.Helper()
	for i := len(test.args) - 1; i >= 0; i-- {
		arg := test.args[i]
		if i > 0 && test.args[i-1] == "--ports" {
			output = strings.ReplaceAll(output, arg, "443")
		}
		if i > 0 && (test.args[i-1] == "--file" || test.args[i-1] == "--output") {
			output = strings.ReplaceAll(output, arg, "<fixture>")
		}
		if jsonMode && i > 0 && (test.args[i-1] == "--file" || test.args[i-1] == "--output") {
			encoded, err := json.Marshal(arg)
			if err != nil {
				t.Fatal(err)
			}
			output = strings.ReplaceAll(output, string(encoded[1:len(encoded)-1]), "<fixture>")
		}
	}
	output = strings.ReplaceAll(output, runtime.Version(), "goX")
	output = strings.ReplaceAll(output, runtime.GOOS, "OS")
	output = strings.ReplaceAll(output, runtime.GOARCH, "ARCH")
	output = randomJobID.ReplaceAllString(output, fixtureJobID)
	if jsonMode {
		var value any
		if err := json.Unmarshal([]byte(output), &value); err != nil {
			t.Fatal(err, output)
		}
		var normalize func(any)
		normalize = func(node any) {
			switch v := node.(type) {
			case map[string]any:
				if id, ok := v["id"].(string); ok && slices.Contains([]string{"routes_counters", "icmp", "wifi", "neighbors", "capture_live", "capture_pktmon", "capture_npcap"}, id) {
					v["state"], v["detail"], v["provider"] = "<native>", "<native>", "<native>"
				}
				if _, ok := v["elapsed_ms"]; ok {
					v["elapsed_ms"] = float64(0)
				}
				for _, child := range v {
					normalize(child)
				}
			case []any:
				for _, child := range v {
					normalize(child)
				}
			}
		}
		normalize(value)
		if test.name == "interfaces" {
			env := value.(map[string]any)
			rows := env["data"].([]any)
			if len(rows) == 0 {
				t.Fatal("no native interfaces to normalize")
			}
			env["data"] = []any{map[string]any{"index": float64(1), "name": "<native>", "mac": "<native>", "mtu": float64(1500), "up": true, "loopback": false, "addresses": []any{"<native>"}}}
		}
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		return string(raw) + "\n"
	}
	output = elapsed.ReplaceAllString(output, "(0 ms,")
	output = progressID.ReplaceAllString(output, "job <id> ")
	if test.name == "interfaces" {
		return "<native interfaces>\n"
	}
	lines := strings.Split(output, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 && slices.Contains([]string{"routes_counters", "icmp", "wifi", "neighbors", "capture_live", "capture_pktmon", "capture_npcap"}, fields[0]) {
			continue
		}
		if len(fields) > 0 && slices.Contains([]string{"CAPABILITY", "CAPACIDADE", "interfaces", "diagnostics", "discovery", "capture_import"}, fields[0]) {
			line = strings.Join(fields, " ")
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func TestGolden(t *testing.T) {
	for _, test := range goldenCases(t) {
		for _, mode := range []string{"human", "json", "pt-BR"} {
			t.Run(test.name+"/"+mode, func(t *testing.T) {
				a := newTestApp(t)
				args := goldenArgs(t, a, test)
				if mode == "json" {
					args = append([]string{"--json"}, args...)
				}
				if mode == "pt-BR" {
					args = append([]string{"--lang", "pt-BR"}, args...)
				}
				var out, stderr bytes.Buffer
				code := cli.Run(t.Context(), a, args, &out, &stderr)
				stdout := normalizeOutput(t, test, out.String(), mode == "json")
				errors := normalizeOutput(t, test, stderr.String(), false)
				compareGolden(t, test.name+"-"+mode+".golden", []byte(fmt.Sprintf("exit=%d\nstdout:\n%sstderr:\n%s", code, stdout, errors)))
			})
		}
	}
}

type shapeFixture struct {
	SchemaVersion int               `json:"schema_version"`
	Command       string            `json:"command"`
	Paths         map[string]string `json:"paths"`
}

func jsonPaths(value any) map[string]string {
	paths := map[string]string{}
	var walk func(string, any)
	walk = func(path string, node any) {
		switch v := node.(type) {
		case map[string]any:
			for key, child := range v {
				next := key
				if path != "" {
					next = path + "." + key
				}
				walk(next, child)
			}
		case []any:
			paths[path] = "array"
			for _, child := range v {
				walk(path+"[]", child)
			}
		case string:
			paths[path] = "string"
		case float64:
			paths[path] = "number"
		case bool:
			paths[path] = "boolean"
		case nil:
			paths[path] = "null"
		}
	}
	walk("", value)
	return paths
}

func checkJSONContract(actual map[string]string, fixture shapeFixture, version int) error {
	if version > fixture.SchemaVersion {
		return nil
	}
	for key, kind := range fixture.Paths {
		if actual[key] != kind {
			return fmt.Errorf("%s: got %q, want %q; schema_version must increase for a breaking change", key, actual[key], kind)
		}
	}
	return nil
}

func TestJSONContractRejectsBreakingChanges(t *testing.T) {
	fixture := shapeFixture{SchemaVersion: 1, Paths: map[string]string{"data.id": "string"}}
	for _, actual := range []map[string]string{{}, {"data.id": "number"}} {
		if checkJSONContract(actual, fixture, 1) == nil {
			t.Fatal("breaking change accepted")
		}
		if err := checkJSONContract(actual, fixture, 2); err != nil {
			t.Fatal("schema bump rejected", err)
		}
	}
	if err := checkJSONContract(map[string]string{"data.id": "string", "data.extra": "boolean"}, fixture, 1); err != nil {
		t.Fatal("additive change rejected", err)
	}
}

func TestJSONContractFixtures(t *testing.T) {
	for _, command := range cli.CommandPathsForTest() {
		if command == "help" {
			continue
		}
		if _, err := os.Stat(filepath.Join("testdata", "contract", strings.ReplaceAll(command, " ", "-")+".json")); err != nil && !*updateGolden {
			t.Fatal("missing command fixture", command, err)
		}
	}
	for _, test := range goldenCases(t) {
		t.Run(test.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			a := newTestApp(t)
			cli.Run(t.Context(), a, append([]string{"--json"}, goldenArgs(t, a, test)...), &out, &stderr)
			var value any
			if err := json.Unmarshal(out.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			actual := jsonPaths(value)
			path := filepath.Join("testdata", "contract", test.name+".json")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				raw, err := json.MarshalIndent(shapeFixture{SchemaVersion: domain.SchemaVersion, Command: strings.ReplaceAll(test.name, "-", " "), Paths: actual}, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(raw, '\n'), 0644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var fixture shapeFixture
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatal(err)
			}
			if err := checkJSONContract(actual, fixture, domain.SchemaVersion); err != nil {
				t.Error(err)
			}
		})
	}
}
