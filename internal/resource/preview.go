package resource

import (
	"errors"
	"io"
	"time"
)

type capturePreviewSource interface {
	OpenComplete(string) (io.ReadSeekCloser, time.Time, error)
}

// OpenCapturePreview accepts only the plugin-scoped key from a validated plan.
// The returned reader must be closed to release its capture lease.
func (r *Resource) OpenCapturePreview(key string) (io.ReadSeekCloser, time.Time, error) {
	source, ok := r.captures.(capturePreviewSource)
	if !ok {
		return nil, time.Time{}, errors.New("capture preview is unavailable")
	}
	return source.OpenComplete(key)
}
