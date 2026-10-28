package model

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

<<<<<<< Updated upstream
=======
// Md5 returns the lowercase hex MD5 digest of data.
>>>>>>> Stashed changes
func Md5(data string) string {
	hashNew := md5.New()
	hashNew.Write([]byte(data))
	hash := hashNew.Sum(nil)
	return hex.EncodeToString(hash)
}

<<<<<<< Updated upstream
=======
// FormatSize renders a byte count as a short human readable string such as
// "1.50MB". Values are compared against 1024 and 1048576 as floats.
>>>>>>> Stashed changes
func FormatSize(size float64) string {
	if size > 1048576 {
		return fmt.Sprintf("%.2fMB", float64(size)/1048576)
	}
	if size > 1024 {
		return fmt.Sprintf("%.2fKB", float64(size)/1024)
	}
	return fmt.Sprintf("%.0fb", size)
}

<<<<<<< Updated upstream
=======
// GetTopLevelDomain returns the registrable domain of rawURL (for example
// "video.qq.com" for a URL on "finder.video.qq.com"). When the input cannot be
// parsed or resolved, the original host or input string is returned unchanged.
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
=======
// FileExist reports whether path exists and is a regular file. Directories and
// stat errors both return false.
>>>>>>> Stashed changes
func FileExist(file string) bool {
	info, err := os.Stat(file)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

<<<<<<< Updated upstream
=======
// CreateDirIfNotExist creates dir with mode 0750 when it does not exist. An
// existing directory is left untouched and reported as success.
>>>>>>> Stashed changes
func CreateDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0750)
	}
	return nil
}

<<<<<<< Updated upstream
=======
// IsDevelopment reports whether APP_ENV is set to "development".
>>>>>>> Stashed changes
func IsDevelopment() bool {
	return os.Getenv("APP_ENV") == "development"
}

<<<<<<< Updated upstream
=======
// GetFileNameFromURL extracts a filesystem-safe file name from the last path
// segment of rawUrl. Reserved characters are replaced, trailing dots and spaces
// are trimmed, and long names are shortened while keeping the extension. It
// returns an empty string when the URL has no usable path segment.
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
=======
// GetCurrentDateTimeFormatted returns the local time as "YYYYMMDDHHMMSS",
// suitable for timestamped file names.
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
=======
// GetUniqueFileName returns filePath when it is free, otherwise appends a
// "(n)" counter before the extension until an unused name is found. The
// returned path is not created on disk.
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
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
=======
// OpenFolder reveals filePath in the platform file manager, selecting the file
// where the platform supports it. Linux tries several managers in order and
// returns the last launch error when none is available. The command is started
// asynchronously, so a nil error only means it launched.
func OpenFolder(filePath string) error {
	var commands [][]string
	switch sysRuntime.GOOS {
	case "darwin":
		commands = [][]string{{"open", "-R", filePath}}
	case "windows":
		commands = [][]string{{"explorer", "/select,", filePath}}
	case "linux":
		commands = [][]string{
			{"nautilus", filePath},
			{"thunar", filePath},
			{"dolphin", filePath},
			{"pcmanfm", filePath},
>>>>>>> Stashed changes
		}
	default:
		return errors.New("unsupported platform")
	}

<<<<<<< Updated upstream
	return cmd.Start()
}

=======
	// Launch exactly once: an earlier fallback may already have started.
	var lastErr error
	for _, command := range commands {
		if err := exec.Command(command[0], command[1:]...).Start(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// OpenDirectory opens directoryPath in the platform file manager. It returns an
// error when the path is missing or is not a directory, or when every candidate
// launcher on the current platform fails.
>>>>>>> Stashed changes
func OpenDirectory(directoryPath string) error {
	info, err := os.Stat(directoryPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", directoryPath)
	}

	var commands [][]string
	switch sysRuntime.GOOS {
	case "darwin":
		commands = [][]string{{"open", directoryPath}}
	case "windows":
		commands = [][]string{{"explorer", directoryPath}}
	case "linux":
		commands = [][]string{
			{"xdg-open", directoryPath},
			{"nautilus", directoryPath},
			{"thunar", directoryPath},
			{"dolphin", directoryPath},
			{"pcmanfm", directoryPath},
		}
	default:
		return errors.New("unsupported platform")
	}

	var lastErr error
	for _, command := range commands {
		if err := exec.Command(command[0], command[1:]...).Start(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}
