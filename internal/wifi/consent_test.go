package wifi

import (
	"errors"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestCurrentConnectionErrorMapping(t *testing.T) {
	for _, test := range []struct {
		name  string
		code  uintptr
		build uint32
		want  string
	}{
		{"at consent build", 5, 26100, "location_consent_required"},
		{"after consent build", 5, 26200, "location_consent_required"},
		{"before consent build", 5, 26099, "permission_required"},
		{"older windows", 5, 22631, "permission_required"},
		{"not found", 1168, 26100, "failed"},
		{"invalid parameter", 87, 19045, "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var mapped *domain.Error
			if err := currentConnectionError(test.code, test.build); !errors.As(err, &mapped) || mapped.Code != test.want {
				t.Fatalf("mapped error=%v", err)
			}
			if strings.Contains(mapped.Message, "ms-settings:privacy-location") != (test.want == "location_consent_required") {
				t.Fatal("settings URI in wrong error", mapped)
			}
			if test.code == 1168 && mapped.Message != "WLAN API returned 1168" {
				t.Fatal(mapped)
			}
		})
	}
}

func TestRevealStateDecisions(t *testing.T) {
	for _, test := range []struct {
		name           string
		security, oneX uint32
		state          string
		ok             bool
	}{
		{"open", 0, 0, "open_network", true},
		{"enterprise", 1, 1, "enterprise", true},
		{"personal", 1, 0, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, ok := revealState(test.security, test.oneX)
			if state != test.state || ok != test.ok {
				t.Fatalf("state=%q ok=%v", state, ok)
			}
		})
	}
}
