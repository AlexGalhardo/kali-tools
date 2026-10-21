package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
)

func TestRunWiresCLI(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"HOME", "APPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(name, dir)
	}
	var out, stderr bytes.Buffer
	if code := run(t.Context(), []string{"version"}, func(string) string { return "" }, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, stderr.String())
	}
	var result struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.SchemaVersion != 1 {
		t.Fatalf("result=%s, err=%v", out.String(), err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("config entries=%v, err=%v", entries, err)
	}
	t.Run("startup error", func(t *testing.T) {
		out.Reset()
		stderr.Reset()
		failure := errors.New("synthetic bootstrap failure")
		boot := func(context.Context, bootstrap.Config) (*app.Application, func() error, error) {
			return nil, nil, failure
		}
		if code := runWithBootstrap(t.Context(), []string{"version"}, &out, &stderr, boot); code != 1 || out.Len() != 0 || stderr.String() != failure.Error()+"\n" {
			t.Fatalf("exit=%d, out=%s, stderr=%s", code, out.String(), stderr.String())
		}
	})
	t.Run("cleanup error", func(t *testing.T) {
		out.Reset()
		stderr.Reset()
		calls := 0
		failure := errors.New("synthetic cleanup failure")
		boot := func(ctx context.Context, cfg bootstrap.Config) (*app.Application, func() error, error) {
			if cfg.DataPath != cliDataPath {
				t.Fatal("persistent CLI path")
			}
			return nil, func() error { calls++; return failure }, nil
		}
		if code := runWithBootstrap(t.Context(), []string{"version"}, &out, &stderr, boot); code != 0 || calls != 1 || stderr.String() != failure.Error()+"\n" {
			t.Fatalf("exit=%d, cleanup calls=%d, stderr=%s", code, calls, stderr.String())
		}
	})
}
