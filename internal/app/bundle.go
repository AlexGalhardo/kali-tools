package app

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/bundle"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const (
	DoctorCaptureNote = "Raw IPv4 and Pktmon require administrator authorization. Optional Npcap is user-installed from https://npcap.com/#download and requires a pcap-tagged build; it is never bundled or installed. See capture backend capabilities."
	DoctorWiFiNote    = "Windows WLAN API; key reveal requires wifi reveal --confirm or the desktop reveal action"
)

// DiagnosticBundle returns a redacted zip. It does not read Wi-Fi keys, scans or capture packets, and it does not upload anything.
func (a *Application) DiagnosticBundle(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	settings, err := a.Settings(ctx)
	if err != nil {
		return nil, err
	}
	history, err := a.History(ctx)
	if err != nil {
		return nil, err
	}
	listed, _ := a.Tools(ctx)
	hostname, _ := os.Hostname()
	caps := a.Capabilities(ctx)
	in := bundle.Input{
		AppVersion: domain.Version, GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH,
		Hostname: hostname, CaptureNote: DoctorCaptureNote, WiFiNote: DoctorWiFiNote,
		Settings:     bundle.Settings{Language: settings.Language, RetentionDays: settings.RetentionDays},
		Capabilities: make([]bundle.Capability, len(caps)),
		Tools:        make([]bundle.Tool, 0, len(listed)),
		Logs:         make([]bundle.Log, 0, min(len(history), bundle.MaxLogs)),
	}
	for i, item := range caps {
		in.Capabilities[i] = bundle.Capability{ID: item.ID, State: item.State, Detail: item.Detail, Provider: item.Provider}
	}
	for _, item := range listed {
		in.Tools = append(in.Tools, bundle.Tool{Name: item.Name, State: item.State, Version: item.Version, Path: item.Path})
	}
	if len(history) > bundle.MaxLogs {
		history = history[:bundle.MaxLogs]
	}
	for _, item := range history {
		in.Logs = append(in.Logs, bundle.Log{
			ID: string(item.ID), Kind: item.Kind, State: string(item.State), Summary: item.Summary,
			Created: item.Created.UTC().Format(time.RFC3339Nano),
		})
	}
	var buf bytes.Buffer
	if err := bundle.Write(&buf, in); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
