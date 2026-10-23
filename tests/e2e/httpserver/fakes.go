//go:build e2e

package main

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

const fixtureHistoryLimit = 200
const fixtureMinRetention = 1
const fixtureMaxRetention = 365

func NewStore() *Store {
	return &Store{statuses: make(map[job.ID]job.Status), settings: app.Settings{Language: "en", RetentionDays: 30}}
}

type Store struct {
	mu       sync.Mutex
	statuses map[job.ID]job.Status
	settings app.Settings
}

func (s *Store) PutJob(ctx context.Context, st job.Status) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fake store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.statuses == nil {
		s.statuses = make(map[job.ID]job.Status)
	}
	if previous, ok := s.statuses[st.ID]; ok {
		st.Created = previous.Created
		st.Kind = previous.Kind
	}
	st.Created = st.Created.UTC()
	if st.Finished != nil {
		finished := st.Finished.UTC()
		st.Finished = &finished
	}
	s.statuses[st.ID] = cloneStatus(st)
	return nil
}

func (s *Store) History(ctx context.Context) ([]job.Status, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fake store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]job.Status, 0, len(s.statuses))
	for _, st := range s.statuses {
		rows = append(rows, cloneStatus(st))
	}
	slices.SortFunc(rows, func(a, b job.Status) int { return b.Created.Compare(a.Created) })
	return rows[:min(len(rows), fixtureHistoryLimit)], nil
}

func (s *Store) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fake store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, st := range s.statuses {
		if st.State.Terminal() {
			delete(s.statuses, id)
		}
	}
	return nil
}

func (s *Store) Retain(ctx context.Context, before time.Time) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fake store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, st := range s.statuses {
		if st.State.Terminal() && st.Created.Before(before) {
			delete(s.statuses, id)
		}
	}
	return nil
}

func cloneStatus(st job.Status) job.Status {
	if st.Finished != nil {
		finished := *st.Finished
		st.Finished = &finished
	}
	return st
}

func (s *Store) Settings(ctx context.Context) (app.Settings, error) {
	if err := ctx.Err(); err != nil {
		return app.Settings{}, fmt.Errorf("fake settings: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settings.Language == "" {
		return app.Settings{Language: "en", RetentionDays: 30}, nil
	}
	return s.settings, nil
}
func (s *Store) SaveSettings(ctx context.Context, value app.Settings) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fake settings: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if (value.Language != "en" && value.Language != "pt-BR") || value.RetentionDays < fixtureMinRetention || value.RetentionDays > fixtureMaxRetention {
		return domain.Invalid("invalid settings")
	}
	s.settings = value
	return nil
}

type Host struct {
	Snapshot      app.HostSnapshot
	Err           error
	InventoryFunc func(context.Context) (app.HostSnapshot, error)
}

func (h Host) Neighbors(ctx context.Context) (network.NeighborTable, error) {
	if err := ctx.Err(); err != nil {
		return network.NeighborTable{}, fmt.Errorf("fake neighbors: %w", err)
	}
	return network.NeighborTable{Neighbors: []network.Neighbor{
		{Address: "192.0.2.1", MAC: "02:00:00:00:00:01", Family: "ipv4", InterfaceIndex: 1, InterfaceName: "Fixture Ethernet", State: "reachable", Router: true},
	}, Provider: "synthetic passive cache"}, nil
}

func (h Host) Inventory(ctx context.Context) (app.HostSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return app.HostSnapshot{}, fmt.Errorf("fake host: %w", err)
	}
	v, err := h.Snapshot, h.Err
	if h.InventoryFunc != nil {
		v, err = h.InventoryFunc(ctx)
	}
	v.Adapters = slices.Clone(v.Adapters)
	for i := range v.Adapters {
		v.Adapters[i].Gateways = slices.Clone(v.Adapters[i].Gateways)
		v.Adapters[i].DNS = slices.Clone(v.Adapters[i].DNS)
	}
	v.Counters = slices.Clone(v.Counters)
	return v, err
}

type Tools struct {
	Tools  []app.Tool
	Result app.ServiceInspection
	Err    error
}

func (f Tools) Inventory(ctx context.Context) []app.Tool {
	if ctx.Err() != nil {
		return nil
	}
	return slices.Clone(f.Tools)
}
func (f Tools) InspectServices(ctx context.Context, cidr string, ports []int) (app.ServiceInspection, error) {
	if err := ctx.Err(); err != nil {
		return app.ServiceInspection{}, fmt.Errorf("fake tools: %w", err)
	}
	if err := network.ValidateDiscovery(cidr, ports, time.Second); err != nil {
		return app.ServiceInspection{}, err
	}
	v := f.Result
	v.Hosts = slices.Clone(v.Hosts)
	for i := range v.Hosts {
		v.Hosts[i].Addresses = slices.Clone(v.Hosts[i].Addresses)
		v.Hosts[i].Ports = slices.Clone(v.Hosts[i].Ports)
	}
	return v, f.Err
}

type WiFi struct {
	Network wifi.Network
	Secret  wifi.Secret
	Err     error
}

func (f WiFi) Current(ctx context.Context) (wifi.Network, error) {
	if err := ctx.Err(); err != nil {
		return wifi.Network{}, fmt.Errorf("fake Wi-Fi: %w", err)
	}
	v := f.Network
	if v.Channel != nil {
		channel := *v.Channel
		v.Channel = &channel
	}
	return v, f.Err
}
func (f WiFi) Reveal(ctx context.Context) (wifi.Secret, error) {
	if err := ctx.Err(); err != nil {
		return wifi.Secret{}, fmt.Errorf("fake reveal: %w", err)
	}
	return f.Secret, f.Err
}

type Prober struct {
	Values []string
	Err    error
}

func (Prober) Provider() string { return "fake" }
func (f Prober) Run(ctx context.Context, target string, _ bool) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fake prober: %w", err)
	}
	if err := network.ValidateHost(target); err != nil {
		return nil, err
	}
	return slices.Clone(f.Values), f.Err
}

type LiveCapture struct {
	Result      capture.LiveResult
	Err         error
	CaptureFunc func(context.Context, capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error)
}

func (f LiveCapture) Status(context.Context) domain.Capability {
	return domain.Capability{ID: "capture_live", State: "available", Provider: "fake", Detail: "synthetic capture"}
}

func (f LiveCapture) Capture(ctx context.Context, r capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return capture.LiveResult{}, nil, fmt.Errorf("fake capture: %w", err)
	}
	if err := capture.ValidateLiveRequest(r); err != nil {
		return capture.LiveResult{}, nil, err
	}
	v, err := f.Result, f.Err
	var artifact *capture.Artifact
	if f.CaptureFunc != nil {
		v, artifact, err = f.CaptureFunc(ctx, r)
	}
	v.Packets = slices.Clone(v.Packets)
	v.Limits = slices.Clone(v.Limits)
	if v.Dropped != nil {
		dropped := *v.Dropped
		v.Dropped = &dropped
	}
	return v, artifact, err
}
