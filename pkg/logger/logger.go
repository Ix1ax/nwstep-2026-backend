package logger

import (
	"os"

	"github.com/rs/zerolog"
)

func NewLogger(env string) zerolog.Logger {
	var log zerolog.Logger

	if env == "production" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		log = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"}).With().Timestamp().Logger()
	}

	return log
}
