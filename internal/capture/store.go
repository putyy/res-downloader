package capture

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	captureCacheTTL = 24 * time.Hour
	maxCaptureSize  = int64(16 * 1024 * 1024 * 1024)
)

type byteRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

type metadata struct {
	Mode      string      `json:"mode,omitempty"`
	Total     int64       `json:"total"`
	Ranges    []byteRange `json:"ranges"`
	Complete  bool        `json:"complete,omitempty"`
	UpdatedAt int64       `json:"updatedAt"`
}

type entry struct {
	mu       sync.Mutex
	dir      string
	dataPath string
	metaPath string
	meta     metadata
	active   int
	changed  chan struct{}
}

type Store struct {
	root    string
	mu      sync.Mutex
	entries map[string]*entry
	closed  bool
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("capture cache directory is empty")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	store := &Store{root: root, entries: make(map[string]*entry)}
	store.pruneExpired()
	return store, nil
}

func (s *Store) Begin(key string, response *http.Response) (io.WriteCloser, error) {
	if s == nil || response == nil {
		return nil, errors.New("capture store is unavailable")
	}
	start, total, err := captureBounds(response)
	if err != nil {
		return nil, err
	}
	if total <= 0 || total > maxCaptureSize || start < 0 || start >= total {
		return nil, fmt.Errorf("capture size %d is outside the supported range", total)
	}
	item, err := s.getEntry(key)
	if err != nil {
		return nil, err
	}

	item.mu.Lock()
	defer item.mu.Unlock()
	if item.meta.Mode == "stream-file" {
		if item.active > 0 {
			return nil, errors.New("capture mode changed while another response is active")
		}
		item.meta = metadata{}
		if err := os.Truncate(item.dataPath, 0); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if item.meta.Total > 0 && item.meta.Total != total {
		if item.active > 0 {
			return nil, errors.New("capture size changed while another response is active")
		}
		item.meta = metadata{Total: total, UpdatedAt: time.Now().UnixMilli()}
		if err := os.Truncate(item.dataPath, 0); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if item.meta.Total == 0 {
		item.meta.Total = total
	}
	item.meta.Mode = "range-file"
	item.meta.Complete = false
	file, err := os.OpenFile(item.dataPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	item.active++
	item.signalLocked()
	return &rangeWriter{entry: item, file: file, start: start, offset: start, total: total}, nil
}

// StartStream resets a capture entry for page-originated media segments. The
// stream has no known final byte length until CompleteStream is called.
func (s *Store) StartStream(key string) error {
	if s == nil {
		return errors.New("capture store is unavailable")
	}
	item, err := s.getEntry(key)
	if err != nil {
		return err
	}
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.active > 0 {
		return errors.New("capture is active")
	}
	if err := os.Truncate(item.dataPath, 0); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	item.meta = metadata{Mode: "stream-file", UpdatedAt: time.Now().UnixMilli()}
	if err := item.persistLocked(); err != nil {
		return err
	}
	item.signalLocked()
	return nil
}

// AppendStream appends one already ordered media segment to a page-originated
// capture. Ordering and de-duplication are intentionally performed by the page
// hook, which has access to the SourceBuffer timeline.
func (s *Store) AppendStream(key string, value []byte) (int64, error) {
	if s == nil {
		return 0, errors.New("capture store is unavailable")
	}
	if len(value) == 0 {
		return 0, errors.New("capture segment is empty")
	}
	item, err := s.getEntry(key)
	if err != nil {
		return 0, err
	}
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.meta.Mode != "stream-file" || item.meta.Complete {
		return 0, errors.New("stream capture is not active")
	}
	if int64(len(value)) > maxCaptureSize-item.meta.Total {
		return 0, fmt.Errorf("capture size exceeds %d bytes", maxCaptureSize)
	}
	file, err := os.OpenFile(item.dataPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return 0, err
	}
	offset := item.meta.Total
	written, writeErr := file.WriteAt(value, offset)
	closeErr := file.Close()
	if writeErr != nil {
		return 0, writeErr
	}
	if written != len(value) {
		return 0, io.ErrShortWrite
	}
	if closeErr != nil {
		return 0, closeErr
	}
	item.meta.Total += int64(written)
	item.meta.Ranges = []byteRange{{Start: 0, End: item.meta.Total}}
	item.meta.UpdatedAt = time.Now().UnixMilli()
	item.signalLocked()
	return item.meta.Total, nil
}

// CompleteStream makes a page-originated capture available to capture-file.
func (s *Store) CompleteStream(key string) error {
	if s == nil {
		return errors.New("capture store is unavailable")
	}
	item, err := s.getEntry(key)
	if err != nil {
		return err
	}
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.meta.Mode != "stream-file" {
		return errors.New("stream capture was not started")
	}
	if item.meta.Total <= 0 {
		return errors.New("stream capture is empty")
	}
	file, err := os.OpenFile(item.dataPath, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	item.meta.Complete = true
	item.meta.UpdatedAt = time.Now().UnixMilli()
	if err := item.persistLocked(); err != nil {
		return err
	}
	item.signalLocked()
	return nil
}

// AbortStream discards an incomplete page-originated capture immediately.
func (s *Store) AbortStream(key string) error {
	if s == nil {
		return errors.New("capture store is unavailable")
	}
	item, err := s.getEntry(key)
	if err != nil {
		return err
	}
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.meta.Mode != "stream-file" || item.meta.Complete {
		return nil
	}
	if err := os.Truncate(item.dataPath, 0); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	item.meta = metadata{UpdatedAt: time.Now().UnixMilli()}
	if err := item.persistLocked(); err != nil {
		return err
	}
	item.signalLocked()
	return nil
}

func (s *Store) CopyComplete(ctx context.Context, key, destination string, progress func(int64, int64)) error {
	if s == nil {
		return errors.New("capture store is unavailable")
	}
	item, err := s.getEntry(key)
	if err != nil {
		return err
	}
	for {
		item.mu.Lock()
		complete := captureComplete(item.meta)
		active := item.active
		total := item.meta.Total
		captured := capturedBytes(item.meta)
		changed := item.changed
		dataPath := item.dataPath
		item.mu.Unlock()

		if complete {
			return copyCapture(ctx, dataPath, destination, total, progress)
		}
		if active == 0 {
			return fmt.Errorf("captured response is incomplete: %d/%d bytes; continue loading the resource and retry", captured, total)
		}
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-changed:
		}
	}
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.closed = true
	entries := make([]*entry, 0, len(s.entries))
	for _, item := range s.entries {
		entries = append(entries, item)
	}
	s.mu.Unlock()
	for _, item := range entries {
		item.mu.Lock()
		item.signalLocked()
		item.mu.Unlock()
	}
	return nil
}

func (s *Store) getEntry(key string) (*entry, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("capture key is empty")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("capture store is closed")
	}
	if existing := s.entries[digest]; existing != nil {
		return existing, nil
	}
	directory := filepath.Join(s.root, digest)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	item := &entry{
		dir: directory, dataPath: filepath.Join(directory, "data.bin"),
		metaPath: filepath.Join(directory, "metadata.json"), changed: make(chan struct{}),
	}
	if raw, err := os.ReadFile(item.metaPath); err == nil {
		if json.Unmarshal(raw, &item.meta) != nil || item.meta.Total < 0 || item.meta.Total > maxCaptureSize {
			item.meta = metadata{}
		}
	}
	s.entries[digest] = item
	return item, nil
}

type rangeWriter struct {
	entry  *entry
	file   *os.File
	start  int64
	offset int64
	total  int64
	closed bool
}

func (w *rangeWriter) Write(value []byte) (int, error) {
	if w == nil || w.file == nil || w.closed {
		return 0, os.ErrClosed
	}
	if w.offset >= w.total {
		return 0, io.ErrShortWrite
	}
	if int64(len(value)) > w.total-w.offset {
		value = value[:w.total-w.offset]
	}
	written, err := w.file.WriteAt(value, w.offset)
	w.offset += int64(written)
	return written, err
}

func (w *rangeWriter) Close() error {
	if w == nil || w.closed {
		return nil
	}
	w.closed = true
	closeErr := w.file.Close()
	w.entry.mu.Lock()
	if w.offset > w.start {
		w.entry.meta.Ranges = mergeRanges(append(w.entry.meta.Ranges, byteRange{Start: w.start, End: w.offset}))
	}
	w.entry.meta.UpdatedAt = time.Now().UnixMilli()
	if w.entry.active > 0 {
		w.entry.active--
	}
	persistErr := w.entry.persistLocked()
	w.entry.signalLocked()
	w.entry.mu.Unlock()
	if closeErr != nil {
		return closeErr
	}
	return persistErr
}

func (e *entry) persistLocked() error {
	raw, err := json.Marshal(e.meta)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(e.dir, ".metadata-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	succeeded := false
	defer func() {
		_ = temporary.Close()
		if !succeeded {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0600); err != nil {
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, e.metaPath); err != nil {
		return err
	}
	succeeded = true
	return nil
}

func (e *entry) signalLocked() {
	close(e.changed)
	e.changed = make(chan struct{})
}

func captureBounds(response *http.Response) (int64, int64, error) {
	encoding := strings.TrimSpace(response.Header.Get("Content-Encoding"))
	if encoding != "" && !strings.EqualFold(encoding, "identity") {
		return 0, 0, fmt.Errorf("encoded response capture is unsupported: %s", encoding)
	}
	if start, _, total, ok := parseContentRange(response.Header.Get("Content-Range")); ok {
		return start, total, nil
	}
	if response.Request != nil && response.Request.URL != nil {
		if start, _, ok := parseSimpleRange(response.Request.URL.Query().Get("range")); ok {
			total, _ := strconv.ParseInt(response.Request.URL.Query().Get("clen"), 10, 64)
			return start, total, nil
		}
		if start, _, ok := parseHTTPRange(response.Request.Header.Get("Range")); ok {
			total, _ := strconv.ParseInt(response.Request.URL.Query().Get("clen"), 10, 64)
			return start, total, nil
		}
	}
	return 0, response.ContentLength, nil
}

func parseContentRange(value string) (int64, int64, int64, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "bytes ") {
		return 0, 0, 0, false
	}
	parts := strings.SplitN(strings.TrimSpace(value[6:]), "/", 2)
	if len(parts) != 2 || parts[1] == "*" {
		return 0, 0, 0, false
	}
	start, end, ok := parseSimpleRange(parts[0])
	total, err := strconv.ParseInt(parts[1], 10, 64)
	return start, end, total, ok && err == nil && total > 0 && end < total
}

func parseSimpleRange(value string) (int64, int64, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, startErr := strconv.ParseInt(parts[0], 10, 64)
	end, endErr := strconv.ParseInt(parts[1], 10, 64)
	return start, end, startErr == nil && endErr == nil && start >= 0 && end >= start
}

func parseHTTPRange(value string) (int64, int64, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "bytes=") {
		return 0, 0, false
	}
	return parseSimpleRange(value[6:])
}

func mergeRanges(values []byteRange) []byteRange {
	filtered := make([]byteRange, 0, len(values))
	for _, value := range values {
		if value.Start >= 0 && value.End > value.Start {
			filtered = append(filtered, value)
		}
	}
	for index := 1; index < len(filtered); index++ {
		for current := index; current > 0 && filtered[current].Start < filtered[current-1].Start; current-- {
			filtered[current], filtered[current-1] = filtered[current-1], filtered[current]
		}
	}
	merged := make([]byteRange, 0, len(filtered))
	for _, value := range filtered {
		if len(merged) == 0 || value.Start > merged[len(merged)-1].End {
			merged = append(merged, value)
			continue
		}
		if value.End > merged[len(merged)-1].End {
			merged[len(merged)-1].End = value.End
		}
	}
	return merged
}

func captureComplete(value metadata) bool {
	covered := value.Total > 0 && len(value.Ranges) == 1 && value.Ranges[0].Start == 0 && value.Ranges[0].End >= value.Total
	if value.Mode == "stream-file" {
		return value.Complete && covered
	}
	return covered
}

func capturedBytes(value metadata) int64 {
	var total int64
	for _, item := range value.Ranges {
		end := item.End
		if value.Total > 0 && end > value.Total {
			end = value.Total
		}
		if end > item.Start {
			total += end - item.Start
		}
	}
	return total
}

func copyCapture(ctx context.Context, sourcePath, destination string, total int64, progress func(int64, int64)) error {
	source, err := os.Open(filepath.Clean(sourcePath))
	if err != nil {
		return err
	}
	defer source.Close()
	destinationFile, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		_ = destinationFile.Close()
		if !succeeded {
			_ = os.Remove(destination)
		}
	}()
	buffer := make([]byte, 256*1024)
	var copied int64
	for copied < total {
		if err := ctx.Err(); err != nil {
			return context.Cause(ctx)
		}
		limit := int64(len(buffer))
		if total-copied < limit {
			limit = total - copied
		}
		read, readErr := io.ReadFull(source, buffer[:limit])
		if read > 0 {
			written, writeErr := destinationFile.Write(buffer[:read])
			copied += int64(written)
			if progress != nil {
				progress(copied, total)
			}
			if writeErr != nil {
				return writeErr
			}
			if written != read {
				return io.ErrShortWrite
			}
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
			return readErr
		}
		if readErr != nil {
			return fmt.Errorf("captured response ended at %d of %d bytes", copied, total)
		}
	}
	if err := destinationFile.Sync(); err != nil {
		return err
	}
	if err := destinationFile.Close(); err != nil {
		return err
	}
	succeeded = true
	return nil
}

func (s *Store) pruneExpired() {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-captureCacheTTL)
	for _, item := range entries {
		if !item.IsDir() || len(item.Name()) != sha256.Size*2 {
			continue
		}
		info, statErr := item.Info()
		if statErr == nil && info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(s.root, item.Name()))
		}
	}
}
