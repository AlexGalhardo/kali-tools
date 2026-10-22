package httpapi

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

func testMux(t *testing.T, a *app.Application) *http.ServeMux {
	t.Helper()
	s := &Server{app: a, subscribers: make(chan struct{}, maxSSESubscribers), transfers: make(chan struct{}, maxFileTransfers), keepAlive: sseKeepAlive}
	mux := http.NewServeMux()
	addRoutes(mux, a, s)
	return mux
}

func TestCaptureBackendRouteMatchesApplication(t *testing.T) {
	a := testApp(t)
	response := httptest.NewRecorder()
	testMux(t, a).ServeHTTP(response, httptest.NewRequest("GET", "/api/capture/backends", nil))
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body)
	}
	var got []domain.Capability
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, a.CaptureBackendStatus(t.Context())) {
		t.Fatal(got)
	}
}

func TestWriteRoutes(t *testing.T) {
	a := testApp(t)
	mux := testMux(t, a)
	for _, test := range []struct {
		method, path, body string
		status             int
	}{
		{"PUT", "/api/settings", `{"language":"pt-BR","retention_days":7}`, 204},
		{"DELETE", "/api/history", "", 204},
		{"POST", "/api/wifi/reveal", "", 200},
		{"POST", "/api/wifi/reveal", "x", 413},
		{"POST", "/api/capture/stop", "", 204},
		{"POST", "/api/capture/start", `{"address":"127.0.0.1","duration_seconds":1,"packet_limit":1}`, 400},
		{"POST", "/api/diagnostics", `{"kind":"tcp","target":"invalid;host","port":443,"timeout_ms":100}`, 400},
		{"POST", "/api/diagnose", `{"kind":"tcp","target":"invalid;host","port":443,"timeout_ms":100}`, 400},
		{"POST", "/api/discovery", `{"cidr":"0.0.0.0/0","ports":[443]}`, 400},
		{"POST", "/api/inspect", `{"cidr":"127.0.0.1/32","ports":[443]}`, 424},
		{"POST", "/api/jobs/invalid/cancel", "", 400},
		{"POST", "/api/jobs/0123456789abcdef0123456789abcdef/cancel", "", 200},
	} {
		t.Run(test.method+test.path, func(t *testing.T) {
			r := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.body != "" {
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, test.status, w.Body)
			}
		})
	}
	settings, err := a.Settings(t.Context())
	if err != nil || settings.Language != "pt-BR" || settings.RetentionDays != 7 {
		t.Fatal(settings, err)
	}
}

func TestJobRoutesAndSSE(t *testing.T) {
	a := testApp(t)
	mux := testMux(t, a)
	request := httptest.NewRequest("POST", "/api/diagnostics", strings.NewReader(`{"kind":"tcp","target":"127.0.0.1","port":9,"timeout_ms":100}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body)
	}
	var value struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		response = httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", "/api/jobs/"+value.ID, nil))
		if response.Code != 200 {
			t.Fatal(response.Body)
		}
		if strings.Contains(response.Body.String(), `"finished"`) {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("job did not finish")
		default:
		}
	}
}

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

func TestWiFiConsentHTTPError(t *testing.T) {
	store := fake.NewStore()
	a := app.New(app.Deps{Jobs: store, Settings: store, WiFi: fake.WiFi{
		Network: wifi.Network{SSID: "synthetic-ssid", BSSID: "synthetic-bssid", Profile: "synthetic-profile"},
		Secret:  wifi.Secret{State: "available", Value: "synthetic-psk"},
		Err:     &domain.Error{Code: domain.CodeLocationConsentRequired, Message: "Windows location access is off"},
	}})
	t.Cleanup(func() { _ = a.Close() })
	s, err := New(a, Options{Addr: "127.0.0.1:0", Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("fixture")}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.listener.Close()
	for _, route := range []struct{ method, path string }{{"GET", "/api/wifi"}, {"POST", "/api/wifi/reveal"}} {
		w := serveRequest(s, route.method, route.path, "", true)
		var body struct {
			Error contract.Error `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != 403 || body.Error.Code != "location_consent_required" || body.Error.SettingsURI != domain.LocationSettingsURI {
			t.Fatal(w.Code, w.Body)
		}
		for _, secret := range []string{"synthetic-ssid", "synthetic-bssid", "synthetic-profile", "synthetic-psk"} {
			if strings.Contains(w.Body.String(), secret) {
				t.Fatal("Wi-Fi data leaked in consent response")
			}
		}
	}
}

func TestStatusForEveryCode(t *testing.T) {
	for code, want := range map[string]int{"invalid_input": 400, "unauthorized": 401, "forbidden": 403, "permission_required": 403, "location_consent_required": 403, "not_found": 404, "conflict": 409, "dependency_missing": 424, "cancelled": 499, "failed": 500, "unsupported": 501, "no_connection": 502, "certificate_invalid": 502, "timeout": 504, "unknown": 500} {
		if got := statusFor(code); got != want {
			t.Errorf("%s=%d want %d", code, got, want)
		}
	}
}

func TestReadRoutes(t *testing.T) {
	mux := testMux(t, testApp(t))
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
	mux := testMux(t, a)
	for path, want := range map[string]int{"/api/jobs/invalid": 400, "/api/jobs/0123456789abcdef0123456789abcdef": 404, "/api/wifi": 403, "/api/inventory": 501, "/api/tools": 501} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != want || !strings.Contains(response.Body.String(), `"error"`) {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body)
		}
	}
}

func TestRoutesMatchApplication(t *testing.T) {
	// Lifecycle and preflight helpers are not remotely callable operations.
	excluded := map[string]bool{"Close": true, "StartMetrics": true, "CanImportCapture": true, "CanExportCapture": true, "RunMetricsForTest": true, "PublishForTest": true, "WaitForTest": true}
	operations := map[string]string{"Capabilities": "GET /api/capabilities", "CaptureBackendStatus": "GET /api/capture/backends", "Interfaces": "GET /api/interfaces", "Inventory": "GET /api/inventory", "Tools": "GET /api/tools", "Diagnose": "POST /api/diagnose", "StartDiagnostic": "POST /api/diagnostics", "CancelJob": "POST /api/jobs/{id}/cancel", "JobSnapshot": "GET /api/jobs/{id}", "Subscribe": "GET /api/events", "History": "GET /api/history", "ClearHistory": "DELETE /api/history", "Settings": "GET /api/settings", "SaveSettings": "PUT /api/settings", "CurrentWiFi": "GET /api/wifi", "RevealWiFi": "POST /api/wifi/reveal", "Discover": "POST /api/discovery", "InspectServices": "POST /api/inspect", "StartCapture": "POST /api/capture/start", "StopCapture": "POST /api/capture/stop", "ImportCapture": "POST /api/capture/import", "ExportCapture": "GET /api/capture/export"}
	file, err := parser.ParseFile(token.NewFileSet(), "routes.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	routes := map[string]int{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Handle" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return true
		}
		route, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatal(err)
		}
		routes[route]++
		return true
	})
	typ := reflect.TypeFor[*app.Application]()
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if excluded[name] {
			continue
		}
		route, ok := operations[name]
		if !ok || routes[route] != 1 {
			t.Errorf("%s has no unique route (%q count=%d)", name, route, routes[route])
		}
	}
}
