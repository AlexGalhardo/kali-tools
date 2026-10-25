package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestJobStatusJSONMatchesLegacyRow(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	created := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	finished := created.Add(time.Second)
	if err := store.PutJob(t.Context(), Job{ID: "1", Kind: "tcp", State: "completed", Created: created, Finished: &finished, Summary: "x"}); err != nil {
		t.Fatal(err)
	}
	rows, err := store.History(t.Context())
	if err != nil || len(rows) != 1 {
		t.Fatalf("history = %+v, err = %v", rows, err)
	}
	raw, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"id":"1","kind":"tcp","state":"completed","created":"2026-10-21T12:00:00Z","finished":"2026-10-21T12:00:01Z","summary":"x"}`
	if string(raw) != want {
		t.Fatalf("JSON = %s, want %s", raw, want)
	}
}

func TestPersistenceRecoveryAndRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	if e = s.SaveSettings(t.Context(), Settings{Language: "pt-BR", RetentionDays: 7}); e != nil {
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
	if e = s.SaveSettings(t.Context(), Settings{Language: "unknown", RetentionDays: 0}); e == nil {
		t.Fatal("invalid settings accepted")
	}
}
func TestMigrationRollback(t *testing.T) {
	s, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = s.Close() }()
	if _, e = s.db.Exec("INSERT INTO schema_migrations VALUES(99); INSERT INTO jobs(id,kind,state,created,finished,summary) VALUES('x','tcp','running','2026-01-01T00:00:00Z',NULL,'unchanged')"); e != nil {
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

func TestClosedDatabaseCopyKeepsSettingsAndJob(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	finished := created.Add(time.Second)
	if err = store.SaveSettings(t.Context(), Settings{Language: "pt-BR", RetentionDays: 14}); err != nil {
		t.Fatal(err)
	}
	if err = store.PutJob(t.Context(), Job{ID: "backup-job", Kind: "tcp", State: "completed", Created: created, Finished: &finished, Summary: "fixture kept"}); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "backup.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := path + suffix
		if _, err = os.Stat(src); err != nil {
			continue
		}
		copyDBFile(t, src, backup+suffix)
	}
	store, err = Open(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	assertKeptFixture(t, store, "backup-job")
	original, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = original.Close() }()
	assertKeptFixture(t, original, "backup-job")
}

func TestSchemaVersionOneFixtureReopens(t *testing.T) {
	script, err := os.ReadFile("testdata/schema-v1.sql")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(script), "synthetic-secret") {
		t.Fatal("schema fixture contains a secret")
	}
	path := filepath.Join(t.TempDir(), "history.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(script)); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	assertKeptFixture(t, store, "fixture-job")
}

func assertKeptFixture(t *testing.T, store *Store, id string) {
	t.Helper()
	settings, err := store.Settings(t.Context())
	if err != nil || settings.Language != "pt-BR" || settings.RetentionDays != 14 {
		t.Fatalf("settings = %+v, err = %v", settings, err)
	}
	history, err := store.History(t.Context())
	if err != nil || len(history) != 1 || string(history[0].ID) != id || history[0].State != "completed" || history[0].Summary != "fixture kept" {
		t.Fatalf("history = %+v, err = %v", history, err)
	}
	if strings.Contains(history[0].Summary, "synthetic-secret") {
		t.Fatal("history exposed a secret")
	}
}

func copyDBFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		t.Fatal(err)
	}
	if err = out.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDataPathIgnoresLegacyDirectory(t *testing.T) {
	base := t.TempDir()
	legacy := filepath.Join(base, "hacker-tools", "history.db")
	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := dataPath(base)
	if want := filepath.Join(base, "kali-tools", "history.db"); err != nil || path != want {
		t.Fatalf("path = %s, %v", path, err)
	}
}

func TestMigrationsAreNumberedAndDropDeadTable(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	var versions, inventory, owner int
	if err = store.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&versions); err != nil || versions != len(migrations) {
		t.Fatal(versions, err)
	}
	if err = store.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name='tool_inventory'").Scan(&inventory); err != nil || inventory != 0 {
		t.Fatal("tool_inventory kept", inventory, err)
	}
	if err = store.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('jobs') WHERE name='owner_pid'").Scan(&owner); err != nil || owner != 1 {
		t.Fatal("owner_pid missing", owner, err)
	}
	if err = store.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name='diagnostic_results'").Scan(&owner); err != nil || owner != 1 {
		t.Fatal("diagnostic_results dropped", owner, err)
	}
}

func TestHelperSleep(t *testing.T) {
	if os.Getenv("STORAGE_HELPER_SLEEP") == "1" {
		time.Sleep(30 * time.Second)
	}
}

func TestOpenCancelsOnlyJobsOfDeadOwners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	live := exec.Command(os.Args[0], "-test.run=^TestHelperSleep$")
	live.Env = append(os.Environ(), "STORAGE_HELPER_SLEEP=1")
	if err = live.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = live.Process.Kill(); _ = live.Wait() }()
	dead := exec.Command(os.Args[0], "-test.run=^$")
	if err = dead.Run(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows := []struct {
		id  string
		pid any
	}{{"live", live.Process.Pid}, {"dead", dead.Process.Pid}, {"legacy", nil}, {"self", os.Getpid()}}
	for _, r := range rows {
		if _, err = s.db.Exec("INSERT INTO jobs(id,kind,state,created,summary,owner_pid) VALUES(?,?,?,?,?,?)", r.id, "tcp", "running", now, "Running", r.pid); err != nil {
			t.Fatal(err)
		}
	}
	other, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Close() }()
	got := map[string]string{}
	history, err := other.History(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range history {
		got[string(j.ID)] = string(j.State)
	}
	want := map[string]string{"live": "running", "dead": "cancelled", "legacy": "cancelled", "self": "cancelled"}
	for id, state := range want {
		if got[id] != state {
			t.Fatalf("%s = %q, want %q (all: %v)", id, got[id], state, got)
		}
	}
}
