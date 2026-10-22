package app_test

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
)

func TestWiFiObservationUpdatesCapability(t *testing.T) {
	store := fake.NewStore()
	calls := 0
	counted := app.New(app.Deps{Jobs: store, Settings: store, WiFi: recordingWiFi{see: func(context.Context) { calls++ }}})
	t.Cleanup(func() { _ = counted.Close() })
	counted.Capabilities(t.Context())
	if calls != 0 {
		t.Fatal("capabilities queried Wi-Fi")
	}
	for _, reveal := range []bool{false, true} {
		t.Run(fmt.Sprint(reveal), func(t *testing.T) {
			provider := &fake.WiFi{}
			store := fake.NewStore()
			a := app.New(app.Deps{Jobs: store, Settings: store, WiFi: provider})
			t.Cleanup(func() { _ = a.Close() })
			for _, step := range []struct{ code, want string }{
				{"location_consent_required", "location_consent_required"},
				{"failed", "location_consent_required"},
				{"permission_required", "permission_required"},
				{"", "available"},
			} {
				provider.Err = nil
				if step.code != "" {
					provider.Err = fmt.Errorf("wrapped: %w", &domain.Error{Code: step.code, Message: "synthetic"})
				}
				if reveal {
					_, _ = a.RevealWiFi(t.Context())
				} else {
					_, _ = a.CurrentWiFi(t.Context())
				}
				all := a.Capabilities(t.Context())
				index := slices.IndexFunc(all, func(c domain.Capability) bool { return c.ID == "wifi" })
				if index < 0 || all[index].State != step.want {
					t.Fatalf("capabilities=%+v want=%s", all, step.want)
				}
			}
			provider.Err = context.Canceled
			_, _ = a.CurrentWiFi(t.Context())
			all := a.Capabilities(t.Context())
			if c := all[slices.IndexFunc(all, func(c domain.Capability) bool { return c.ID == "wifi" })]; c.State != "available" {
				t.Fatal("unrelated cancellation changed observed state", c)
			}
		})
	}
}

func TestWithObservedOverlay(t *testing.T) {
	base := []domain.Capability{{ID: "wifi", State: "unsupported", Detail: "original"}}
	for _, test := range []struct{ name, id, state, detail string }{
		{"unknown observation", "wifi", "", "ignored"},
		{"unknown capability", "other", "available", "ignored"},
		{"replace observation", "wifi", "location_consent_required", "consent"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := app.WithObserved(base, test.id, test.state, test.detail)
			want := []domain.Capability{{ID: "wifi", State: "unsupported", Detail: "original"}}
			if test.name == "replace observation" {
				want[0].State, want[0].Detail = test.state, test.detail
			}
			if !reflect.DeepEqual(got, want) || base[0].Detail != "original" {
				t.Fatalf("overlay=%+v base=%+v", got, base)
			}
		})
	}
}
