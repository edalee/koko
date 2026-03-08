package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	tm := NewTerminalManager()
	app := NewApp(tm)
	gh := NewGitHubService()
	git := NewGitService()
	cfg := NewConfigService()
	slack := NewSlackService(cfg)

	err := wails.Run(&options.App{
		Title:     "Koko",
		Width:     1280,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 17, B: 23, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Frameless:        false,
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
		},
		Bind: []interface{}{
			app,
			tm,
			gh,
			git,
			cfg,
			slack,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
