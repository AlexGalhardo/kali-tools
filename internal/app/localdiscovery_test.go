package app_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/localdiscovery"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
)

func TestLocalDiscoveryHistoryRedaction(t *testing.T) {
	store := fake.NewStore()
	application := app.New(app.Deps{Jobs: store, Settings: store})
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Error(err)
		}
	})
	secret, err := localdiscovery.NewSecret("synthetic-community")
	if err != nil {
		t.Fatal(err)
	}
	_, err = application.QuerySNMP(t.Context(), localdiscovery.SNMPRequest{
		Scope: localdiscovery.Scope{Interface: "missing", CIDR: "192.0.2.0/24"}, Target: "192.0.2.1",
		Owned: true, Version: "v2c", Community: secret, TimeoutMS: 500,
	})
	if err == nil || strings.Contains(err.Error(), "synthetic-community") {
		t.Fatal(err)
	}
	_, err = application.DiscoverLocal(t.Context(), localdiscovery.Request{
		Scope: localdiscovery.Scope{Interface: "missing", CIDR: "192.0.2.0/24"}, Protocols: []string{"mdns"}, DurationSeconds: 1,
	})
	if err == nil {
		t.Fatal("invalid discovery accepted")
	}
	history, err := application.History(t.Context())
	if err != nil || len(history) != 0 {
		t.Fatalf("%+v %v", history, err)
	}
	raw, err := json.Marshal(history)
	if err != nil || strings.Contains(string(raw), "synthetic") {
		t.Fatal(string(raw), err)
	}
	foundDiscovery, foundSNMP := false, false
	for _, capability := range application.Capabilities(t.Context()) {
		if capability.ID == "local_discovery" && capability.State == domain.CapabilityAvailable {
			foundDiscovery = true
		}
		if capability.ID == "snmp" && capability.State == domain.CapabilityAvailable {
			foundSNMP = true
		}
	}
	if !foundDiscovery || !foundSNMP {
		t.Fatal(application.Capabilities(t.Context()))
	}
}
