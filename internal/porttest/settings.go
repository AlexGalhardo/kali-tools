package porttest

import (
	"fmt"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func SettingsStoreContract(t *testing.T, newStore func(*testing.T) app.SettingsStore) {
	t.Helper()
	t.Run("defaults", func(t *testing.T) {
		s := newStore(t)
		v, err := s.Settings(deadline(t))
		if err != nil || v != (app.Settings{Language: "en", RetentionDays: 30}) {
			t.Fatalf("settings=%+v err=%v", v, err)
		}
	})
	t.Run("save_then_load", func(t *testing.T) {
		s := newStore(t)
		ctx := deadline(t)
		want := app.Settings{Language: "pt-BR", RetentionDays: 365}
		if err := s.SaveSettings(ctx, want); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Settings(ctx); err != nil || got != want {
			t.Fatalf("settings=%+v err=%v", got, err)
		}
	})
	t.Run("retention_bounds", func(t *testing.T) {
		for _, days := range []int{MinRetentionDays - 1, MinRetentionDays, MaxRetentionDays, MaxRetentionDays + 1} {
			t.Run(fmt.Sprint(days), func(t *testing.T) {
				s := newStore(t)
				err := s.SaveSettings(deadline(t), app.Settings{Language: "en", RetentionDays: days})
				if days < MinRetentionDays || days > MaxRetentionDays {
					requireCode(t, err, domain.CodeInvalidInput)
				} else if err != nil {
					t.Fatal(err)
				}
			})
		}
	})
	t.Run("language_allow_list", func(t *testing.T) {
		s := newStore(t)
		requireCode(t, s.SaveSettings(deadline(t), app.Settings{Language: "fr", RetentionDays: 30}), domain.CodeInvalidInput)
	})
	t.Run("rejected_save_keeps_previous", func(t *testing.T) {
		s := newStore(t)
		ctx := deadline(t)
		want := app.Settings{Language: "pt-BR", RetentionDays: 7}
		if err := s.SaveSettings(ctx, want); err != nil {
			t.Fatal(err)
		}
		requireCode(t, s.SaveSettings(ctx, app.Settings{Language: "fr", RetentionDays: 30}), domain.CodeInvalidInput)
		if got, err := s.Settings(ctx); err != nil || got != want {
			t.Fatalf("settings=%+v err=%v", got, err)
		}
	})
}
