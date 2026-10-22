package app

import (
	"context"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

type JobStore interface {
	PutJob(context.Context, job.Status) error
	History(context.Context) ([]job.Status, error)
	Clear(context.Context) error
	Retain(context.Context, time.Time) error
}

type SettingsStore interface {
	Settings(context.Context) (Settings, error)
	SaveSettings(context.Context, Settings) error
}

type HostNetwork interface {
	Inventory(context.Context) (HostSnapshot, error)
	Neighbors(context.Context) (network.NeighborTable, error)
}

type ToolRunner interface {
	Inventory(context.Context) []Tool
	RunNmap(context.Context, string, []int) (Nmap, error)
}

type WiFiProvider interface {
	Current(context.Context) (wifi.Network, error)
	Reveal(context.Context) (wifi.Secret, error)
}

type CaptureBackend interface {
	capture.LiveProvider
	Status(context.Context) domain.Capability
}

type Deps struct {
	Jobs            JobStore
	Settings        SettingsStore
	Diagnostics     network.Diagnostics
	Host            HostNetwork
	Tools           ToolRunner
	WiFi            WiFiProvider
	CaptureBackends map[string]CaptureBackend
	Clock           func() time.Time
}
