package app_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
)

type neighborHost struct {
	fake.Host
	query func(context.Context) (network.NeighborTable, error)
}

func (h neighborHost) Neighbors(ctx context.Context) (network.NeighborTable, error) {
	return h.query(ctx)
}

func TestApplicationNeighborsUsesPortAndBounds(t *testing.T) {
	deps := fakeDeps()
	rows := make([]network.Neighbor, 4097, 4098)
	address := netip.MustParseAddr("10.0.0.1")
	for i := range rows {
		rows[i] = network.Neighbor{Address: address.String(), Family: "ipv4", InterfaceIndex: 1}
		address = address.Next()
	}
	rows = append(rows, network.Neighbor{Address: "2001:db8::1", Family: "ipv6", InterfaceIndex: 1})
	calls := 0
	deps.Host = neighborHost{query: func(ctx context.Context) (network.NeighborTable, error) {
		calls++
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Fatal("missing neighbor deadline")
		}
		return network.NeighborTable{Neighbors: rows, Provider: "synthetic passive cache"}, nil
	}}
	a := serviceWithDeps(t, deps)
	got, err := a.Neighbors(t.Context(), "ipv4")
	if err != nil || len(got.Neighbors) != 4096 || !got.Limited || got.Provider != "synthetic passive cache" {
		t.Fatal(len(got.Neighbors), got.Limited, err)
	}
	got, err = a.Neighbors(t.Context(), "ipv6")
	if err != nil || len(got.Neighbors) != 1 || got.Limited || got.Neighbors[0].Family != "ipv6" {
		t.Fatal(got, err)
	}
	if _, err := a.Neighbors(t.Context(), "bogus"); err == nil || calls != 2 {
		t.Fatal("invalid family called host", err, calls)
	}
	if history, err := a.History(t.Context()); err != nil || len(history) != 0 {
		t.Fatal("passive inventory wrote history", history, err)
	}
}

func TestApplicationNeighborsErrorsAndCancellation(t *testing.T) {
	for _, mode := range []string{"missing", "error", "cancelled before", "cancelled after", "short deadline", "limited provider"} {
		t.Run(mode, func(t *testing.T) {
			deps := fakeDeps()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := 0
			if mode != "missing" {
				deps.Host = neighborHost{query: func(inner context.Context) (network.NeighborTable, error) {
					calls++
					if mode == "error" {
						return network.NeighborTable{}, domain.Unsupported("synthetic provider state")
					}
					if mode == "cancelled after" {
						cancel()
					}
					if mode == "short deadline" {
						deadline, _ := inner.Deadline()
						if time.Until(deadline) > time.Second {
							t.Fatal("parent deadline extended")
						}
					}
					return network.NeighborTable{Neighbors: []network.Neighbor{}, Limited: mode == "limited provider", Provider: "fixture"}, nil
				}}
			}
			if mode == "cancelled before" {
				cancel()
			}
			if mode == "short deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, time.Second)
				defer stop()
			}
			a := serviceWithDeps(t, deps)
			got, err := a.Neighbors(ctx, "all")
			switch mode {
			case "missing", "error":
				var de *domain.Error
				if !errors.As(err, &de) || de.Code != "unsupported" {
					t.Fatal(err)
				}
			case "cancelled before", "cancelled after":
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				if mode == "cancelled before" && calls != 0 {
					t.Fatal("cancelled request queried host")
				}
			default:
				if err != nil || got.Neighbors == nil || got.Limited != (mode == "limited provider") {
					t.Fatal(got, err)
				}
			}
		})
	}
}
