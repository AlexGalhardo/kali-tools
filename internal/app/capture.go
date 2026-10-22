package app

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

const CaptureExportTimeout = 30 * time.Second

func (a *Application) CaptureBackendStatus(ctx context.Context) []domain.Capability {
	result := make([]domain.Capability, 0, 3)
	for _, entry := range []struct{ backend, id string }{{capture.BackendRawSocket, "capture_live"}, {capture.BackendPktmon, "capture_pktmon"}, {capture.BackendNpcap, "capture_npcap"}} {
		c := domain.Capability{ID: entry.id, Provider: entry.backend, State: domain.CapabilityUnsupported, Detail: "live capture provider is not configured"}
		if provider := a.deps.CaptureBackends[entry.backend]; provider != nil {
			c = provider.Status(ctx)
			c.ID = entry.id
		}
		result = append(result, c)
	}
	return result
}

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
	backend := request.Backend
	if backend == "" {
		backend = capture.BackendRawSocket
	}
	provider := a.deps.CaptureBackends[backend]
	if provider == nil {
		return domain.Unsupported("live capture provider is not configured")
	}
	if status := provider.Status(ctx); status.State != domain.CapabilityAvailable {
		return &domain.Error{Code: status.State, Message: status.Detail}
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
		var result capture.LiveResult
		var artifact *capture.Artifact
		err := a.runOperation(ctx, "capture "+backend, func(jobCtx context.Context) (string, error) {
			var err error
			result, artifact, err = provider.Capture(jobCtx, request)
			dropped := "unknown"
			if result.Dropped != nil {
				dropped = fmt.Sprint(*result.Dropped)
			}
			return fmt.Sprintf("%d packets, %s dropped", len(result.Packets), dropped), err
		})
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
