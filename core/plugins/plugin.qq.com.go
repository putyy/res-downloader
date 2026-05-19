package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/elazarl/goproxy"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"res-downloader/core/shared"
	"strconv"
	"strings"
	"time"
)

var qqMediaRegex = regexp.MustCompile(`get\s*media\(\)\{`)
var qqCommentRegex = regexp.MustCompile(`async\s*finderGetCommentDetail\((\w+)\)\s*\{return(.*?)\s*}\s*async`)

type QqPlugin struct {
	bridge *shared.Bridge
}

func (p *QqPlugin) SetBridge(bridge *shared.Bridge) {
	p.bridge = bridge
}

func (p *QqPlugin) Domains() []string {
	return []string{"qq.com"}
}

func (p *QqPlugin) OnRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	if strings.Contains(r.Host, "qq.com") && strings.Contains(r.URL.Path, "/res-downloader/wechat") {
		if p.bridge.GetConfig("WxAction").(bool) && r.URL.Query().Get("type") == "1" {
			return p.handleWechatRequest(r, ctx)
		} else if !p.bridge.GetConfig("WxAction").(bool) && r.URL.Query().Get("type") == "2" {
			return p.handleWechatRequest(r, ctx)
		} else {
			return r, p.buildEmptyResponse(r)
		}
	}

	return nil, nil
}

func (p *QqPlugin) OnResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return nil
	}

	host := resp.Request.Host
	Path := resp.Request.URL.Path

	classify, _ := p.bridge.TypeSuffix(resp.Header.Get("Content-Type"))
	if classify == "video" && strings.HasSuffix(host, "finder.video.qq.com") {
		if strings.Contains(resp.Request.Header.Get("Origin"), "mp.weixin.qq.com") {
			return nil
		}
		return resp
	}

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
			newBody := qqMediaRegex.
				ReplaceAllString(bodyStr, `
							get media(){
								if(this.objectDesc){
									fetch("https://wxapp.tc.qq.com/res-downloader/wechat?type=1", {
									  method: "POST",
									  mode: "no-cors",
									  body: JSON.stringify(this.objectDesc),
									});
								};
			
			`)

			newBody = qqCommentRegex.
				ReplaceAllString(newBody, `
							async finderGetCommentDetail($1) {
								var res = await$2;
								if (res?.data?.object?.objectDesc) {
									fetch("https://wxapp.tc.qq.com/res-downloader/wechat?type=2", {
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

	return nil
}

func (p *QqPlugin) handleWechatRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return r, p.buildEmptyResponse(r)
	}

	go p.handleMedia(body)

	return r, p.buildEmptyResponse(r)
}

func (p *QqPlugin) handleMedia(body []byte) {
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	mediaArr, ok := result["media"].([]interface{})
	if !ok || len(mediaArr) == 0 {
		return
	}

	if p.isImageSet(result) {
		p.handleImageSet(result, mediaArr)
		return
	}

	firstMedia, ok := mediaArr[0].(map[string]interface{})
	if !ok {
		return
	}

	rawUrl, ok := firstMedia["url"].(string)
	if !ok || rawUrl == "" {
		return
	}

	// 先拼接完整 URL（包含 urlToken）
	if urlToken, ok := firstMedia["urlToken"].(string); ok {
		rawUrl += urlToken
	}

	// 基于完整 URL 计算签名
	urlSign := shared.Md5(rawUrl)
	if p.bridge.MediaIsMarked(urlSign) {
		return
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
		Size:        0,
		Domain:      shared.GetTopLevelDomain(rawUrl),
		Classify:    "video",
		Suffix:      ".mp4",
		Status:      shared.DownloadStatusReady,
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

	isAll, _ := p.bridge.GetResType("all")
	isImage, _ := p.bridge.GetResType("image")
	if res.Classify == "image" && !isImage && !isAll {
		return
	}

	isVideo, _ := p.bridge.GetResType("video")
	if res.Classify == "video" && !isVideo && !isAll {
		return
	}

	switch size := firstMedia["fileSize"].(type) {
	case float64:
		res.Size = size
	case string:
		if value, err := strconv.ParseFloat(size, 64); err == nil {
			res.Size = value
		}
	}

	if coverUrl, ok := firstMedia["coverUrl"].(string); ok {
		res.CoverUrl = coverUrl
	}

	if decodeKey, ok := firstMedia["decodeKey"].(string); ok {
		res.DecodeKey = decodeKey
	}

	// 提取完整标题和话题，构建新的文件名格式
	title := p.extractTitle(result)
	topic := p.extractTopic(result)

	// 构建 Description 用于文件名显示
	if title != "" {
		if topic != "" {
			res.Description = fmt.Sprintf("%s#%s", title, topic)
		} else {
			res.Description = title
		}
	} else if desc, ok := result["description"].(string); ok {
		// 降级：如果提取失败，使用原始 description
		res.Description = desc
	}

	if spec, ok := firstMedia["spec"].([]interface{}); ok {
		var fileFormats []string
		for _, item := range spec {
			if m, ok := item.(map[string]interface{}); ok {
				if format, ok := m["fileFormat"].(string); ok {
					fileFormats = append(fileFormats, format)
				}
			}
		}
		res.OtherData["wx_file_formats"] = strings.Join(fileFormats, "#")
	}

	p.bridge.MarkMedia(urlSign)

	go func(res shared.MediaInfo) {
		p.bridge.Send("newResources", res)
	}(res)
}

func (p *QqPlugin) isImageSet(result map[string]interface{}) bool {
	if mediaType, ok := result["mediaType"].(float64); ok && mediaType == 2 {
		return true
	}
	return false
}

func (p *QqPlugin) handleImageSet(result map[string]interface{}, mediaArr []interface{}) {
	urls := make([]string, 0, len(mediaArr))
	totalSize := float64(0)
	coverUrl := ""

	for _, item := range mediaArr {
		media, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		rawUrl, ok := media["url"].(string)
		if !ok || rawUrl == "" {
			continue
		}
		if urlToken, ok := media["urlToken"].(string); ok {
			rawUrl += urlToken
		}
		urls = append(urls, rawUrl)

		switch size := media["fileSize"].(type) {
		case float64:
			totalSize += size
		case string:
			if value, err := strconv.ParseFloat(size, 64); err == nil {
				totalSize += value
			}
		}

		if coverUrl == "" {
			coverUrl = firstString(media, "thumbUrl", "coverUrl", "fullThumbUrl")
		}
	}

	if len(urls) == 0 {
		return
	}

	isAll, _ := p.bridge.GetResType("all")
	isImage, _ := p.bridge.GetResType("image")
	isImageSet, _ := p.bridge.GetResType("image_set")
	if !isAll && !isImage && !isImageSet {
		return
	}

	urlSign := shared.Md5(strings.Join(urls, "\n"))
	if p.bridge.MediaIsMarked(urlSign) {
		return
	}

	id, err := gonanoid.New()
	if err != nil {
		id = urlSign
	}

	title := p.extractTitle(result)
	topic := p.extractTopic(result)
	description := title
	if description != "" && topic != "" {
		description = fmt.Sprintf("%s#%s", description, topic)
	} else if description == "" {
		description, _ = result["description"].(string)
	}

	urlBytes, _ := json.Marshal(urls)
	res := shared.MediaInfo{
		Id:          id,
		Url:         urls[0],
		UrlSign:     urlSign,
		CoverUrl:    coverUrl,
		Size:        totalSize,
		Domain:      shared.GetTopLevelDomain(urls[0]),
		Classify:    "image_set",
		Suffix:      "",
		Status:      shared.DownloadStatusReady,
		SavePath:    "",
		DecodeKey:   "",
		OtherData:   map[string]string{},
		Description: description,
		ContentType: "application/vnd.res-downloader.image-set",
	}
	res.OtherData["image_set_urls"] = string(urlBytes)
	res.OtherData["image_set_count"] = strconv.Itoa(len(urls))
	res.OtherData["image_set_title"] = title
	res.OtherData["image_set_description"] = firstResultString(result, "description")
	res.OtherData["image_set_topic"] = topic
	res.OtherData["image_set_original_id"] = firstResultString(result, "id", "objectId", "objectNonceId")
	res.OtherData["image_set_publish_time"] = extractNumberString(result, "createtime", "createTime")
	res.OtherData["image_set_captured_at"] = time.Now().Format(time.RFC3339)
	if audioURL, audioName := p.extractImageSetAudio(result); audioURL != "" {
		res.OtherData["image_set_audio_url"] = audioURL
		res.OtherData["image_set_audio_name"] = audioName
		res.OtherData["image_set_audio_file_name"] = p.imageSetAudioFileName(audioURL)
	}

	p.bridge.MarkMedia(urlSign)

	go func(res shared.MediaInfo) {
		p.bridge.Send("newResources", res)
	}(res)
}

func (p *QqPlugin) buildEmptyResponse(r *http.Request) *http.Response {
	body := "The content does not exist"
	resp := &http.Response{
		Status:        http.StatusText(http.StatusOK),
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}
	resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
	return resp
}

func (p *QqPlugin) replaceWxJsContent(resp *http.Response, old, new string) *http.Response {
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

func (p *QqPlugin) v() string {
	return p.bridge.GetVersion()
}

// extractTitle 提取完整标题
func (p *QqPlugin) extractTitle(result map[string]interface{}) string {
	// 优先从 short_title 提取
	if shortTitles, ok := result["short_title"].([]interface{}); ok && len(shortTitles) > 0 {
		if st, ok := shortTitles[0].(map[string]interface{}); ok {
			if title, ok := st["short_title"].(string); ok && title != "" {
				return p.sanitizeTitle(title, 70)
			}
		}
	}

	// 降级：从 flow_card_desc 提取
	if flowCard, ok := result["flow_card_desc"].(map[string]interface{}); ok {
		if desc, ok := flowCard["description"].(string); ok && desc != "" {
			return p.sanitizeTitle(desc, 70)
		}
	}

	// 最后降级：从 description 提取（去除话题标签）
	if desc, ok := result["description"].(string); ok && desc != "" {
		// 移除所有 #话题 标签
		re := regexp.MustCompile(`#[^\s#]+`)
		cleanDesc := re.ReplaceAllString(desc, "")
		cleanDesc = strings.TrimSpace(cleanDesc)
		return p.sanitizeTitle(cleanDesc, 70)
	}

	return ""
}

// extractTopic 提取话题标签
func (p *QqPlugin) extractTopic(result map[string]interface{}) string {
	// 优先从 topic 字段提取
	if topicObj, ok := result["topic"].(map[string]interface{}); ok {
		if topicName, ok := topicObj["topicName"].(string); ok && topicName != "" {
			return p.sanitizeFilename(topicName)
		}
	}

	// 降级：从 description 中提取第一个 #话题
	if desc, ok := result["description"].(string); ok {
		re := regexp.MustCompile(`#([^\s#]+)`)
		matches := re.FindStringSubmatch(desc)
		if len(matches) > 1 {
			return p.sanitizeFilename(matches[1])
		}
	}

	return ""
}

// sanitizeTitle 清理标题并限制长度（按字符数）
func (p *QqPlugin) sanitizeTitle(title string, maxChars int) string {
	// 移除非法文件名字符
	title = p.sanitizeFilename(title)

	// 限制长度（按字符数，不是字节数）
	runes := []rune(title)
	if len(runes) > maxChars {
		title = string(runes[:maxChars])
	}

	return strings.TrimSpace(title)
}

// sanitizeFilename 移除文件名中的非法字符
func (p *QqPlugin) sanitizeFilename(s string) string {
	// Windows 文件名非法字符: < > : " / \ | ? *
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)

	// 移除换行符和制表符
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")

	// 合并多个空格为一个
	re := regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")

	return strings.TrimSpace(s)
}

func (p *QqPlugin) extractImageSetAudio(result map[string]interface{}) (string, string) {
	return p.extractAudioFromValue(result, false)
}

func (p *QqPlugin) extractAudioFromValue(value interface{}, force bool) (string, string) {
	switch data := value.(type) {
	case map[string]interface{}:
		if force {
			audioURL := firstString(data,
				"url", "Url", "audioUrl", "audio_url", "musicUrl", "music_url",
				"bgmUrl", "bgm_url", "playUrl", "play_url", "mediaStreamingUrl",
				"media_streaming_url", "streamingUrl", "streaming_url", "src",
			)
			if token := firstString(data, "urlToken", "url_token", "UrlToken"); audioURL != "" && token != "" && !strings.Contains(audioURL, token) {
				audioURL += token
			}
			audioName := firstString(data,
				"name", "Name", "title", "Title", "songName", "song_name",
				"SongName", "musicName", "music_name", "MusicName",
			)
			if audioURL != "" {
				return audioURL, audioName
			}
		}
		for key, nested := range data {
			if isAudioLikeKey(key) {
				if audioURL, audioName := p.extractAudioFromValue(nested, true); audioURL != "" {
					return audioURL, audioName
				}
			}
		}
		for _, nested := range data {
			if audioURL, audioName := p.extractAudioFromValue(nested, false); audioURL != "" {
				return audioURL, audioName
			}
		}
	case []interface{}:
		for _, item := range data {
			if audioURL, audioName := p.extractAudioFromValue(item, force); audioURL != "" {
				return audioURL, audioName
			}
		}
	case string:
		if force && (strings.HasPrefix(data, "http://") || strings.HasPrefix(data, "https://")) {
			return data, ""
		}
	}
	return "", ""
}

func (p *QqPlugin) imageSetAudioFileName(rawURL string) string {
	fileName := ""
	if parsedURL, err := url.Parse(rawURL); err == nil {
		fileName = path.Base(parsedURL.Path)
		if fileName == "." || fileName == "/" {
			fileName = ""
		}
	}
	fileName = p.sanitizeFilename(fileName)
	if fileName == "" {
		return "bgm" + audioSuffixFromURL(rawURL)
	}
	if filepath.Ext(fileName) == "" {
		fileName += audioSuffixFromURL(rawURL)
	}
	return fileName
}

func isAudioLikeKey(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "audio") ||
		strings.Contains(key, "music") ||
		strings.Contains(key, "bgm") ||
		strings.Contains(key, "song") ||
		strings.Contains(key, "voice") ||
		strings.Contains(key, "sound")
}

func firstString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func audioSuffixFromURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err == nil {
		ext := strings.ToLower(filepath.Ext(parsedURL.Path))
		switch ext {
		case ".mp3", ".m4a", ".aac", ".wav", ".ogg", ".flac", ".amr", ".mp4":
			return ext
		}
	}
	return ".m4a"
}

func firstResultString(result map[string]interface{}, keys ...string) string {
	return firstString(result, keys...)
}

func extractNumberString(result map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		switch value := result[key].(type) {
		case float64:
			return strconv.FormatInt(int64(value), 10)
		case string:
			return value
		}
	}
	return ""
}
