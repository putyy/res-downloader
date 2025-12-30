package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"res-downloader/core/shared"
	"strings"
	"sync"
	"time"
)

const (
	MaxRetries  = 3               // 最大重试次数
	RetryDelay  = 3 * time.Second // 重试延迟
	MinPartSize = 1 * 1024 * 1024 // 最小分片大小（1MB）
)

type ProgressCallback func(totalDownloaded float64, totalSize float64, taskID int, taskProgress float64)

type ProgressChan struct {
	taskID int
	bytes  int64
}

type DownloadTask struct {
	taskID         int
	rangeStart     int64
	rangeEnd       int64
	downloadedSize int64
	isCompleted    bool
	err            error
}

type FileDownloader struct {
	Url              string
	Referer          string
	ProxyUrl         *url.URL
	FileName         string
	File             *os.File
	totalTasks       int
	TotalSize        int64
	IsMultiPart      bool
	RetryOnError     bool
	Headers          map[string]string
	DownloadTaskList []*DownloadTask
	progressCallback ProgressCallback
	ctx              context.Context
	cancelFunc       context.CancelFunc
}

func NewFileDownloader(url, filename string, totalTasks int, headers map[string]string) *FileDownloader {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &FileDownloader{
		Url:              url,
		FileName:         filename,
		totalTasks:       totalTasks,
		IsMultiPart:      false,
		RetryOnError:     false,
		TotalSize:        0,
		Headers:          headers,
		DownloadTaskList: make([]*DownloadTask, 0),
		ctx:              ctx,
		cancelFunc:       cancelFunc,
	}
}

func (fd *FileDownloader) buildClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}
	if fd.ProxyUrl != nil {
		transport.Proxy = http.ProxyURL(fd.ProxyUrl)
	}
	return &http.Client{
		Transport: transport,
	}
}

var forbiddenDownloadHeaders = map[string]struct{}{
	"accept-encoding":   {},
	"content-length":    {},
	"host":              {},
	"connection":        {},
	"keep-alive":        {},
	"proxy-connection":  {},
	"transfer-encoding": {},

	"sec-fetch-site":     {},
	"sec-fetch-mode":     {},
	"sec-fetch-dest":     {},
	"sec-fetch-user":     {},
	"sec-ch-ua":          {},
	"sec-ch-ua-mobile":   {},
	"sec-ch-ua-platform": {},

	"if-none-match":     {},
	"if-modified-since": {},

	"x-forwarded-for": {},
	"x-real-ip":       {},
}

func (fd *FileDownloader) setHeaders(request *http.Request) {
	for key, value := range fd.Headers {
		if globalConfig.UseHeaders == "default" {
			lk := strings.ToLower(key)
			if _, forbidden := forbiddenDownloadHeaders[lk]; forbidden {
				continue
			}
			request.Header.Set(key, value)
			continue
		}
		
		if strings.Contains(globalConfig.UseHeaders, key) {
			request.Header.Set(key, value)
		}
	}
}

func (fd *FileDownloader) init() error {
	parsedURL, err := url.Parse(fd.Url)
	if err != nil {
		return fmt.Errorf("parse URL failed: %w", err)
	}
	if parsedURL.Scheme != "" && parsedURL.Host != "" {
		fd.Referer = parsedURL.Scheme + "://" + parsedURL.Host + "/"
	}

	if globalConfig.DownloadProxy && globalConfig.UpstreamProxy != "" && !strings.Contains(globalConfig.UpstreamProxy, globalConfig.Port) {
		proxyURL, err := url.Parse(globalConfig.UpstreamProxy)
		if err == nil {
			fd.ProxyUrl = proxyURL
		}
	}

	request, err := http.NewRequest("HEAD", fd.Url, nil)
	if err != nil {
		return fmt.Errorf("create HEAD request failed: %w", err)
	}

	if _, ok := fd.Headers["User-Agent"]; !ok {
		fd.Headers["User-Agent"] = globalConfig.UserAgent
	}
	if _, ok := fd.Headers["Referer"]; !ok {
		fd.Headers["Referer"] = fd.Referer
	}

	fd.setHeaders(request)

	var resp *http.Response
	for retries := 0; retries < MaxRetries; retries++ {
		resp, err = fd.buildClient().Do(request)
		if err == nil {
			break
		}
		if retries < MaxRetries-1 {
			time.Sleep(RetryDelay)
			globalLogger.Warn().Msgf("HEAD request failed, retrying (%d/%d): %v", retries+1, MaxRetries, err)
		}
	}

	if err != nil {
		return fmt.Errorf("HEAD request failed after %d retries: %w", MaxRetries, err)
	}
	defer resp.Body.Close()

	fd.TotalSize = resp.ContentLength
	if fd.TotalSize <= 0 {
		fd.IsMultiPart = false
		fd.TotalSize = -1
	} else if resp.Header.Get("Accept-Ranges") == "bytes" && fd.TotalSize > MinPartSize {
		fd.IsMultiPart = true
	}

	dir := filepath.Dir(fd.FileName)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("create directory failed: %w", err)
	}

	fd.FileName = shared.GetUniqueFileName(fd.FileName)

	fd.File, err = os.OpenFile(fd.FileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("file open failed: %w", err)
	}
	if fd.TotalSize > 0 {
		if err := fd.File.Truncate(fd.TotalSize); err != nil {
			fd.File.Close()
			return fmt.Errorf("file truncate failed: %w", err)
		}
	}
	return nil
}

func (fd *FileDownloader) createDownloadTasks() {
	if fd.IsMultiPart {
		if fd.totalTasks <= 0 {
			fd.totalTasks = 4
		}
		eachSize := fd.TotalSize / int64(fd.totalTasks)
		if eachSize < MinPartSize {
			fd.totalTasks = int(fd.TotalSize / MinPartSize)
			if fd.totalTasks < 1 {
				fd.totalTasks = 1
			}
			eachSize = fd.TotalSize / int64(fd.totalTasks)
		}

		for i := 0; i < fd.totalTasks; i++ {
			start := eachSize * int64(i)
			end := eachSize*int64(i+1) - 1
			if i == fd.totalTasks-1 {
				end = fd.TotalSize - 1
			}
			fd.DownloadTaskList = append(fd.DownloadTaskList, &DownloadTask{
				taskID:     i,
				rangeStart: start,
				rangeEnd:   end,
			})
		}
	} else {
		fd.totalTasks = 1
		rangeEnd := int64(-1)
		if fd.TotalSize > 0 {
			rangeEnd = fd.TotalSize - 1
		}
		fd.DownloadTaskList = append(fd.DownloadTaskList, &DownloadTask{
			taskID:     0,
			rangeStart: 0,
			rangeEnd:   rangeEnd,
		})
	}
}

func (fd *FileDownloader) startDownload() error {
	wg := &sync.WaitGroup{}
	progressChan := make(chan ProgressChan, len(fd.DownloadTaskList))
	errorChan := make(chan error, len(fd.DownloadTaskList))

	for _, task := range fd.DownloadTaskList {
		wg.Add(1)
		go fd.startDownloadTask(wg, progressChan, errorChan, task)
	}

	go func() {
		taskProgress := make([]int64, len(fd.DownloadTaskList))
		totalDownloaded := int64(0)

		for progress := range progressChan {
			taskProgress[progress.taskID] += progress.bytes
			totalDownloaded += progress.bytes

			if fd.progressCallback != nil {
				taskPercentage := float64(0)
				if task := fd.DownloadTaskList[progress.taskID]; task != nil {
					taskSize := task.rangeEnd - task.rangeStart + 1
					if taskSize > 0 {
						taskPercentage = float64(taskProgress[progress.taskID]) / float64(taskSize) * 100
					}
				}
				fd.progressCallback(float64(totalDownloaded), float64(fd.TotalSize), progress.taskID, taskPercentage)
			}
		}
	}()

	go func() {
		wg.Wait()
		close(progressChan)
		close(errorChan)
	}()

	var errArr []error
	for err := range errorChan {
		errArr = append(errArr, err)
	}

	if len(errArr) > 0 {
		if !fd.RetryOnError && fd.IsMultiPart {
			// 降级
			fd.RetryOnError = true
			fd.DownloadTaskList = []*DownloadTask{}
			fd.totalTasks = 1
			fd.IsMultiPart = false
			fd.createDownloadTasks()
			return fd.startDownload()
		}
		return fmt.Errorf("download failed with %d errors: %v", len(errArr), errArr[0])
	}

	if err := fd.verifyDownload(); err != nil {
		return err
	}

	return nil
}

func (fd *FileDownloader) startDownloadTask(wg *sync.WaitGroup, progressChan chan ProgressChan, errorChan chan error, task *DownloadTask) {
	defer wg.Done()

	for retries := 0; retries < MaxRetries; retries++ {
		err := fd.doDownloadTask(progressChan, task)
		if err == nil {
			task.isCompleted = true
			return
		}

		if strings.Contains(err.Error(), "cancelled") {
			errorChan <- err
			return
		}

		task.err = err
		globalLogger.Warn().Msgf("Task %d failed (attempt %d/%d): %v", task.taskID, retries+1, MaxRetries, err)

		if retries < MaxRetries-1 {
			select {
			case <-fd.ctx.Done():
				errorChan <- fmt.Errorf("task %d cancelled during retry", task.taskID)
				return
			case <-time.After(RetryDelay):
			}
		}
	}

	errorChan <- fmt.Errorf("task %d failed after %d attempts: %v", task.taskID, MaxRetries, task.err)
}

func (fd *FileDownloader) doDownloadTask(progressChan chan ProgressChan, task *DownloadTask) error {
	select {
	case <-fd.ctx.Done():
		return fmt.Errorf("download cancelled")
	default:
	}

	request, err := http.NewRequestWithContext(fd.ctx, "GET", fd.Url, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	fd.setHeaders(request)

	if fd.IsMultiPart {
		rangeStart := task.rangeStart + task.downloadedSize
		rangeHeader := fmt.Sprintf("bytes=%d-%d", rangeStart, task.rangeEnd)
		request.Header.Set("Range", rangeHeader)
	}

	client := fd.buildClient()
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if fd.IsMultiPart && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server does not support range requests, status: %d", resp.StatusCode)
	} else if !fd.IsMultiPart && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-fd.ctx.Done():
			return fmt.Errorf("download cancelled")
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			writeSize := int64(n)
			offset := task.rangeStart + task.downloadedSize
			_, writeErr := fd.File.WriteAt(buf[:writeSize], offset)
			if writeErr != nil {
				return fmt.Errorf("write file failed at offset %d: %w", offset, writeErr)
			}

			task.downloadedSize += writeSize
			progressChan <- ProgressChan{taskID: task.taskID, bytes: writeSize}

			if fd.TotalSize > 0 && task.rangeStart+task.downloadedSize-1 >= task.rangeEnd {
				return nil
			}
		}

		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read response failed: %w", err)
		}
	}
}

func (fd *FileDownloader) verifyDownload() error {
	for _, task := range fd.DownloadTaskList {
		if !task.isCompleted {
			return fmt.Errorf("task %d not completed", task.taskID)
		}
	}

	if fd.TotalSize > 0 {
		_, err := fd.File.Stat()
		if err != nil {
			return fmt.Errorf("get file info failed: %w", err)
		}
	}

	return nil
}

func (fd *FileDownloader) Start() error {
	if err := fd.init(); err != nil {
		return err
	}
	fd.createDownloadTasks()

	err := fd.startDownload()

	if fd.File != nil {
		fd.File.Close()
	}

	return err
}

func (fd *FileDownloader) Cancel() {
	if fd.cancelFunc != nil {
		fd.cancelFunc()
	}

	if fd.File != nil {
		fd.File.Close()
	}

	if fd.FileName != "" {
		_ = os.Remove(fd.FileName)
	}
}
