package httpapi

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func TestSSEKeepAliveAndShutdown(t *testing.T) {
	s := &Server{app: testApp(t), subscribers: make(chan struct{}, maxSSESubscribers), keepAlive: 10 * time.Millisecond}
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { defer close(done); s.handleEvents()(w, r) }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	r, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == ": keep-alive\n" {
			break
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SSE did not stop")
	}
	if len(s.subscribers) != 0 {
		t.Fatal("SSE slot leaked")
	}
}

func TestSSEResyncRecoversTerminalState(t *testing.T) {
	a := testApp(t)
	s := &Server{app: a, subscribers: make(chan struct{}, maxSSESubscribers), keepAlive: sseKeepAlive}
	server := httptest.NewServer(s.handleEvents())
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	r, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Last-Event-ID", "1")
	response, err := server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "event: resync\n" {
			break
		}
	}
	status, err := a.StartDiagnostic(ctx, network.Request{Kind: "tcp", Target: "127.0.0.1", Port: 9, TimeoutMS: 100})
	if err != nil {
		t.Fatal(err)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, string(status.ID)) && strings.Contains(line, `"finished"`) {
			break
		}
	}
	if value, ok := a.JobSnapshot(ctx, status.ID); !ok || !value.State.Terminal() {
		t.Fatal("terminal state not recovered", value, ok)
	}
}
