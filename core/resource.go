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
	URLSign     string  `json:"url_sign"`     // URL 的 MD5 签名（主键）
	URL         string  `json:"url"`          // 原始 URL
	Description string  `json:"description"`  // 文件描述（标题#话题）
	SavePath    string  `json:"save_path"`    // 保存路径
	DownloadAt  int64   `json:"download_at"`  // 下载时间戳（Unix timestamp）
	FileSize    float64 `json:"file_size"`    // 文件大小
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
		"all": true,
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

		if globalConfig.FilenameTime {
			mediaInfo.SavePath = filepath.Join(globalConfig.SaveDirectory, fileName+"_"+shared.GetCurrentDateTimeFormatted())
		} else {
			mediaInfo.SavePath = filepath.Join(globalConfig.SaveDirectory, fileName)
		}

		if !strings.HasSuffix(mediaInfo.SavePath, mediaInfo.Suffix) {
			mediaInfo.SavePath = mediaInfo.SavePath + mediaInfo.Suffix
		}

		// 新增：检查文件是否已存在
		if shared.FileExist(mediaInfo.SavePath) {
			r.progressEventsEmit(mediaInfo, "文件已存在，跳过下载", shared.DownloadStatusDone)
			return
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
