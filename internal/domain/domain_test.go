package domain_test

import (
	"slices"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestCapabilityStatesAndErrorCodes(t *testing.T) {
	wantStates := []string{"available", "dependency_missing", "permission_required", "location_consent_required", "unsupported", "failed"}
	wantCodes := []string{"invalid_input", "unsupported", "permission_required", "dependency_missing", "location_consent_required", "not_found", "timeout", "cancelled", "failed", "certificate_invalid", "no_connection", "conflict", "unauthorized", "forbidden", "unavailable"}
	if !slices.Equal(domain.CapabilityStates(), wantStates) || !slices.Equal(domain.ErrorCodes(), wantCodes) {
		t.Fatal("state/code lists differ")
	}
	for _, test := range []struct {
		values []string
		has    func(string) bool
	}{{domain.CapabilityStates(), domain.IsCapabilityState}, {domain.ErrorCodes(), domain.IsErrorCode}} {
		seen := make(map[string]bool)
		for _, value := range test.values {
			if seen[value] || !test.has(value) {
				t.Fatalf("duplicate or unknown %q", value)
			}
			seen[value] = true
		}
		if test.has("") || test.has("made_up") {
			t.Fatal("accepted unknown value")
		}
	}
	states, codes := domain.CapabilityStates(), domain.ErrorCodes()
	states[0] = "changed"
	codes[0] = "changed"
	if !slices.Equal(domain.CapabilityStates(), wantStates) || !slices.Equal(domain.ErrorCodes(), wantCodes) {
		t.Fatal("lists alias mutable state")
	}
	if got := domain.Invalid("value %d", 1).Error(); got != "invalid_input: value 1" {
		t.Fatal(got)
	}
	if got := domain.Unsupported("x").Error(); got != "unsupported: x" {
		t.Fatal(got)
	}
}
