package core

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"res-downloader/core/shared"
	"strconv"
	"strings"
	"sync"
	"time"
)

type WxFileDecodeResult struct {
	SavePath string
	Message  string
}

type Resource struct {
	mediaMark  sync.Map
	tasks      sync.Map
	resType    map[string]bool
	resTypeMux sync.RWMutex
	history    *DownloadHistory // 新增：下载历史管理器
}

// DownloadRecord 下载历史记录
type DownloadRecord struct {
	URLSign     string  `json:"url_sign"`    // URL 的 MD5 签名（主键）
	URL         string  `json:"url"`         // 原始 URL
	Description string  `json:"description"` // 文件描述（标题#话题）
	SavePath    string  `json:"save_path"`   // 保存路径
	DownloadAt  int64   `json:"download_at"` // 下载时间戳（Unix timestamp）
	FileSize    float64 `json:"file_size"`   // 文件大小
}

type ImageSetFile struct {
	Index    int     `json:"index"`
	FileName string  `json:"file_name"`
	URL      string  `json:"url"`
	Size     float64 `json:"size"`
}

type ImageSetAudioFile struct {
	FileName string  `json:"file_name"`
	Path     string  `json:"path"`
	URL      string  `json:"url"`
	Size     float64 `json:"size"`
	Name     string  `json:"name"`
}

type ImageSetMetadata struct {
	Type        string             `json:"type"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Topic       string             `json:"topic"`
	SourceURL   string             `json:"source_url"`
	PublishTime string             `json:"publish_time"`
	OriginalID  string             `json:"original_id"`
	ImageCount  int                `json:"image_count"`
	Cover       string             `json:"cover"`
	Images      []ImageSetFile     `json:"images"`
	Audio       *ImageSetAudioFile `json:"audio,omitempty"`
	AudioStatus string             `json:"audio_status"`
	CapturedAt  string             `json:"captured_at"`
}

// DownloadHistory 历史记录管理器
type DownloadHistory struct {
	Records map[string]DownloadRecord `json:"records"` // key: urlSign
	storage *Storage
	mu      sync.RWMutex
}

// newDownloadHistory 创建并加载下载历史
func newDownloadHistory() *DownloadHistory {
	h := &DownloadHistory{
		Records: make(map[string]DownloadRecord),
		storage: NewStorage("download_history.json", []byte(`{"records":{}}`)),
	}
	h.load()
	return h
}

// load 从文件加载历史记录
func (h *DownloadHistory) load() {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := h.storage.Load()
	if err != nil {
		globalLogger.Warn().Msgf("Failed to load download history: %v", err)
		return
	}

	var historyData struct {
		Records map[string]DownloadRecord `json:"records"`
	}
	if err := json.Unmarshal(data, &historyData); err != nil {
		globalLogger.Warn().Msgf("Failed to parse download history: %v", err)
		return
	}

	h.Records = historyData.Records
	globalLogger.Info().Msgf("Loaded %d download records", len(h.Records))
}

// save 保存历史记录到文件
func (h *DownloadHistory) save() error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	historyData := struct {
		Records map[string]DownloadRecord `json:"records"`
	}{
		Records: h.Records,
	}

	data, err := json.Marshal(historyData)
	if err != nil {
		return err
	}

	return h.storage.Store(data)
}

// isMarked 检查 URL 是否已在历史记录中
func (h *DownloadHistory) isMarked(urlSign string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.Records[urlSign]
	return exists
}

// isDownloaded 检查 URL 是否已下载且文件仍存在
func (h *DownloadHistory) isDownloaded(urlSign string) bool {
	h.mu.RLock()
	record, exists := h.Records[urlSign]
	h.mu.RUnlock()

	if !exists {
		return false
	}

	// 检查文件或目录是否还存在
	if _, err := os.Stat(record.SavePath); err == nil {
		return true
	}

	// 文件被删除了，从历史记录中移除，允许重新下载
	h.mu.Lock()
	delete(h.Records, urlSign)
	h.mu.Unlock()
	go h.save()

	return false
}

// add 添加下载记录
func (h *DownloadHistory) add(record DownloadRecord) {
	h.mu.Lock()
	h.Records[record.URLSign] = record
	h.mu.Unlock()

	// 异步保存，避免阻塞
	go func() {
		if err := h.save(); err != nil {
			globalLogger.Warn().Msgf("Failed to save download history: %v", err)
		}
	}()
}

// clear 清空所有历史记录
func (h *DownloadHistory) clear() error {
	h.mu.Lock()
	h.Records = make(map[string]DownloadRecord)
	h.mu.Unlock()

	return h.save()
}

func initResource() *Resource {
	if resourceOnce == nil {
		resourceOnce = &Resource{
			history: newDownloadHistory(), // 新增：初始化历史管理器
		}
		resourceOnce.resType = resourceOnce.buildResType(globalConfig.MimeMap)
	}
	return resourceOnce
}

func (r *Resource) buildResType(mime map[string]MimeInfo) map[string]bool {
	t := map[string]bool{
		"all":       true,
		"image_set": true,
	}

	for _, item := range mime {
		if _, ok := t[item.Type]; !ok {
			t[item.Type] = true
		}
	}

	return t
}

func (r *Resource) mediaIsMarked(key string) bool {
	// 先检查持久化历史
	if r.history.isMarked(key) {
		return true
	}
	// 再检查内存标记（兼容性保留）
	_, loaded := r.mediaMark.Load(key)
	return loaded
}

func (r *Resource) markMedia(key string) {
	// 只标记内存，不立即保存到历史
	// 历史记录在下载完成后由 download() 函数添加
	r.mediaMark.Store(key, true)
}

func (r *Resource) getResType(key string) (bool, bool) {
	r.resTypeMux.RLock()
	value, ok := r.resType[key]
	r.resTypeMux.RUnlock()
	return value, ok
}

func (r *Resource) setResType(n []string) {
	r.resTypeMux.Lock()
	for key := range r.resType {
		r.resType[key] = false
	}

	for _, value := range n {
		if _, ok := r.resType[value]; ok {
			r.resType[value] = true
		}
	}
	r.resTypeMux.Unlock()
}

func (r *Resource) clear() {
	r.mediaMark.Clear()
}

func (r *Resource) delete(sign string) {
	r.mediaMark.Delete(sign)
}

func (r *Resource) cancel(id string) error {
	if d, ok := r.tasks.Load(id); ok {
		d.(*FileDownloader).Cancel()
		r.tasks.Delete(id) // 可选：取消后清理
		return nil
	}
	return errors.New("task not found")
}

func (r *Resource) download(mediaInfo shared.MediaInfo, decodeStr string) {
	if globalConfig.SaveDirectory == "" {
		return
	}
	go func(mediaInfo shared.MediaInfo) {
		// 首先检查是否已在历史记录中且文件仍存在
		if r.history.isDownloaded(mediaInfo.UrlSign) {
			r.progressEventsEmit(mediaInfo, "已下载过，跳过", shared.DownloadStatusDone)
			return
		}

		if mediaInfo.Classify == "image_set" {
			r.downloadImageSet(mediaInfo)
			return
		}

		rawUrl := mediaInfo.Url
		fileName := shared.Md5(rawUrl)

		if v := shared.GetFileNameFromURL(rawUrl); v != "" {
			fileName = v
		}

		if mediaInfo.Description != "" {
			// 直接使用 Description，不做额外的字符过滤
			// Description 已经在 plugin.qq.com.go 中经过 sanitizeFilename 处理
			fileName = mediaInfo.Description
		}

		// 应用时间戳（如果启用）
		if globalConfig.FilenameTime {
			mediaInfo.SavePath = filepath.Join(globalConfig.SaveDirectory, fileName+"_"+shared.GetCurrentDateTimeFormatted())
		} else {
			mediaInfo.SavePath = filepath.Join(globalConfig.SaveDirectory, fileName)
		}

		if !strings.HasSuffix(mediaInfo.SavePath, mediaInfo.Suffix) {
			mediaInfo.SavePath = mediaInfo.SavePath + mediaInfo.Suffix
		}

		if strings.Contains(rawUrl, "qq.com") {
			if globalConfig.Quality == 1 &&
				strings.Contains(rawUrl, "encfilekey=") &&
				strings.Contains(rawUrl, "token=") {
				parseUrl, err := url.Parse(rawUrl)
				queryParams := parseUrl.Query()
				if err == nil && queryParams.Has("encfilekey") && queryParams.Has("token") {
					rawUrl = parseUrl.Scheme + "://" + parseUrl.Host + "/" + parseUrl.Path +
						"?encfilekey=" + queryParams.Get("encfilekey") +
						"&token=" + queryParams.Get("token")
				}
			} else if globalConfig.Quality > 1 && mediaInfo.OtherData["wx_file_formats"] != "" {
				format := strings.Split(mediaInfo.OtherData["wx_file_formats"], "#")
				qualityMap := []string{
					format[0],
					format[len(format)/2],
					format[len(format)-1],
				}
				rawUrl += "&X-snsvideoflag=" + qualityMap[globalConfig.Quality-2]
			}
		}

		headers, _ := r.parseHeaders(mediaInfo)

		downloader := NewFileDownloader(rawUrl, mediaInfo.SavePath, globalConfig.TaskNumber, headers)
		downloader.progressCallback = func(totalDownloaded, totalSize float64, taskID int, taskProgress float64) {
			r.progressEventsEmit(mediaInfo, strconv.Itoa(int(totalDownloaded*100/totalSize))+"%", shared.DownloadStatusRunning)
		}
		r.tasks.Store(mediaInfo.Id, downloader)
		err := downloader.Start()
		mediaInfo.SavePath = downloader.FileName
		if err != nil {
			if !strings.Contains(err.Error(), "cancelled") {
				r.progressEventsEmit(mediaInfo, err.Error())
			}
			return
		}
		if decodeStr != "" {
			r.progressEventsEmit(mediaInfo, "decrypting in progress", shared.DownloadStatusRunning)
			if err := r.decodeWxFile(mediaInfo.SavePath, decodeStr); err != nil {
				r.progressEventsEmit(mediaInfo, "decryption error: "+err.Error())
				return
			}
		}
		r.progressEventsEmit(mediaInfo, "complete", shared.DownloadStatusDone)

		// 新增：添加到下载历史
		r.history.add(DownloadRecord{
			URLSign:     mediaInfo.UrlSign,
			URL:         mediaInfo.Url,
			Description: mediaInfo.Description,
			SavePath:    mediaInfo.SavePath,
			DownloadAt:  time.Now().Unix(),
			FileSize:    mediaInfo.Size,
		})
	}(mediaInfo)
}

func (r *Resource) downloadImageSet(mediaInfo shared.MediaInfo) {
	urls, err := parseImageSetURLs(mediaInfo)
	if err != nil {
		r.progressEventsEmit(mediaInfo, err.Error())
		return
	}

	dirName := imageSetDirName(mediaInfo)
	saveDir := uniqueDir(filepath.Join(globalConfig.SaveDirectory, dirName))
	imagesDir := filepath.Join(saveDir, "images")
	if err := os.MkdirAll(imagesDir, os.ModePerm); err != nil {
		r.progressEventsEmit(mediaInfo, "create image set directory failed: "+err.Error())
		return
	}

	headers, _ := r.parseHeaders(mediaInfo)
	files := make([]ImageSetFile, 0, len(urls))

	for index, rawUrl := range urls {
		fileName := fmt.Sprintf("%03d%s", index+1, imageSuffixFromURL(rawUrl))
		savePath := filepath.Join(imagesDir, fileName)
		downloader := NewFileDownloader(rawUrl, savePath, 1, headers)
		r.tasks.Store(mediaInfo.Id, downloader)
		if err := downloader.Start(); err != nil {
			r.progressEventsEmit(mediaInfo, fmt.Sprintf("download image %d failed: %v", index+1, err))
			return
		}

		size := float64(0)
		if stat, err := os.Stat(downloader.FileName); err == nil {
			size = float64(stat.Size())
		}
		files = append(files, ImageSetFile{
			Index:    index + 1,
			FileName: filepath.Base(downloader.FileName),
			URL:      rawUrl,
			Size:     size,
		})
		r.progressEventsEmit(mediaInfo, fmt.Sprintf("%d/%d", index+1, len(urls)), shared.DownloadStatusRunning)
	}

	audio := r.downloadImageSetAudio(mediaInfo, saveDir, headers)
	metadata := buildImageSetMetadata(mediaInfo, files, audio)
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		r.progressEventsEmit(mediaInfo, "build metadata failed: "+err.Error())
		return
	}
	if err := os.WriteFile(filepath.Join(saveDir, "metadata.json"), metadataBytes, 0644); err != nil {
		r.progressEventsEmit(mediaInfo, "write metadata failed: "+err.Error())
		return
	}

	mediaInfo.SavePath = saveDir
	r.progressEventsEmit(mediaInfo, "complete", shared.DownloadStatusDone)

	r.history.add(DownloadRecord{
		URLSign:     mediaInfo.UrlSign,
		URL:         mediaInfo.Url,
		Description: mediaInfo.Description,
		SavePath:    mediaInfo.SavePath,
		DownloadAt:  time.Now().Unix(),
		FileSize:    mediaInfo.Size,
	})
}

func parseImageSetURLs(mediaInfo shared.MediaInfo) ([]string, error) {
	rawURLs := strings.TrimSpace(mediaInfo.OtherData["image_set_urls"])
	if rawURLs == "" {
		return nil, errors.New("image set urls is empty")
	}

	var urls []string
	if err := json.Unmarshal([]byte(rawURLs), &urls); err != nil {
		return nil, fmt.Errorf("parse image set urls failed: %w", err)
	}

	result := make([]string, 0, len(urls))
	for _, rawURL := range urls {
		if strings.TrimSpace(rawURL) != "" {
			result = append(result, rawURL)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("image set urls is empty")
	}
	return result, nil
}

func (r *Resource) downloadImageSetAudio(mediaInfo shared.MediaInfo, saveDir string, headers map[string]string) *ImageSetAudioFile {
	rawURL := strings.TrimSpace(mediaInfo.OtherData["image_set_audio_url"])
	if rawURL == "" {
		return nil
	}

	fileName := strings.TrimSpace(mediaInfo.OtherData["image_set_audio_file_name"])
	if fileName == "" {
		fileName = "bgm" + audioSuffixFromURL(rawURL)
	}
	fileName = sanitizePathName(fileName)
	if filepath.Ext(fileName) == "" {
		fileName += audioSuffixFromURL(rawURL)
	}

	audioDir := filepath.Join(saveDir, "audio")
	if err := os.MkdirAll(audioDir, os.ModePerm); err != nil {
		r.progressEventsEmit(mediaInfo, "create audio directory failed: "+err.Error())
		return nil
	}

	savePath := filepath.Join(audioDir, fileName)
	downloader := NewFileDownloader(rawURL, savePath, 1, headers)
	r.tasks.Store(mediaInfo.Id, downloader)
	if err := downloader.Start(); err != nil {
		r.progressEventsEmit(mediaInfo, "download audio failed: "+err.Error())
		return nil
	}

	size := float64(0)
	if stat, err := os.Stat(downloader.FileName); err == nil {
		size = float64(stat.Size())
	}
	baseName := filepath.Base(downloader.FileName)

	return &ImageSetAudioFile{
		FileName: baseName,
		Path:     filepath.ToSlash(filepath.Join("audio", baseName)),
		URL:      rawURL,
		Size:     size,
		Name:     mediaInfo.OtherData["image_set_audio_name"],
	}
}

func buildImageSetMetadata(mediaInfo shared.MediaInfo, files []ImageSetFile, audio *ImageSetAudioFile) ImageSetMetadata {
	cover := ""
	if len(files) > 0 {
		cover = filepath.ToSlash(filepath.Join("images", files[0].FileName))
	}

	var audioMeta *ImageSetAudioFile
	if audio != nil {
		copyAudio := *audio
		if copyAudio.Path == "" && copyAudio.FileName != "" {
			copyAudio.Path = filepath.ToSlash(filepath.Join("audio", copyAudio.FileName))
		}
		audioMeta = &copyAudio
	}

	return ImageSetMetadata{
		Type:        "wechat_channels_image_set",
		Title:       mediaInfo.Description,
		Description: mediaInfo.OtherData["image_set_description"],
		Topic:       mediaInfo.OtherData["image_set_topic"],
		SourceURL:   mediaInfo.OtherData["image_set_source_url"],
		PublishTime: mediaInfo.OtherData["image_set_publish_time"],
		OriginalID:  mediaInfo.OtherData["image_set_original_id"],
		ImageCount:  len(files),
		Cover:       cover,
		Images:      files,
		Audio:       audioMeta,
		AudioStatus: imageSetAudioStatus(mediaInfo, audio),
		CapturedAt:  mediaInfo.OtherData["image_set_captured_at"],
	}
}

func imageSetAudioStatus(mediaInfo shared.MediaInfo, audio *ImageSetAudioFile) string {
	if audio != nil {
		return "downloaded"
	}
	if strings.TrimSpace(mediaInfo.OtherData["image_set_audio_url"]) != "" {
		return "download_failed"
	}
	return "no_audio_found"
}

func imageSetDirName(mediaInfo shared.MediaInfo) string {
	name := strings.TrimSpace(mediaInfo.Description)
	if name == "" {
		name = mediaInfo.UrlSign
	}
	name = sanitizePathName(name)
	if globalConfig.FilenameTime {
		return shared.GetCurrentDateTimeFormatted() + "_" + name
	}
	return name
}

func sanitizePathName(name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.ReplaceAll(name, "\n", " ")
	name = strings.ReplaceAll(name, "\r", " ")
	name = strings.ReplaceAll(name, "\t", " ")
	name = strings.Join(strings.Fields(name), " ")
	name = strings.Trim(name, ". ")
	if name == "" {
		return "image_set"
	}
	runes := []rune(name)
	if len(runes) > 80 {
		return string(runes[:80])
	}
	return name
}

func uniqueDir(dir string) string {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return dir
	}
	for index := 1; ; index++ {
		next := fmt.Sprintf("%s(%d)", dir, index)
		if _, err := os.Stat(next); os.IsNotExist(err) {
			return next
		}
	}
}

func imageSuffixFromURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err == nil {
		ext := strings.ToLower(filepath.Ext(parsedURL.Path))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".avif":
			return ext
		}
	}
	return ".jpg"
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

func (r *Resource) parseHeaders(mediaInfo shared.MediaInfo) (map[string]string, error) {
	headers := make(map[string]string)

	if hh, ok := mediaInfo.OtherData["headers"]; ok {
		var tempHeaders map[string][]string
		if err := json.Unmarshal([]byte(hh), &tempHeaders); err != nil {
			return headers, fmt.Errorf("parse headers JSON err: %v", err)
		}

		for key, values := range tempHeaders {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}

	return headers, nil
}

func (r *Resource) wxFileDecode(mediaInfo shared.MediaInfo, fileName, decodeStr string) (string, error) {
	sourceFile, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()
	mediaInfo.SavePath = strings.ReplaceAll(fileName, ".mp4", "_decrypt.mp4")

	destinationFile, err := os.Create(mediaInfo.SavePath)
	if err != nil {
		return "", err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return "", err
	}
	err = r.decodeWxFile(mediaInfo.SavePath, decodeStr)
	if err != nil {
		return "", err
	}
	return mediaInfo.SavePath, nil
}

func (r *Resource) progressEventsEmit(mediaInfo shared.MediaInfo, args ...string) {
	Status := shared.DownloadStatusError
	Message := "ok"

	if len(args) > 0 {
		Message = args[0]
	}
	if len(args) > 1 {
		Status = args[1]
	}

	httpServerOnce.send("downloadProgress", map[string]interface{}{
		"Id":       mediaInfo.Id,
		"Status":   Status,
		"SavePath": mediaInfo.SavePath,
		"Message":  Message,
	})
	return
}

func (r *Resource) decodeWxFile(fileName, decodeStr string) error {
	decodedBytes, err := base64.StdEncoding.DecodeString(decodeStr)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(fileName, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	byteCount := len(decodedBytes)
	fileBytes := make([]byte, byteCount)
	n, err := file.Read(fileBytes)
	if err != nil && err != io.EOF {
		return err
	}

	if n < byteCount {
		byteCount = n
	}

	xorResult := make([]byte, byteCount)
	for i := 0; i < byteCount; i++ {
		xorResult[i] = decodedBytes[i] ^ fileBytes[i]
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = file.Write(xorResult)
	if err != nil {
		return err
	}
	return nil
}
