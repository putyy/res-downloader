package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"

	"github.com/rs/zerolog"
)

type Logger struct {
	zerolog.Logger
	logFile *os.File
}

func (l *Logger) Close() {
	if l != nil && l.logFile != nil {
		_ = l.logFile.Close()
	}
}

func (l *Logger) Err(err error) { l.Error().Stack().Err(err) }

func (l *Logger) Esg(err error, format string, values ...interface{}) {
	l.Error().Stack().Err(err).Msgf(fmt.Sprintf(format, values...))
}

func New(logFile bool, logPath string) *Logger {
	var out io.Writer
	if logFile {
		logDir := filepath.Dir(logPath)
		if err := shared.CreateDirIfNotExist(logDir); err != nil {
			panic(err)
		}
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			panic(err)
		}
		out = file
	} else {
		out = os.Stdout
	}

	logger := &Logger{}
	if logFile {
		logger.logFile = out.(*os.File)
	}
	logger.Logger = zerolog.New(zerolog.ConsoleWriter{
		NoColor: true, Out: out, TimeFormat: "2006-01-02 15:04:05",
	}).With().Timestamp().Logger()
	return logger
}
