package download

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	shared "res-downloader/internal/model"
)

func runXHSImageFallback(ctx context.Context, input shared.DownloadInput, acquire func(shared.DownloadInput, bool) (string, error)) (string, error) {
	path, err := acquire(input, false)
	if err == nil || ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return path, err
	}
	enabled, _ := input.Options["xiaohongshuBackup"].(bool)
	primary, parseErr := url.Parse(input.URL)
	if !enabled || parseErr != nil || primary.Scheme != "https" || primary.Host != "sns-webpic-qc.xhscdn.com" || primary.User != nil {
		return "", err
	}
	var status *downloadHTTPStatusError
	var network net.Error
	if errors.As(err, &status) {
		if status.statusCode != http.StatusNotFound && status.statusCode < 500 {
			return "", err
		}
	} else if !errors.As(err, &network) {
		return "", err
	}
	primary.Host = "sns-webpic-backup.xhscdn.com"
	input.URL = primary.String()
	path, backupErr := acquire(input, true)
	if backupErr != nil {
		return "", fmt.Errorf("primary and backup image downloads failed: %v; backup: %w", err, backupErr)
	}
	return path, nil
}

