package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
)

func TestCLIConnectionsAndThroughput(t *testing.T) {
	store := fake.NewStore()
	application := app.New(app.Deps{Jobs: store, Settings: store})
	t.Cleanup(func() { _ = application.Close() })

	var out, stderr bytes.Buffer
	code := cli.Run(t.Context(), application, []string{"--json", "connections"}, &out, &stderr)
	if code != 0 && code != cli.ExitUnsupported {
		t.Fatalf("connections exit=%d out=%s err=%s", code, out.String(), stderr.String())
	}
	if strings.Contains(out.String(), "bits_per_second") {
		t.Fatal(out.String())
	}

	out.Reset()
	stderr.Reset()
	if code := cli.Run(t.Context(), application, []string{"--json", "throughput"}, &out, &stderr); code != cli.ExitCode(domain.CodeInvalidInput) {
		t.Fatalf("missing duration exit=%d out=%s", code, out.String())
	}

	out.Reset()
	stderr.Reset()
	if code := cli.Run(t.Context(), application, []string{"--json", "throughput", "--duration", "1"}, &out, &stderr); code != 0 {
		t.Fatalf("throughput exit=%d out=%s err=%s", code, out.String(), stderr.String())
	}
	var env struct {
		Data struct {
			Endpoint   string `json:"endpoint"`
			DurationMS int64  `json:"duration_ms"`
			ByteCount  uint64 `json:"byte_count"`
			Source     string `json:"source"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(env.Data.Endpoint, "127.0.0.1:") || env.Data.DurationMS != 1000 || env.Data.ByteCount == 0 || env.Data.Source != "kali-tools" {
		t.Fatalf("%+v", env.Data)
	}

	report := []byte(`{"start":{"connecting_to":{"host":"192.0.2.10","port":5201}},"end":{"sum_sent":{"bytes":4096,"bits_per_second":999999,"seconds":1.5}}}`)
	path := filepath.Join(t.TempDir(), "iperf.json")
	if err := os.WriteFile(path, report, 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := cli.Run(t.Context(), application, []string{"--json", "throughput", "--iperf-json", path}, &out, &stderr); code != 0 || strings.Contains(out.String(), "999999") {
		t.Fatalf("iperf exit=%d out=%s", code, out.String())
	}
}
