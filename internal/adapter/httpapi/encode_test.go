package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONDecodeLimits(t *testing.T) {
	valid := `{"value":"ok"}`
	for _, test := range []struct {
		name, body, contentType string
		status                  int
	}{
		{"at limit", valid + strings.Repeat(" ", maxJSONBody-len(valid)), "application/json", 200},
		{"over limit", valid + strings.Repeat(" ", maxJSONBody-len(valid)+1), "application/json", 413},
		{"unknown field", `{"other":"synthetic-secret"}`, "application/json", 400},
		{"trailing value", valid + ` {}`, "application/json", 400},
		{"trailing garbage", valid + ` synthetic-secret`, "application/json", 400},
		{"malformed", `{`, "application/json", 400},
		{"missing type", valid, "", 415},
		{"wrong type", valid, "text/plain", 415},
		{"type parameters", valid, "Application/JSON; charset=utf-8", 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(test.body))
			r.Header.Set("Content-Type", test.contentType)
			w := httptest.NewRecorder()
			var body struct {
				Value string `json:"value"`
			}
			if err := decodeJSON(w, r, &body, maxJSONBody); err != nil {
				writeError(w, r, err)
			}
			if w.Code != test.status || strings.Contains(w.Body.String(), "synthetic-secret") {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
		})
	}
}

func TestEmptyBodyRejectsAnyByte(t *testing.T) {
	for _, body := range []string{"", "x"} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		if err := decodeEmpty(w, r); err != nil {
			writeError(w, r, err)
		}
		want := http.StatusOK
		if body != "" {
			want = http.StatusRequestEntityTooLarge
		}
		if w.Code != want {
			t.Fatalf("status=%d body=%s", w.Code, w.Body)
		}
	}
}
