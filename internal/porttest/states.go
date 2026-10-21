package porttest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const (
	HistoryLimit     = 200
	MinRetentionDays = 1
	MaxRetentionDays = 365
	contractDeadline = 3 * time.Second
)

func deadline(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), contractDeadline)
	t.Cleanup(cancel)
	return ctx
}
func honestState(err error, allowed ...string) error {
	if err == nil {
		return nil
	}
	var d *domain.Error
	if errors.As(err, &d) && d != nil && slices.Contains(allowed, d.Code) {
		return nil
	}
	return fmt.Errorf("unexpected provider state: %w", err)
}
func RequireHonestState(t *testing.T, err error, allowed ...string) {
	t.Helper()
	if failure := honestState(err, allowed...); failure != nil {
		t.Fatal(failure)
	}
	if err == nil {
		t.Log("data branch")
	} else {
		var d *domain.Error
		_ = errors.As(err, &d)
		t.Logf("honest-state branch: %s", d.Code)
	}
}
func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var d *domain.Error
	if !errors.As(err, &d) || d == nil || d.Code != code {
		t.Fatalf("error=%v, want code %s", err, code)
	}
}
