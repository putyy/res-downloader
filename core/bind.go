package core

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
