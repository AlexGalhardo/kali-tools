package app

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func TestCancellationAndRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	store, e := storage.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	s := New(store, network.Diagnostics{})
	defer func() {
		if e := s.Close(); e != nil {
			t.Error(e)
		}
	}()
	j, e := s.Start(t.Context(), network.Request{Kind: "http", Target: server.URL + "?secret=synthetic-psk", TimeoutMS: 30000})
	if e != nil {
		t.Fatal(e)
	}
	if !s.Cancel(j.ID) {
		t.Fatal("missing active job")
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case event := <-s.Events:
			if event.Job.State == "cancelled" {
				history, e := store.History(context.Background())
				if e != nil || len(history) != 1 {
					t.Fatal(history, e)
				}
				if strings.Contains(history[0].Summary, "synthetic-psk") {
					t.Fatal("secret leaked")
				}
				return
			}
		case <-deadline.C:
			t.Fatal("job did not cancel promptly")
		}
	}
}

func TestDiscoverySummarySurvivesRestartWithoutTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	store, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	service := New(store, network.Diagnostics{})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	results, err := service.Discover(t.Context(), "127.0.0.1/32", []int{listener.Addr().(*net.TCPAddr).Port}, 100*time.Millisecond)
	if err != nil || len(results) != 1 || results[0].State != "open" {
		t.Fatal(results, err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	history, err := store.History(t.Context())
	if err != nil || len(history) != 1 || history[0].Kind != "discovery" || history[0].State != "completed" || history[0].Summary != "Discovery completed: 1 endpoints checked, 1 open" {
		t.Fatal(history, err)
	}
	if strings.Contains(history[0].Summary, "127.0.0.1") {
		t.Fatal("target persisted")
	}
}

func TestConcurrentDiagnosticsBoundAndShutdown(t *testing.T) {
	started := make(chan struct{}, 8)
	stopped := make(chan struct{}, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
		stopped <- struct{}{}
	}))
	defer server.Close()
	store, err := storage.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := New(store, network.Diagnostics{})
	defer func() {
		if err := service.Close(); err != nil {
			t.Error(err)
		}
	}()
	for range 8 {
		if _, err := service.Start(t.Context(), network.Request{Kind: "http", Target: server.URL, TimeoutMS: 30000}); err != nil {
			t.Fatal(err)
		}
	}
	for range 8 {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("diagnostic did not reach fixture")
		}
	}
	if _, err := service.Start(t.Context(), network.Request{Kind: "http", Target: server.URL, TimeoutMS: 30000}); err == nil {
		t.Fatal("unbounded concurrent diagnostics")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	for range 8 {
		select {
		case <-stopped:
		case <-time.After(2 * time.Second):
			t.Fatal("shutdown left a request alive")
		}
	}
	if _, err := service.Start(t.Context(), network.Request{Kind: "tcp", Target: "localhost", Port: 80, TimeoutMS: 100}); err == nil {
		t.Fatal("closed service accepted a job")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(store, network.Diagnostics{})
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal("second close:", err)
	}
}

func TestTerminalEventSurvivesFullEventBuffer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	store, err := storage.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(store, network.Diagnostics{})
	defer func() { _ = s.Close() }()
	for len(s.Events) < cap(s.Events) {
		s.Events <- Event{}
	}
	j, err := s.Start(t.Context(), network.Request{Kind: "tcp", Target: "127.0.0.1", Port: port, TimeoutMS: 500})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case e := <-s.Events:
			if e.Job.ID == j.ID && e.Job.Finished != nil {
				if e.Job.State != "failed" {
					t.Fatal("unexpected terminal state", e.Job.State)
				}
				return
			}
		case <-deadline:
			t.Fatal("terminal event was dropped")
		}
	}
}
