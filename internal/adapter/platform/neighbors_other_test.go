//go:build !windows && !linux

package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestNeighborsUnsupportedFallback(t *testing.T) {
	result, err := (Host{}).Neighbors(t.Context())
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.CodeUnsupported || result.Neighbors == nil {
		t.Fatal(result, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (Host{}).Neighbors(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
