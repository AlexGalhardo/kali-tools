package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type readDeadlineWriter struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *readDeadlineWriter) SetReadDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

func TestBodyReadDeadlinesAndReset(t *testing.T) {
	for _, test := range []struct {
		name  string
		body  string
		empty bool
	}{
		{name: "JSON success", body: `{}`},
		{name: "JSON invalid", body: `{`},
		{name: "empty success", empty: true},
		{name: "empty invalid", empty: true, body: "x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(test.body))
			r.Header.Set("Content-Type", "application/json")
			w := &readDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
			before := time.Now()
			if test.empty {
				_ = decodeEmpty(w, r)
			} else {
				var value struct{}
				_ = decodeJSON(w, r, &value, maxJSONBody)
			}
			after := time.Now()
			if len(w.deadlines) != 2 || !w.deadlines[1].IsZero() {
				t.Fatalf("read deadline not reset: %v", w.deadlines)
			}
			if w.deadlines[0].Before(before.Add(30*time.Second)) || w.deadlines[0].After(after.Add(30*time.Second)) {
				t.Fatalf("JSON deadline outside 30-second bound: %v", w.deadlines[0])
			}
		})
	}
}

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
