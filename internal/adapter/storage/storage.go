package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

type Store struct {
	db    *sql.DB
	write sync.Mutex
}
type Job = job.Status

func DefaultPath() (string, error) {
	base, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return dataPath(base)
}
func dataPath(base string) (string, error) {
	preferred := filepath.Join(base, "kali-tools", "history.db")
	if _, err := os.Stat(preferred); err == nil {
		return preferred, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	// Preserve existing development preferences without moving an open SQLite database.
	legacy := filepath.Join(base, "hacker-tools", "history.db")
	if _, err := os.Stat(legacy); err == nil {
		return legacy, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return preferred, nil
}
func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return nil, e
		}
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, e = db.ExecContext(ctx, "PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL;"); e != nil {
		db.Close()
		return nil, e
	}
	e = s.migrate(ctx)
	if e != nil {
		db.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) migrate(ctx context.Context) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY);
CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS jobs(id TEXT PRIMARY KEY,kind TEXT NOT NULL,state TEXT NOT NULL,created TEXT NOT NULL,finished TEXT,summary TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS diagnostic_results(job_id TEXT PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,result TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS tool_inventory(name TEXT PRIMARY KEY,version TEXT NOT NULL,checked TEXT NOT NULL);
INSERT OR IGNORE INTO schema_migrations VALUES(1);`)
	if e != nil {
		return e
	}
	var version int
	if e = tx.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&version); e != nil {
		return e
	}
	if version != 1 {
		return fmt.Errorf("unsupported database schema %d", version)
	}
	// Interrupted diagnostics cannot still be running after process restart.
	_, e = tx.ExecContext(ctx, "UPDATE jobs SET state='cancelled',summary='Application stopped',finished=? WHERE state IN ('running','queued')", time.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) PutJob(ctx context.Context, j Job) error {
	s.write.Lock()
	defer s.write.Unlock()
	var finished any
	if j.Finished != nil {
		finished = j.Finished.UTC().Format(time.RFC3339Nano)
	}
	_, e := s.db.ExecContext(ctx, `INSERT INTO jobs VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET state=excluded.state,finished=excluded.finished,summary=excluded.summary`, string(j.ID), j.Kind, string(j.State), j.Created.UTC().Format(time.RFC3339Nano), finished, j.Summary)
	return e
}
func (s *Store) History(ctx context.Context) ([]Job, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,kind,state,created,finished,summary FROM jobs ORDER BY created DESC LIMIT 200")
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	all := []Job{}
	for rows.Next() {
		var j Job
		var created string
		var id, state string
		var finished sql.NullString
		if e = rows.Scan(&id, &j.Kind, &state, &created, &finished, &j.Summary); e != nil {
			return nil, e
		}
		j.ID, j.State = job.ID(id), job.State(state)
		j.Created, e = time.Parse(time.RFC3339Nano, created)
		if e != nil {
			return nil, e
		}
		if finished.Valid {
			t, e := time.Parse(time.RFC3339Nano, finished.String)
			if e != nil {
				return nil, e
			}
			j.Finished = &t
		}
		all = append(all, j)
	}
	return all, rows.Err()
}
func (s *Store) Clear(ctx context.Context) error {
	s.write.Lock()
	defer s.write.Unlock()
	_, e := s.db.ExecContext(ctx, "DELETE FROM jobs WHERE state NOT IN ('queued','running')")
	return e
}
func (s *Store) Retain(ctx context.Context, before time.Time) error {
	s.write.Lock()
	defer s.write.Unlock()
	_, e := s.db.ExecContext(ctx, "DELETE FROM jobs WHERE created < ? AND state NOT IN ('queued','running')", before.UTC().Format(time.RFC3339Nano))
	return e
}

type Settings = app.Settings

func (s *Store) Settings(ctx context.Context) (Settings, error) {
	v := Settings{Language: "en", RetentionDays: 30}
	var raw string
	e := s.db.QueryRowContext(ctx, "SELECT substr(value,1,1025) FROM settings WHERE key='preferences'").Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return v, nil
	}
	if e != nil {
		return v, e
	}
	if len(raw) > 1024 {
		return v, fmt.Errorf("stored settings exceed 1024 bytes")
	}
	if e = json.Unmarshal([]byte(raw), &v); e != nil {
		return v, fmt.Errorf("invalid stored settings")
	}
	return v, validateSettings(v)
}
func validateSettings(v Settings) error {
	if (v.Language != "en" && v.Language != "pt-BR") || v.RetentionDays < 1 || v.RetentionDays > 365 {
		return domain.Invalid("invalid settings")
	}
	return nil
}
func (s *Store) SaveSettings(ctx context.Context, v Settings) error {
	if err := validateSettings(v); err != nil {
		return err
	}
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	s.write.Lock()
	defer s.write.Unlock()
	_, e = s.db.ExecContext(ctx, "INSERT INTO settings VALUES('preferences',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(raw))
	return e
}
