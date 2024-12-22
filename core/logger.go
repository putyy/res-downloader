package core

import (
	"fmt"
	"github.com/rs/zerolog"
	"io"
	"os"
	"path/filepath"
)

type Logger struct {
	zerolog.Logger
	logFile *os.File
}

func initLogger() *Logger {
	if globalLogger == nil {
		globalLogger = NewLogger(!IsDevelopment(), filepath.Join(appOnce.UserDir, "logs", "app.log"))
	}
	return globalLogger
}

func (l *Logger) Close() {
	_ = l.logFile.Close()
}

func (l *Logger) err(err error) {
	l.Error().Stack().Err(err)
}

func (l *Logger) Esg(err error, format string, v ...interface{}) {
	l.Error().Stack().Err(err).Msgf(fmt.Sprintf(format, v...))
}

// NewLogger create a new logger
func NewLogger(logFile bool, logPath string) *Logger {
	var out io.Writer
	if logFile {
		// log to file
		logDir := filepath.Dir(logPath)
		if err := CreateDirIfNotExist(logDir); err != nil {
			panic(err)
		}
		var (
			logfile *os.File
			err     error
		)
		logfile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			panic(err)
		}
		out = logfile
	} else {
		out = os.Stdout
	}

	logger := &Logger{}
	if logFile {
		logger.logFile = out.(*os.File)
	}
	logger.Logger = zerolog.New(zerolog.ConsoleWriter{
		NoColor:    true,
		Out:        out,
		TimeFormat: "2006-01-02 15:04:05",
	}).With().Timestamp().Logger()
	return logger
}
