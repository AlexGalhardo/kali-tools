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

func (a *Application) StartCapture(ctx context.Context, request capture.LiveRequest) error {
	if err := capture.ValidateLiveRequest(request); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return domain.Invalid("application is shutting down")
	}
	if a.deps.Capture == nil {
		return domain.Unsupported("live capture provider is not configured")
	}
	a.capMu.Lock()
	defer a.capMu.Unlock()
	if a.capCancel != nil {
		return domain.Invalid("capture already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.capCancel = cancel
	a.capArtifact = nil
	a.bg.Add(1)
	go func() {
		defer a.bg.Done()
		defer cancel()
		result, artifact, err := a.deps.Capture.Capture(ctx, request)
		a.capMu.Lock()
		a.capArtifact = artifact
		a.capCancel = nil
		a.capMu.Unlock()
		message := ""
		if err != nil {
			message = err.Error()
		}
		a.jobs.Publish(job.TopicCapture, CaptureEvent{Result: result, Error: message, Err: err})
	}()
	return nil
}

func (a *Application) StopCapture(ctx context.Context) {
	a.capMu.Lock()
	defer a.capMu.Unlock()
	if a.capCancel != nil {
		a.capCancel()
	}
}

func (a *Application) CanImportCapture(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return domain.Invalid("application is shutting down")
	}
	a.capMu.Lock()
	defer a.capMu.Unlock()
	if a.capCancel != nil {
		return domain.Invalid("stop capture before importing a file")
	}
	return nil
}

func (a *Application) ImportCapture(ctx context.Context, path string) (capture.ImportResult, error) {
	if err := a.CanImportCapture(ctx); err != nil {
		return capture.ImportResult{}, err
	}
	result, artifact, err := capture.ReadArtifactFile(path)
	if err != nil {
		return result, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return capture.ImportResult{}, domain.Invalid("application is shutting down")
	}
	a.capMu.Lock()
	defer a.capMu.Unlock()
	if a.capCancel != nil {
		return capture.ImportResult{}, domain.Invalid("capture started while importing")
	}
	a.capArtifact = artifact
	return result, nil
}

func (a *Application) exportArtifact(format string) (*capture.Artifact, error) {
	if format != "pcap" && format != "pcapng" {
		return nil, domain.Invalid("capture format must be pcap or pcapng")
	}
	a.capMu.Lock()
	defer a.capMu.Unlock()
	if a.capCancel != nil || a.capArtifact == nil {
		return nil, domain.Invalid("import or finish a capture before saving")
	}
	return a.capArtifact, nil
}

func (a *Application) CanExportCapture(ctx context.Context, format string) error {
	_, err := a.exportArtifact(format)
	return err
}

func (a *Application) ExportCapture(ctx context.Context, path, format string) error {
	artifact, err := a.exportArtifact(format)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, CaptureExportTimeout)
	defer cancel()
	return artifact.Save(ctx, path, format)
}
