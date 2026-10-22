package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const tokenBytes = 32
const sessionCookie = "kali_session_"
const maxSessionBody = 1 << 10

type session struct {
	launch, cookie, name string
	secure               bool
}

func newSession(port int, secure bool) (*session, error) {
	launch := make([]byte, tokenBytes)
	cookie := make([]byte, tokenBytes)
	// Go's crypto/rand.Read terminates the process on entropy failure and always returns nil.
	_, _ = rand.Read(launch)
	_, _ = rand.Read(cookie)
	return &session{launch: base64.RawURLEncoding.EncodeToString(launch), cookie: base64.RawURLEncoding.EncodeToString(cookie), name: sessionCookie + strconv.Itoa(port), secure: secure}, nil
}

func (s *session) valid(r *http.Request) bool {
	if cookie, err := r.Cookie(s.name); err == nil && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.cookie)) == 1 {
		return true
	}
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return subtle.ConstantTimeCompare([]byte(bearer), []byte(s.launch)) == 1
	}
	return false
}

func (s *session) exchange(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &request, maxSessionBody); err != nil {
		writeError(w, r, err)
		return
	}
	if subtle.ConstantTimeCompare([]byte(request.Token), []byte(s.launch)) != 1 {
		writeError(w, r, requestError(http.StatusUnauthorized, domain.CodeUnauthorized, "invalid token"))
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.name, Value: s.cookie, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.secure})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && (r.Method != "POST" || r.URL.Path != "/api/session") && !s.session.valid(r) {
			writeError(w, r, requestError(http.StatusUnauthorized, domain.CodeUnauthorized, "session required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
