package httpapi

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestHTTPContractMatchesRoutes(t *testing.T) {
	source, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile("../../../docs/http.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc)
	registered := regexp.MustCompile(`mux\.Handle\("([^"]+)"`).FindAllStringSubmatch(string(source), -1)
	if len(registered) == 0 {
		t.Fatal("no routes")
	}
	seen := map[string]bool{}
	for _, match := range registered {
		pattern := match[1]
		seen[pattern] = true
		if !strings.Contains(text, "`"+pattern+"`") {
			t.Errorf("docs/http.md missing %s", pattern)
		}
	}
	for _, match := range regexp.MustCompile("`((?:GET|POST|PUT|DELETE) /[^`]+)`").FindAllStringSubmatch(text, -1) {
		if !seen[match[1]] {
			t.Errorf("docs/http.md lists %s, which routes.go does not register", match[1])
		}
	}
	for _, code := range domain.ErrorCodes() {
		row := fmt.Sprintf("| `%s` | %d |", code, statusFor(code))
		if !strings.Contains(text, row) {
			t.Errorf("docs/http.md missing %s", row)
		}
	}
	limits := []struct {
		phrase string
		limit  int64
	}{
		{"Default JSON body limit: %s.", maxJSONBody},
		{"`POST /api/session` body limit: %s.", maxSessionBody},
		{"`POST /api/tls/audit` body limit: %s.", maxTLSAuditBody},
		{"`POST /api/dns/inspect` body limit: %s.", maxDNSInspectBody},
		{"`POST /api/discovery/local` body limit: %s.", maxLocalDiscoveryBody},
		{"`POST /api/snmp` body limit: %s.", maxSNMPBody},
		{"`POST /api/capture/import` body limit: %s.", maxCaptureUpload},
	}
	for _, item := range limits {
		phrase := fmt.Sprintf(item.phrase, bodyLimit(item.limit))
		if !strings.Contains(text, phrase) {
			t.Errorf("docs/http.md missing %s", phrase)
		}
	}
}

func bodyLimit(n int64) string {
	if n%(1<<20) == 0 {
		return fmt.Sprintf("%d MiB", n>>20)
	}
	return fmt.Sprintf("%d KiB", n>>10)
}
