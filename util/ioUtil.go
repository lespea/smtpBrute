package util

import (
	"io"

	"github.com/rs/zerolog/log"
)

// Helper for closing file handles and logging if there were any issues
func SafeClose(name, path string, fh io.Closer) func() {
	return func() {
		if err := fh.Close(); err != nil {
			log.Err(err).Str("Name", name).Str("Path", path).Msg("Couldn't close fh")
		}
	}
}
