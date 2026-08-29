package download

import (
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
	"res-downloader/internal/media"
)

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strconv"
	"sync"
	"time"
)

type cancellableDownload interface {
	Cancel()
}

type Processor interface {
	ProcessDownload(path string, processors []shared.DownloadStep, initialOffset uint64, reportProgress bool) error
}

type CaptureSource interface {
	CopyComplete(context.Context, string, string, func(int64, int64)) error
}

type PlanRunner struct {
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex
	downloaders  map[string]*FileDownloader
	progress     func(downloaded, total float64)
	partial      map[string]float64
	totals       map[string]float64
	processor    Processor
	captures     CaptureSource
	config       *config.Config
	logger       *logging.Logger
	media        *media.Engine
	workDir      string
	onProcessing func()
}

func (r *PlanRunner) SetCaptureSource(captures CaptureSource) { r.captures = captures }

func (r *PlanRunner) OnProcessing(callback func()) {
	r.onProcessing = callback
}

func NewPlanRunner(progress func(downloaded, total float64), cfg *config.Config, logger *logging.Logger, mediaEngine *media.Engine, processor Processor) *PlanRunner {
	return NewPlanRunnerContext(context.Background(), progress, cfg, logger, mediaEngine, processor, "")
}

func NewPlanRunnerContext(parent context.Context, progress func(downloaded, total float64), cfg *config.Config, logger *logging.Logger, mediaEngine *media.Engine, processor Processor, workDir string) *PlanRunner {
	ctx, cancel := context.WithCancel(parent)
	runner := &PlanRunner{
		ctx: ctx, cancel: cancel, downloaders: make(map[string]*FileDownloader),
		progress: progress, partial: make(map[string]float64), totals: make(map[string]float64),
		config: cfg, logger: logger, media: mediaEngine, processor: processor, workDir: workDir,
	}
	return runner
}

func (r *PlanRunner) Cancel() {
	r.cancel()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, downloader := range r.downloaders {
		downloader.Cancel()
	}
}

func (r *PlanRunner) Run(plan shared.DownloadPlan, directory string, taskNumber int) (string, error) {
	if r.workDir != "" {
		if err := os.MkdirAll(r.workDir, 0700); err != nil {
			return "", fmt.Errorf("create task workspace: %w", err)
		}
	}
	paths := make(map[string]string, len(plan.Inputs)+len(plan.Pipeline))
	cleanup := make(map[string]struct{}, len(plan.Inputs)+len(plan.Pipeline))
	succeeded := false
	defer func() {
		if r.workDir != "" && !succeeded {
			return
		}
		for path := range cleanup {
			_ = os.Remove(path)
		}
	}()

	type inputResult struct {
		id   string
		path string
		err  error
	}
	results := make(chan inputResult, len(plan.Inputs))
	var wait sync.WaitGroup
	for _, input := range plan.Inputs {
		input := input
		wait.Add(1)
		go func() {
			defer wait.Done()
			path, err := r.acquire(input, directory, taskNumber)
			results <- inputResult{id: input.ID, path: path, err: err}
		}()
	}
	go func() {
		wait.Wait()
		close(results)
	}()
	var firstErr error
	for result := range results {
		if result.err != nil && firstErr == nil {
			firstErr = fmt.Errorf("acquire input %q: %w", result.id, result.err)
			r.Cancel()
		}
		if result.path != "" {
			paths[result.id] = result.path
			cleanup[result.path] = struct{}{}
		}
	}
	if firstErr != nil {
		return "", firstErr
	}

	needsProcessing := len(plan.Pipeline) > 0 || len(plan.Output.Processors) > 0
	for _, input := range plan.Inputs {
		if len(input.Processors) > 0 {
			needsProcessing = true
			break
		}
	}
	if needsProcessing && r.onProcessing != nil {
		r.onProcessing()
	}
	if err := r.ctx.Err(); err != nil && !downloadPlanIsRecording(plan) {
		return "", context.Cause(r.ctx)
	}
	for _, input := range plan.Inputs {
		if len(input.Processors) == 0 {
			continue
		}
		if r.processor == nil {
			return "", errors.New("resource processor service is unavailable")
		}
		if err := r.processor.ProcessDownload(paths[input.ID], input.Processors, 0, false); err != nil {
			return "", fmt.Errorf("process input %q: %w", input.ID, err)
		}
		if err := r.ctx.Err(); err != nil {
			return "", context.Cause(r.ctx)
		}
	}

	for _, step := range plan.Pipeline {
		if err := r.ctx.Err(); err != nil {
			return "", err
		}
		inputPaths := make([]string, 0, len(step.Inputs))
		for _, inputID := range step.Inputs {
			inputPaths = append(inputPaths, paths[inputID])
		}
		var outputPath string
		var err error
		switch step.Executor {
		case "builtin.concat":
			outputPath, err = concatDownloadInputs(directory, inputPaths)
		case "builtin.media.mux", "builtin.media.remux", "builtin.media.extract_audio", "plugin.ffmpeg":
			outputPath, err = r.media.RunPipeline(r.ctx, step.Executor, directory, inputPaths, step.Options)
		default:
			err = fmt.Errorf("unsupported pipeline executor %q", step.Executor)
		}
		if err != nil {
			return "", fmt.Errorf("pipeline step %q: %w", step.ID, err)
		}
		paths[step.ID] = outputPath
		cleanup[outputPath] = struct{}{}
	}

	outputPath := paths[plan.Output.Input]
	if outputPath == "" {
		return "", errors.New("download plan produced no output")
	}
	if len(plan.Output.Processors) > 0 {
		if r.processor == nil {
			return "", errors.New("resource processor service is unavailable")
		}
		if err := r.processor.ProcessDownload(outputPath, plan.Output.Processors, 0, false); err != nil {
			return "", fmt.Errorf("process output: %w", err)
		}
		if err := r.ctx.Err(); err != nil {
			return "", context.Cause(r.ctx)
		}
	}
	delete(cleanup, outputPath)
	succeeded = true
	return outputPath, nil
}

func downloadPlanIsRecording(plan shared.DownloadPlan) bool {
	for _, input := range plan.Inputs {
		if input.Executor == "ffmpeg-hls" {
			return true
		}
	}
	return false
}

func (r *PlanRunner) acquire(input shared.DownloadInput, directory string, taskNumber int) (string, error) {
	if err := r.ctx.Err(); err != nil {
		return "", err
	}
	switch input.Executor {
	case "http-file":
		return r.acquireHTTP(input, directory, taskNumber)
	case "capture-file":
		return r.acquireCapture(input, directory)
	case "hls":
		return r.acquireHLS(input, directory, taskNumber)
	case "ffmpeg-hls":
		return r.acquireFFmpegHLS(input, directory)
	default:
		return "", fmt.Errorf("unsupported acquisition executor %q", input.Executor)
	}
}

func (r *PlanRunner) acquireCapture(input shared.DownloadInput, directory string) (string, error) {
	if r.captures == nil {
		return "", errors.New("response capture service is unavailable")
	}
	var path string
	if r.workDir != "" {
		digest := sha256.Sum256([]byte(input.ID))
		path = filepath.Join(r.workDir, fmt.Sprintf("capture-%x.part", digest[:8]))
	} else {
		temporary, err := os.CreateTemp(directory, ".res-downloader-capture-*")
		if err != nil {
			return "", err
		}
		path = temporary.Name()
		if err := temporary.Close(); err != nil {
			_ = os.Remove(path)
			return "", err
		}
	}
	if err := r.captures.CopyComplete(r.ctx, input.CaptureKey, path, func(downloaded, total int64) {
		r.reportProgress(input.ID, float64(downloaded), float64(total))
	}); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func (r *PlanRunner) acquireFFmpegHLS(input shared.DownloadInput, directory string) (string, error) {
	extension := input.Extension
	if extension == "" {
		extension = ".mp4"
	}
	output, err := os.CreateTemp(directory, ".res-downloader-recording-*"+extension)
	if err != nil {
		return "", err
	}
	path := output.Name()
	_ = output.Close()
	_ = os.Remove(path)
	args := make([]string, 0, 20)
	if headers := media.HeaderArgument(input.Headers); headers != "" {
		args = append(args, "-headers", headers)
	}
	if reconnect, _ := input.Options["reconnect"].(bool); reconnect {
		args = append(args, "-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5")
	}
	args = append(args, "-i", input.URL)
	if seconds, ok := numericOption(input.Options["maxDurationSeconds"]); ok && seconds > 0 {
		args = append(args, "-t", strconv.FormatInt(seconds, 10))
	}
	args = append(args, "-c", "copy", path)
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ticker.C:
				if info, statErr := os.Stat(path); statErr == nil {
					r.reportProgress(input.ID, float64(info.Size()), -1)
				}
			}
		}
	}()
	err = r.media.RunFFmpeg(r.ctx, args)
	close(progressDone)
	if err != nil {
		if r.ctx.Err() != nil {
			if info, statErr := os.Stat(path); statErr == nil && info.Size() > 0 {
				return path, nil
			}
		}
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func numericOption(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	default:
		return 0, false
	}
}

func (r *PlanRunner) acquireHTTP(input shared.DownloadInput, directory string, taskNumber int) (string, error) {
	path := ""
	checkpointPath := ""
	if r.workDir != "" {
		digest := sha256.Sum256([]byte(input.ID))
		path = filepath.Join(r.workDir, fmt.Sprintf("input-%x.part", digest[:8]))
		checkpointPath = path + ".json"
	} else {
		temp, err := os.CreateTemp(directory, ".res-downloader-input-*")
		if err != nil {
			return "", err
		}
		path = temp.Name()
		if err := temp.Close(); err != nil {
			_ = os.Remove(path)
			return "", err
		}
		if err := os.Remove(path); err != nil {
			return "", err
		}
	}
	downloader := NewFileDownloaderContext(r.ctx, input.URL, path, checkpointPath, taskNumber, input.Headers, r.config, r.logger)
	downloader.progressCallback = func(downloaded, total float64, _ int, _ float64) {
		r.reportProgress(input.ID, downloaded, total)
	}
	r.mu.Lock()
	r.downloaders[input.ID] = downloader
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.downloaders, input.ID)
		r.mu.Unlock()
	}()
	if err := downloader.Start(); err != nil {
		if r.workDir == "" {
			_ = os.Remove(downloader.FileName)
		}
		return "", err
	}
	return downloader.FileName, nil
}

func (r *PlanRunner) reportProgress(id string, downloaded, total float64) {
	r.mu.Lock()
	r.partial[id] = downloaded
	r.totals[id] = total
	var totalDownloaded, totalSize float64
	allKnown := true
	for inputID, value := range r.partial {
		totalDownloaded += value
		if r.totals[inputID] <= 0 {
			allKnown = false
		}
	}
	for _, value := range r.totals {
		if value <= 0 {
			allKnown = false
			continue
		}
		totalSize += value
	}
	callback := r.progress
	r.mu.Unlock()
	if callback != nil {
		if !allKnown {
			totalSize = -1
		}
		callback(totalDownloaded, totalSize)
	}
}

func concatDownloadInputs(directory string, paths []string) (string, error) {
	output, err := os.CreateTemp(directory, ".res-downloader-concat-*")
	if err != nil {
		return "", err
	}
	outputPath := output.Name()
	succeeded := false
	defer func() {
		_ = output.Close()
		if !succeeded {
			_ = os.Remove(outputPath)
		}
	}()
	for _, path := range paths {
		input, err := os.Open(filepath.Clean(path))
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := input.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	if err := output.Sync(); err != nil {
		return "", err
	}
	if err := output.Close(); err != nil {
		return "", err
	}
	succeeded = true
	return outputPath, nil
}
