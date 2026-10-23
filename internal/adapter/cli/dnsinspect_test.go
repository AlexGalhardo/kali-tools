package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
)

func TestCLIDNSInspectFlags(t *testing.T) {
	a := newTestApp(t)
	var out, stderr bytes.Buffer
	if code := cli.Run(t.Context(), a, []string{"dns-inspect", "--target", "192.0.2.1", "--json"}, &out, &stderr); code != 2 || !strings.Contains(out.String(), "invalid_input") {
		t.Fatalf("exit=%d out=%s", code, out.String())
	}
	out.Reset()
	long := strings.Repeat("a", 1025)
	if code := cli.Run(t.Context(), a, []string{"dns-inspect", "--target", "example.com", "--resolvers", long, "--json"}, &out, &stderr); code != 2 || !strings.Contains(out.String(), "invalid_input") {
		t.Fatalf("long list exit=%d out=%s", code, out.String())
	}
	out.Reset()
	if code := cli.Run(t.Context(), a, []string{"dns-inspect", "--target", "example.com", "--resolvers", "192.0.2.1", "--timeout", "200", "--json"}, &out, &stderr); code != 0 || !strings.Contains(out.String(), `"compare"`) {
		t.Fatalf("exit=%d out=%s err=%s", code, out.String(), stderr.String())
	}
	if strings.Contains(out.String(), `"checks":null`) {
		t.Fatal(out.String())
	}
}
