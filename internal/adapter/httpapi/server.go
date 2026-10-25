package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const maxSSESubscribers = 8
const maxFileTransfers = 1

type Options struct {
	Addr        string
	Assets      fs.FS
	AllowRemote bool
	TLSCert     string
	TLSKey      string
	// AllowFrom lists CIDRs allowed to connect; empty allows every peer. It requires AllowRemote.
	AllowFrom []string
	Logger    *slog.Logger
}
type Server struct {
	app          *app.Application
	subscribers  chan struct{}
	transfers    chan struct{}
	keepAlive    time.Duration
	listener     net.Listener
	http         *http.Server
	assets       fs.FS
	log          *slog.Logger
	session      *session
	cop          *http.CrossOriginProtection
	allowedHosts map[string]bool
	inflight     chan struct{}
	url          string
	fingerprint  string
	tlsConfig    *tls.Config
	started      atomic.Bool
}

func New(a *app.Application, opts Options) (*Server, error) {
	host, port, err := net.SplitHostPort(opts.Addr)
	if err != nil {
		return nil, domain.Invalid("invalid listen address")
	}
	if host == "localhost" {
		host = "127.0.0.1"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return nil, domain.Invalid("bind a specific address")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 0 || portNumber > 65535 {
		return nil, domain.Invalid("invalid listen port")
	}
	address := net.ParseIP(host)
	loopback := address != nil && address.IsLoopback()
	if !loopback && (!opts.AllowRemote || opts.TLSCert == "" || opts.TLSKey == "") {
		return nil, domain.Invalid("non-loopback listen requires --allow-remote, --tls-cert and --tls-key")
	}
	if (opts.TLSCert == "") != (opts.TLSKey == "") {
		return nil, domain.Invalid("both TLS certificate and key are required")
	}
	if len(opts.AllowFrom) > 0 && !opts.AllowRemote {
		return nil, domain.Invalid("--allow-from requires --allow-remote")
	}
	allowFrom, err := parseAllowFrom(opts.AllowFrom)
	if err != nil {
		return nil, err
	}
	var config *tls.Config
	var fingerprint string
	if opts.TLSCert != "" {
		pair, err := tls.LoadX509KeyPair(opts.TLSCert, opts.TLSKey)
		if err != nil {
			return nil, domain.Invalid("invalid TLS certificate or key")
		}
		config = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}
		sum := sha256.Sum256(pair.Certificate[0])
		parts := make([]string, len(sum))
		for i, value := range sum {
			parts[i] = fmt.Sprintf("%02X", value)
		}
		fingerprint = strings.Join(parts, ":")
	}
	if opts.Assets == nil {
		return nil, &domain.Error{Code: domain.CodeDependencyMissing, Message: "frontend assets are not built; run bun run build in frontend/"}
	}
	if info, err := fs.Stat(opts.Assets, "index.html"); err != nil || info.IsDir() {
		return nil, &domain.Error{Code: domain.CodeDependencyMissing, Message: "frontend assets are not built; run bun run build in frontend/"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), readHeaderTimeout)
	defer cancel()
	var listen net.ListenConfig
	listener, err := listen.Listen(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, &domain.Error{Code: domain.CodeFailed, Message: "could not bind HTTP listener"}
	}
	_, actualPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	actualNumber, err := strconv.Atoi(actualPort)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	launch, err := newSession(actualNumber, config != nil)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if len(allowFrom) > 0 {
		listener = &allowListener{Listener: listener, allow: allowFrom, log: logger}
	}
	allowed := map[string]bool{}
	if loopback {
		for _, name := range []string{"127.0.0.1", "localhost", "::1"} {
			allowed[net.JoinHostPort(name, actualPort)] = true
		}
	} else {
		allowed[strings.ToLower(net.JoinHostPort(host, actualPort))] = true
	}
	scheme := "http"
	if config != nil {
		scheme = "https"
	}
	s := &Server{app: a, listener: listener, assets: opts.Assets, log: logger, session: launch, allowedHosts: allowed, subscribers: make(chan struct{}, maxSSESubscribers), transfers: make(chan struct{}, maxFileTransfers), inflight: make(chan struct{}, maxInflight), keepAlive: sseKeepAlive, tlsConfig: config, fingerprint: fingerprint, url: scheme + "://" + net.JoinHostPort(host, actualPort) + "/#token=" + launch.launch}
	s.cop = http.NewCrossOriginProtection()
	s.cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, requestError(http.StatusForbidden, domain.CodeForbidden, "cross-origin request rejected"))
	}))
	s.http = &http.Server{Handler: s.Handler(), ReadHeaderTimeout: readHeaderTimeout, IdleTimeout: idleTimeout, MaxHeaderBytes: maxHeaderBytes, ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelWarn)}
	return s, nil
}
func (s *Server) URL() string             { return s.url }
func (s *Server) Addr() net.Addr          { return s.listener.Addr() }
func (s *Server) CertFingerprint() string { return s.fingerprint }
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, s.app, s)
	mux.Handle("GET /", http.FileServerFS(s.assets))
	var handler http.Handler = mux
	handler = s.requireSession(handler)
	handler = s.limitRequests(handler)
	handler = s.fetchMetadata(handler)
	handler = s.cop.Handler(handler)
	handler = s.hostAllowlist(handler)
	handler = securityHeaders(handler)
	handler = s.accessLog(handler)
	return recoverPanics(handler, s.log)
}
func (s *Server) Run(ctx context.Context) error {
	if !s.started.CompareAndSwap(false, true) {
		return domain.Invalid("server already started")
	}
	s.http.BaseContext = func(net.Listener) context.Context { return ctx }
	done := make(chan struct{})
	shutdown := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
			defer cancel()
			err := s.http.Shutdown(stop)
			if err != nil {
				_ = s.http.Close()
			}
			shutdown <- err
		case <-done:
			shutdown <- nil
		}
	}()
	listener := s.listener
	if s.tlsConfig != nil {
		listener = tls.NewListener(listener, s.tlsConfig)
	}
	err := s.http.Serve(listener)
	close(done)
	shutdownErr := <-shutdown
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return shutdownErr
}
