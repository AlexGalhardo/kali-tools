//go:build !linux

package conn

import (
	"errors"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestReadIsUnsupported(t *testing.T) {
	table, err := Read(t.Context())
	var target *domain.Error
	if len(table.Connections) != 0 || !errors.As(err, &target) || target.Code != domain.CodeUnsupported {
		t.Fatalf("table=%+v err=%v", table, err)
	}
}
