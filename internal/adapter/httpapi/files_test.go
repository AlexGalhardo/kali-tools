package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
)

type zeros struct{}

func (zeros) Read(out []byte) (int, error) { clear(out); return len(out), nil }

func tempFiles(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, key := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(key, dir)
	}
	return dir
}

func requireEmptyTemp(t *testing.T, dir string) {
	t.Helper()
	// Windows can briefly retain a delete-pending directory entry while an
	// antivirus handle closes. Require eventual cleanup within a fixed bound.
	deadline := time.Now().Add(time.Second)
	for {
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("temporary files=%v err=%v", entries, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCaptureUploadLimit(t *testing.T) {
	dir := tempFiles(t)
	a := testApp(t)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	mux := testMux(t, a)
	for _, size := range []int64{maxCaptureUpload, maxCaptureUpload + 1} {
		r := httptest.NewRequest("POST", "/api/capture/import", io.LimitReader(zeros{}, size))
		r.Header.Set("Content-Type", "application/octet-stream")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		want := 400
		if size > maxCaptureUpload {
			want = 413
		}
		if w.Code != want {
			t.Fatalf("size=%d status=%d body=%s", size, w.Code, w.Body)
		}
		if size == maxCaptureUpload && !strings.Contains(w.Body.String(), "application is shutting down") {
			t.Fatal("upload did not reach application", w.Body)
		}
		requireEmptyTemp(t, dir)
	}
}

func fixtureCapture(t *testing.T) []byte {
	t.Helper()
	a := capture.NewArtifact(layers.LinkTypeRaw)
	packet := []byte{0x45, 0, 0, 20, 0, 0, 0, 0, 64, 1, 0, 0, 192, 0, 2, 1, 192, 0, 2, 2}
	if err := a.Add(packet, gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: len(packet), Length: len(packet)}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := a.Write(t.Context(), &out, "pcap"); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestCaptureExportDownload(t *testing.T) {
	dir := tempFiles(t)
	mux := testMux(t, testApp(t))
	r := httptest.NewRequest("POST", "/api/capture/import", bytes.NewReader(fixtureCapture(t)))
	r.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	requireEmptyTemp(t, dir)
	for _, format := range []string{"pcap", "pcapng"} {
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/capture/export?format="+format, nil))
		if w.Code != 200 || w.Header().Get("Content-Disposition") != `attachment; filename="capture.`+format+`"` || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w.Code, w.Header(), w.Body)
		}
		value, err := capture.Read(bytes.NewReader(w.Body.Bytes()))
		if err != nil || value.Format != format || len(value.Packets) != 1 {
			t.Fatal(value, err)
		}
		requireEmptyTemp(t, dir)
	}
	for _, format := range []string{"../invalid", strings.Repeat("x", maxFormatBytes), strings.Repeat("x", maxFormatBytes+1)} {
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/capture/export?format="+format, nil))
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body)
		}
		requireEmptyTemp(t, dir)
	}
}

func TestCaptureTransferCap(t *testing.T) {
	s := &Server{app: testApp(t), transfers: make(chan struct{}, maxFileTransfers)}
	s.transfers <- struct{}{}
	for _, handler := range []http.HandlerFunc{s.handleImport(), s.handleExport()} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest("POST", "/", nil))
		if w.Code != 429 {
			t.Fatal(w.Code, w.Body)
		}
	}
}

func TestCaptureUploadReadDeadlineAndReset(t *testing.T) {
	dir := tempFiles(t)
	s := &Server{app: testApp(t), transfers: make(chan struct{}, maxFileTransfers)}
	for _, body := range [][]byte{fixtureCapture(t), []byte("invalid synthetic capture")} {
		r := httptest.NewRequest("POST", "/api/capture/import", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/octet-stream")
		w := &readDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
		before := time.Now()
		s.handleImport()(w, r)
		after := time.Now()
		if len(w.deadlines) != 2 || !w.deadlines[1].IsZero() {
			t.Fatalf("upload read deadline not reset: %v", w.deadlines)
		}
		if w.deadlines[0].Before(before.Add(120*time.Second)) || w.deadlines[0].After(after.Add(120*time.Second)) {
			t.Fatalf("upload deadline outside 120-second bound: %v", w.deadlines[0])
		}
		if len(s.transfers) != 0 {
			t.Fatal("upload transfer slot leaked")
		}
		requireEmptyTemp(t, dir)
	}
}
