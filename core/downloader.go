package core

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type ProgressCallback func(totalDownloaded float64, totalSize float64)

type DownloadTask struct {
	taskID         int
	rangeStart     int64
	rangeEnd       int64
	downloadedSize int64
	isCompleted    bool
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
	Headers          map[string]string
	DownloadTaskList []*DownloadTask
	progressCallback ProgressCallback
}

func NewFileDownloader(url, filename string, totalTasks int, headers map[string]string) *FileDownloader {
	return &FileDownloader{
		Url:              url,
		FileName:         filename,
		totalTasks:       totalTasks,
		IsMultiPart:      false,
		TotalSize:        0,
		Headers:          headers,
		DownloadTaskList: make([]*DownloadTask, 0),
	}
}

func (fd *FileDownloader) buildClient() *http.Client {
	transport := &http.Transport{}
	if fd.ProxyUrl != nil {
		transport.Proxy = http.ProxyURL(fd.ProxyUrl)
	}
	return &http.Client{
		Transport: transport,
	}
}

func (fd *FileDownloader) setHeaders(request *http.Request) {
	for key, value := range fd.Headers {
		if strings.Contains(globalConfig.UseHeaders, key) {
			request.Header.Set(key, value)
		}
	}
}

func (fd *FileDownloader) init() error {
	parsedURL, err := url.Parse(fd.Url)
	if err != nil {
		return err
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
	resp, err := fd.buildClient().Do(request)
	if err != nil {
		return fmt.Errorf("HEAD request failed: %w", err)
	}
	defer resp.Body.Close()

	fd.TotalSize = resp.ContentLength
	if fd.TotalSize <= 0 {
		return fmt.Errorf("invalid file size")
	}
	if resp.Header.Get("Accept-Ranges") == "bytes" && fd.TotalSize > 10*1024*1024 {
		fd.IsMultiPart = true
	}

	dir := filepath.Dir(fd.FileName)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	fd.File, err = os.OpenFile(fd.FileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("file open failed: %w", err)
	}
	if err := fd.File.Truncate(fd.TotalSize); err != nil {
		fd.File.Close()
		return fmt.Errorf("file truncate failed: %w", err)
	}
	return nil
}

func (fd *FileDownloader) createDownloadTasks() {
	if fd.IsMultiPart {
		if int64(fd.totalTasks) > fd.TotalSize {
			fd.totalTasks = int(fd.TotalSize)
		}
		eachSize := fd.TotalSize / int64(fd.totalTasks)
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
		fd.DownloadTaskList = append(fd.DownloadTaskList, &DownloadTask{taskID: 0})
	}
}

func (fd *FileDownloader) startDownload() {
	wg := &sync.WaitGroup{}
	progressChan := make(chan int64)

	for _, task := range fd.DownloadTaskList {
		wg.Add(1)
		go fd.startDownloadTask(wg, progressChan, task)
	}
	go func() {
		wg.Wait()
		close(progressChan)
	}()

	if fd.progressCallback != nil {
		totalDownloaded := int64(0)
		for p := range progressChan {
			totalDownloaded += p
			fd.progressCallback(float64(totalDownloaded), float64(fd.TotalSize))
		}
	}
}

func (fd *FileDownloader) startDownloadTask(wg *sync.WaitGroup, progressChan chan int64, task *DownloadTask) {
	defer wg.Done()
	request, err := http.NewRequest("GET", fd.Url, nil)
	if err != nil {
		globalLogger.Error().Stack().Err(err).Msgf("任务%d创建请求出错", task.taskID)
		return
	}
	fd.setHeaders(request)

	if fd.IsMultiPart {
		rangeHeader := fmt.Sprintf("bytes=%d-%d", task.rangeStart, task.rangeEnd)
		request.Header.Set("Range", rangeHeader)
	}

	client := fd.buildClient()
	resp, err := client.Do(request)
	if err != nil {
		log.Printf("任务%d发送下载请求出错！%s", task.taskID, err)
		return
	}
	defer resp.Body.Close()

	buf := make([]byte, 8192)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			remain := task.rangeEnd - (task.rangeStart + task.downloadedSize) + 1
			n64 := int64(n)
			if n64 > remain {
				n = int(remain)
			}
			_, writeErr := fd.File.WriteAt(buf[:n], task.rangeStart+task.downloadedSize)
			if writeErr != nil {
				log.Printf("任务%d写入文件时出现错误！位置:%d, err: %s\n", task.taskID, task.rangeStart+task.downloadedSize, writeErr)
				return
			}
			task.downloadedSize += n64
			progressChan <- n64

			if task.rangeStart+task.downloadedSize-1 >= task.rangeEnd {
				task.isCompleted = true
				break
			}
		}
		if err != nil {
			if err == io.EOF {
				task.isCompleted = true
			}
			break
		}
	}
}

func (fd *FileDownloader) Start() error {
	if err := fd.init(); err != nil {
		return err
	}
	fd.createDownloadTasks()
	fd.startDownload()
	defer fd.File.Close()
	return nil
}
