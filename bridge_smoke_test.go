//go:build windows && desktop && production && nativesmoke

package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	syswindows "golang.org/x/sys/windows"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/wailsui"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
)

type SmokeBridge struct {
	ctx    context.Context
	result chan bool
	ready  chan struct{}
}

func (s *SmokeBridge) Complete(ok bool) {
	<-s.ready
	select {
	case s.result <- ok:
	default:
	}
	wruntime.Quit(s.ctx)
}
func TestNativeBridge(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := syswindows.CoInitializeEx(0, syswindows.COINIT_APARTMENTTHREADED); err != nil {
		t.Fatal(err)
	}
	defer syswindows.CoUninitialize()
	data := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(data, "appdata"))
	t.Cleanup(func() {
		// WebView2 can release its last file handles after the window has closed.
		deadline := time.Now().Add(3 * time.Second)
		for {
			err := os.RemoveAll(data)
			if err == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Errorf("native data cleanup: %v", err)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	application, cleanup, err := bootstrap.New(t.Context(), bootstrap.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	}()
	b := wailsui.New(application)
	smoke := &SmokeBridge{result: make(chan bool, 1), ready: make(chan struct{})}
	var timer *time.Timer
	e := wails.Run(&options.App{Title: "kali-tools bridge verification", Width: 960, Height: 680, StartHidden: true, Windows: &windows.Options{WebviewUserDataPath: filepath.Join(data, "webview")}, AssetServer: &assetserver.Options{Assets: assets}, Bind: []interface{}{b, smoke}, OnStartup: b.Startup, OnDomReady: func(ctx context.Context) {
		smoke.ctx = ctx
		timer = time.AfterFunc(20*time.Second, func() { smoke.Complete(false) })
		close(smoke.ready)
		wruntime.WindowExecJS(ctx, `Promise.all([window.go.wailsui.Bindings.Interfaces(),window.go.wailsui.Bindings.Settings(),window.go.wailsui.Bindings.Version(),window.go.wailsui.Bindings.CancelJob("missing"),window.go.wailsui.Bindings.SaveCapture("invalid").then(()=>false,()=>true),window.go.wailsui.Bindings.InspectServices("0.0.0.0/0",[80]).then(()=>false,()=>true)]).then(([interfaces,settings,version,cancelled,invalidSave,invalidScope])=>window.go.main.SmokeBridge.Complete(interfaces.length>0&&settings.language==='en'&&version==='0.1.0-alpha.2-dev'&&cancelled===false&&invalidSave===true&&invalidScope===true)).catch(()=>window.go.main.SmokeBridge.Complete(false));`)
	}, OnShutdown: b.Shutdown})
	if e != nil {
		t.Fatal(e)
	}
	<-smoke.ready
	if timer != nil {
		timer.Stop()
	}
	select {
	case ok := <-smoke.result:
		if !ok {
			t.Fatal("native bridge did not verify interface, settings and version calls")
		}
	default:
		t.Fatal("native WebView exited before verification")
	}
}
