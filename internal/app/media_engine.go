package app

import "res-downloader/internal/media"

type MediaToolStatus = media.MediaToolStatus
type MediaEngineStatus = media.MediaEngineStatus
type mediaEngine = media.Engine

func NewMediaEngine(config *Config) *media.Engine {
	return media.New(func() (string, string) {
		snapshot := config.Snapshot()
		return snapshot.FFmpegPath, snapshot.FFprobePath
	})
}

func ffmpegHeaderArgument(headers map[string]string) string {
	return media.HeaderArgument(headers)
}
