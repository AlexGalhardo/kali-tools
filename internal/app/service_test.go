package app

import (
	"context"
	"github.com/AlexGalhardo/hacker-tools/internal/network"
	"github.com/AlexGalhardo/hacker-tools/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCancellationAndRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	store, e := storage.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	s := New(store)
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
