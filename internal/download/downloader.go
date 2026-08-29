package download

import (
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MaxRetries                    = 3                // 最大重试次数
	RetryDelay                    = 3 * time.Second  // 重试延迟
	MinPartSize                   = 1 * 1024 * 1024  // 最小分片大小（1MB）
	downloadDialTimeout           = 15 * time.Second // 直连不可用时避免任务永久阻塞
	downloadTLSHandshakeTimeout   = 15 * time.Second
	downloadResponseHeaderTimeout = 30 * time.Second
)

var errIncompleteDownload = errors.New("download response ended before the expected size")

type downloadHTTPStatusError struct {
	statusCode int
	multipart  bool
}

func (e *downloadHTTPStatusError) Error() string {
	if e.multipart {
		return fmt.Sprintf("server does not support range requests, status: %d", e.statusCode)
	}
	if e.statusCode == http.StatusUnauthorized || e.statusCode == http.StatusForbidden {
		return fmt.Sprintf("unexpected status code: %d (access denied; recapture the resource and verify download proxy/header consistency)", e.statusCode)
	}
	return fmt.Sprintf("unexpected status code: %d", e.statusCode)
}

func retryableDownloadError(err error) bool {
	var statusErr *downloadHTTPStatusError
	if !errors.As(err, &statusErr) {
		return true
	}
	switch statusErr.statusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusGone:
		return false
	default:
		return true
	}
}

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

type fileDownloadCheckpoint struct {
	TotalSize    int64                        `json:"totalSize"`
	ETag         string                       `json:"etag,omitempty"`
	LastModified string                       `json:"lastModified,omitempty"`
	IsMultiPart  bool                         `json:"isMultiPart"`
	Tasks        []fileDownloadCheckpointTask `json:"tasks"`
}

type fileDownloadCheckpointTask struct {
	TaskID         int   `json:"taskId"`
	RangeStart     int64 `json:"rangeStart"`
	RangeEnd       int64 `json:"rangeEnd"`
	DownloadedSize int64 `json:"downloadedSize"`
	Completed      bool  `json:"completed"`
}

type FileDownloader struct {
	config           *config.Config
	logger           *logging.Logger
	Url              string
	Referer          string
	ProxyUrl         *url.URL
	FileName         string
	CheckpointPath   string
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
	etag             string
	lastModified     string
	restored         bool
	checkpointMu     sync.Mutex
	checkpointTasks  map[int]fileDownloadCheckpointTask
	lastCheckpoint   time.Time
}

func NewFileDownloader(url, filename string, totalTasks int, headers map[string]string, config *config.Config, logger *logging.Logger) *FileDownloader {
	return NewFileDownloaderContext(context.Background(), url, filename, "", totalTasks, headers, config, logger)
}

func NewFileDownloaderContext(parent context.Context, url, filename, checkpointPath string, totalTasks int, headers map[string]string, config *config.Config, logger *logging.Logger) *FileDownloader {
	ctx, cancelFunc := context.WithCancel(parent)
	if headers == nil {
		headers = make(map[string]string)
	}
	return &FileDownloader{
		config:           config,
		logger:           logger,
		Url:              url,
		FileName:         filename,
		CheckpointPath:   checkpointPath,
		totalTasks:       totalTasks,
		IsMultiPart:      false,
		RetryOnError:     false,
		TotalSize:        0,
		Headers:          headers,
		DownloadTaskList: make([]*DownloadTask, 0),
		ctx:              ctx,
		cancelFunc:       cancelFunc,
		checkpointTasks:  make(map[int]fileDownloadCheckpointTask),
	}
}

func (fd *FileDownloader) buildClient() *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   downloadDialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   downloadTLSHandshakeTimeout,
		ResponseHeaderTimeout: downloadResponseHeaderTimeout,
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
	"range":             {},
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
		lk := strings.ToLower(key)
		if _, forbidden := forbiddenDownloadHeaders[lk]; forbidden {
			continue
		}
		if fd.config.UseHeaders == "default" {
			request.Header.Set(key, value)
			continue
		}

		if strings.Contains(fd.config.UseHeaders, key) {
			request.Header.Set(key, value)
		}
	}
}

func contentRangeTotal(value string) int64 {
	slash := strings.LastIndex(value, "/")
	if slash < 0 || slash+1 >= len(value) {
		return 0
	}
	total, err := strconv.ParseInt(strings.TrimSpace(value[slash+1:]), 10, 64)
	if err != nil || total <= 0 {
		return 0
	}
	return total
}

func (fd *FileDownloader) rangeProbe() (*http.Response, error) {
	request, err := http.NewRequestWithContext(fd.ctx, http.MethodGet, fd.Url, nil)
	if err != nil {
		return nil, fmt.Errorf("create GET probe request failed: %w", err)
	}
	fd.setHeaders(request)
	request.Header.Set("Range", "bytes=0-0")
	return fd.buildClient().Do(request)
}

func (fd *FileDownloader) init() error {
	parsedURL, err := url.Parse(fd.Url)
	if err != nil {
		return fmt.Errorf("parse URL failed: %w", err)
	}
	if parsedURL.Scheme != "" && parsedURL.Host != "" {
		fd.Referer = parsedURL.Scheme + "://" + parsedURL.Host + "/"
	}

	if fd.config.DownloadProxy && fd.config.UpstreamProxy != "" && !strings.Contains(fd.config.UpstreamProxy, fd.config.Port) {
		proxyURL, err := url.Parse(fd.config.UpstreamProxy)
		if err == nil {
			fd.ProxyUrl = proxyURL
		}
	}

	request, err := http.NewRequestWithContext(fd.ctx, http.MethodHead, fd.Url, nil)
	if err != nil {
		return fmt.Errorf("create HEAD request failed: %w", err)
	}

	if _, ok := fd.Headers["User-Agent"]; !ok {
		fd.Headers["User-Agent"] = fd.config.UserAgent
	}
	if _, ok := fd.Headers["Referer"]; !ok {
		fd.Headers["Referer"] = fd.Referer
	}

	fd.setHeaders(request)

	var resp *http.Response
	for retries := 0; retries < MaxRetries; retries++ {
		resp, err = fd.buildClient().Do(request)
		if err == nil {
			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented {
				_ = resp.Body.Close()
				resp, err = fd.rangeProbe()
			}
		}
		if err == nil {
			break
		}
		if retries < MaxRetries-1 {
			select {
			case <-fd.ctx.Done():
				return fd.ctx.Err()
			case <-time.After(RetryDelay):
			}
			fd.logger.Warn().Msgf("download probe failed, retrying (%d/%d): %v", retries+1, MaxRetries, err)
		}
	}

	if err != nil {
		return fmt.Errorf("download probe failed after %d retries: %w", MaxRetries, err)
	}
	defer resp.Body.Close()

	fd.TotalSize = contentRangeTotal(resp.Header.Get("Content-Range"))
	if fd.TotalSize <= 0 {
		fd.TotalSize = resp.ContentLength
	}
	fd.etag = resp.Header.Get("ETag")
	fd.lastModified = resp.Header.Get("Last-Modified")
	if fd.TotalSize <= 0 {
		fd.IsMultiPart = false
		fd.TotalSize = -1
	} else if (resp.StatusCode == http.StatusPartialContent || resp.Header.Get("Accept-Ranges") == "bytes") && fd.TotalSize > MinPartSize {
		fd.IsMultiPart = true
	}

	dir := filepath.Dir(fd.FileName)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("create directory failed: %w", err)
	}

	if fd.CheckpointPath == "" {
		fd.FileName = shared.GetUniqueFileName(fd.FileName)
	} else if err := fd.restoreCheckpoint(); err != nil {
		return err
	}

	fd.File, err = os.OpenFile(fd.FileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("file open failed: %w", err)
	}
	if !fd.restored && fd.TotalSize > 0 {
		if err := fd.File.Truncate(fd.TotalSize); err != nil {
			fd.File.Close()
			return fmt.Errorf("file truncate failed: %w", err)
		}
	} else if !fd.restored {
		if err := fd.File.Truncate(0); err != nil {
			fd.File.Close()
			return fmt.Errorf("file truncate failed: %w", err)
		}
	}
	return nil
}

func (fd *FileDownloader) restoreCheckpoint() error {
	raw, err := os.ReadFile(fd.CheckpointPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read download checkpoint: %w", err)
	}
	var checkpoint fileDownloadCheckpoint
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		_ = os.Remove(fd.CheckpointPath)
		_ = os.Remove(fd.FileName)
		return nil
	}
	if _, err := os.Stat(fd.FileName); err != nil || checkpoint.TotalSize != fd.TotalSize ||
		(checkpoint.ETag != "" && fd.etag != "" && checkpoint.ETag != fd.etag) ||
		(checkpoint.LastModified != "" && fd.lastModified != "" && checkpoint.LastModified != fd.lastModified) {
		_ = os.Remove(fd.CheckpointPath)
		_ = os.Remove(fd.FileName)
		return nil
	}
	if len(checkpoint.Tasks) == 0 {
		return nil
	}
	fd.IsMultiPart = checkpoint.IsMultiPart
	fd.totalTasks = len(checkpoint.Tasks)
	fd.DownloadTaskList = make([]*DownloadTask, 0, len(checkpoint.Tasks))
	for _, saved := range checkpoint.Tasks {
		if saved.TaskID < 0 || saved.DownloadedSize < 0 ||
			(saved.RangeEnd >= saved.RangeStart && saved.DownloadedSize > saved.RangeEnd-saved.RangeStart+1) {
			_ = os.Remove(fd.CheckpointPath)
			_ = os.Remove(fd.FileName)
			fd.DownloadTaskList = nil
			fd.checkpointTasks = make(map[int]fileDownloadCheckpointTask)
			return nil
		}
		task := &DownloadTask{
			taskID: saved.TaskID, rangeStart: saved.RangeStart, rangeEnd: saved.RangeEnd,
			downloadedSize: saved.DownloadedSize, isCompleted: saved.Completed,
		}
		fd.DownloadTaskList = append(fd.DownloadTaskList, task)
		fd.checkpointTasks[saved.TaskID] = saved
	}
	fd.restored = true
	return nil
}

func (fd *FileDownloader) updateCheckpoint(task *DownloadTask, force bool) error {
	if fd.CheckpointPath == "" {
		return nil
	}
	fd.checkpointMu.Lock()
	defer fd.checkpointMu.Unlock()
	fd.checkpointTasks[task.taskID] = fileDownloadCheckpointTask{
		TaskID: task.taskID, RangeStart: task.rangeStart, RangeEnd: task.rangeEnd,
		DownloadedSize: task.downloadedSize, Completed: task.isCompleted,
	}
	if !force && time.Since(fd.lastCheckpoint) < 500*time.Millisecond {
		return nil
	}
	return fd.writeCheckpointLocked()
}

func (fd *FileDownloader) writeCheckpoint(force bool) error {
	if fd.CheckpointPath == "" {
		return nil
	}
	fd.checkpointMu.Lock()
	defer fd.checkpointMu.Unlock()
	if !force && time.Since(fd.lastCheckpoint) < 500*time.Millisecond {
		return nil
	}
	return fd.writeCheckpointLocked()
}

func (fd *FileDownloader) writeCheckpointLocked() error {
	tasks := make([]fileDownloadCheckpointTask, 0, len(fd.checkpointTasks))
	for _, task := range fd.checkpointTasks {
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].TaskID < tasks[j].TaskID })
	checkpoint := fileDownloadCheckpoint{
		TotalSize: fd.TotalSize, ETag: fd.etag, LastModified: fd.lastModified,
		IsMultiPart: fd.IsMultiPart, Tasks: tasks,
	}
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	temporary := fd.CheckpointPath + ".tmp"
	if err := os.WriteFile(temporary, raw, 0600); err != nil {
		return err
	}
	if err := os.Rename(temporary, fd.CheckpointPath); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	fd.lastCheckpoint = time.Now()
	return nil
}

func (fd *FileDownloader) createDownloadTasks() {
	if len(fd.DownloadTaskList) > 0 {
		return
	}
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
	for _, task := range fd.DownloadTaskList {
		fd.checkpointTasks[task.taskID] = fileDownloadCheckpointTask{
			TaskID: task.taskID, RangeStart: task.rangeStart, RangeEnd: task.rangeEnd,
		}
	}
}

func (fd *FileDownloader) startDownload() error {
	wg := &sync.WaitGroup{}
	progressChan := make(chan ProgressChan, len(fd.DownloadTaskList))
	errorChan := make(chan error, len(fd.DownloadTaskList))
	taskProgress := make([]int64, len(fd.DownloadTaskList))
	totalDownloaded := int64(0)

	for _, task := range fd.DownloadTaskList {
		if task.taskID >= 0 && task.taskID < len(taskProgress) {
			taskProgress[task.taskID] = task.downloadedSize
		}
		totalDownloaded += task.downloadedSize
		if task.isCompleted {
			continue
		}
		wg.Add(1)
		go fd.startDownloadTask(wg, progressChan, errorChan, task)
	}
	if fd.progressCallback != nil && totalDownloaded > 0 {
		fd.progressCallback(float64(totalDownloaded), float64(fd.TotalSize), 0, 0)
	}

	go func() {
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
		if fd.ctx.Err() != nil {
			return context.Cause(fd.ctx)
		}
		if !fd.RetryOnError && fd.IsMultiPart && retryableDownloadError(errArr[0]) {
			// 降级
			fd.RetryOnError = true
			fd.DownloadTaskList = []*DownloadTask{}
			fd.totalTasks = 1
			fd.IsMultiPart = false
			fd.checkpointTasks = make(map[int]fileDownloadCheckpointTask)
			fd.createDownloadTasks()
			_ = fd.writeCheckpoint(true)
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

	attempts := 0
	for retries := 0; retries < MaxRetries; retries++ {
		attempts = retries + 1
		err := fd.doDownloadTask(progressChan, task)
		if err == nil {
			task.isCompleted = true
			if checkpointErr := fd.updateCheckpoint(task, true); checkpointErr != nil {
				errorChan <- fmt.Errorf("save download checkpoint: %w", checkpointErr)
				return
			}
			return
		}

		if strings.Contains(err.Error(), "cancelled") {
			errorChan <- err
			return
		}

		task.err = err
		fd.logger.Warn().Msgf("Task %d failed (attempt %d/%d): %v", task.taskID, retries+1, MaxRetries, err)

		if !retryableDownloadError(err) {
			break
		}

		if retries < MaxRetries-1 && !errors.Is(err, errIncompleteDownload) {
			select {
			case <-fd.ctx.Done():
				errorChan <- fmt.Errorf("task %d cancelled during retry", task.taskID)
				return
			case <-time.After(RetryDelay):
			}
		}
	}

	errorChan <- fmt.Errorf("task %d failed after %d attempts: %w", task.taskID, attempts, task.err)
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

	requestedRange := fd.IsMultiPart || task.downloadedSize > 0
	if requestedRange {
		rangeStart := task.rangeStart + task.downloadedSize
		rangeHeader := fmt.Sprintf("bytes=%d-", rangeStart)
		if task.rangeEnd >= 0 {
			rangeHeader = fmt.Sprintf("bytes=%d-%d", rangeStart, task.rangeEnd)
		}
		request.Header.Set("Range", rangeHeader)
	}

	client := fd.buildClient()
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if fd.IsMultiPart && resp.StatusCode != http.StatusPartialContent {
		return &downloadHTTPStatusError{statusCode: resp.StatusCode, multipart: true}
	} else if !fd.IsMultiPart && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return &downloadHTTPStatusError{statusCode: resp.StatusCode}
	}
	if requestedRange && !fd.IsMultiPart && resp.StatusCode == http.StatusOK && task.downloadedSize > 0 {
		previous := task.downloadedSize
		task.downloadedSize = 0
		if checkpointErr := fd.updateCheckpoint(task, true); checkpointErr != nil {
			return fmt.Errorf("reset download checkpoint: %w", checkpointErr)
		}
		progressChan <- ProgressChan{taskID: task.taskID, bytes: -previous}
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
			if checkpointErr := fd.updateCheckpoint(task, false); checkpointErr != nil {
				return fmt.Errorf("save download checkpoint: %w", checkpointErr)
			}
			progressChan <- ProgressChan{taskID: task.taskID, bytes: writeSize}

			if fd.TotalSize > 0 && task.rangeStart+task.downloadedSize-1 >= task.rangeEnd {
				return nil
			}
		}

		if err != nil {
			if err == io.EOF {
				if task.rangeEnd >= task.rangeStart {
					expected := task.rangeEnd - task.rangeStart + 1
					if task.downloadedSize < expected {
						return fmt.Errorf("%w: got %d of %d bytes", errIncompleteDownload, task.downloadedSize, expected)
					}
				}
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
		var downloaded int64
		for _, task := range fd.DownloadTaskList {
			downloaded += task.downloadedSize
		}
		if downloaded != fd.TotalSize {
			return fmt.Errorf("downloaded size is %d, expected %d", downloaded, fd.TotalSize)
		}
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
	if err := fd.writeCheckpoint(true); err != nil {
		if fd.File != nil {
			_ = fd.File.Close()
		}
		return fmt.Errorf("initialize download checkpoint: %w", err)
	}

	err := fd.startDownload()
	if checkpointErr := fd.writeCheckpoint(true); err == nil && checkpointErr != nil {
		err = fmt.Errorf("finalize download checkpoint: %w", checkpointErr)
	}

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

}
