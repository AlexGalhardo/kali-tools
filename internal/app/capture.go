package app

import (
	"context"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

const CaptureExportTimeout = 30 * time.Second

type CaptureEvent struct {
	Result capture.LiveResult `json:"result"`
	Error  string             `json:"error"`
	Err    error              `json:"-"`
}

func (s *Service) StartCapture(request capture.LiveRequest) error {
	if err := capture.ValidateLiveRequest(request); err != nil {
		return err
	}
	if s.deps.Capture == nil {
		return domain.Unsupported("live capture provider is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return domain.Invalid("application is shutting down")
	}
	s.capMu.Lock()
	defer s.capMu.Unlock()
	if s.capCancel != nil {
		return domain.Invalid("capture already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.capCancel = cancel
	s.capArtifact = nil
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		defer cancel()
		result, artifact, err := s.deps.Capture.Capture(ctx, request)
		s.capMu.Lock()
		s.capArtifact = artifact
		s.capCancel = nil
		s.capMu.Unlock()
		message := ""
		if err != nil {
			message = err.Error()
		}
		s.Jobs.Publish(job.TopicCapture, CaptureEvent{Result: result, Error: message, Err: err})
	}()
	return nil
}

func (s *Service) StopCapture() {
	s.capMu.Lock()
	defer s.capMu.Unlock()
	if s.capCancel != nil {
		s.capCancel()
	}
}

func (s *Service) CanImportCapture() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return domain.Invalid("application is shutting down")
	}
	s.capMu.Lock()
	defer s.capMu.Unlock()
	if s.capCancel != nil {
		return domain.Invalid("stop capture before importing a file")
	}
	return nil
}

func (s *Service) ImportCapture(path string) (capture.ImportResult, error) {
	if err := s.CanImportCapture(); err != nil {
		return capture.ImportResult{}, err
	}
	result, artifact, err := capture.ReadArtifactFile(path)
	if err != nil {
		return result, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return capture.ImportResult{}, domain.Invalid("application is shutting down")
	}
	s.capMu.Lock()
	defer s.capMu.Unlock()
	if s.capCancel != nil {
		return capture.ImportResult{}, domain.Invalid("capture started while importing")
	}
	s.capArtifact = artifact
	return result, nil
}

func (s *Service) exportArtifact(format string) (*capture.Artifact, error) {
	if format != "pcap" && format != "pcapng" {
		return nil, domain.Invalid("capture format must be pcap or pcapng")
	}
	s.capMu.Lock()
	defer s.capMu.Unlock()
	if s.capCancel != nil || s.capArtifact == nil {
		return nil, domain.Invalid("import or finish a capture before saving")
	}
	return s.capArtifact, nil
}

func (s *Service) CanExportCapture(format string) error {
	_, err := s.exportArtifact(format)
	return err
}

func (s *Service) ExportCapture(ctx context.Context, path, format string) error {
	artifact, err := s.exportArtifact(format)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, CaptureExportTimeout)
	defer cancel()
	return artifact.Save(ctx, path, format)
}
