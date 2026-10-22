package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSessionExchange(t *testing.T) {
	for _, secure := range []bool{false, true} {
		s, err := newSession(12345, secure)
		if err != nil {
			t.Fatal(err)
		}
		if len(s.launch) != 43 || len(s.cookie) != 43 || s.launch == s.cookie {
			t.Fatal("session entropy/encoding")
		}
		for _, token := range []string{s.launch, "wrong"} {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/api/session", strings.NewReader(`{"token":"`+token+`"}`))
			r.Header.Set("Content-Type", "application/json")
			s.exchange(w, r)
			if token == "wrong" {
				if w.Code != 401 || len(w.Result().Cookies()) != 0 {
					t.Fatal(w.Code, w.Header())
				}
				continue
			}
			if w.Code != 204 {
				t.Fatal(w.Code, w.Body)
			}
			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatal(cookies)
			}
			cookie := cookies[0]
			if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Secure != secure || !cookie.Expires.IsZero() || cookie.MaxAge != 0 || cookie.Name != "kali_session_12345" {
				t.Fatal(cookie)
			}
			request := httptest.NewRequest("GET", "/api/history", nil)
			request.AddCookie(cookie)
			if !s.valid(request) {
				t.Fatal("valid cookie rejected")
			}
		}
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer "+s.launch)
		if !s.valid(r) {
			t.Fatal("valid bearer rejected")
		}
		r.Header.Set("Authorization", "Bearer wrong")
		if s.valid(r) {
			t.Fatal("invalid bearer accepted")
		}
	}
}

func TestOversizedBodyRejected(t *testing.T) {
	s, _ := securedServer(t)
	valid := `{"token":"` + s.session.launch + `"}`
	for _, size := range []int{maxSessionBody, maxSessionBody + 1} {
		w := serveRequest(s, "POST", "/api/session", valid+strings.Repeat(" ", size-len(valid)), false)
		want := 204
		if size > maxSessionBody {
			want = 413
		}
		if w.Code != want {
			t.Fatal(size, w.Code, w.Body)
		}
	}
	valid = `{"language":"en","retention_days":30}`
	for _, size := range []int{maxJSONBody, maxJSONBody + 1} {
		w := serveRequest(s, "PUT", "/api/settings", valid+strings.Repeat(" ", size-len(valid)), true)
		want := 204
		if size > maxJSONBody {
			want = 413
		}
		if w.Code != want {
			t.Fatal(size, w.Code, w.Body)
		}
	}
}

func TestSessionUsesConstantTimeCompare(t *testing.T) {
	raw, err := os.ReadFile("session.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "subtle.ConstantTimeCompare") != 3 {
		t.Fatal("cookie, bearer and exchange must use constant-time comparison")
	}
	for _, comparison := range []string{"request.Token ==", "request.Token !=", "bearer ==", "bearer !=", "cookie.Value ==", "cookie.Value !="} {
		if strings.Contains(string(raw), comparison) {
			t.Fatal("unsafe token comparison", comparison)
		}
	}
}

func TestTokenNeverLogged(t *testing.T) {
	s, logs := securedServer(t)
	for _, token := range []string{s.session.launch, s.session.cookie} {
		w := serveRequest(s, "POST", "/api/session?token="+s.session.launch, `{"token":"`+token+`"}`, false)
		if strings.Contains(w.Body.String(), s.session.launch) || strings.Contains(w.Body.String(), s.session.cookie) {
			t.Fatal("token reflected in response")
		}
	}
	w := serveRequest(s, "POST", "/api/wifi/reveal", "", true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "synthetic-test-key") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Header(), w.Body)
	}
	serveRequest(s, "GET", "/api/events?token="+s.session.launch, "", true)
	r := httptest.NewRequest("GET", "/panic?secret="+s.session.launch, nil)
	w = httptest.NewRecorder()
	handler := recoverPanics(s.accessLog(securityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(s.session.launch + "synthetic-test-key") }))), s.log)
	handler.ServeHTTP(w, r)
	if w.Code != 500 || strings.Contains(w.Body.String(), s.session.launch) {
		t.Fatal(w.Code, w.Body)
	}
	history, err := s.app.History(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(history)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{s.session.launch, s.session.cookie, "synthetic-test-key"} {
		if strings.Contains(logs.String()+string(raw), secret) {
			t.Fatal("secret logged or persisted")
		}
	}
	if !strings.Contains(logs.String(), "panic recovered") || !strings.Contains(logs.String(), "session exchange rejected") {
		t.Fatal(logs.String())
	}
}

func TestRevealNoStoreAndNotLogged(t *testing.T) { TestTokenNeverLogged(t) }
