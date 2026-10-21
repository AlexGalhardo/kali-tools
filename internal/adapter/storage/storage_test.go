package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPersistenceRecoveryAndRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	if e = s.SaveSettings(t.Context(), Settings{"pt-BR", 7}); e != nil {
		t.Fatal(e)
	}
	if e = s.PutJob(t.Context(), Job{ID: "1", Kind: "tcp", State: "running", Created: now.AddDate(0, 0, -60), Summary: "Running"}); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	v, e := s.Settings(t.Context())
	if e != nil || v.Language != "pt-BR" {
		t.Fatal(v, e)
	}
	history, e := s.History(t.Context())
	if e != nil || len(history) != 1 || history[0].State != "cancelled" {
		t.Fatal(history, e)
	}
	if e = s.Retain(t.Context(), now.AddDate(0, 0, -30)); e != nil {
		t.Fatal(e)
	}
	history, e = s.History(t.Context())
	if e != nil || len(history) != 0 {
		t.Fatal(history, e)
	}
}
func TestConcurrentReadsAndCancelledWrite(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, e := s.History(t.Context()); e != nil {
				t.Error(e)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if s.PutJob(ctx, Job{ID: "cancelled", Created: time.Now()}) == nil {
		t.Fatal("cancelled write succeeded")
	}
	wg.Wait()
	rows, e := s.History(t.Context())
	if e != nil || len(rows) != 0 {
		t.Fatal(rows, e)
	}
	if e = s.SaveSettings(t.Context(), Settings{"unknown", 0}); e == nil {
		t.Fatal("invalid settings accepted")
	}
}
func TestMigrationRollback(t *testing.T) {
	s, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	if _, e = s.db.Exec("UPDATE schema_migrations SET version=99; INSERT INTO jobs VALUES('x','tcp','running','2026-01-01T00:00:00Z',NULL,'unchanged')"); e != nil {
		t.Fatal(e)
	}
	if s.migrate(t.Context()) == nil {
		t.Fatal("accepted future schema")
	}
	var summary string
	if e = s.db.QueryRow("SELECT summary FROM jobs WHERE id='x'").Scan(&summary); e != nil || summary != "unchanged" {
		t.Fatal(summary, e)
	}
}

func TestStoredSettingsAreValidatedAndBounded(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	for _, raw := range []string{
		`{"language":"unknown","retention_days":30}`,
		`{"language":"en","retention_days":0}`,
		`{"language":"en","retention_days":366}`,
		`{"language":"synthetic-secret"`,
		strings.Repeat("synthetic-secret", 100),
	} {
		if _, err := store.db.Exec("INSERT OR REPLACE INTO settings VALUES('preferences',?)", raw); err != nil {
			t.Fatal(err)
		}
		_, err := store.Settings(t.Context())
		if err == nil {
			t.Fatal("invalid stored preferences accepted")
		}
		if strings.Contains(err.Error(), "synthetic-secret") {
			t.Fatal("settings error exposed input")
		}
	}
}

func TestRenamedApplicationPreservesExistingData(t *testing.T) {
	base := t.TempDir()
	preferred := filepath.Join(base, "kali-tools", "history.db")
	path, err := dataPath(base)
	if err != nil || path != preferred {
		t.Fatalf("new path: %s %v", path, err)
	}
	legacy := filepath.Join(base, "hacker-tools", "history.db")
	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err = dataPath(base)
	if err != nil || path != legacy {
		t.Fatalf("legacy path lost: %s %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(preferred), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(preferred, []byte("new-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err = dataPath(base)
	if err != nil || path != preferred {
		t.Fatalf("new data not preferred: %s %v", path, err)
	}
}
