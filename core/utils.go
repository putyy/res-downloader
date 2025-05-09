package core

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"net/url"
	"os"
	"strings"
	"time"
)

func DialogErr(message string) {
	_, _ = runtime.MessageDialog(appOnce.ctx, runtime.MessageDialogOptions{
		Type:          runtime.ErrorDialog,
		Title:         "Error",
		Message:       message,
		DefaultButton: "Cancel",
	})
}

func IsDevelopment() bool {
	return os.Getenv("APP_ENV") == "development"
}

func FileExist(file string) bool {
	info, err := os.Stat(file)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func CreateDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0750)
	}
	return nil
}

func TypeSuffix(mime string) (string, string) {
	mimeMux.RLock()
	defer mimeMux.RUnlock()
	mime = strings.ToLower(strings.Split(mime, ";")[0])
	if v, ok := globalConfig.MimeMap[mime]; ok {
		return v.Type, v.Suffix
	}
	return "", ""
}

func Md5(data string) string {
	hashNew := md5.New()
	hashNew.Write([]byte(data))
	hash := hashNew.Sum(nil)
	return hex.EncodeToString(hash)
}

func FormatSize(size float64) string {
	if size > 1048576 {
		return fmt.Sprintf("%.2fMB", float64(size)/1048576)
	}
	if size > 1024 {
		return fmt.Sprintf("%.2fKB", float64(size)/1024)
	}
	return fmt.Sprintf("%.0fb", size)
}

func GetTopLevelDomain(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := parsedURL.Hostname()
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func GetCurrentDateTimeFormatted() string {
	now := time.Now()
	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d",
		now.Year(),
		now.Month(),
		now.Day(),
		now.Hour(),
		now.Minute(),
		now.Second())
}
