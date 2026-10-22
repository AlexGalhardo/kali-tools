package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const (
	readHeaderTimeout     = 5 * time.Second
	idleTimeout           = 120 * time.Second
	maxHeaderBytes        = 16 << 10
	shutdownTimeout       = 5 * time.Second
	maxInflight           = 16
	contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, value := range map[string]string{"Content-Security-Policy": contentSecurityPolicy, "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer", "X-Frame-Options": "DENY", "Cross-Origin-Opener-Policy": "same-origin", "Cross-Origin-Resource-Policy": "same-origin", "Permissions-Policy": "camera=(), microphone=(), geolocation=()"} {
			w.Header().Set(key, value)
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) hostAllowlist(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowedHosts[strings.ToLower(r.Host)] {
			writeError(w, r, requestError(http.StatusForbidden, domain.CodeForbidden, "host not allowed"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) fetchMetadata(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if site := r.Header.Get("Sec-Fetch-Site"); strings.HasPrefix(r.URL.Path, "/api/") && site != "" && site != "same-origin" && site != "none" {
			writeError(w, r, requestError(http.StatusForbidden, domain.CodeForbidden, "cross-site request rejected"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) limitRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api/events" {
			next.ServeHTTP(w, r)
			return
		}
		select {
		case s.inflight <- struct{}{}:
			defer func() { <-s.inflight }()
		default:
			writeError(w, r, requestError(http.StatusTooManyRequests, domain.CodeConflict, "too many concurrent requests"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		out := &statusWriter{ResponseWriter: w}
		finished := false
		defer func() {
			status := out.status
			if status == 0 {
				status = http.StatusOK
			}
			if !finished {
				status = http.StatusInternalServerError
			}
			s.log.Info("http request", "method", r.Method, "route", r.Pattern, "status", status, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(out, r)
		finished = true
	})
}

func recoverPanics(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				logger.Error("panic recovered", "route", r.Pattern)
				writeError(w, r, &domain.Error{Code: domain.CodeFailed, Message: "operation failed"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
