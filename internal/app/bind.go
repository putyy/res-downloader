package app

import (
	"os"
	"path/filepath"
	"strings"

	"res-downloader/internal/httpapi"
	shared "res-downloader/internal/model"

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
	return httpapi.NewResponse(1, "ok", b.runtime.App)
}

func (b *Bind) APISession() *httpapi.ResponseData {
	return httpapi.NewResponse(1, "ok", map[string]string{"token": b.runtime.HTTP.SessionToken()})
}

func (b *Bind) OpenLogDirectory() error {
	logDirectory := filepath.Join(b.runtime.App.UserDir, "logs")
	if err := os.MkdirAll(logDirectory, 0750); err != nil {
		return err
	}
	return shared.OpenDirectory(logDirectory)
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
	return b.runtime.App.PrepareReset(password)
}

func (b *Bind) ResetApp() {
	if !b.runtime.App.IsReset {
		return
	}
	runtime.Quit(b.runtime.App.ctx)
}
