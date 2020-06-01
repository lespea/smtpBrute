package util

import (
	"runtime"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Somewhat sensible default logging
func SetupLogging() {
	log.Level(zerolog.InfoLevel)

	out := zerolog.NewConsoleWriter()

	// We should be able to use colors everywhere but windows
	if runtime.GOOS == "windows" {
		out.NoColor = true
	}

	log.Logger = log.With().Timestamp().Caller().Logger().Output(out)
}
