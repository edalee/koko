package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// MCP subcommand — runs as stdio MCP server, no GUI
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCPServer()
		return
	}

	// Log to file so we can debug Finder launches
	configDir, _ := os.UserConfigDir()
	logPath := filepath.Join(configDir, "koko", "koko.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o700)
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err == nil {
		log.SetOutput(f)
	}

	tm := NewTerminalManager()
	gh := NewGitHubService()
	git := NewGitService()
	cfg := NewConfigService()
	pm := NewProcessMonitor()
	claude := NewClaudeService()
	api := NewAPIServer(tm, git, cfg)
	app := NewApp(tm, cfg, api)
	slackCmd := NewSlackCommandHandler(cfg, tm, git, api)

	// Start Slack bot command listener if configured
	go slackCmd.Start()

	// macOS Edit menu enables Cmd+C/V/X/A in the webview
	editMenu := menu.NewMenu()
	editMenu.Append(menu.EditMenu())
	appMenu := menu.NewMenu()
	appMenu.Append(menu.AppMenu())
	appMenu.Merge(editMenu)

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
		Menu:             appMenu,
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
			pm,
			claude,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
