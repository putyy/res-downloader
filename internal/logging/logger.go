package logging

import (
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

// Close releases the rotating log file. It is safe on a nil receiver and on a
// logger that writes to stdout.
func (l *Logger) Close() {
	if l != nil && l.logFile != nil {
		_ = l.logFile.Close()
	}
}

// Err logs err at error level together with its stack trace.
func (l *Logger) Err(err error) { l.Error().Stack().Err(err) }

// Esg logs err with a stack trace and a formatted message.
func (l *Logger) Esg(err error, format string, values ...interface{}) {
	l.Error().Stack().Err(err).Msgf(format, values...)
}

// New builds a logger that writes to logPath (rotating at 10 MiB, keeping 5
// backups for 7 days) when logFile is true, and to stdout otherwise. It panics
// when the log directory cannot be created, because a desktop build without a
// writable log directory cannot report failures any other way.
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
