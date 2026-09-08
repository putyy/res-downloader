package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	zerolog.Logger
	logFile io.Closer
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
		out = &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    10,
			MaxBackups: 5,
			MaxAge:     7,
		}
	} else {
		out = os.Stdout
	}

	logger := &Logger{}
	if logFile {
		logger.logFile = out.(io.Closer)
	}
	logger.Logger = zerolog.New(zerolog.ConsoleWriter{
		NoColor: true, Out: out, TimeFormat: "2006-01-02 15:04:05",
	}).With().Timestamp().Logger()
	return logger
}
