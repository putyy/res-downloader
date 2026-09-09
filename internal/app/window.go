package app

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SaveWindowSize reads native window dimensions, which use the same units as
// the startup options regardless of webview zoom or display scaling.
func (a *App) SaveWindowSize(ctx context.Context) {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	if a.windowClosed {
		return
	}
	if runtime.WindowIsMinimised(ctx) || runtime.WindowIsMaximised(ctx) || runtime.WindowIsFullscreen(ctx) {
		return
	}
	width, height := runtime.WindowGetSize(ctx)
	if err := a.runtime.Config.SaveWindowSize(width, height); err != nil {
		a.runtime.Logger.Esg(err, "save window size")
	}
}
