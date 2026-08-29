package app

import (
	appconfig "res-downloader/internal/config"
)

type Config = appconfig.Config

func newConfig(app *App, logger *Logger) *Config {
	return appconfig.New(app.UserDir, logger)
}
