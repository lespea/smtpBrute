package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
)

// Helper to watch for early termination signals (ctrl-c, etc) and handle it gracefully
func setupSigCatch() (context.Context, context.CancelFunc) {
	sigs := make(chan os.Signal, 1)

	ctx, done := context.WithCancel(context.Background())

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Warn().Interface("Signal", sig).Msg("Caught signal; initiating shutdown")
		done()
	}()

	return ctx, done
}
