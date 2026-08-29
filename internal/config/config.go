package config

import (
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"res-downloader/internal/logging"
	"res-downloader/internal/naming"
	"res-downloader/internal/rules"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config struct
type Config struct {
	storage              *Storage
	onApply              func(previous, current Config) error
	state                *configState
	Theme                string         `json:"Theme"`
	Locale               string         `json:"Locale"`
	Host                 string         `json:"Host"`
	Port                 string         `json:"Port"`
	SaveDirectory        string         `json:"SaveDirectory"`
	FilenameTemplate     string         `json:"FilenameTemplate"`
	FilenameConflict     string         `json:"FilenameConflict"`
	UpstreamProxy        string         `json:"UpstreamProxy"`
	OpenProxy            bool           `json:"OpenProxy"`
	DownloadProxy        bool           `json:"DownloadProxy"`
	FFmpegPath           string         `json:"FFmpegPath"`
	FFprobePath          string         `json:"FFprobePath"`
	AutoProxy            bool           `json:"AutoProxy"`
	TaskNumber           int            `json:"TaskNumber"`
	DownNumber           int            `json:"DownNumber"`
	UserAgent            string         `json:"UserAgent"`
	UseHeaders           string         `json:"UseHeaders"`
	InsertTail           bool           `json:"InsertTail"`
	InterceptionPolicies []rules.Policy `json:"InterceptionPolicies"`
}

type configState struct {
	mu      sync.RWMutex
	applyMu sync.Mutex
}

func New(userDir string, logger *logging.Logger) *Config {
	defaultConfig := &Config{
		state:            &configState{},
		Theme:            "lightTheme",
		Locale:           "zh",
		Host:             "127.0.0.1",
		Port:             "8899",
		SaveDirectory:    getDefaultDownloadDir(),
		FilenameTemplate: "{{title|default:resource|sanitize|truncate:80}}_{{date:20060102_150405}}.{{ext}}",
		FilenameConflict: "rename",
		UpstreamProxy:    "",
		OpenProxy:        false,
		DownloadProxy:    false,
		FFmpegPath:       "",
		FFprobePath:      "",
		AutoProxy:        false,
		TaskNumber:       min(runtime.NumCPU()*2, 64),
		DownNumber:       3,
		UserAgent:        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
		UseHeaders:       "default",
		InsertTail:       true,
		InterceptionPolicies: []rules.Policy{{
			ID: "default", Name: "Default", Enabled: true, Domains: []string{"*"}, Action: rules.ActionMITM,
		}},
	}

	rawDefaults, err := json.Marshal(defaultConfig)
	if err != nil {
		return defaultConfig
	}

	storage := NewStorage(userDir, "config.json", rawDefaults)
	defaultConfig.storage = storage
	data, err := storage.Load()
	if err != nil {
		logger.Esg(err, "load config failed, using defaults")
		return defaultConfig
	}

	var cacheMap map[string]interface{}
	if err := json.Unmarshal(data, &cacheMap); err != nil {
		logger.Esg(err, "parse cached config failed, using defaults")
		return defaultConfig
	}

	var defaultMap map[string]interface{}
	defaultBytes, _ := json.Marshal(defaultConfig)
	_ = json.Unmarshal(defaultBytes, &defaultMap)

	for k, v := range cacheMap {
		if _, ok := defaultMap[k]; ok {
			defaultMap[k] = v
		}
	}

	finalBytes, err := json.Marshal(defaultMap)
	if err != nil {
		logger.Esg(err, "marshal merged config failed")
		return defaultConfig
	}

	if err := json.Unmarshal(finalBytes, defaultConfig); err != nil {
		logger.Esg(err, "unmarshal merged config to struct failed")
	}

	return defaultConfig
}

func getDefaultDownloadDir() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}

	homeDir := usr.HomeDir
	var downloadDir string

	switch runtime.GOOS {
	case "windows", "darwin":
		downloadDir = filepath.Join(homeDir, "Downloads")
	case "linux":
		downloadDir = filepath.Join(homeDir, "Downloads")
		if xdgDir := os.Getenv("XDG_DOWNLOAD_DIR"); xdgDir != "" {
			downloadDir = xdgDir
		}
	}

	if stat, err := os.Stat(downloadDir); err == nil && stat.IsDir() {
		return downloadDir
	}

	return ""
}

func (c *Config) Apply(config Config) error {
	if c.state == nil {
		c.state = &configState{}
	}
	c.state.applyMu.Lock()
	defer c.state.applyMu.Unlock()
	config = config.Snapshot()
	config.FFmpegPath = strings.TrimSpace(config.FFmpegPath)
	config.FFprobePath = strings.TrimSpace(config.FFprobePath)
	config.Host = strings.TrimSpace(config.Host)
	config.Port = strings.TrimSpace(config.Port)
	config.UpstreamProxy = strings.TrimSpace(config.UpstreamProxy)
	if err := validateConfig(config); err != nil {
		return err
	}
	if _, err := naming.ExpandFilenameTemplate(config.FilenameTemplate, map[string]string{}, nil, time.Now()); err != nil {
		return err
	}
	if _, err := naming.ResolveFilenameConflict(filepath.Join(os.TempDir(), "res-downloader-config-check"), config.FilenameConflict); err != nil {
		return err
	}
	if err := rules.Validate(config.InterceptionPolicies); err != nil {
		return err
	}
	previous := c.Snapshot()
	c.replace(config)
	c.state.mu.RLock()
	hook := c.onApply
	storage := c.storage
	c.state.mu.RUnlock()
	rollback := func() {
		c.replace(previous)
		if hook != nil {
			_ = hook(config, previous)
		}
	}
	if hook != nil {
		if err := hook(previous, config); err != nil {
			rollback()
			return err
		}
	}
	jsonData, err := json.Marshal(config)
	if err != nil {
		rollback()
		return err
	}
	if storage == nil {
		rollback()
		return errors.New("config storage is unavailable")
	}
	if err := storage.Store(jsonData); err != nil {
		rollback()
		return err
	}
	return nil
}

func validateConfig(value Config) error {
	if value.Host == "" || strings.ContainsAny(value.Host, "/\\?#@ \t\r\n") {
		return errors.New("invalid listen host")
	}
	if net.ParseIP(strings.Trim(value.Host, "[]")) == nil {
		for _, label := range strings.Split(value.Host, ".") {
			if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return errors.New("invalid listen host")
			}
			for _, char := range label {
				if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' {
					return errors.New("invalid listen host")
				}
			}
		}
	}
	port, err := strconv.Atoi(value.Port)
	if err != nil || port <= 1024 || port >= 65535 {
		return errors.New("listen port must be between 1025 and 65534")
	}
	if value.TaskNumber < 2 || value.TaskNumber > 64 {
		return errors.New("download connections must be between 2 and 64")
	}
	if value.DownNumber < 1 || value.DownNumber > 10 {
		return errors.New("concurrent downloads must be between 1 and 10")
	}
	if value.UpstreamProxy != "" {
		parsed, err := url.Parse(value.UpstreamProxy)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return errors.New("upstream proxy must be a valid HTTP or HTTPS URL")
		}
	}
	return nil
}

func (c *Config) Snapshot() Config {
	if c == nil {
		return Config{}
	}
	if c.state != nil {
		c.state.mu.RLock()
		defer c.state.mu.RUnlock()
	}
	return Config{
		Theme: c.Theme, Locale: c.Locale, Host: c.Host, Port: c.Port, SaveDirectory: c.SaveDirectory,
		FilenameTemplate: c.FilenameTemplate, FilenameConflict: c.FilenameConflict,
		UpstreamProxy: c.UpstreamProxy, OpenProxy: c.OpenProxy, DownloadProxy: c.DownloadProxy,
		FFmpegPath: c.FFmpegPath, FFprobePath: c.FFprobePath, AutoProxy: c.AutoProxy,
		TaskNumber: c.TaskNumber, DownNumber: c.DownNumber, UserAgent: c.UserAgent,
		UseHeaders: c.UseHeaders, InsertTail: c.InsertTail,
		InterceptionPolicies: rules.Clone(c.InterceptionPolicies),
	}
}

func (c *Config) replace(value Config) {
	if c.state != nil {
		c.state.mu.Lock()
		defer c.state.mu.Unlock()
	}
	c.Theme, c.Locale, c.Host, c.Port = value.Theme, value.Locale, value.Host, value.Port
	c.SaveDirectory, c.FilenameTemplate, c.FilenameConflict = value.SaveDirectory, value.FilenameTemplate, value.FilenameConflict
	c.UpstreamProxy, c.OpenProxy, c.DownloadProxy = value.UpstreamProxy, value.OpenProxy, value.DownloadProxy
	c.FFmpegPath, c.FFprobePath, c.AutoProxy = value.FFmpegPath, value.FFprobePath, value.AutoProxy
	c.TaskNumber, c.DownNumber, c.UserAgent = value.TaskNumber, value.DownNumber, value.UserAgent
	c.UseHeaders, c.InsertTail = value.UseHeaders, value.InsertTail
	c.InterceptionPolicies = rules.Clone(value.InterceptionPolicies)
}

func (c *Config) MarshalJSON() ([]byte, error) {
	type configJSON Config
	snapshot := c.Snapshot()
	return json.Marshal(configJSON(snapshot))
}

func (c *Config) Get(key string) interface{} {
	snapshot := c.Snapshot()
	switch key {
	case "Host":
		return snapshot.Host
	case "Port":
		return snapshot.Port
	case "Theme":
		return snapshot.Theme
	case "Locale":
		return snapshot.Locale
	case "SaveDirectory":
		return snapshot.SaveDirectory
	case "FilenameTemplate":
		return snapshot.FilenameTemplate
	case "FilenameConflict":
		return snapshot.FilenameConflict
	case "UpstreamProxy":
		return snapshot.UpstreamProxy
	case "UserAgent":
		return snapshot.UserAgent
	case "OpenProxy":
		return snapshot.OpenProxy
	case "DownloadProxy":
		return snapshot.DownloadProxy
	case "FFmpegPath":
		return snapshot.FFmpegPath
	case "FFprobePath":
		return snapshot.FFprobePath
	case "AutoProxy":
		return snapshot.AutoProxy
	case "TaskNumber":
		return snapshot.TaskNumber
	case "DownNumber":
		return snapshot.DownNumber
	case "UseHeaders":
		return snapshot.UseHeaders
	case "InsertTail":
		return snapshot.InsertTail
	case "InterceptionPolicies":
		return rules.Clone(snapshot.InterceptionPolicies)
	default:
		return nil
	}
}

func (c *Config) SetApplyHook(hook func(previous, current Config) error) {
	if c.state == nil {
		c.state = &configState{}
	}
	c.state.mu.Lock()
	c.onApply = hook
	c.state.mu.Unlock()
}
