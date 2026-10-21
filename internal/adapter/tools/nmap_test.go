package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSupportedToolVersions(t *testing.T) {
	for _, tt := range []struct {
		name, line string
		want       bool
	}{
		{"nmap", "Nmap version 7.98 ( https://nmap.org )", true}, {"nmap", "Nmap version 7.80", true},
		{"nmap", "Nmap version 7.79", false}, {"nmap", "Nmap version 8.0", false}, {"nmap", "Nmap version 7.98SVN", false},
		{"tshark", "TShark (Wireshark) 4.6.0", true}, {"tshark", "TShark (Wireshark) 4.1.0", false}, {"tshark", "TShark (Wireshark) 5.0.0", false},
		{"nmap", "arbitrary output", false},
	} {
		if got := supportedVersion(tt.name, tt.line); got != tt.want {
			t.Fatalf("%s %q: %v", tt.name, tt.line, got)
		}
	}
}

func TestNmapScopeAndStructuredOutput(t *testing.T) {
	dir := t.TempDir()
	exe := "nmap"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	raw, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, exe), raw, 0700); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(dir, "result.xml")
	t.Setenv("PATH", dir)
	t.Setenv("KALI_TOOLS_NMAP_FIXTURE", fixture)
	valid := `<nmaprun scanner="nmap" version="7.98"><host><status state="up"/><address addr="192.0.2.1" addrtype="ipv4"/><ports><port portid="80" protocol="tcp"><state state="open"/><service name="http" product="Synthetic HTTP" version="1" method="probed"/></port></ports></host><runstats><finished exit="success"/></runstats></nmaprun>`
	for _, tt := range []struct {
		name, xml string
		wantErr   bool
	}{
		{"valid", valid, false},
		{"out-of-scope", strings.ReplaceAll(valid, "192.0.2.1", "198.51.100.1"), true},
		{"unrequested-port", strings.ReplaceAll(valid, `portid="80"`, `portid="81"`), true},
		{"wrong-version", strings.ReplaceAll(valid, `version="7.98"`, `version="8.0"`), true},
		{"version-contents", strings.ReplaceAll(valid, `version="7.98"`, `version="7.98 synthetic-private"`), true},
		{"partial", strings.ReplaceAll(valid, `exit="success"`, `exit="error"`), true},
		{"malformed", "<nmaprun>", true},
		{"wrong-root", "<unrelated/>", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(fixture, []byte(tt.xml), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := RunNmap(t.Context(), "192.0.2.1/32", []int{80})
			if (err != nil) != tt.wantErr {
				t.Fatalf("%+v %v", result, err)
			}
			if !tt.wantErr && result.Hosts[0].Ports[0].Service.Product != "Synthetic HTTP" {
				t.Fatalf("lost service data: %+v", result)
			}
		})
	}
	if _, err := nmapArguments("192.0.2.0/16", []int{80}); err == nil {
		t.Fatal("expanded scope accepted")
	}
	if _, err := nmapArguments("192.0.2.1/32", []int{80, 80}); err == nil {
		t.Fatal("duplicate ports accepted")
	}
}
