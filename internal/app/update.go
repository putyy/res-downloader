package app

import (
	"context"
	"errors"
	"fmt"
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
		r.restartMu.Lock()
		defer r.restartMu.Unlock()
		if r.closing || r.portRestart != nil || a.IsReset {
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
			r.closing = true
			time.AfterFunc(300*time.Millisecond, func() { wailsruntime.Quit(a.ctx) })
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
