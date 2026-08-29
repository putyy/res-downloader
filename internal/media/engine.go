package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	maxFFmpegArguments      = 256
	maxFFmpegArgumentLength = 16 * 1024
	maxFFmpegLogBytes       = 512 * 1024
)

type MediaToolStatus struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

type MediaEngineStatus struct {
	FFmpeg    MediaToolStatus `json:"ffmpeg"`
	FFprobe   MediaToolStatus `json:"ffprobe"`
	CheckedAt int64           `json:"checkedAt"`
}

type Engine struct {
	mu     sync.RWMutex
	status MediaEngineStatus
	paths  Paths
}

type Paths func() (ffmpeg string, ffprobe string)

func New(paths Paths) *Engine {
	return &Engine{paths: paths}
}

func (e *Engine) Detect() MediaEngineStatus {
	ffmpegPath, ffprobePath := "", ""
	if e.paths != nil {
		ffmpegPath, ffprobePath = e.paths()
	}
	status := MediaEngineStatus{
		FFmpeg:    detectMediaTool(ffmpegPath, "ffmpeg"),
		FFprobe:   detectMediaTool(ffprobePath, "ffprobe"),
		CheckedAt: time.Now().UnixMilli(),
	}
	e.mu.Lock()
	e.status = status
	e.mu.Unlock()
	return status
}

func detectMediaTool(configured, fallback string) MediaToolStatus {
	path := strings.TrimSpace(configured)
	if path == "" {
		resolved, err := exec.LookPath(fallback)
		if err != nil {
			resolved, err = platformLookPath(fallback)
			if err != nil {
				return MediaToolStatus{Error: fallback + " was not found in PATH"}
			}
		}
		path = resolved
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return MediaToolStatus{Path: path, Error: err.Error()}
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return MediaToolStatus{Path: absolute, Error: "executable file does not exist"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, absolute, "-version").Output()
	if err != nil {
		return MediaToolStatus{Path: absolute, Error: err.Error()}
	}
	firstLine := strings.SplitN(string(output), "\n", 2)[0]
	return MediaToolStatus{Available: true, Path: absolute, Version: strings.TrimSpace(firstLine)}
}

func (e *Engine) ffmpegPath() (string, error) {
	e.mu.RLock()
	status := e.status
	e.mu.RUnlock()
	if status.CheckedAt == 0 {
		status = e.Detect()
	}
	if !status.FFmpeg.Available {
		return "", errors.New("FFmpeg is unavailable: " + status.FFmpeg.Error)
	}
	return status.FFmpeg.Path, nil
}

func (e *Engine) SatisfiesFFmpeg(requirement string) bool {
	if requirement == "" {
		return true
	}
	e.mu.RLock()
	status := e.status
	e.mu.RUnlock()
	if status.CheckedAt == 0 {
		status = e.Detect()
	}
	if !status.FFmpeg.Available {
		return false
	}
	wantText := strings.TrimSpace(strings.TrimPrefix(requirement, ">="))
	wantParts := strings.Split(wantText, ".")
	versionText := regexp.MustCompile(`[0-9]+(?:\.[0-9]+){1,2}`).FindString(status.FFmpeg.Version)
	if versionText == "" {
		return false
	}
	actualParts := strings.Split(versionText, ".")
	for index := 0; index < len(wantParts); index++ {
		want, _ := strconv.Atoi(wantParts[index])
		actual := 0
		if index < len(actualParts) {
			actual, _ = strconv.Atoi(actualParts[index])
		}
		if actual > want {
			return true
		}
		if actual < want {
			return false
		}
	}
	return true
}

func (e *Engine) RunFFmpeg(ctx context.Context, args []string) error {
	path, err := e.ffmpegPath()
	if err != nil {
		return err
	}
	if len(args) == 0 || len(args) > maxFFmpegArguments {
		return fmt.Errorf("FFmpeg argument count must be between 1 and %d", maxFFmpegArguments)
	}
	for _, argument := range args {
		if len(argument) > maxFFmpegArgumentLength || strings.ContainsRune(argument, 0) {
			return errors.New("FFmpeg argument is invalid or too long")
		}
	}
	commandArgs := append([]string{"-nostdin", "-hide_banner", "-y"}, args...)
	command := exec.Command(path, commandArgs...)
	logs := &boundedBuffer{limit: maxFFmpegLogBytes}
	command.Stdout, command.Stderr = logs, logs
	if err := command.Start(); err != nil {
		return fmt.Errorf("start FFmpeg: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var runErr error
	select {
	case runErr = <-done:
	case <-ctx.Done():
		// SIGINT gives FFmpeg a chance to write container trailers. Platforms
		// that do not support Interrupt fall back to terminating the process.
		if signalErr := command.Process.Signal(os.Interrupt); signalErr != nil {
			_ = command.Process.Kill()
			runErr = <-done
			break
		}
		select {
		case runErr = <-done:
		case <-time.After(5 * time.Second):
			_ = command.Process.Kill()
			runErr = <-done
		}
		if runErr == nil {
			runErr = ctx.Err()
		}
	}
	if runErr != nil {
		return fmt.Errorf("FFmpeg failed: %w: %s", runErr, strings.TrimSpace(logs.String()))
	}
	return nil
}

type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	original := len(value)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(value) > remaining {
			value = value[:remaining]
		}
		_, _ = b.buffer.Write(value)
	}
	return original, nil
}

func (b *boundedBuffer) String() string { return b.buffer.String() }

func (e *Engine) RunPipeline(ctx context.Context, executor, directory string, inputs []string, options map[string]interface{}) (string, error) {
	extension, _ := options["extension"].(string)
	if extension == "" {
		extension = ".mp4"
	}
	if !validExtension(extension) || extension == "" {
		return "", errors.New("media output extension is invalid")
	}
	output, err := os.CreateTemp(directory, ".res-downloader-media-*"+extension)
	if err != nil {
		return "", err
	}
	outputPath := output.Name()
	_ = output.Close()
	_ = os.Remove(outputPath)
	args := make([]string, 0)
	switch executor {
	case "builtin.media.mux", "builtin.media.remux":
		args = mediaMuxArguments(inputs, outputPath)
	case "builtin.media.extract_audio":
		if len(inputs) != 1 {
			return "", errors.New("extract_audio requires one input")
		}
		args = []string{"-i", inputs[0], "-vn", "-c:a", "copy", outputPath}
	case "plugin.ffmpeg":
		args = stringSliceValue(options["args"])
		if len(args) == 0 {
			return "", errors.New("plugin.ffmpeg requires an args array")
		}
		if err := validatePluginFFmpegArguments(args, len(inputs)); err != nil {
			return "", err
		}
		for index, argument := range args {
			argument = strings.ReplaceAll(argument, "{{output}}", outputPath)
			for inputIndex, input := range inputs {
				argument = strings.ReplaceAll(argument, "{{input."+strconv.Itoa(inputIndex)+"}}", input)
			}
			if regexp.MustCompile(`\{\{[^}]+\}\}`).MatchString(argument) {
				return "", fmt.Errorf("unsupported FFmpeg placeholder in argument %q", argument)
			}
			args[index] = argument
		}
	default:
		return "", fmt.Errorf("unsupported media executor %q", executor)
	}
	if err := e.RunFFmpeg(ctx, args); err != nil {
		_ = os.Remove(outputPath)
		return "", err
	}
	return outputPath, nil
}

func mediaMuxArguments(inputs []string, outputPath string) []string {
	if len(inputs) == 1 {
		return []string{"-i", inputs[0], "-c", "copy", outputPath}
	}
	args := make([]string, 0, len(inputs)*2+9)
	for _, input := range inputs {
		args = append(args, "-i", input)
	}
	return append(args, "-map", "0:v:0?", "-map", "1:a:0?", "-c", "copy", outputPath)
}

func validatePluginFFmpegArguments(args []string, inputCount int) error {
	if len(args) == 0 || args[len(args)-1] != "{{output}}" {
		return errors.New("plugin.ffmpeg must use {{output}} as its final argument")
	}
	denied := map[string]bool{
		"-filter_script": true, "-filter_complex_script": true, "-report": true,
		"-progress": true, "-passlogfile": true, "-vstats_file": true,
		"-dump_attachment": true, "-attach": true,
	}
	for index, argument := range args {
		if denied[argument] || strings.HasSuffix(argument, "_script") {
			return fmt.Errorf("FFmpeg option %q is not available to plugins", argument)
		}
		if argument == "-i" {
			if index+1 >= len(args) || !validFFmpegInputPlaceholder(args[index+1], inputCount) {
				return errors.New("every plugin.ffmpeg -i value must be a host input placeholder")
			}
			continue
		}
		lower := strings.ToLower(argument)
		if filepath.IsAbs(argument) || strings.Contains(argument, "../") || strings.Contains(argument, `..\`) ||
			strings.Contains(lower, "://") || strings.HasPrefix(lower, "file:") || strings.HasPrefix(lower, "concat:") ||
			strings.HasPrefix(lower, "subfile:") || strings.HasPrefix(lower, "pipe:") {
			return fmt.Errorf("FFmpeg argument %q references an unmanaged input or output", argument)
		}
		if strings.Contains(argument, "{{output}}") && argument != "{{output}}" {
			return errors.New("{{output}} must be used as a complete argument")
		}
	}
	return nil
}

func validFFmpegInputPlaceholder(value string, count int) bool {
	for index := 0; index < count; index++ {
		if value == "{{input."+strconv.Itoa(index)+"}}" {
			return true
		}
	}
	return false
}

func HeaderArgument(headers map[string]string) string {
	var value strings.Builder
	for key, header := range headers {
		value.WriteString(key)
		value.WriteString(": ")
		value.WriteString(strings.ReplaceAll(strings.ReplaceAll(header, "\r", ""), "\n", ""))
		value.WriteString("\r\n")
	}
	return value.String()
}

func validExtension(extension string) bool {
	if extension == "" {
		return true
	}
	if len(extension) > 20 || !strings.HasPrefix(extension, ".") {
		return false
	}
	for _, char := range extension[1:] {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '_' && char != '-' {
			return false
		}
	}
	return len(extension) > 1
}

func stringSliceValue(value interface{}) []string {
	switch values := value.(type) {
	case []string:
		return values
	case []interface{}:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
