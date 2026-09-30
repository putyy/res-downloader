package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"res-downloader/internal/httpapi"
	shared "res-downloader/internal/model"
	"res-downloader/internal/server"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxFrontendErrorRunes = 16 * 1024

type Bind struct {
	runtime *Runtime
}

func NewBind(appRuntime *Runtime) *Bind {
	return &Bind{runtime: appRuntime}
}

func (b *Bind) Config() *httpapi.ResponseData {
	return httpapi.NewResponse(1, "ok", b.runtime.Config.Snapshot())
}

func (b *Bind) AppInfo() *httpapi.ResponseData {
	ctx, cancel := context.WithTimeout(context.Background(), startupWaitTimeout)
	defer cancel()
	if err := b.runtime.awaitStartup(ctx); err != nil {
		return httpapi.NewResponse(0, "Application services are still starting; retry the startup check", nil)
	}
	return httpapi.NewResponse(1, "ok", b.runtime.App)
}

func (b *Bind) APISession() *httpapi.ResponseData {
	// Bind calls can arrive while backend startup is still running. Do not let the
	// frontend probe a port that this instance failed to acquire.
	ctx, cancel := context.WithTimeout(context.Background(), startupWaitTimeout)
	defer cancel()
	if err := b.runtime.awaitStartup(ctx); err != nil {
		return httpapi.NewResponse(0, "Application services are still starting; retry the startup check", nil)
	}
	if b.runtime.startupErr != nil {
		return httpapi.NewResponse(0, b.runtime.startupErr.Error(), map[string]bool{
			"restartRequired": true,
			"portUnavailable": server.IsPortUnavailable(b.runtime.startupErr),
		})
	}
	return httpapi.NewResponse(1, "ok", map[string]string{"token": b.runtime.HTTP.SessionToken()})
}

func (b *Bind) OpenLogDirectory() error {
	logDirectory := filepath.Join(b.runtime.App.UserDir, "logs")
	if err := os.MkdirAll(logDirectory, 0750); err != nil {
		return err
	}
	return shared.OpenDirectory(logDirectory)
}

func (b *Bind) RestartWithNewPort() error {
	if err := b.runtime.App.preparePortRestart(); err != nil {
		return err
	}
	runtime.Quit(b.runtime.App.ctx)
	return nil
}

func (b *Bind) LogFrontendError(message string) {
	message = strings.TrimSpace(message)
	if message == "" || b == nil || b.runtime == nil || b.runtime.Logger == nil {
		return
	}
	runes := []rune(message)
	if len(runes) > maxFrontendErrorRunes {
		message = string(runes[:maxFrontendErrorRunes]) + "…"
	}
	b.runtime.Logger.Error().Str("source", "frontend").Msg(message)
}

func (b *Bind) PrepareReset(password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), startupWaitTimeout)
	defer cancel()
	if err := b.runtime.awaitStartup(ctx); err != nil {
		return err
	}
	return b.runtime.App.PrepareReset(password)
}

func (b *Bind) ResetApp() {
	a := b.runtime.App
	a.exitMu.Lock()
	reset := a.IsReset && !a.closing
	a.exitMu.Unlock()
	if !reset {
		return
	}
	runtime.Quit(b.runtime.App.ctx)
}
