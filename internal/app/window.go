package app

import (
<<<<<<< Updated upstream
	"context"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Keep the native window hidden until the frontend has prepared its first view.
// The native timer also covers failures that prevent frontend JavaScript loading.
func (a *App) PrepareStartupWindow(ctx context.Context) {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	if a.ctx == nil {
		a.ctx = ctx
	}
	if a.windowClosed || a.windowShown || a.windowShowTimer != nil {
		return
	}
	runtime.EventsOn(ctx, "window:ready", func(...interface{}) {
		a.showStartupWindow(ctx)
	})
	a.windowShowTimer = time.AfterFunc(10*time.Second, func() {
		a.reportStartupWindowTimeout(ctx)
	})
=======
	"time"
)

const startupWindowTimeout = 10 * time.Second

// PrepareStartupWindow arms the fallback that reports a startup failure when the
// frontend never signals readiness. The native window stays hidden until
// RevealStartupWindow runs.
func (a *App) PrepareStartupWindow() {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	if a.window == nil || a.windowClosed || a.windowShown || a.windowShowTimer != nil {
		return
	}
	a.windowShowTimer = time.AfterFunc(startupWindowTimeout, a.reportStartupWindowTimeout)
>>>>>>> Stashed changes
}

// A launch received during startup needs no extra action: the initial ready
// event will show the window. Never bring back a window already shutting down.
func (a *App) ActivateWindow() {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
<<<<<<< Updated upstream
	if a.windowClosed || !a.windowShown || a.ctx == nil {
		return
	}
	if runtime.WindowIsMinimised(a.ctx) {
		runtime.WindowUnminimise(a.ctx)
	}
	runtime.Show(a.ctx)
	runtime.WindowShow(a.ctx)
}

func (a *App) reportStartupWindowTimeout(ctx context.Context) {
=======
	if a.window == nil || a.windowClosed || !a.windowShown {
		return
	}
	if a.window.IsMinimised() {
		a.window.UnMinimise()
	}
	a.window.Show()
	a.window.Focus()
}

func (a *App) reportStartupWindowTimeout() {
>>>>>>> Stashed changes
	a.windowMu.Lock()
	if a.windowClosed || a.windowShown {
		a.windowMu.Unlock()
		return
	}
	a.runtime.Logger.Error().Msg("frontend did not prepare a startup view before the startup deadline")
	a.windowMu.Unlock()

<<<<<<< Updated upstream
	_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type: runtime.ErrorDialog, Title: "Startup failed",
		Message: "The app could not start. Please reopen the app.",
		Buttons: []string{"OK"}, DefaultButton: "OK", CancelButton: "OK",
	})
=======
	a.errorDialog("The app could not start. Please reopen the app.")
>>>>>>> Stashed changes

	a.windowMu.Lock()
	if a.windowClosed || a.windowShown {
		a.windowMu.Unlock()
		return
	}
	a.windowClosed = true
	a.windowMu.Unlock()
<<<<<<< Updated upstream
	runtime.Quit(ctx)
}

func (a *App) showStartupWindow(ctx context.Context) {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	if a.windowClosed || a.windowShown {
=======
	a.quit()
}

// RevealStartupWindow shows the native window once the frontend has rendered
// its first view.
func (a *App) RevealStartupWindow() {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	if a.window == nil || a.windowClosed || a.windowShown {
>>>>>>> Stashed changes
		return
	}
	a.windowShown = true
	if a.windowShowTimer != nil {
		a.windowShowTimer.Stop()
	}
<<<<<<< Updated upstream
	runtime.WindowShow(ctx)
=======
	a.window.Show()
>>>>>>> Stashed changes
}

// SaveWindowSize reads native window dimensions, which use the same units as
// the startup options regardless of webview zoom or display scaling.
<<<<<<< Updated upstream
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
=======
func (a *App) SaveWindowSize() {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	if a.window == nil || a.windowClosed {
		return
	}
	if a.window.IsMinimised() || a.window.IsMaximised() || a.window.IsFullscreen() {
		return
	}
	width, height := a.window.Size()
>>>>>>> Stashed changes
	if err := a.runtime.Config.SaveWindowSize(width, height); err != nil {
		a.runtime.Logger.Esg(err, "save window size")
	}
}
