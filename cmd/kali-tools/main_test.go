package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
)

func noEnv(string) string { return "" }

var updateGolden = flag.Bool("update", false, "update CLI golden fixtures")

func TestCLIGolden(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "synthetic.pcapng")
	artifact := capture.NewArtifact(layers.LinkTypeRaw)
	ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, SrcIP: []byte{192, 0, 2, 1}, DstIP: []byte{192, 0, 2, 2}, Protocol: layers.IPProtocolTCP}
	tcp := &layers.TCP{SrcPort: 12345, DstPort: 80, DataOffset: 5}
	buffer := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buffer, gopacket.SerializeOptions{FixLengths: true}, ip, tcp, gopacket.Payload("synthetic-private")); err != nil {
		t.Fatal(err)
	}
	if err := artifact.Add(buffer.Bytes(), gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: len(buffer.Bytes()), Length: len(buffer.Bytes())}); err != nil {
		t.Fatal(err)
	}
	if err := artifact.Save(t.Context(), fixture, "pcapng"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
		code int
	}{
		{"version", []string{"version"}, 0},
		{"unknown", []string{"unknown"}, 2},
		{"invalid-target", []string{"diagnose", "--target", "a;whoami"}, 2},
		{"invalid-scope", []string{"discover", "--target", "0.0.0.0/0"}, 2},
		{"invalid-format", []string{"discover", "--target", "127.0.0.1/32", "--format", "invalid"}, 2},
		{"too-many-ports", []string{"discover", "--target", "127.0.0.1/32", "--ports", strings.Repeat("80,", 32) + "80"}, 2},
		{"inspection-csv", []string{"inspect", "--format", "csv", "--target", "127.0.0.1/32"}, 2},
		{"import", []string{"import", "--file", fixture}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			if code := run(t.Context(), test.args, noEnv, &out, &stderr); code != test.code {
				t.Fatalf("exit=%d, want=%d, out=%s, stderr=%s", code, test.code, out.String(), stderr.String())
			}
			actual := out.Bytes()
			if test.name == "version" {
				var result map[string]any
				if err := json.Unmarshal(actual, &result); err != nil {
					t.Fatal(err)
				}
				data, ok := result["data"].(map[string]any)
				if !ok {
					t.Fatal("missing version data")
				}
				data["go"], data["os"], data["arch"] = "<go>", "<os>", "<arch>"
				encoded, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				actual = append(encoded, '\n')
			}
			compareGolden(t, test.name+".json", actual)
		})
	}
}

func compareGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, actual, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, want) {
		t.Fatalf("%s mismatch\ngot:\n%s\nwant:\n%s", name, actual, want)
	}
}

func TestCLIUsageWithoutArgs(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run(t.Context(), nil, noEnv, &out, &stderr); code != 2 || out.Len() != 0 {
		t.Fatalf("exit=%d, out=%s", code, out.String())
	}
	compareGolden(t, "usage.txt", stderr.Bytes())
}

func TestRunDoesNotWriteUserConfig(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"HOME", "APPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(name, dir)
	}
	var out, stderr bytes.Buffer
	if code := run(t.Context(), []string{"version"}, noEnv, &out, &stderr); code != 0 {
		t.Fatalf("exit = %d, output = %s, stderr = %s", code, out.String(), stderr.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("config entries = %v, error = %v; want empty", entries, err)
	}
}

func TestCLIUsageBeforeBootstrap(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "no command"},
		{name: "malformed flag", args: []string{"version", "--unknown"}},
		{name: "positional argument", args: []string{"version", "extra"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			if code := run(t.Context(), test.args, noEnv, &out, &stderr); code != 2 {
				t.Fatalf("exit = %d, output = %s, stderr = %s; want 2", code, out.String(), stderr.String())
			}
		})
	}
}

func TestCLIContract(t *testing.T) {
	for _, test := range []struct {
		args      []string
		code      int
		errorCode string
	}{{[]string{"version"}, 0, ""}, {[]string{"interfaces"}, 0, ""}, {[]string{"diagnose", "--target", "a;whoami"}, 2, "invalid_input"}, {[]string{"discover", "--target", "0.0.0.0/0"}, 2, "invalid_input"}, {[]string{"unknown"}, 2, "invalid_input"}} {
		var out, stderr bytes.Buffer
		code := run(t.Context(), test.args, noEnv, &out, &stderr)
		if code != test.code {
			t.Fatal(test.args, code, out.String(), stderr.String())
		}
		var response envelope
		if e := json.Unmarshal(out.Bytes(), &response); e != nil {
			t.Fatal(e, out.String())
		}
		if response.SchemaVersion != 1 {
			t.Fatal(response)
		}
		if test.errorCode != "" && (response.Error == nil || response.Error.Code != test.errorCode) {
			t.Fatal(response)
		}
	}
}
func TestCLIHTTPAndMalformedFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer server.Close()
	var out, stderr bytes.Buffer
	if code := run(t.Context(), []string{"diagnose", "--kind", "http", "--target", server.URL}, noEnv, &out, &stderr); code != 0 {
		t.Fatal(code, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("204 No Content")) {
		t.Fatal(out.String())
	}
	if code := run(t.Context(), []string{"diagnose", "--timeout", "NaN"}, noEnv, &out, &stderr); code != 2 {
		t.Fatal(code)
	}
}

func TestDiscoveryRejectsInvalidOptionsBeforeScanning(t *testing.T) {
	for _, args := range [][]string{
		{"discover", "--target", "127.0.0.1/32", "--format", "invalid"},
		{"discover", "--target", "127.0.0.1/32", "--ports", strings.Repeat("80,", 32) + "80"},
	} {
		var out, stderr bytes.Buffer
		if code := run(t.Context(), args, noEnv, &out, &stderr); code != 2 {
			t.Fatalf("%v: exit %d: %s", args, code, out.String())
		}
		if !bytes.Contains(out.Bytes(), []byte(`"invalid_input"`)) || bytes.Contains(out.Bytes(), []byte(`"host"`)) {
			t.Fatal(out.String())
		}
	}
}

func TestDiscoveryExports(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	for _, format := range []string{"json", "csv"} {
		var out, stderr bytes.Buffer
		if code := run(t.Context(), []string{"discover", "--target", "127.0.0.1/32", "--ports", port, "--format", format}, noEnv, &out, &stderr); code != 0 {
			t.Fatal(code, out.String())
		}
		if format == "csv" {
			rows, err := csv.NewReader(&out).ReadAll()
			if err != nil || len(rows) != 2 || strings.Join(rows[0], ",") != "schema_version,host,port,state" || strings.Join(rows[1], ",") != "1,127.0.0.1,"+port+",open" {
				t.Fatal(rows, err)
			}
		} else {
			var response struct {
				SchemaVersion int `json:"schema_version"`
				Data          []struct {
					Host  string `json:"host"`
					Port  int    `json:"port"`
					State string `json:"state"`
				} `json:"data"`
			}
			if err := json.Unmarshal(out.Bytes(), &response); err != nil || response.SchemaVersion != 1 || len(response.Data) != 1 || response.Data[0].State != "open" || response.Data[0].Host != "127.0.0.1" {
				t.Fatal(out.String(), err)
			}
		}
	}
}

func TestCLICaptureConversion(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "input.pcapng")
	artifact := capture.NewArtifact(layers.LinkTypeRaw)
	ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, SrcIP: []byte{192, 0, 2, 1}, DstIP: []byte{192, 0, 2, 2}, Protocol: layers.IPProtocolTCP}
	tcp := &layers.TCP{SrcPort: 12345, DstPort: 80, DataOffset: 5}
	b := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(b, gopacket.SerializeOptions{FixLengths: true}, ip, tcp, gopacket.Payload("GET /synthetic-private HTTP/1.1\r\n")); err != nil {
		t.Fatal(err)
	}
	if err := artifact.Add(b.Bytes(), gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: len(b.Bytes()), Length: len(b.Bytes())}); err != nil {
		t.Fatal(err)
	}
	if err := artifact.Save(t.Context(), source, "pcapng"); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"pcap", "pcapng"} {
		destination := filepath.Join(dir, "output."+format)
		var out, stderr bytes.Buffer
		if code := run(t.Context(), []string{"import", "--file", source, "--output", destination, "--format", format}, noEnv, &out, &stderr); code != 0 {
			t.Fatalf("%d %s", code, out.String())
		}
		if bytes.Contains(out.Bytes(), []byte("synthetic-private")) {
			t.Fatal("payload leaked in JSON")
		}
		result, err := capture.ReadFile(destination)
		if err != nil || result.Format != format || len(result.Packets) != 1 || result.Packets[0].Protocol != "HTTP" {
			t.Fatalf("%+v %v", result, err)
		}
	}
	destination := filepath.Join(dir, "invalid.pcap")
	var out, stderr bytes.Buffer
	if code := run(t.Context(), []string{"capture", "--output", destination}, noEnv, &out, &stderr); code != 2 {
		t.Fatalf("invalid format not rejected first: %d %s", code, out.String())
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("invalid export created a file")
	}
}
