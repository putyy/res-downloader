package plugins

import (
	"encoding/json"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/elazarl/goproxy"
	gonanoid "github.com/matoous/go-nanoid/v2"

	"res-downloader/core/shared"
)

type DefaultPlugin struct {
	bridge *shared.Bridge
}

func (p *DefaultPlugin) SetBridge(bridge *shared.Bridge) {
	p.bridge = bridge
}

func (p *DefaultPlugin) Domains() []string {
	return []string{"default"}
}

func (p *DefaultPlugin) OnRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return r, nil
}

func (p *DefaultPlugin) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp == nil || resp.Request == nil || (resp.StatusCode != 200 && resp.StatusCode != 206) {
		return resp
	}

	rawUrl := resp.Request.URL.String()
	contentType := resp.Header.Get("Content-Type")
	classify, suffix := p.bridge.TypeSuffix(contentType)

	// Try to infer by URL first to avoid reading body unnecessarily
	if classify == "" {
		urlPath := resp.Request.URL.Path
		ext := strings.ToLower(path.Ext(urlPath))
		switch ext {
		case ".m3u8":
			classify = "stream"
			suffix = ".m3u8"
		case ".mpd":
			classify = "stream"
			suffix = ".mpd"
		case ".m4s", ".ts":
			// segment files
			if strings.Contains(contentType, "video") || strings.HasSuffix(contentType, "/mp4") {
				classify = "video"
			} else if strings.Contains(contentType, "audio") {
				classify = "audio"
			} else {
				classify = "video"
			}
			suffix = ext
		}
	}

	// If still unknown, do a lightweight body check for playlists only
	var body []byte
	if classify == "" || classify == "stream" {
		// Only read body when needed to detect playlist markers
		var err error
		body, err = shared.ReadResponseBody(resp)
		if err != nil {
			return resp
		}
		if classify == "" {
			s := string(body)
			if strings.HasPrefix(s, "#EXTM3U") {
				classify = "stream"
				suffix = ".m3u8"
			} else if strings.Contains(s, "<MPD") {
				classify = "stream"
				suffix = ".mpd"
			} else if strings.Contains(rawUrl, ".m4s") || strings.Contains(rawUrl, ".ts") {
				if len(body) < 1024 {
					return resp
				}
				if strings.Contains(contentType, "video") || strings.HasSuffix(contentType, "/mp4") {
					classify = "video"
				} else if strings.Contains(contentType, "audio") {
					classify = "audio"
				} else {
					classify = "video"
				}
				if strings.Contains(rawUrl, ".m4s") {
					suffix = ".m4s"
				} else {
					suffix = ".ts"
				}
			} else {
				// Not a known type
				return resp
			}
		}
	}

	isAll, _ := p.bridge.GetResType("all")
	isClassify, _ := p.bridge.GetResType(classify)

	urlSign := shared.Md5(rawUrl)
	if ok := p.bridge.MediaIsMarked(urlSign); !ok && (isAll || isClassify) {
		// Prefer total size from Content-Range when 206
		var value float64
		if total := parseTotalFromContentRange(resp.Header.Get("Content-Range")); total > 0 {
			value = float64(total)
		} else if cl, err := strconv.ParseFloat(resp.Header.Get("Content-Length"), 64); err == nil && cl > 0 {
			value = cl
		} else if len(body) > 0 {
			value = float64(len(body))
		}
		id, err := gonanoid.New()
		if err != nil {
			id = urlSign
		}
		res := shared.MediaInfo{
			Id:          id,
			Url:         rawUrl,
			UrlSign:     urlSign,
			CoverUrl:    "",
			Size:        shared.FormatSize(value),
			Domain:      shared.GetTopLevelDomain(rawUrl),
			Classify:    classify,
			Suffix:      suffix,
			Status:      shared.DownloadStatusReady,
			SavePath:    "",
			DecodeKey:   "",
			OtherData:   map[string]string{},
			Description: "",
			ContentType: resp.Header.Get("Content-Type"),
		}

		if classify == "stream" {
			res.Type = "stream"
		}

		// Store entire request headers as JSON
		if headers, err := json.Marshal(resp.Request.Header); err == nil {
			res.OtherData["headers"] = string(headers)
		}

		p.bridge.MarkMedia(urlSign)
		go func(res shared.MediaInfo) {
			p.bridge.Send("newResources", res)
		}(res)
	}

	return resp
}

// parseTotalFromContentRange parses header like: bytes start-end/total
// and returns total bytes if available.
func parseTotalFromContentRange(cr string) int64 {
	if cr == "" {
		return 0
	}
	// Example: bytes 0-1023/99999 or bytes 0-1023/*
	cr = strings.TrimSpace(cr)
	if !strings.HasPrefix(strings.ToLower(cr), "bytes") {
		return 0
	}
	slashIdx := strings.LastIndex(cr, "/")
	if slashIdx <= 0 || slashIdx+1 >= len(cr) {
		return 0
	}
	totalStr := cr[slashIdx+1:]
	if totalStr == "*" {
		return 0
	}
	if v, err := strconv.ParseInt(totalStr, 10, 64); err == nil && v > 0 {
		return v
	}
	return 0
}
