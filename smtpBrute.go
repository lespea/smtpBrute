package main

import (
	"sync"

	"github.com/rs/zerolog/log"
)

func init() {
	setupLogging()
}

// Does the scanning
func main() {
	// Setup signal handling
	ctx, stop := setupSigCatch()
	defer stop()

	// Parse the cmd opts
	opts := getOpts()

	// Channels used for caching
	var cacheWG sync.WaitGroup
	valid, attemptC, attemptErrC := getCaches(opts.skip, len(opts.hosts), &cacheWG)

	// Channels used to process the users across the various hosts
	usersChans := make([]chan string, len(opts.hosts))
	for i := range usersChans {
		usersChans[i] = make(chan string, opts.connsPerHost*10)
	}

	// Scan each of the hosts
	var hostsWG sync.WaitGroup
	hostsWG.Add(len(opts.hosts))
	for i, host := range opts.hosts {
		go scanHost(ctx, host, opts, valid, attemptC, attemptErrC, usersChans[i], &hostsWG)
	}

	// Get the usernames to test and send them to the scanners
	getUsers(ctx, usersChans, opts)
	// Wait for the scanners to finish
	hostsWG.Wait()

	// Shutdown the cache writers
	log.Info().Msg("Closing the cache writers")
	close(attemptC)
	close(attemptErrC)
	cacheWG.Wait()

	// Terminate
	log.Info().Msg("Finished")
}
