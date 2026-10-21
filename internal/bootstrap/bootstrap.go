// Package bootstrap wires the application graph shared by composition roots.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/platform"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/probe"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/tools"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

// Config holds inputs supplied by a composition root.
type Config struct {
	DataPath string           // Empty uses storage.DefaultPath; :memory: keeps SQLite in memory.
	Now      func() time.Time // Nil uses the current UTC time.
}

// New opens storage, applies retention and injects driven adapters. The owner
// must call the idempotent cleanup when the application stops.
func New(ctx context.Context, cfg Config) (*app.Application, func() error, error) {
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	path := cfg.DataPath
	if path == "" {
		var err error
		path, err = storage.DefaultPath()
		if err != nil {
			return nil, nil, fmt.Errorf("bootstrap: data path: %w", err)
		}
	}
	store, err := storage.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("bootstrap: open storage: %w", err)
	}
	service := app.New(app.Deps{
		Jobs: store, Settings: store, Diagnostics: network.Diagnostics{Prober: probe.ICMP{}},
		Host: platform.Host{}, Tools: tools.Runner{}, WiFi: wifi.Native{}, Capture: capture.Native{}, Clock: now,
	})
	application := &app.Application{Service: service}
	var once sync.Once
	var closeErr error
	cleanup := func() error {
		once.Do(func() { closeErr = errors.Join(application.Close(), store.Close()) })
		return closeErr
	}
	settings, err := store.Settings(ctx)
	if err == nil {
		if err := store.Retain(ctx, now().AddDate(0, 0, -settings.RetentionDays)); err != nil {
			_ = cleanup()
			return nil, nil, fmt.Errorf("bootstrap: retention: %w", err)
		}
	}
	return application, cleanup, nil
}
