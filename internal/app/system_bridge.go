package app

import desktopsystem "res-downloader/internal/system"

type SystemSetup = desktopsystem.Setup

func newSystemSetup(app *App, config *Config, logger *Logger) *SystemSetup {
	return desktopsystem.NewSetup(desktopsystem.Environment{
		UserDir: app.UserDir, AppName: app.AppName, PublicCrt: app.PublicCrt,
	}, config, logger)
}
