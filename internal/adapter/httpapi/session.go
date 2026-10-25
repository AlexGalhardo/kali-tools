package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const tokenBytes = 32
const sessionCookie = "kali_session_"
const maxSessionBody = 1 << 10

const (
	maxFailures    = 5
	failureWindow  = time.Minute
	maxTrackedPeer = 1024
)

type failure struct {
	count int
	reset time.Time
}

// failureTracker locks a peer out for failureWindow after maxFailures bad
// tokens. The map is bounded; when full of live entries it is cleared, which
// favors availability over remembering every attacker address.
type failureTracker struct {
	mu   sync.Mutex
	now  func() time.Time
	seen map[string]*failure
}

func (t *failureTracker) locked(peer string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	f := t.seen[peer]
	return f != nil && f.count >= maxFailures && t.now().Before(f.reset)
}

func (t *failureTracker) record(peer string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	f := t.seen[peer]
	if f == nil || !now.Before(f.reset) {
		if len(t.seen) >= maxTrackedPeer {
			for key, entry := range t.seen {
				if !now.Before(entry.reset) {
					delete(t.seen, key)
				}
			}
			if len(t.seen) >= maxTrackedPeer {
				clear(t.seen)
			}
		}
		f = &failure{}
		t.seen[peer] = f
	}
	f.count++
	f.reset = now.Add(failureWindow)
}

type session struct {
	launch, cookie, name string
	secure               bool
	failures             *failureTracker
}

func newSession(port int, secure bool) (*session, error) {
	launch := make([]byte, tokenBytes)
	cookie := make([]byte, tokenBytes)
	// Go's crypto/rand.Read terminates the process on entropy failure and always returns nil.
	_, _ = rand.Read(launch)
	_, _ = rand.Read(cookie)
	return &session{launch: base64.RawURLEncoding.EncodeToString(launch), cookie: base64.RawURLEncoding.EncodeToString(cookie), name: cookieName(port, secure), secure: secure, failures: &failureTracker{now: time.Now, seen: map[string]*failure{}}}, nil
}

// The __Host- prefix needs Secure, Path=/ and no Domain, so it applies only over TLS.
func cookieName(port int, secure bool) string {
	name := sessionCookie + strconv.Itoa(port)
	if secure {
		return "__Host-" + name
	}
	return name
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
	peer, _, _ := net.SplitHostPort(r.RemoteAddr)
	if s.failures.locked(peer) {
		writeError(w, r, requestError(http.StatusTooManyRequests, domain.CodeConflict, "too many failed attempts"))
		return
	}
	if subtle.ConstantTimeCompare([]byte(request.Token), []byte(s.launch)) != 1 {
		s.failures.record(peer)
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
