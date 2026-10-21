package bootstrap

import (
	"github.com/AlexGalhardo/kali-tools/internal/adapter/platform"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/probe"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/storage"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/tools"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

var (
	_ app.JobStore        = (*storage.Store)(nil)
	_ app.SettingsStore   = (*storage.Store)(nil)
	_ app.HostNetwork     = platform.Host{}
	_ app.ToolRunner      = tools.Runner{}
	_ app.WiFiProvider    = wifi.Native{}
	_ app.CaptureProvider = capture.Native{}
	_ network.Prober      = probe.ICMP{}
)
