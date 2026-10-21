package storage_test

import (
	"path/filepath"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/porttest"
)

func contractStore(t *testing.T) *storage.Store {
	t.Helper()
	s, err := storage.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}
func TestJobStoreContract(t *testing.T) {
	porttest.JobStoreContract(t, func(t *testing.T) app.JobStore { return contractStore(t) })
}
func TestSettingsStoreContract(t *testing.T) {
	porttest.SettingsStoreContract(t, func(t *testing.T) app.SettingsStore { return contractStore(t) })
}
func TestStoreLimitsMatchContract(t *testing.T) {
	if storage.HistoryLimit != porttest.HistoryLimit || storage.MinRetentionDays != porttest.MinRetentionDays || storage.MaxRetentionDays != porttest.MaxRetentionDays {
		t.Fatal("store and contract limits differ")
	}
}
