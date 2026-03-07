package main

import (
	"context"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const version = "0.1.0"

type App struct {
	ctx context.Context
	tm  *TerminalManager
}

func NewApp(tm *TerminalManager) *App {
	return &App{tm: tm}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.tm.setContext(ctx)
}

func (a *App) shutdown(ctx context.Context) {}

func (a *App) GetVersion() string {
	return version
}

func (a *App) PickDirectory() (string, error) {
	return wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Choose Project Directory",
	})
}
