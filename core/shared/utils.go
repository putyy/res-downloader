package shared

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/net/publicsuffix"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	sysRuntime "runtime"
	"strings"
	"time"
)

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
	u, err := url.Parse(rawURL)
	if err == nil && u.Host != "" {
		rawURL = u.Host
	}
	domain, err := publicsuffix.EffectiveTLDPlusOne(rawURL)
	if err != nil {
		return rawURL
	}
	return domain
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

func IsDevelopment() bool {
	return os.Getenv("APP_ENV") == "development"
}

func GetFileNameFromURL(rawUrl string) string {
	parsedURL, err := url.Parse(rawUrl)
	if err != nil {
		return ""
	}

	fileName := path.Base(parsedURL.Path)
	if fileName == "" || fileName == "/" {
		return ""
	}

	if decoded, err := url.QueryUnescape(fileName); err == nil {
		fileName = decoded
	}

	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	fileName = re.ReplaceAllString(fileName, "_")

	fileName = strings.TrimRightFunc(fileName, func(r rune) bool {
		return r == '.' || r == ' '
	})

	const maxFileNameLen = 255
	runes := []rune(fileName)
	if len(runes) > maxFileNameLen {
		ext := path.Ext(fileName)
		name := strings.TrimSuffix(fileName, ext)

		runes = []rune(name)
		if len(runes) > maxFileNameLen-len(ext) {
			runes = runes[:maxFileNameLen-len(ext)]
		}
		name = string(runes)
		fileName = name + ext
	}

	return fileName
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

func GetUniqueFileName(filePath string) string {
	if !FileExist(filePath) {
		return filePath
	}

	ext := filepath.Ext(filePath)
	baseName := strings.TrimSuffix(filePath, ext)
	count := 1

	for {
		newFileName := fmt.Sprintf("%s(%d)%s", baseName, count, ext)
		if !FileExist(newFileName) {
			return newFileName
		}
		count++
	}
}

func OpenFolder(filePath string) error {
	var cmd *exec.Cmd

	switch sysRuntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", filePath)
	case "windows":
		cmd = exec.Command("explorer", "/select,", filePath)
	case "linux":
		cmd = exec.Command("nautilus", filePath)
		if err := cmd.Start(); err != nil {
			cmd = exec.Command("thunar", filePath)
			if err := cmd.Start(); err != nil {
				cmd = exec.Command("dolphin", filePath)
				if err := cmd.Start(); err != nil {
					cmd = exec.Command("pcmanfm", filePath)
					if err := cmd.Start(); err != nil {
						return err
					}
				}
			}
		}
	default:
		return errors.New("unsupported platform")
	}

	return cmd.Start()
}
