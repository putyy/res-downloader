package shared

import (
	"github.com/elazarl/goproxy"
	"net/http"
)

type Bridge struct {
	GetVersion    func() string
	GetResType    func(key string) (bool, bool)
	TypeSuffix    func(mime string) (string, string)
	MediaIsMarked func(key string) bool
	MarkMedia     func(key string)
	GetConfig     func(key string) interface{}
	Send          func(t string, data interface{})
}

type MediaInfo struct {
	Id          string
	Url         string
	UrlSign     string
	CoverUrl    string
	Size        string
	Domain      string
	Classify    string
	Suffix      string
	SavePath    string
	Status      string
	DecodeKey   string
	Description string
	ContentType string
	OtherData   map[string]string
}

type Plugin interface {
	SetBridge(*Bridge)
	Domains() []string
	OnRequest(*http.Request, *goproxy.ProxyCtx) (*http.Request, *http.Response)
	OnResponse(*http.Response, *goproxy.ProxyCtx) *http.Response
}
