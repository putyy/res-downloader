package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) update(ctx context.Context, action string, direct bool) (interface{}, error) {
	r := a.runtime
	var err error
	switch action {
	case "check":
		if err := r.Updates.Check(ctx); err != nil {
			r.Logger.Esg(err, "check application update")
		}
		// Keep the structured failure status available to the UI. Detailed,
		// source-specific errors stay in the log for deployment diagnostics.
		return r.Updates.Status(r.Config.Snapshot().Locale), nil
	case "download":
		err = r.Updates.Download(direct)
	case "cancel":
		r.Updates.Cancel()
	case "install":
		a.exitMu.Lock()
		defer a.exitMu.Unlock()
		if a.closing || a.portRestart != nil || a.IsReset {
			return nil, errors.New("application is shutting down")
		}
		if !r.Downloads.ReserveUpdate() {
			return map[string]bool{"busy": true}, nil
		}

		err = r.Updates.Prepare()
		if err != nil {
			r.Downloads.ReleaseUpdate()
		}
		if err == nil {
			a.closing = true
			time.AfterFunc(300*time.Millisecond, a.quitForUpdate)
		}
	case "status":
	default:
		return nil, fmt.Errorf("unknown update action %q", action)
	}
	if err != nil {
		return nil, err
	}
	return r.Updates.Status(r.Config.Snapshot().Locale), nil
}

func (a *App) quitForUpdate() {
	if runtime.GOOS == "windows" {
		// Windows native-window teardown can stall before OnShutdown runs.
		// Release application resources while the UI message loop is still alive.
		a.SaveWindowSize(a.ctx)
		a.runtime.Logger.Info().Msg("preparing Windows update shutdown; process will exit after cleanup")
		if err := a.shutdown(); err != nil {
			// Release withheld the helper gate. Never force an update through a
			// failed cleanup; the failure is saved for the next application launch.
			fmt.Println("update shutdown:", err)
		} else {
			// Arm this only after cleanup and the helper handoff succeeded. The
			// helper still waits for this PID to exit before replacing any files.
			time.AfterFunc(5*time.Second, func() { os.Exit(0) })
		}
	}
	wailsruntime.Quit(a.ctx)
}
