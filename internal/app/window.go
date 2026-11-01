package app

import (
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AttachDesktop connects the business runtime to the single native window.
func (a *App) AttachDesktop(desktop *application.App, window *application.WebviewWindow) {
	a.windowMu.Lock()
	a.desktop = desktop
	a.window = window
	a.windowMu.Unlock()
	a.events.SetApplication(desktop)
	desktop.Event.On("window:ready", func(*application.CustomEvent) {
		a.showStartupWindow()
	})
}

// Keep the native window hidden until Vue has prepared its first view.
func (a *App) PrepareStartupWindow() {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	if a.windowClosed || a.windowShown || a.windowShowTimer != nil {
		return
	}
	a.windowShowTimer = time.AfterFunc(10*time.Second, a.reportStartupWindowTimeout)
}

// A launch received during startup needs no extra action: the initial ready
// event will show the window. Never bring back a window already shutting down.
func (a *App) ActivateWindow() {
	a.windowMu.Lock()
	if a.windowClosed || !a.windowShown || a.window == nil {
		a.windowMu.Unlock()
		return
	}
	window := a.window
	a.windowMu.Unlock()
	if window.IsMinimised() {
		window.Restore()
	}
	window.Show()
}

func (a *App) reportStartupWindowTimeout() {
	a.windowMu.Lock()
	if a.windowClosed || a.windowShown {
		a.windowMu.Unlock()
		return
	}
	a.runtime.Logger.Error().Msg("frontend did not prepare a startup view before the startup deadline")
	desktop := a.desktop
	a.windowMu.Unlock()

	if desktop != nil {
		desktop.Dialog.Error().SetTitle("Startup failed").SetMessage("The app could not start. Please reopen the app.").Show()
	}

	a.windowMu.Lock()
	if a.windowClosed || a.windowShown {
		a.windowMu.Unlock()
		return
	}
	a.windowClosed = true
	a.windowMu.Unlock()
	if desktop != nil {
		desktop.Quit()
	}
}

func (a *App) showStartupWindow() {
	a.windowMu.Lock()
	if a.windowClosed || a.windowShown {
		a.windowMu.Unlock()
		return
	}
	a.windowShown = true
	if a.windowShowTimer != nil {
		a.windowShowTimer.Stop()
	}
	window := a.window
	a.windowMu.Unlock()
	if window != nil {
		window.Show()
	}
}

// ScheduleWindowSizeSave keeps native window queries and config writes out of
// continuous resize events. Only save after resizing has settled for 300ms.
func (a *App) ScheduleWindowSizeSave() {
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	if a.windowClosed || a.window == nil {
		return
	}
	a.cancelWindowSizeSaveLocked()
	generation := a.windowSizeGeneration
	a.windowSizeTimer = time.AfterFunc(300*time.Millisecond, func() {
		a.saveWindowSize(generation)
	})
}

// Caller must hold windowMu. The generation also invalidates timer callbacks
// that have already started and cannot be cancelled by Timer.Stop.
func (a *App) cancelWindowSizeSaveLocked() {
	a.windowSizeGeneration++
	if a.windowSizeTimer != nil {
		a.windowSizeTimer.Stop()
		a.windowSizeTimer = nil
	}
}

// SaveWindowSize cancels a pending save and immediately saves the final size
// before closing or handing off an update.
func (a *App) SaveWindowSize() {
	a.windowMu.Lock()
	a.cancelWindowSizeSaveLocked()
	generation := a.windowSizeGeneration
	a.windowMu.Unlock()
	a.saveWindowSize(generation)
}

// Native dimensions use the startup options' units regardless of webview zoom
// or display scaling. Do not hold windowMu while querying the native UI thread.
func (a *App) saveWindowSize(generation uint64) {
	a.windowMu.Lock()
	if a.windowClosed || a.window == nil || generation != a.windowSizeGeneration {
		a.windowMu.Unlock()
		return
	}
	window := a.window
	a.windowMu.Unlock()
	if window.IsMinimised() || window.IsMaximised() || window.IsFullscreen() {
		return
	}
	width, height := window.Size()
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	if a.windowClosed || generation != a.windowSizeGeneration {
		return
	}
	if err := a.runtime.Config.SaveWindowSize(width, height); err != nil {
		a.runtime.Logger.Esg(err, "save window size")
	}
}
