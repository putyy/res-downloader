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
)

// Config struct
type Config struct {
	storage              *Storage
	onApply              func(previous, current Config) error
	state                *configState
	Theme                string         `json:"Theme"`
	Locale               string         `json:"Locale"`
	WindowWidth          int            `json:"WindowWidth"`
	WindowHeight         int            `json:"WindowHeight"`
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

type ValidationError struct {
	Field string
	Err   error
}

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

type configState struct {
	mu      sync.RWMutex
	applyMu sync.Mutex
}

func New(userDir string, logger *logging.Logger, defaultLocale string) *Config {
	if defaultLocale != "zh" {
		defaultLocale = "en"
	}
	defaultConfig := &Config{
		state:            &configState{},
		Theme:            "lightTheme",
		Locale:           defaultLocale,
		WindowWidth:      DefaultWindowWidth,
		WindowHeight:     DefaultWindowHeight,
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
	previous := c.Snapshot()
	config = config.Snapshot()
	config.FFmpegPath = strings.TrimSpace(config.FFmpegPath)
	config.FFprobePath = strings.TrimSpace(config.FFprobePath)
	config.Host = strings.TrimSpace(config.Host)
	config.Port = strings.TrimSpace(config.Port)
	config.UpstreamProxy = strings.TrimSpace(config.UpstreamProxy)
	if err := validateConfig(config); err != nil {
		return err
	}
	if config.SaveDirectory != previous.SaveDirectory {
		if strings.TrimSpace(config.SaveDirectory) == "" || !filepath.IsAbs(config.SaveDirectory) {
			return &ValidationError{Field: "SaveDirectory", Err: errors.New("save directory must be an absolute folder")}
		}
		info, err := os.Stat(config.SaveDirectory)
		if err != nil || !info.IsDir() {
			return &ValidationError{Field: "SaveDirectory", Err: errors.New("save directory does not exist or is not a folder")}
		}
	}
	if err := naming.ValidateFilenameTemplate(config.FilenameTemplate); err != nil {
		return &ValidationError{Field: "FilenameTemplate", Err: err}
	}
	if config.FFmpegPath != previous.FFmpegPath {
		if err := validateMediaToolPath(config.FFmpegPath); err != nil {
			return &ValidationError{Field: "FFmpegPath", Err: err}
		}
	}
	if config.FFprobePath != previous.FFprobePath {
		if err := validateMediaToolPath(config.FFprobePath); err != nil {
			return &ValidationError{Field: "FFprobePath", Err: err}
		}
	}
	if _, err := naming.ResolveFilenameConflict(filepath.Join(os.TempDir(), "res-downloader-config-check"), config.FilenameConflict); err != nil {
		return err
	}
	if err := rules.Validate(config.InterceptionPolicies); err != nil {
		return err
	}
	// Window dimensions are maintained by the desktop lifecycle. Settings forms
	// may contain an older snapshot, so they must not overwrite the latest size.
	config.WindowWidth, config.WindowHeight = previous.WindowWidth, previous.WindowHeight
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
	if !validHost(value.Host) {
		return &ValidationError{Field: "Host", Err: errors.New("invalid listen host")}
	}
	port, err := strconv.Atoi(value.Port)
	if err != nil || port <= 1024 || port >= 65535 || !decimalDigits(value.Port) {
		return &ValidationError{Field: "Port", Err: errors.New("listen port must be between 1025 and 65534")}
	}
	if value.TaskNumber < 2 || value.TaskNumber > 64 {
		return errors.New("download connections must be between 2 and 64")
	}
	if value.DownNumber < 1 || value.DownNumber > 10 {
		return errors.New("concurrent downloads must be between 1 and 10")
	}
	if value.UpstreamProxy == "" && (value.OpenProxy || value.DownloadProxy) {
		return &ValidationError{Field: "UpstreamProxy", Err: errors.New("upstream proxy is required when proxy is enabled")}
	}
	if value.UpstreamProxy != "" && !validUpstreamProxy(value.UpstreamProxy) {
		return &ValidationError{Field: "UpstreamProxy", Err: errors.New("upstream proxy must be a valid HTTP or HTTPS URL")}
	}
	return nil
}

func validHost(host string) bool {
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "/\\?#@ \t\r\n") {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	labels := strings.Split(host, ".")
	if len(labels) == 4 {
		allNumeric := true
		for _, label := range labels {
			allNumeric = allNumeric && decimalDigits(label)
		}
		if allNumeric {
			return false
		}
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validUpstreamProxy(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Opaque != "" || !strings.HasPrefix(value, parsed.Scheme+"://") ||
		!validHost(parsed.Hostname()) || (parsed.Path != "" && parsed.Path != "/") ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		strings.HasSuffix(parsed.Host, ":") {
		return false
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		return err == nil && decimalDigits(port) && number >= 1 && number <= 65535
	}
	return true
}

func validateMediaToolPath(path string) error {
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return errors.New("media tool path must be absolute")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("media tool path must point to an existing file")
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
		WindowWidth: c.WindowWidth, WindowHeight: c.WindowHeight,
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
	c.WindowWidth, c.WindowHeight = value.WindowWidth, value.WindowHeight
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
