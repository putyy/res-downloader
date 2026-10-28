package app

import downloadengine "res-downloader/internal/download"

type DownloadScheduler = downloadengine.Scheduler

func newDownloadScheduler(app *App, config *Config, resources *Resource, plugins *PluginManager, logger *Logger) *DownloadScheduler {
	return downloadengine.NewScheduler(app.UserDir, config, resources, plugins, logger)
}
