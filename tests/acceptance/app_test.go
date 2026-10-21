package acceptance

import (
	"path/filepath"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
)

func newApplication(t *testing.T) *app.Application {
	t.Helper()
	a, cleanup, err := bootstrap.New(t.Context(), bootstrap.Config{DataPath: filepath.Join(t.TempDir(), "acceptance.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	})
	return a
}
