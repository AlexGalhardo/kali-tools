// Package bootstrap wires the application graph shared by composition roots.
package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/probe"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/network"
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
	service := app.New(store, network.Diagnostics{Prober: probe.ICMP{}})
	application := &app.Application{Service: service}
	settings, err := store.Settings(ctx)
	if err == nil {
		if err := store.Retain(ctx, now().AddDate(0, 0, -settings.RetentionDays)); err != nil {
			_ = application.Close()
			return nil, nil, fmt.Errorf("bootstrap: retention: %w", err)
		}
	}
	return application, application.Close, nil
}
