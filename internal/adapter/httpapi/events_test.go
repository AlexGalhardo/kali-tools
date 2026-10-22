package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

type eventWriter struct {
	*httptest.ResponseRecorder
	mu         sync.Mutex
	deadline   time.Time
	delay      time.Duration
	keepalives int
}

func (w *eventWriter) SetWriteDeadline(deadline time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deadline = deadline
	return nil
}

func (w *eventWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	if string(data) == ": keep-alive\n\n" {
		w.keepalives++
	}
	deadline := w.deadline
	w.mu.Unlock()
	if w.delay > 0 {
		time.Sleep(w.delay) // synthetic time inside synctest
		if !time.Now().Before(deadline) {
			return 0, context.DeadlineExceeded
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ResponseRecorder.Write(data)
}

func (w *eventWriter) snapshot() (time.Time, int, string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.deadline, w.keepalives, w.Body.String()
}

func TestSSEKeepAliveBoundary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &Server{app: testApp(t), subscribers: make(chan struct{}, maxSSESubscribers), keepAlive: sseKeepAlive}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		w := &eventWriter{ResponseRecorder: httptest.NewRecorder()}
		done := make(chan struct{})
		go func() {
			defer close(done)
			s.handleEvents()(w, httptest.NewRequestWithContext(ctx, "GET", "/api/events", nil))
		}()
		synctest.Wait()
		time.Sleep(15*time.Second - time.Nanosecond)
		synctest.Wait()
		if _, count, _ := w.snapshot(); count != 0 {
			t.Fatal("keepalive arrived before 15 seconds")
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if _, count, _ := w.snapshot(); count != 1 {
			t.Fatalf("at 15 seconds: keepalives=%d", count)
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if _, count, _ := w.snapshot(); count != 1 {
			t.Fatal("duplicate keepalive immediately after boundary")
		}
		cancel()
		<-done
		if len(s.subscribers) != 0 {
			t.Fatal("subscriber slot leaked")
		}
	})
}

func TestSSEWriteDeadlineBoundary(t *testing.T) {
	for _, delay := range []time.Duration{10*time.Second - time.Nanosecond, 10 * time.Second, 10*time.Second + time.Nanosecond} {
		t.Run(delay.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := &Server{app: testApp(t), subscribers: make(chan struct{}, maxSSESubscribers), keepAlive: sseKeepAlive}
				w := &eventWriter{ResponseRecorder: httptest.NewRecorder(), delay: delay}
				start := time.Now()
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				done := make(chan struct{})
				go func() {
					defer close(done)
					s.handleEvents()(w, httptest.NewRequestWithContext(ctx, "GET", "/api/events", nil))
				}()
				synctest.Wait()
				if deadline, _, _ := w.snapshot(); !deadline.Equal(start.Add(10 * time.Second)) {
					t.Fatalf("write deadline=%v, want start + 10 seconds", deadline)
				}
				time.Sleep(delay)
				synctest.Wait()
				_, _, body := w.snapshot()
				if delay < 10*time.Second {
					if !strings.Contains(body, "retry: 3000") {
						t.Fatal("write before deadline did not complete")
					}
					cancel()
				} else if body != "" {
					t.Fatal("expired write produced data")
				}
				<-done
				if len(s.subscribers) != 0 {
					t.Fatal("failed write leaked subscription")
				}
			})
		})
	}
}

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
	server := httptest.NewServer(testMux(t, a))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	stream, disconnect := context.WithCancel(ctx)
	defer disconnect()
	r, err := http.NewRequestWithContext(stream, "GET", server.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	if line, err := reader.ReadString('\n'); err != nil || line != "retry: 3000\n" {
		t.Fatalf("initial stream: %q %v", line, err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	defer unblock()
	events, _ := a.Subscribe(ctx)
	status, err := a.StartDiagnostic(ctx, network.Request{Kind: "http", Target: target.URL, TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	disconnect() // terminal event will be missed by this client
	_ = response.Body.Close()
	unblock()
	terminal := false
	for !terminal {
		select {
		case event := <-events:
			terminal = event.Job != nil && event.Job.ID == status.ID && event.Job.State.Terminal()
		case <-ctx.Done():
			t.Fatal("diagnostic did not finish while disconnected")
		}
	}
	r, err = http.NewRequestWithContext(ctx, "GET", server.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Last-Event-ID", "1")
	reconnected, err := server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer reconnected.Body.Close()
	reader = bufio.NewReader(reconnected.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "event: resync\n" {
			break
		}
	}
	r, err = http.NewRequestWithContext(ctx, "GET", server.URL+"/api/jobs/"+string(status.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Body.Close()
	var value contract.Job
	if err := json.NewDecoder(snapshot.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	if snapshot.StatusCode != 200 || value.ID != string(status.ID) || value.State != "completed" {
		t.Fatalf("terminal state not recovered over HTTP: %+v", value)
	}
}
