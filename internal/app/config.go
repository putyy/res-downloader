package app

import (
	appconfig "res-downloader/internal/config"
	"res-downloader/internal/system"
)

type Config = appconfig.Config

func newConfig(app *App, logger *Logger) *Config {
	return appconfig.New(app.UserDir, logger, system.DefaultLocale())
}
