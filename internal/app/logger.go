package app

import (
	"os"
	"path/filepath"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
)

type Logger = logging.Logger

func newAppLogger(app *App) *Logger {
	logger := logging.New(!shared.IsDevelopment(), filepath.Join(app.UserDir, "logs", "app.log"))
	logger.Logger = logger.With().Int("pid", os.Getpid()).Str("version", app.Version).Logger()
	logger.Info().Msg("application starting")
	return logger
}

func NewLogger(logFile bool, logPath string) *Logger {
	return logging.New(logFile, logPath)
}
