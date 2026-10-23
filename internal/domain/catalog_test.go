package domain

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestTranslationCatalogsHaveTheSameKeys(t *testing.T) {
	src := readRepoFile(t, "frontend/src/shared/i18n/i18n.ts")
	en := catalogKeys(t, src, "export const en = {", "export const pt")
	pt := catalogKeys(t, src, "export const pt: typeof en = {", "")
	if !slices.Equal(en, pt) {
		t.Fatalf("catalog keys differ\nen: %v\npt: %v", en, pt)
	}
}

func TestUILabelsCoverErrorCodesAndCapabilityStates(t *testing.T) {
	src := readRepoFile(t, "frontend/src/app/App.tsx")
	catalog := readRepoFile(t, "frontend/src/shared/i18n/i18n.ts")
	en := catalogKeys(t, catalog, "export const en = {", "export const pt")
	errors := objectFields(t, src, "const errorLabels: Record<string, string> = {", "const errorText")
	codes := ErrorCodes()
	slices.Sort(codes)
	if !slices.Equal(sortedKeys(errors), codes) {
		t.Fatalf("error labels = %v, codes = %v", sortedKeys(errors), codes)
	}
	states := objectFields(t, src, "const labels: Record<string, string> = {", "const stateText")
	for _, state := range CapabilityStates() {
		if _, ok := states[state]; !ok {
			t.Fatalf("capability state %s has no UI label", state)
		}
	}
	for _, fields := range []map[string]string{errors, states} {
		for code, key := range fields {
			if !slices.Contains(en, key) {
				t.Fatalf("%s uses missing catalog key %s", code, key)
			}
		}
	}
}

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func catalogKeys(t *testing.T, src, start, end string) []string {
	t.Helper()
	block := between(t, src, start, end)
	seen := map[string]struct{}{}
	var keys []string
	for _, match := range regexp.MustCompile(`(?m)^  ([A-Za-z_][A-Za-z0-9_]*):`).FindAllStringSubmatch(block, -1) {
		key := match[1]
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate catalog key %s", key)
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if len(keys) == 0 {
		t.Fatal("no catalog keys")
	}
	return keys
}

func objectFields(t *testing.T, src, start, end string) map[string]string {
	t.Helper()
	block := between(t, src, start, end)
	fields := map[string]string{}
	for _, match := range regexp.MustCompile(`(?m)^    ([a-z0-9_]+): t\.([A-Za-z0-9_]+),`).FindAllStringSubmatch(block, -1) {
		fields[match[1]] = match[2]
	}
	if len(fields) == 0 {
		t.Fatalf("no fields between %s and %s", start, end)
	}
	return fields
}

func between(t *testing.T, src, start, end string) string {
	t.Helper()
	at := strings.Index(src, start)
	if at < 0 {
		t.Fatalf("missing %s", start)
	}
	rest := src[at+len(start):]
	if end == "" {
		return rest
	}
	stop := strings.Index(rest, end)
	if stop < 0 {
		t.Fatalf("missing %s", end)
	}
	return rest[:stop]
}

func sortedKeys(fields map[string]string) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
