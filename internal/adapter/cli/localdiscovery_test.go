package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
)

func TestCLISNMPSecretFile(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := cli.Run(t.Context(), nil, []string{"snmp", "--community", "x"}, &out, &stderr); code != 2 {
		t.Fatalf("community flag exit %d %s %s", code, &out, &stderr)
	}
	out.Reset()
	stderr.Reset()
	if code := cli.Run(t.Context(), nil, []string{"snmp", "--help"}, &out, &stderr); code != 0 {
		t.Fatal(code, &stderr)
	}
	help := strings.ToLower(out.String())
	if strings.Contains(help, "community") || strings.Contains(help, "password") {
		t.Fatal(out.String())
	}
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, bytes.Repeat([]byte("a"), 5<<10), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	code := cli.Run(t.Context(), nil, []string{"--json", "snmp", "--interface", "lo", "--scope", "127.0.0.1/32", "--target", "127.0.0.1", "--owned", "--secret-file", path}, &out, &stderr)
	if code != 2 || !strings.Contains(out.String(), "invalid_input") || strings.Contains(out.String()+stderr.String(), "aaaa") {
		t.Fatalf("exit %d %s %s", code, &out, &stderr)
	}
}
