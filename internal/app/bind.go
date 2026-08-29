package app

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"res-downloader/internal/httpapi"
)

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

func (b *Bind) PrepareReset(password string) error {
	return b.runtime.App.PrepareReset(password)
}

func (b *Bind) ResetApp() {
	if !b.runtime.App.IsReset {
		return
	}
	runtime.Quit(b.runtime.App.ctx)
}
