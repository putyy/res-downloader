package capture

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

var ErrIncomplete = errors.New("capture is missing or incomplete; reopen the resource and retry")

// OpenComplete leases a complete, stable file. Writers cannot replace its bytes
// until the caller closes the reader. It never waits for a capture to complete.
func (s *Store) OpenComplete(key string) (io.ReadSeekCloser, time.Time, error) {
	if s == nil {
		return nil, time.Time{}, ErrIncomplete
	}
	item, err := s.getEntry(key)
	if err != nil {
		return nil, time.Time{}, err
	}
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.active != 0 || !captureComplete(item.meta) {
		return nil, time.Time{}, ErrIncomplete
	}
	file, err := os.Open(item.dataPath)
	if err != nil {
		return nil, time.Time{}, ErrIncomplete
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != item.meta.Total {
		_ = file.Close()
		return nil, time.Time{}, ErrIncomplete
	}
	item.readers++
	return &previewReader{File: file, entry: item}, time.UnixMilli(item.meta.UpdatedAt), nil
}

type previewReader struct {
	*os.File
	entry *entry
	once  sync.Once
	err   error
}

func (r *previewReader) Close() error {
	r.once.Do(func() {
		r.err = r.File.Close()
		r.entry.mu.Lock()
		r.entry.readers--
		r.entry.mu.Unlock()
	})
	return r.err
}
