//go:build unix

package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseFilesArePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(name)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("%s: %v %v", name, info, err)
		}
	}
}
