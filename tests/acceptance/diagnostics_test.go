package acceptance

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
)

func DiagnosticsSpec(t *testing.T, newDriver scenarioDriver) {
	t.Helper()
	t.Run("http_target_reports_status", func(t *testing.T) {
		d := newDriver(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
		t.Cleanup(server.Close)
		v, err := d.Diagnose(testContext(t), contract.DiagnosticRequest{Kind: "http", Target: server.URL, TimeoutMS: 3000})
		if err != nil || v.Status != "completed" || !slices.ContainsFunc(v.Values, func(v string) bool { return strings.Contains(v, "204 No Content") }) {
			t.Fatalf("result=%+v err=%v", v, err)
		}
	})
	t.Run("rejects_command_like_target", func(t *testing.T) {
		d := newDriver(t)
		_, err := d.Diagnose(testContext(t), contract.DiagnosticRequest{Kind: "tcp", Target: "a;whoami", Port: 443, TimeoutMS: 3000})
		assertCode(t, err, "invalid_input")
		rows, err := d.History(testContext(t))
		if err != nil || len(rows) != 0 {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
	})
	t.Run("timeout_bounds", func(t *testing.T) {
		for _, timeout := range []int{99, 100, 30000, 30001} {
			t.Run(strconv.Itoa(timeout), func(t *testing.T) {
				d := newDriver(t)
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				port := listener.Addr().(*net.TCPAddr).Port
				if err := listener.Close(); err != nil {
					t.Fatal(err)
				}
				_, err = d.Diagnose(testContext(t), contract.DiagnosticRequest{Kind: "tcp", Target: "127.0.0.1", Port: port, TimeoutMS: timeout})
				if timeout == 99 || timeout == 30001 {
					assertCode(t, err, "invalid_input")
				} else {
					var mapped *contract.Error
					if errors.As(err, &mapped) && mapped.Code == "invalid_input" {
						t.Fatal("valid timeout rejected")
					}
				}
			})
		}
	})
}
