//go:build desktop || bindings

package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
)

//go:embed all:frontend/dist
var assets embed.FS

func NewApp() *App {
	return &App{boot: func(ctx context.Context) (*app.Application, func() error, error) {
		return bootstrap.New(ctx, bootstrap.Config{})
	}}
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

// run is the desktop composition-root seam; H01 does not read ctx, args or getenv.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	app := NewApp()
	if e := wails.Run(&options.App{Title: "kali-tools", Width: 1320, Height: 900, MinWidth: 960, MinHeight: 680, AssetServer: &assetserver.Options{Assets: assets}, OnStartup: app.startup, OnShutdown: app.shutdown, Bind: []interface{}{app}, BackgroundColour: &options.RGBA{R: 12, G: 17, B: 24, A: 255}}); e != nil {
		_, _ = fmt.Fprintln(stderr, e)
		return 1
	}
	return 0
}
