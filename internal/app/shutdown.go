package app

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// BeginShutdown runs before Wails closes the window, so slow startup can stop
// between stages instead of continuing to start listeners and downloads.
func (a *App) BeginShutdown() {
	a.exitMu.Lock()
	a.closing = true
	a.exitMu.Unlock()
	a.runtime.cancelStartup()

	// Drain any pending size save before configuration reset or logger shutdown.
	a.windowMu.Lock()
	a.windowClosed = true
	if a.windowShowTimer != nil {
		a.windowShowTimer.Stop()
	}
	a.windowMu.Unlock()
}

func (a *App) OnExit() {
	if err := a.shutdown(); err != nil {
		fmt.Println("application shutdown:", err)
	}
}

// Windows update shutdown, OnShutdown and the main fallback share one handoff.
// Runtime.Close only releases resources; follow-up actions belong to App.
func (a *App) shutdown() error {
	a.shutdownOnce.Do(func() {
		a.BeginShutdown()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.shutdownErr = a.finishShutdown(ctx)
		if logger := a.runtime.Logger; logger != nil {
			if a.shutdownErr != nil {
				logger.Esg(a.shutdownErr, "application shutdown failed")
			} else {
				logger.Info().Msg("application shutdown completed")
			}
			logger.Close()
		}
	})
	return a.shutdownErr
}

func (a *App) finishShutdown(ctx context.Context) error {
	r := a.runtime
	err := r.Close(ctx)
	a.exitMu.Lock()
	restart, reset := a.portRestart, a.IsReset
	a.portRestart = nil
	a.exitMu.Unlock()

	// Release a reserved port even when cleanup fails. Never launch a new
	// instance, erase state, or allow installation after failed cleanup.
	if restart != nil {
		if closeErr := restart.listener.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("release reserved listen port: %w", closeErr))
		}
	}
	if err == nil && reset {
		if r.Downloads != nil {
			if cleanupErr := r.Downloads.CleanupWorkspaces(); cleanupErr != nil {
				err = fmt.Errorf("cleanup download workspaces before reset: %w", cleanupErr)
			}
		}
		if err == nil {
			err = a.ResetApp()
		}
	} else if err == nil && restart != nil {
		if startErr := restart.command.Start(); startErr != nil {
			err = fmt.Errorf("restart with new listen port: %w", startErr)
		}
	}
	if r.Updates != nil {
		r.logShutdownStage("release update helper")
		if releaseErr := r.Updates.Release(err); releaseErr != nil {
			err = errors.Join(err, fmt.Errorf("release update helper: %w", releaseErr))
		}
	}
	return err
}
