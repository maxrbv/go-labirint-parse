package logger

import (
	"os"

	"labirint-parser/config"

	"github.com/rs/zerolog"
)

type Logger struct {
	log zerolog.Logger
}

func NewLogger(cfg config.LoggerConfig) *Logger {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	var output zerolog.ConsoleWriter
	if cfg.Format == "console" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		}
	}

	log := zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Logger()

	return &Logger{log: log}
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.log.Info().Fields(args).Msg(msg)
}

func (l *Logger) Error(err error, msg string, args ...interface{}) {
	l.log.Error().Err(err).Fields(args).Msg(msg)
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log.Debug().Fields(args).Msg(msg)
}

func (l *Logger) Fatal(err error, msg string, args ...interface{}) {
	l.log.Fatal().Err(err).Fields(args).Msg(msg)
}
