package tlsaudit

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestZlintFindingsDeterministic(t *testing.T) {
	cert := fixtureCertificate(t, time.Now())
	a, engine, err := zlintFindings(cert.Raw)
	if err != nil || len(a) == 0 || !strings.HasPrefix(engine, "zlint") {
		t.Fatalf("%+v %s %v", a, engine, err)
	}
	b, _, err := zlintFindings(cert.Raw)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("nondeterministic lint")
	}
	if _, _, err := zlintFindings([]byte("bad certificate")); err == nil {
		t.Fatal("malformed accepted")
	}
	for _, f := range a {
		if f.Source != "zlint" || len(f.Detail) > MaxDetailBytes {
			t.Fatal(f)
		}
	}
}
