package main

import (
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/lespea/smtpBrute/util"
)

func init() {
	util.SetupLogging()
}

// Does the scanning
func main() {
	// Setup signal handling
	ctx, stop := util.SetupSigCatch()
	defer stop()

	// Parse the cmd opts
	opts := getOpts()

	// Channels used to process the users across the various hosts
	usersChans := make([]chan string, len(opts.hosts))
	for i := range usersChans {
		usersChans[i] = make(chan string, opts.connsPerHost*10)
	}

	// Scan each of the hosts
	var hostsWG sync.WaitGroup
	hostsWG.Add(len(opts.hosts))

	var writerWG sync.WaitGroup
	var srChan chan scanResults
	// Put the cache map + go roroutines in their own scope so the outer cache map can be GC'd early
	{
		cacheName := opts.outName
		if opts.fresh {
			cacheName = ""
		}
		m := getCachePath(cacheName)

		srChan = startWriterPath(opts.outName, !opts.fresh && len(m) > 0, &writerWG)

		for i, host := range opts.hosts {
			var cache map[string]bool

			hm := m[host]
			if hm != nil {
				cache = *hm
			}

			go scanHost(ctx, host, opts, srChan, usersChans[i], cache, &hostsWG)
		}
	}

	// Get the usernames to test and send them to the scanners
	getUsers(ctx, usersChans, opts)
	// Wait for the scanners to finish
	hostsWG.Wait()

	// Shutdown the cache writers
	log.Info().Msg("Closing the cache writers")
	close(srChan)
	writerWG.Wait()

	// Terminate
	log.Info().Msg("Finished")
}
