package app_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/dnsinspect"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
)

func TestInspectDNSHistoryRedaction(t *testing.T) {
	const name = "redact-me.example"
	const resolver = "203.0.113.53"
	store := fake.NewStore()
	a := app.New(app.Deps{Jobs: store, Settings: store, Host: fake.Host{Err: domain.Unsupported("synthetic")}})
	defer func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := a.InspectDNS(t.Context(), dnsinspect.Request{}); err == nil {
		t.Fatal("invalid inspection accepted")
	}
	report, err := a.InspectDNS(t.Context(), dnsinspect.Request{
		Name: name, Type: "A", Resolvers: []string{resolver}, Checks: []string{"compare"}, TimeoutMS: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	history, err := a.History(t.Context())
	if err != nil || len(history) != 1 {
		t.Fatalf("%+v %v", history, err)
	}
	raw, err := json.Marshal(history)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, name) || strings.Contains(text, resolver) || strings.Contains(text, "192.0.2.") || !strings.Contains(text, "DNS inspection completed: "+strconv.Itoa(report.Queries)+" queries, checks=1") {
		t.Fatal(text)
	}
	found := false
	for _, capability := range a.Capabilities(t.Context()) {
		if capability.ID == "dns_inspect" && capability.State == domain.CapabilityAvailable && capability.Provider == "miekg/dns" {
			found = true
		}
	}
	if !found {
		t.Fatal("dns_inspect capability missing")
	}
}
