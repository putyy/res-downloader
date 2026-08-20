package plugins

import (
	"net/http"
	"testing"
)

func TestResponseSizeUsesContentRangeTotal(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusPartialContent,
		Header: http.Header{
			"Content-Length": {"65536"},
			"Content-Range":  {"bytes 0-65535/1234567"},
		},
	}

	if got := responseSize(resp); got != 1234567 {
		t.Fatalf("responseSize() = %v, want 1234567", got)
	}
}

func TestResponseSizeFallsBackToContentLength(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Length": {"7654321"},
		},
	}

	if got := responseSize(resp); got != 7654321 {
		t.Fatalf("responseSize() = %v, want 7654321", got)
	}
}
