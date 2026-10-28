package plugin

import (
	"res-downloader/internal/logging"
	"res-downloader/internal/media"
)

type Logger = logging.Logger
type mediaEngine = media.Engine

type NetworkSettings struct {
	DownloadProxy bool
	UpstreamProxy string
	Port          string
}

type NetworkSettingsProvider func() NetworkSettings
