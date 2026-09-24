package app

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"

	"res-downloader/internal/server"
)

type portRestart struct {
	listener net.Listener
	command  *exec.Cmd
}

func (r *Runtime) preparePortRestart() error {
	r.restartMu.Lock()
	defer r.restartMu.Unlock()
	if r.closing || r.portRestart != nil || r.App.IsReset {
		return errors.New("application is already shutting down or restarting")
	}
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
	r.portRestart = &portRestart{listener: listener, command: exec.Command(executable)}
	r.Logger.Info().Str("previousPort", previous.Port).Str("port", port).Msg("restart requested with a new listen port")
	return nil
}
