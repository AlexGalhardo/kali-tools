package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

func testApp(t *testing.T) *app.Application {
	t.Helper()
	store := fake.NewStore()
	a := app.New(app.Deps{Jobs: store, Settings: store, Host: fake.Host{}, Tools: fake.Tools{Tools: []app.Tool{{Name: "nmap", State: "dependency_missing"}, {Name: "tshark", State: "dependency_missing"}}}, WiFi: fake.WiFi{Network: wifi.Network{SSID: "Fixture Network", State: "connected"}, Secret: wifi.Secret{State: "available", Value: "synthetic-test-key"}}})
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	return a
}

func TestStatusForEveryCode(t *testing.T) {
	for code, want := range map[string]int{"invalid_input": 400, "unauthorized": 401, "forbidden": 403, "permission_required": 403, "location_consent_required": 403, "not_found": 404, "conflict": 409, "dependency_missing": 424, "cancelled": 499, "failed": 500, "unsupported": 501, "no_connection": 502, "certificate_invalid": 502, "timeout": 504, "unknown": 500} {
		if got := statusFor(code); got != want {
			t.Errorf("%s=%d want %d", code, got, want)
		}
	}
}

func TestReadRoutes(t *testing.T) {
	mux := http.NewServeMux()
	addRoutes(mux, testApp(t))
	for _, path := range []string{"/healthz", "/api/version", "/api/capabilities", "/api/interfaces", "/api/inventory", "/api/tools", "/api/history", "/api/settings", "/api/wifi"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
			if response.Code != 200 || !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			var data any
			if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReadRouteErrors(t *testing.T) {
	store := fake.NewStore()
	a := app.New(app.Deps{Jobs: store, Settings: store, WiFi: fake.WiFi{Err: &domain.Error{Code: domain.CodeLocationConsentRequired, Message: "synthetic consent"}}})
	defer func() { _ = a.Close() }()
	mux := http.NewServeMux()
	addRoutes(mux, a)
	for path, want := range map[string]int{"/api/jobs/invalid": 400, "/api/jobs/0123456789abcdef0123456789abcdef": 404, "/api/wifi": 403, "/api/inventory": 501, "/api/tools": 501} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != want || !strings.Contains(response.Body.String(), `"error"`) {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body)
		}
	}
}
