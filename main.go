//go:build desktop || bindings

package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	if e := wails.Run(&options.App{Title: "kali-tools", Width: 1320, Height: 900, MinWidth: 960, MinHeight: 680, AssetServer: &assetserver.Options{Assets: assets}, OnStartup: app.startup, OnShutdown: app.shutdown, Bind: []interface{}{app}, BackgroundColour: &options.RGBA{R: 12, G: 17, B: 24, A: 255}}); e != nil {
		log.Fatal(e)
	}
}
