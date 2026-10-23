package bootstrap_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/platform"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/probe"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/tools"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

func TestNewWiresStoreAndProber(t *testing.T) {
	application, cleanup, err := bootstrap.New(t.Context(), bootstrap.Config{DataPath: filepath.Join(t.TempDir(), "history.db")})
	if err != nil {
		t.Fatal(err)
	}
	if application == nil || cleanup == nil {
		t.Fatal("missing application or cleanup")
	}
	t.Cleanup(func() { _ = cleanup() })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, _ := application.Diagnose(ctx, network.Request{Kind: "ping", Target: "127.0.0.1", TimeoutMS: 100})
	if result.Provider != (probe.ICMP{}).Provider() {
		t.Fatalf("provider=%q, want native probe", result.Provider)
	}
	if _, err := application.History(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupIsIdempotent(t *testing.T) {
	application, cleanup, err := bootstrap.New(t.Context(), bootstrap.Config{DataPath: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup() })
	for range 2 {
		if err := cleanup(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := application.History(t.Context()); err == nil {
		t.Fatal("cleanup left storage open")
	}
}

func TestNativeProviderAdapters(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (platform.Host{}).Inventory(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	_, err := (platform.Host{}).Inventory(t.Context())
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		if err != nil {
			t.Fatal(err)
		}
	} else {
		var typed *domain.Error
		if !errors.As(err, &typed) || typed.Code != domain.CodeUnsupported {
			t.Fatalf("error=%v, want unsupported native provider", err)
		}
	}
	if _, err := (wifi.Native{}).Current(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := (wifi.Native{}).Reveal(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if got := (tools.Runner{}).Inventory(ctx); len(got) != 1 || got[0].Name != "tshark" {
		t.Fatalf("tool inventory = %+v", got)
	}
	if _, err := (tools.Runner{}).InspectServices(ctx, "0.0.0.0/0", []int{80}); err == nil {
		t.Fatal("invalid scope accepted")
	}
	if _, _, err := (capture.Native{}).Capture(ctx, capture.LiveRequest{}); err == nil {
		t.Fatal("invalid capture accepted")
	}
}

func TestNewAppliesRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	now := time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC)
	store, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.SaveSettings(t.Context(), storage.Settings{Language: "en", RetentionDays: 1}); err != nil {
		t.Fatal(err)
	}
	for _, job := range []storage.Job{
		{ID: "old", Kind: "tcp", State: "completed", Created: now.Add(-48 * time.Hour), Finished: &now},
		{ID: "recent", Kind: "tcp", State: "completed", Created: now.Add(-time.Hour), Finished: &now},
	} {
		if err := store.PutJob(t.Context(), job); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	application, cleanup, err := bootstrap.New(t.Context(), bootstrap.Config{DataPath: path, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup() })
	history, err := application.History(t.Context())
	if err != nil || len(history) != 1 || history[0].ID != "recent" {
		t.Fatalf("history = %+v, error = %v; want recent job only", history, err)
	}
}

func TestNewReportsStorageError(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, nil, 0600); err != nil {
		t.Fatal(err)
	}
	application, cleanup, err := bootstrap.New(t.Context(), bootstrap.Config{DataPath: filepath.Join(parent, "history.db")})
	if err == nil || errors.Unwrap(err) == nil || application != nil || cleanup != nil {
		t.Fatalf("application = %v, cleanup nil = %v, error = %v; want wrapped error only", application, cleanup == nil, err)
	}
}

func TestNewInMemory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"HOME", "APPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(name, dir)
	}
	_, cleanup, err := bootstrap.New(t.Context(), bootstrap.Config{DataPath: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("config entries = %v, error = %v; want empty", entries, err)
	}
}

func TestNewDefaultPath(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"HOME", "APPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(name, dir)
	}
	_, cleanup, err := bootstrap.New(t.Context(), bootstrap.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cleanup() })
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, "kali-tools", "history.db")); err != nil {
		t.Fatal(err)
	}
}

func TestNewReportsDataPathError(t *testing.T) {
	for _, name := range []string{"HOME", "APPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(name, "")
	}
	application, cleanup, err := bootstrap.New(t.Context(), bootstrap.Config{})
	if err == nil || errors.Unwrap(err) == nil || application != nil || cleanup != nil {
		t.Fatalf("application = %v, cleanup nil = %v, error = %v; want wrapped error only", application, cleanup == nil, err)
	}
}

func TestNewReportsRetentionError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	path := filepath.Join(t.TempDir(), "history.db")
	application, cleanup, err := bootstrap.New(ctx, bootstrap.Config{
		DataPath: path,
		Now: func() time.Time {
			cancel() // Settings succeeds before Now; retention must observe cancellation.
			return time.Now().UTC()
		},
	})
	if !errors.Is(err, context.Canceled) || application != nil || cleanup != nil {
		t.Fatalf("application = %v, cleanup nil = %v, error = %v; want cancellation only", application, cleanup == nil, err)
	}
	store, err := storage.Open(path)
	if err != nil {
		t.Fatalf("reopening after failed bootstrap: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewSkipsRetentionWhenSettingsFail(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, cleanup, err := bootstrap.New(ctx, bootstrap.Config{
		DataPath: ":memory:",
		Now: func() time.Time {
			t.Fatal("retention must be skipped after a settings read error")
			return time.Time{}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
}
