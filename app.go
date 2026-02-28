package main

import "context"

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
