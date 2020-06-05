package main

import (
	"context"
	"fmt"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Try verifying the users against this host.  If the initial connection fails then panic for now.
// If we want to fail more gracefully we need to figure out how the best default action to take
func scanHost(
	// Signal early termination
	ctx context.Context,

	// The host to connect to -- can contain an optional port info
	host string,

	// Runtime options
	opts opts,

	// Channel to send user attempts that errored
	results chan scanResults,

	// Incoming usernames to test
	userChan chan string,

	// Previous users that were attempted for this server
	cache map[string]bool,

	// Signal when we're done
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	// Use 25 as the default port of this host string doesn't specify
	if !strings.Contains(host, ":") {
		host = host + ":25"
	}

	// Default logger
	l := log.With().Str("Host", host).Logger()

	// So we can easily terminate early (from a ctrl-c or whatever) we need to spawn 2
	// goroutines so we can return without waiting for the usersC to have space available
	done := make(chan interface{}, 0)

	// Notify we should exit early if the context ends early
	go func() {
		<-ctx.Done()
		done <- nil
	}()

	// Process all of the test users
	go func() {
		defer func() { done <- nil }()

		n := int(opts.connsPerHost)
		var swg sync.WaitGroup
		swg.Add(n)

		doCache := cache != nil && len(cache) > 0

		for i := 0; i < n; i++ {
			go func() {
				defer swg.Done()

				var lastUser string

			connLoop:
				for {
					// Make a connection to the smtp server prepare for sending txt commands to it.  Panic if there's an issue
					conn, err := textproto.Dial("tcp", host)
					if err != nil {
						l.Err(err).Msg("Couldn't initiate connection to host")
						time.Sleep(5 * time.Second)
						continue
					}

					if lastUser != "" {
						if testUser(scanResults{server: host, user: lastUser}, results, conn) {
							if err := conn.Close(); err != nil {
								log.Warn().Err(err).Msg("Error closing connection")
							}
							continue connLoop
						} else {
							lastUser = ""
						}
					}

					// Process all of the usernames we want
					for user := range userChan {
						// Bail early if the context says to
						if err := ctx.Err(); err != nil {
							return
						}

						// Maybe skip this user
						if doCache && cache[user] {
							continue
						}

						// Test the host with the current user
						if testUser(scanResults{server: host, user: user}, results, conn) {
							lastUser = user
							if err := conn.Close(); err != nil {
								log.Warn().Err(err).Msg("Error closing connection")
							}
							continue connLoop
						}
					}

					if err := conn.Close(); err != nil {
						log.Warn().Err(err).Msg("Error closing connection")
					}

					// We're done
					return
				}
			}()
		}

		swg.Wait()
	}()

	<-done
}

// Test the host/user combo against the smtp server.  If it's valid send it to the valid channel
func testUser(sr scanResults, results chan scanResults, conn *textproto.Conn) bool {
	l := log.With().Str("Host", sr.server).Str("User", sr.user).Logger()

	// Format the host/user combo we want to verify
	verifyStr := fmt.Sprintf("%s:%s", sr.server, sr.user)
	if id, err := conn.Cmd("VRFY %s", verifyStr); err != nil {

		if strings.Contains(err.Error(), "reset by peer") {
			l.Err(err).Msg("Connection reset; retrying")
			return true
		} else {
			// Do we want to bail here?
			l.Err(err).Msg("Error sending the verify message?")
			sr.err = err
			results <- sr
		}
	} else {
		// Do the comms
		conn.StartResponse(id)
		defer conn.EndResponse(id)

		// We'll handle the error code ourselves
		if code, msg, err := conn.ReadResponse(-1); err != nil {
			l.Err(err).Msg("Error verifying user")

			sr.err = err
			results <- sr

			// Valid codes from https://cr.yp.to/smtp/vrfy.html
		} else {
			l.Info().Int("Code", code).Str("Msg", msg).Msg("Got response from server")

			sr.code = int64(code)
			sr.msg = msg
			results <- sr
		}
	}

	return false
}
