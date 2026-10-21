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

	"github.com/AlexGalhardo/kali-tools/internal/adapter/wailsui"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() { os.Exit(run(context.Background(), os.Args[1:], os.Getenv, os.Stdout, os.Stderr)) }

func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	a, cleanup, err := bootstrap.New(ctx, bootstrap.Config{})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() {
		if err := cleanup(); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
		}
	}()
	b := wailsui.New(a)
	if err := wails.Run(&options.App{Title: "kali-tools", Width: 1320, Height: 900, MinWidth: 960, MinHeight: 680, AssetServer: &assetserver.Options{Assets: assets}, OnStartup: b.Startup, OnShutdown: b.Shutdown, Bind: []any{b}, BackgroundColour: &options.RGBA{R: 12, G: 17, B: 24, A: 255}}); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
