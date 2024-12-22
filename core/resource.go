package core

import (
	"encoding/base64"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	DownloadStatusReady   string = "ready" // task create but not start
	DownloadStatusRunning string = "running"
	DownloadStatusError   string = "error"
	DownloadStatusDone    string = "done"
	DownloadStatusHandle  string = "handle"
)

type WxFileDecodeResult struct {
	SavePath string
	Message  string
}

type Resource struct {
	mark      map[string]bool
	markMu    sync.RWMutex
	resType   map[string]bool
	resTypeMu sync.RWMutex
}

func initResource() *Resource {
	if resourceOnce == nil {
		resourceOnce = &Resource{
			mark: make(map[string]bool),
			resType: map[string]bool{
				"all":   true,
				"image": true,
				"audio": true,
				"video": true,
				"m3u8":  true,
				"live":  true,
				"xls":   true,
				"doc":   true,
				"pdf":   true,
			},
		}
	}
	return resourceOnce
}

func (r *Resource) getMark(key string) (bool, bool) {
	r.markMu.RLock()
	defer r.markMu.RUnlock()
	value, ok := r.mark[key]
	return value, ok
}

func (r *Resource) setMark(key string, value bool) {
	r.markMu.Lock()
	defer r.markMu.Unlock()
	r.mark[key] = value
}

func (r *Resource) getResType(key string) (bool, bool) {
	r.resTypeMu.RLock()
	defer r.resTypeMu.RUnlock()
	value, ok := r.resType[key]
	return value, ok
}

func (r *Resource) setResType(n []string) {
	r.resTypeMu.Lock()
	defer r.resTypeMu.Unlock()
	r.resType = map[string]bool{
		"all":   false,
		"image": false,
		"audio": false,
		"video": false,
		"m3u8":  false,
		"live":  false,
		"xls":   false,
		"doc":   false,
		"pdf":   false,
	}

	for _, value := range n {
		r.resType[value] = true
	}
}

func (r *Resource) clear() {
	r.markMu.Lock()
	defer r.markMu.Unlock()
	r.mark = make(map[string]bool)
}

func (r *Resource) delete(sign string) {
	r.markMu.Lock()
	defer r.markMu.Unlock()
	delete(r.mark, sign)
}

func (r *Resource) download(mediaInfo MediaInfo, decodeStr string) {
	if globalConfig.SaveDirectory == "" {
		return
	}
	go func(mediaInfo MediaInfo) {
		rawUrl := mediaInfo.Url
		fileName := Md5(rawUrl)
		if mediaInfo.Description != "" {
			fileName = regexp.MustCompile(`[^\w\p{Han}]`).ReplaceAllString(mediaInfo.Description, "")
			runes := []rune(fileName)
			if len(runes) > 10 {
				fileName = string(runes[:10])
			}
		}

		mediaInfo.SavePath = filepath.Join(globalConfig.SaveDirectory, fileName+"_"+GetCurrentDateTimeFormatted()+mediaInfo.Suffix)

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

		downloader := NewFileDownloader(rawUrl, mediaInfo.SavePath, globalConfig.TaskNumber)
		downloader.progressCallback = func(totalDownloaded float64) {
			r.progressEventsEmit(mediaInfo, strconv.Itoa(int(totalDownloaded))+"%", DownloadStatusRunning)
		}
		err := downloader.Start()
		if err != nil {
			r.progressEventsEmit(mediaInfo, err.Error())
			return
		}
		if decodeStr != "" {
			r.progressEventsEmit(mediaInfo, "解密中", DownloadStatusRunning)
			if err := r.decodeWxFile(mediaInfo.SavePath, decodeStr); err != nil {
				r.progressEventsEmit(mediaInfo, "解密出错"+err.Error())
				return
			}
		}
		r.progressEventsEmit(mediaInfo, "完成", DownloadStatusDone)
	}(mediaInfo)
}

func (r *Resource) wxFileDecode(mediaInfo MediaInfo, fileName, decodeStr string) (string, error) {
	sourceFile, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()
	mediaInfo.SavePath = strings.ReplaceAll(fileName, ".mp4", "_解密.mp4")

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

func (r *Resource) progressEventsEmit(mediaInfo MediaInfo, args ...string) {
	Status := DownloadStatusError
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
	_, err = file.Read(fileBytes)
	if err != nil && err != io.EOF {
		return err
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
