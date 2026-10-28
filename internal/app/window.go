package app

import (
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
}

// A launch received during startup needs no extra action: the initial ready
// event will show the window. Never bring back a window already shutting down.
func (a *App) ActivateWindow() {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
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
	a.windowMu.Lock()
	if a.windowClosed || a.windowShown {
		a.windowMu.Unlock()
		return
	}
	a.runtime.Logger.Error().Msg("frontend did not prepare a startup view before the startup deadline")
	a.windowMu.Unlock()

	a.errorDialog("The app could not start. Please reopen the app.")

	a.windowMu.Lock()
	if a.windowClosed || a.windowShown {
		a.windowMu.Unlock()
		return
	}
	a.windowClosed = true
	a.windowMu.Unlock()
	a.quit()
}

// RevealStartupWindow shows the native window once the frontend has rendered
// its first view.
func (a *App) RevealStartupWindow() {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	if a.window == nil || a.windowClosed || a.windowShown {
		return
	}
	a.windowShown = true
	if a.windowShowTimer != nil {
		a.windowShowTimer.Stop()
	}
	a.window.Show()
}

// SaveWindowSize reads native window dimensions, which use the same units as
// the startup options regardless of webview zoom or display scaling.
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
	if err := a.runtime.Config.SaveWindowSize(width, height); err != nil {
		a.runtime.Logger.Esg(err, "save window size")
	}
}
