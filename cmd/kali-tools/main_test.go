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
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
)

func TestRunWiresCLI(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"HOME", "APPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(name, dir)
	}
	var out, stderr bytes.Buffer
	if code := run(t.Context(), []string{"--json", "version"}, func(string) string { return "" }, &out, &stderr); code != 0 {
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
		if code := runWithBootstrap(t.Context(), []string{"--json", "capabilities"}, &out, &stderr, boot); code != 1 || out.Len() != 0 || stderr.String() != failure.Error()+"\n" {
			t.Fatalf("exit=%d, out=%s, stderr=%s", code, out.String(), stderr.String())
		}
	})
	t.Run("cleanup error", func(t *testing.T) {
		out.Reset()
		stderr.Reset()
		calls := 0
		failure := errors.New("synthetic cleanup failure")
		boot := func(ctx context.Context, cfg bootstrap.Config) (*app.Application, func() error, error) {
			if cfg.DataPath != "" {
				t.Fatal("CLI must use the shared persistent path")
			}
			store := fake.NewStore()
			a := app.New(app.Deps{Jobs: store, Settings: store})
			return a, func() error { calls++; _ = a.Close(); return failure }, nil
		}
		if code := runWithBootstrap(t.Context(), []string{"--json", "settings", "get"}, &out, &stderr, boot); code != 0 || calls != 1 || stderr.String() != failure.Error()+"\n" {
			t.Fatalf("exit=%d, cleanup calls=%d, stderr=%s", code, calls, stderr.String())
		}
	})
}

func TestReadOnlyInvocationSkipsBootstrap(t *testing.T) {
	for _, args := range [][]string{{"--json", "version"}, {"help"}, {"diagnose", "--help"}, {"unknown"}, {"version", "--port", "1"}, {"serve"}} {
		var out, stderr bytes.Buffer
		boot := func(context.Context, bootstrap.Config) (*app.Application, func() error, error) {
			t.Fatal("unexpected bootstrap")
			return nil, nil, nil
		}
		runWithBootstrap(t.Context(), args, &out, &stderr, boot)
	}
}

func TestCLIPersistsSettings(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"HOME", "APPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(name, dir)
	}
	var out, stderr bytes.Buffer
	getenv := func(string) string { return "1" }
	if code := run(t.Context(), []string{"--json", "settings", "set", "--language", "pt-BR"}, getenv, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d %s %s", code, &out, &stderr)
	}
	out.Reset()
	if code := run(t.Context(), []string{"--json", "settings", "get"}, getenv, &out, &stderr); code != 0 || !bytes.Contains(out.Bytes(), []byte(`"language": "pt-BR"`)) {
		t.Fatalf("exit=%d %s %s", code, &out, &stderr)
	}
}
