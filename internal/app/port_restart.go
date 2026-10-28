package app

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"

	"res-downloader/internal/server"
	"res-downloader/internal/system"
)

type portRestart struct {
	listener net.Listener
	command  *exec.Cmd
}

func (a *App) preparePortRestart() error {
	a.exitMu.Lock()
	defer a.exitMu.Unlock()
	if a.closing || a.portRestart != nil || a.IsReset {
		return errors.New("application is already shutting down or restarting")
	}
	r := a.runtime
	select {
	case <-r.startupDone:
		if !server.IsPortUnavailable(r.startupErr) {
			return errors.New("changing the port is only available after a listen port failure")
		}
	default:
		return errors.New("application startup has not finished")
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate application for restart: %w", err)
	}
	if _, err := os.Stat(executable); err != nil {
		return fmt.Errorf("locate application for restart: %w", err)
	}
	previous := r.Config.Snapshot()
	listener, port, err := server.ReserveAlternativePort(previous.Host, previous.Port)
	if err != nil {
		return err
	}
	if err := r.Config.SavePortForRestart(port); err != nil {
		_ = listener.Close()
		return fmt.Errorf("save new listen port: %w", err)
	}
	a.portRestart = &portRestart{listener: listener, command: system.RelaunchCommand(executable)}
	r.Logger.Info().Str("previousPort", previous.Port).Str("port", port).Msg("restart requested with a new listen port")
	return nil
}
