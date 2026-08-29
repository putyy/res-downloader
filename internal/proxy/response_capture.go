package proxy

import (
	"io"
	"net/http"
	shared "res-downloader/internal/model"
)

func attachResponseCaptures(response *http.Response, captures []shared.ResponseCapture, store ResponseCaptureStore) {
	if response == nil || response.Body == nil || store == nil {
		return
	}
	writers := make([]io.WriteCloser, 0, len(captures))
	for _, capture := range captures {
		if capture.Mode != "range-file" {
			continue
		}
		writer, err := store.Begin(capture.Key, response)
		if err == nil && writer != nil {
			writers = append(writers, writer)
		}
	}
	if len(writers) == 0 {
		return
	}
	response.Body = &capturingReadCloser{source: response.Body, writers: writers}
}

type capturingReadCloser struct {
	source  io.ReadCloser
	writers []io.WriteCloser
	closed  bool
}

func (r *capturingReadCloser) Read(value []byte) (int, error) {
	read, readErr := r.source.Read(value)
	if read > 0 {
		active := r.writers[:0]
		for _, writer := range r.writers {
			written, writeErr := writer.Write(value[:read])
			if writeErr != nil || written != read {
				_ = writer.Close()
				continue
			}
			active = append(active, writer)
		}
		r.writers = active
	}
	if readErr != nil {
		r.closeWriters()
	}
	return read, readErr
}

func (r *capturingReadCloser) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	r.closeWriters()
	return r.source.Close()
}

func (r *capturingReadCloser) closeWriters() {
	for _, writer := range r.writers {
		_ = writer.Close()
	}
	r.writers = nil
}
