//go:build windows && desktop && nativesmoke

package main

import (
	"context"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"testing"
	"time"
)

type SmokeBridge struct {
	ctx    context.Context
	result chan bool
}

func (s *SmokeBridge) Complete(ok bool) {
	select {
	case s.result <- ok:
	default:
	}
	wruntime.Quit(s.ctx)
}
func TestNativeBridge(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app := NewApp()
	smoke := &SmokeBridge{result: make(chan bool, 1)}
	var timer *time.Timer
	e := wails.Run(&options.App{Title: "Hacker Tools bridge verification", Width: 960, Height: 680, StartHidden: true, AssetServer: &assetserver.Options{Assets: assets}, Bind: []interface{}{app, smoke}, OnStartup: func(ctx context.Context) {
		smoke.ctx = ctx
		app.startup(ctx)
		timer = time.AfterFunc(20*time.Second, func() { smoke.Complete(false) })
	}, OnDomReady: func(ctx context.Context) {
		wruntime.WindowExecJS(ctx, `Promise.all([window.go.main.App.Interfaces(),window.go.main.App.Settings(),window.go.main.App.Version()]).then(([interfaces,settings,version])=>window.go.main.SmokeBridge.Complete(interfaces.length>0&&settings.language==='en'&&version==='0.1.0-alpha.1-dev')).catch(()=>window.go.main.SmokeBridge.Complete(false));`)
	}, OnShutdown: app.shutdown})
	if timer != nil {
		timer.Stop()
	}
	if e != nil {
		t.Fatal(e)
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
