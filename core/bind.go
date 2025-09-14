package core

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Bind struct {
}

func NewBind() *Bind {
	return &Bind{}
}

func (b *Bind) Config() *ResponseData {
	return httpServerOnce.buildResp(1, "ok", globalConfig)
}

func (b *Bind) AppInfo() *ResponseData {
	return httpServerOnce.buildResp(1, "ok", appOnce)
}

func (b *Bind) ResetApp() {
	appOnce.IsReset = true
	runtime.Quit(appOnce.ctx)
}
