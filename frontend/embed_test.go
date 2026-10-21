package frontend_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/frontend"
)

func TestDistIsTheOnlyEmbed(t *testing.T) {
	if _, err := fs.Stat(frontend.Dist(), ".gitkeep"); err != nil {
		t.Fatal(err)
	}
	var embeds []string
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() != ".." && (entry.Name() == "node_modules" || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "build") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "//go:embed ") && strings.Contains(line, "dist") {
				embeds = append(embeds, filepath.ToSlash(path))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(embeds) != 1 || embeds[0] != "../frontend/embed.go" {
		t.Fatalf("frontend embeds=%v", embeds)
	}
}
