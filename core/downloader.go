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

type ProgressCallback func(totalDownloaded float64)

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
	// Cookie handle
	return &http.Client{
		Transport: transport,
	}
}

func (fd *FileDownloader) setHeaders(request *http.Request) {
	for key, values := range fd.Headers {
		if strings.Contains(globalConfig.UseHeaders, key) {
			request.Header.Set(key, values)
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
		return fmt.Errorf("create request failed")
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
		return fmt.Errorf("request failed" + err.Error())
	}
	defer resp.Body.Close()

	fd.TotalSize = resp.ContentLength

	if fd.TotalSize <= 0 {
		return fmt.Errorf("request init failed: size 0")
	}

	if resp.Header.Get("Accept-Ranges") == "bytes" && fd.TotalSize > 10485760 {
		fd.IsMultiPart = true
	}

	fd.FileName = filepath.Clean(fd.FileName)

	dir := filepath.Dir(fd.FileName)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	fd.File, err = os.OpenFile(fd.FileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("文件初始化失败: %w", err)
	}

	if err = fd.File.Truncate(fd.TotalSize); err != nil {
		fd.File.Close()
		return fmt.Errorf("文件大小设置失败: %w", err)
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
			fd.DownloadTaskList = append(fd.DownloadTaskList, &DownloadTask{
				taskID:         i,
				rangeStart:     eachSize * int64(i),
				rangeEnd:       eachSize*int64(i+1) - 1,
				downloadedSize: 0,
				isCompleted:    false,
			})
		}
		fd.DownloadTaskList[len(fd.DownloadTaskList)-1].rangeEnd = fd.TotalSize - 1

	} else {
		fd.DownloadTaskList = append(fd.DownloadTaskList, &DownloadTask{
			taskID:         0,
			rangeStart:     0,
			rangeEnd:       0,
			downloadedSize: 0,
			isCompleted:    false,
		})
	}
}

func (fd *FileDownloader) startDownload() {
	waitGroup := &sync.WaitGroup{}
	progressChan := make(chan int64)
	for _, task := range fd.DownloadTaskList {
		go fd.startDownloadTask(waitGroup, progressChan, task)
		waitGroup.Add(1)
	}
	go func() {
		waitGroup.Wait()
		close(progressChan)
	}()

	if fd.progressCallback != nil {
		totalDownloaded := int64(0)
		for progress := range progressChan {
			totalDownloaded += progress
			fd.progressCallback(float64(totalDownloaded) * 100 / float64(fd.TotalSize))
		}
	}
}

func (fd *FileDownloader) startDownloadTask(waitGroup *sync.WaitGroup, progressChan chan int64, task *DownloadTask) {
	defer waitGroup.Done()
	request, err := http.NewRequest("GET", fd.Url, nil)
	if err != nil {
		globalLogger.Error().Stack().Err(err).Msgf("任务%d创建请求出错", task.taskID)
		return
	}

	fd.setHeaders(request)

	if fd.IsMultiPart {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", task.rangeStart, task.rangeEnd))
	}

	resp, err := fd.buildClient().Do(request)
	if err != nil {
		log.Printf("任务%d发送下载请求出错！%s", task.taskID, err)
		return
	}
	defer resp.Body.Close()
	buf := make([]byte, 8192)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, err := fd.File.WriteAt(buf[:n], task.rangeStart+task.downloadedSize)
			if err != nil {
				log.Printf("任务%d写入文件时出现错误！位置:%d, err: %s\n", task.taskID, task.rangeStart+task.downloadedSize, err)
				return
			}
			downSize := int64(n)
			task.downloadedSize += downSize
			progressChan <- downSize
		}
		if err != nil {
			if err == io.EOF {
				task.isCompleted = true
				break
			}
			log.Printf("任务%d读取响应错误！%s", task.taskID, err)
			return
		}
	}
}

func (fd *FileDownloader) Start() error {
	err := fd.init()
	if err != nil {
		return err
	}
	fd.createDownloadTasks()
	fd.startDownload()
	defer fd.File.Close()
	return nil
}
