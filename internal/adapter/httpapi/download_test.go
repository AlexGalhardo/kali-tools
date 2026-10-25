package httpapi

import (
	"net/http/httptest"
	"testing"
	"time"
)

type writeDeadlineWriter struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *writeDeadlineWriter) SetWriteDeadline(t time.Time) error {
	w.deadlines = append(w.deadlines, t)
	return nil
}

func TestDownloadsBoundSlowClientsAndReset(t *testing.T) {
	s := &Server{app: testApp(t), transfers: make(chan struct{}, maxFileTransfers)}
	w := &writeDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	before := time.Now()
	s.handleBundle()(w, httptest.NewRequest("GET", "/api/diagnostics/bundle", nil))
	after := time.Now()
	if w.Code != 200 || len(w.deadlines) != 2 || !w.deadlines[1].IsZero() {
		t.Fatalf("code=%d deadlines=%v", w.Code, w.deadlines)
	}
	if w.deadlines[0].Before(before.Add(downloadWriteTimeout)) || w.deadlines[0].After(after.Add(downloadWriteTimeout)) {
		t.Fatalf("deadline outside bound: %v", w.deadlines[0])
	}
	if len(s.transfers) != 0 {
		t.Fatal("transfer slot leaked")
	}
}
