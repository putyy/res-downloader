package app

import resourceservice "res-downloader/internal/resource"

type Resource = resourceservice.Resource

func newResource(app *App, config *Config, media *mediaEngine, logger *Logger) *Resource {
	return resourceservice.New(app.UserDir, config, media, logger, func(name string, values ...interface{}) {
		if len(values) > 0 {
			app.emitEvent(name, values[0])
		}
	})
}
