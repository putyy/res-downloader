package core

import (
	"encoding/json"
	"runtime"
	"strconv"
	"strings"
)

// Config struct
type Config struct {
	storage       *Storage
	Theme         string `json:"Theme"`
	Host          string `json:"Host"`
	Port          string `json:"Port"`
	Quality       int    `json:"Quality"`
	SaveDirectory string `json:"SaveDirectory"`
	FilenameLen   int    `json:"FilenameLen"`
	FilenameTime  bool   `json:"FilenameTime"`
	UpstreamProxy string `json:"UpstreamProxy"`
	OpenProxy     bool   `json:"OpenProxy"`
	DownloadProxy bool   `json:"DownloadProxy"`
	AutoProxy     bool   `json:"AutoProxy"`
	WxAction      bool   `json:"WxAction"`
	TaskNumber    int    `json:"TaskNumber"`
	UserAgent     string `json:"UserAgent"`
}

func initConfig() *Config {
	if globalConfig == nil {
		def := `
{
  "Host": "127.0.0.1",
  "Port": "8899",
  "Theme": "lightTheme",
  "Quality": 0,
  "SaveDirectory": "",
  "FilenameLen": 0,
  "FilenameTime": true,
  "UpstreamProxy": "",
  "OpenProxy": false,
  "DownloadProxy": false,
  "AutoProxy": true,
  "WxAction": true,
  "TaskNumber": __TaskNumber__,
  "UserAgent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36"
}
`
		def = strings.ReplaceAll(def, "__TaskNumber__", strconv.Itoa(runtime.NumCPU()*2))
		globalConfig = &Config{
			storage: NewStorage("config.json", []byte(def)),
		}

		data, err := globalConfig.storage.Load()
		if err == nil {
			_ = json.Unmarshal(data, &globalConfig)
		} else {
			globalLogger.Esg(err, "load config err")
		}
	}
	return globalConfig
}

func (c *Config) setConfig(config Config) {
	oldProxy := c.UpstreamProxy
	c.Host = config.Host
	c.Port = config.Port
	c.Theme = config.Theme
	c.Quality = config.Quality
	c.SaveDirectory = config.SaveDirectory
	c.FilenameLen = config.FilenameLen
	c.FilenameTime = config.FilenameTime
	c.UpstreamProxy = config.UpstreamProxy
	c.UserAgent = config.UserAgent
	c.OpenProxy = config.OpenProxy
	c.DownloadProxy = config.DownloadProxy
	c.AutoProxy = config.AutoProxy
	c.TaskNumber = config.TaskNumber
	c.WxAction = config.WxAction
	if oldProxy != c.UpstreamProxy {
		proxyOnce.setTransport()
	}
	jsonData, err := json.Marshal(c)
	if err == nil {
		_ = globalConfig.storage.Store(jsonData)
	}
}
