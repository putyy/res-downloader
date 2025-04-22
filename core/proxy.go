package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/elazarl/goproxy"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Proxy struct {
	ctx   context.Context
	Proxy *goproxy.ProxyHttpServer
	Is    bool
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

func initProxy() *Proxy {
	if proxyOnce == nil {
		proxyOnce = &Proxy{}
		proxyOnce.Startup()
	}
	return proxyOnce
}

func (p *Proxy) Startup() {
	err := p.setCa()
	if err != nil {
		DialogErr("启动代理服务失败：" + err.Error())
		return
	}

	p.Proxy = goproxy.NewProxyHttpServer()
	//p.Proxy.KeepDestinationHeaders = true
	//p.Proxy.Verbose = false
	p.setTransport()
	p.Proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)
	p.Proxy.OnRequest().DoFunc(p.httpRequestEvent)
	p.Proxy.OnResponse().DoFunc(p.httpResponseEvent)
}

func (p *Proxy) setCa() error {
	ca, err := tls.X509KeyPair(appOnce.PublicCrt, appOnce.PrivateKey)
	if err != nil {
		DialogErr("启动代理服务失败1")
		return err
	}
	if ca.Leaf, err = x509.ParseCertificate(ca.Certificate[0]); err != nil {
		return err
	}
	goproxy.GoproxyCa = ca
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: goproxy.TLSConfigFromCA(&ca)}
	return nil
}

func (p *Proxy) setTransport() {
	transport := &http.Transport{
		DisableKeepAlives: false,
		// MaxIdleConnsPerHost: 10,
		DialContext: (&net.Dialer{
			Timeout: 60 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   60 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}

	if globalConfig.UpstreamProxy != "" && globalConfig.OpenProxy && !strings.Contains(globalConfig.UpstreamProxy, globalConfig.Port) {
		proxyURL, err := url.Parse(globalConfig.UpstreamProxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	p.Proxy.Tr = transport
}

func (p *Proxy) httpRequestEvent(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	if strings.Contains(r.Host, "res-downloader.666666.com") && strings.Contains(r.URL.Path, "/wechat") {
		if globalConfig.WxAction && r.URL.Query().Get("type") == "1" {
			return p.handleWechatRequest(r, ctx)
		} else if !globalConfig.WxAction && r.URL.Query().Get("type") == "2" {
			return p.handleWechatRequest(r, ctx)
		} else {
			return r, p.buildEmptyResponse(r)
		}
	}
	return r, nil
}

func (p *Proxy) handleWechatRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
		return r, p.buildEmptyResponse(r)
	}

	isAll, _ := resourceOnce.getResType("all")
	isClassify, _ := resourceOnce.getResType("video")

	if !isAll && !isClassify {
		return r, p.buildEmptyResponse(r)
	}
	go func(body []byte) {
		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		if err != nil {
			return
		}
		media, ok := result["media"].([]interface{})
		if !ok || len(media) <= 0 {
			return
		}
		firstMedia, ok := media[0].(map[string]interface{})
		if !ok {
			return
		}
		rowUrl, ok := firstMedia["url"]
		if !ok {
			return
		}

		urlSign := Md5(rowUrl.(string))
		if resourceOnce.mediaIsMarked(urlSign) {
			return
		}

		id, err := gonanoid.New()
		if err != nil {
			id = urlSign
		}
		res := MediaInfo{
			Id:          id,
			Url:         rowUrl.(string),
			UrlSign:     urlSign,
			CoverUrl:    "",
			Size:        "0",
			Domain:      GetTopLevelDomain(rowUrl.(string)),
			Classify:    "video",
			Suffix:      ".mp4",
			Status:      DownloadStatusReady,
			SavePath:    "",
			DecodeKey:   "",
			OtherData:   map[string]string{},
			Description: "",
			ContentType: "video/mp4",
		}

		if mediaType, ok := firstMedia["mediaType"].(float64); ok && mediaType == 9 {
			res.Classify = "image"
			res.Suffix = ".png"
			res.ContentType = "image/png"
		}

		if urlToken, ok := firstMedia["urlToken"].(string); ok {
			res.Url = res.Url + urlToken
		}
		if fileSize, ok := firstMedia["fileSize"].(float64); ok {
			res.Size = FormatSize(fileSize)
		}
		if coverUrl, ok := firstMedia["coverUrl"].(string); ok {
			res.CoverUrl = coverUrl
		}
		if fileSize, ok := firstMedia["fileSize"].(string); ok {
			value, err := strconv.ParseFloat(fileSize, 64)
			if err == nil {
				res.Size = FormatSize(value)
			}
		}
		if decodeKey, ok := firstMedia["decodeKey"].(string); ok {
			res.DecodeKey = decodeKey
		}
		if desc, ok := result["description"].(string); ok {
			res.Description = desc
		}
		if spec, ok := firstMedia["spec"].([]interface{}); ok {
			var fileFormats []string
			for _, item := range spec {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if format, exists := itemMap["fileFormat"].(string); exists {
						fileFormats = append(fileFormats, format)
					}
				}
			}

			res.OtherData["wx_file_formats"] = strings.Join(fileFormats, "#")
		}
		resourceOnce.markMedia(urlSign)
		httpServerOnce.send("newResources", res)
	}(body)
	return r, p.buildEmptyResponse(r)
}

func (p *Proxy) buildEmptyResponse(r *http.Request) *http.Response {
	body := "内容不存在"
	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}
	resp.Header.Set("Content-Type", "text/plain")
	return resp
}

func (p *Proxy) httpResponseEvent(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp == nil || resp.Request == nil || (resp.StatusCode != 200 && resp.StatusCode != 206) {
		return resp
	}

	host := resp.Request.Host
	Path := resp.Request.URL.Path

	if strings.HasSuffix(host, "channels.weixin.qq.com") &&
		(strings.Contains(Path, "/web/pages/feed") || strings.Contains(Path, "/web/pages/home")) {
		return p.replaceWxJsContent(resp, ".js\"", ".js?v="+p.v()+"\"")
	}

	if strings.HasSuffix(host, "res.wx.qq.com") {
		respTemp := resp
		is := false
		if strings.HasSuffix(respTemp.Request.URL.RequestURI(), ".js?v="+p.v()) {
			respTemp = p.replaceWxJsContent(respTemp, ".js\"", ".js?v="+p.v()+"\"")
			is = true
		}

		if strings.Contains(Path, "web/web-finder/res/js/virtual_svg-icons-register.publish") {
			body, err := io.ReadAll(respTemp.Body)
			if err != nil {
				return respTemp
			}
			bodyStr := string(body)
			newBody := regexp.MustCompile(`get\s*media\(\)\{`).
				ReplaceAllString(bodyStr, `
							get media(){
								if(this.objectDesc){
									fetch("https://res-downloader.666666.com/wechat?type=1", {
									  method: "POST",
									  mode: "no-cors",
									  body: JSON.stringify(this.objectDesc),
									});
								};
			
			`)

			newBody = regexp.MustCompile(`async\s*finderGetCommentDetail\((\w+)\)\s*\{return(.*?)\s*}\s*async`).
				ReplaceAllString(newBody, `
							async finderGetCommentDetail($1) {
								var res = await$2;
								if (res?.data?.object?.objectDesc) {
									fetch("https://res-downloader.666666.com/wechat?type=2", {
									  method: "POST",
									  mode: "no-cors",
									  body: JSON.stringify(res.data.object.objectDesc),
									});
								}
								return res;
							}async
			`)
			newBodyBytes := []byte(newBody)
			respTemp.Body = io.NopCloser(bytes.NewBuffer(newBodyBytes))
			respTemp.ContentLength = int64(len(newBodyBytes))
			respTemp.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBodyBytes)))
			return respTemp
		}
		if is {
			return respTemp
		}
	}

	classify, suffix := TypeSuffix(resp.Header.Get("Content-Type"))
	if classify == "" {
		return resp
	}

	if classify == "video" && strings.HasSuffix(host, "finder.video.qq.com") {
		//if !globalConfig.WxAction && classify == "video" && strings.HasSuffix(host, "finder.video.qq.com") {
		return resp
	}

	rawUrl := resp.Request.URL.String()
	isAll, _ := resourceOnce.getResType("all")
	isClassify, _ := resourceOnce.getResType(classify)

	urlSign := Md5(rawUrl)
	if ok := resourceOnce.mediaIsMarked(urlSign); !ok && (isAll || isClassify) {
		value, _ := strconv.ParseFloat(resp.Header.Get("content-length"), 64)
		id, err := gonanoid.New()
		if err != nil {
			id = urlSign
		}
		res := MediaInfo{
			Id:          id,
			Url:         rawUrl,
			UrlSign:     urlSign,
			CoverUrl:    "",
			Size:        FormatSize(value),
			Domain:      GetTopLevelDomain(rawUrl),
			Classify:    classify,
			Suffix:      suffix,
			Status:      DownloadStatusReady,
			SavePath:    "",
			DecodeKey:   "",
			OtherData:   map[string]string{},
			Description: "",
			ContentType: resp.Header.Get("Content-Type"),
		}
		resourceOnce.markMedia(urlSign)
		httpServerOnce.send("newResources", res)
	}
	return resp
}

func (p *Proxy) replaceWxJsContent(resp *http.Response, old, new string) *http.Response {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp
	}
	bodyString := string(body)
	newBodyString := strings.ReplaceAll(bodyString, old, new)
	newBodyBytes := []byte(newBodyString)
	resp.Body = io.NopCloser(bytes.NewBuffer(newBodyBytes))
	resp.ContentLength = int64(len(newBodyBytes))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBodyBytes)))
	return resp
}

func (p *Proxy) v() string {
	return appOnce.Version
}
