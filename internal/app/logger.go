package app

import (
	"path/filepath"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
)

type Logger = logging.Logger

func newAppLogger(app *App) *Logger {
	return logging.New(!shared.IsDevelopment(), filepath.Join(app.UserDir, "logs", "app.log"))
}

func NewLogger(logFile bool, logPath string) *Logger {
	return logging.New(logFile, logPath)
}
