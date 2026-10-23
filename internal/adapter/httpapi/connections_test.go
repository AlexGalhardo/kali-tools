package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConnectionsAndThroughputRoutes(t *testing.T) {
	mux := testMux(t, testApp(t))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/connections", nil))
	if response.Code != http.StatusOK && response.Code != http.StatusNotImplemented {
		t.Fatalf("connections %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "bits_per_second") {
		t.Fatal(response.Body.String())
	}

	response = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/throughput", strings.NewReader(`{"duration_ms":50}`))
	request.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"source":"kali-tools"`) || !strings.Contains(response.Body.String(), `"duration_ms":50`) {
		t.Fatalf("throughput %d %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/throughput", strings.NewReader(`{"duration_ms":50,"iperf_json":"{}"}`))
	request.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("both %d %s", response.Code, response.Body.String())
	}
}
