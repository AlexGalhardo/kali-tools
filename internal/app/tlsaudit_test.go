package app_test

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
	"github.com/AlexGalhardo/kali-tools/internal/tlsaudit"
)

func TestAuditTLSHistoryRedaction(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	store := fake.NewStore()
	a := app.New(app.Deps{Jobs: store, Settings: store})
	defer func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := a.AuditTLS(t.Context(), tlsaudit.Request{}); err == nil {
		t.Fatal("invalid audit accepted")
	}
	report, err := a.AuditTLS(t.Context(), tlsaudit.Request{Host: host, Port: n, TimeoutMS: 500})
	if err != nil || len(report.Chain) == 0 {
		t.Fatalf("%+v %v", report, err)
	}
	history, err := a.History(t.Context())
	if err != nil || len(history) != 1 {
		t.Fatalf("%+v %v", history, err)
	}
	raw, err := json.Marshal(history)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), host) || strings.Contains(string(raw), report.Chain[0].Subject) || !strings.Contains(string(raw), "TLS audit completed: 1 certificates") {
		t.Fatal(string(raw))
	}
}
