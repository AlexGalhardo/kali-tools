package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const maxCaptureUpload = app.MaxCaptureFileSize
const uploadReadTimeout = 120 * time.Second
const maxFormatBytes = 16

func (s *Server) acquireTransfer(w http.ResponseWriter, r *http.Request) bool {
	select {
	case s.transfers <- struct{}{}:
		return true
	default:
		writeError(w, r, requestError(http.StatusTooManyRequests, domain.CodeConflict, "too many concurrent requests"))
		return false
	}
}

func (s *Server) handleImport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.acquireTransfer(w, r) {
			return
		}
		defer func() { <-s.transfers }()
		if !strings.EqualFold(r.Header.Get("Content-Type"), "application/octet-stream") {
			writeError(w, r, requestError(http.StatusUnsupportedMediaType, domain.CodeInvalidInput, "expected application/octet-stream"))
			return
		}
		rc := http.NewResponseController(w)
		if err := rc.SetReadDeadline(time.Now().Add(uploadReadTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			writeError(w, r, err)
			return
		}
		defer func() { _ = rc.SetReadDeadline(time.Time{}) }()
		file, err := os.CreateTemp("", "kali-tools-upload-*")
		if err != nil {
			writeError(w, r, err)
			return
		}
		path := file.Name()
		defer func() { _ = os.Remove(path) }()
		_, copyErr := io.Copy(file, http.MaxBytesReader(w, r.Body, maxCaptureUpload))
		closeErr := file.Close()
		if copyErr != nil {
			writeError(w, r, bodyError(copyErr))
			return
		}
		if closeErr != nil {
			writeError(w, r, closeErr)
			return
		}
		value, err := s.app.ImportCapture(r.Context(), path)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, contract.FromImportResult(value))
	}
}

func (s *Server) handleExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.acquireTransfer(w, r) {
			return
		}
		defer func() { <-s.transfers }()
		format := r.URL.Query().Get("format")
		if len(format) > maxFormatBytes {
			writeError(w, r, domain.Invalid("invalid capture format"))
			return
		}
		if err := s.app.CanExportCapture(r.Context(), format); err != nil {
			writeError(w, r, err)
			return
		}
		dir, err := os.MkdirTemp("", "kali-tools-export-*")
		if err != nil {
			writeError(w, r, err)
			return
		}
		defer func() { _ = os.RemoveAll(dir) }()
		path := filepath.Join(dir, "capture."+format)
		ctx, cancel := context.WithTimeout(r.Context(), app.CaptureExportTimeout)
		defer cancel()
		if err := s.app.ExportCapture(ctx, path, format); err != nil {
			writeError(w, r, err)
			return
		}
		file, err := os.Open(path)
		if err != nil {
			writeError(w, r, err)
			return
		}
		defer func() { _ = file.Close() }()
		media := "application/vnd.tcpdump.pcap"
		if format == "pcapng" {
			media = "application/x-pcapng"
		}
		w.Header().Set("Content-Type", media)
		w.Header().Set("Content-Disposition", `attachment; filename="capture.`+format+`"`)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.Copy(w, file)
	}
}

func (s *Server) handleBundle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.acquireTransfer(w, r) {
			return
		}
		defer func() { <-s.transfers }()
		body, err := s.app.DiagnosticBundle(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="kali-tools-diagnostic.zip"`)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	}
}
