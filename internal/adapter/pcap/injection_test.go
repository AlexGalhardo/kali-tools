package pcap

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestNoPacketInjectionCalls(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			case *ast.Ident:
				name = fn.Name
			}
			if name == "WritePacketData" || name == "ZeroCopyWritePacketData" || strings.Contains(name, "SendPacket") {
				t.Errorf("packet injection call %s in %s", name, entry.Name())
			}
			return true
		})
	}
}

func TestDefaultBuildExcludesGopacketPcap(t *testing.T) {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go executable unavailable for dependency inspection")
	}
	for _, target := range []string{"github.com/AlexGalhardo/kali-tools/cmd/kali-tools", "github.com/AlexGalhardo/kali-tools"} {
		t.Run(target, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, goBinary, "list", "-deps", "-tags", "desktop", target)
			cmd.Stderr = io.Discard
			cmd.WaitDelay = time.Second
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			const maxDependencyOutput = 1 << 20
			raw, readErr := io.ReadAll(io.LimitReader(out, maxDependencyOutput+1))
			if len(raw) > maxDependencyOutput {
				cancel()
			}
			waitErr := cmd.Wait()
			if readErr != nil || waitErr != nil || len(raw) > maxDependencyOutput {
				t.Fatalf("dependency inspection failed: read=%v wait=%v bytes=%d", readErr, waitErr, len(raw))
			}
			for _, path := range strings.Fields(string(raw)) {
				if path == "github.com/gopacket/gopacket/pcap" {
					t.Fatal("default build links optional pcap backend")
				}
			}
		})
	}
}
