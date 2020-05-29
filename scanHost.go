package main

import (
	"context"
	"fmt"
	"net/textproto"
	"strings"
	"sync"

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

	// Channel to send host/user combinations that pass
	valid chan string,

	// Channel to send user attempts that passed
	attemptedC chan string,

	// Channel to send user attempts that errored
	attemptedErrC chan string,

	// Incoming usernames to test
	userChan chan string,

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

		for i := 0; i < n; i++ {
			go func() {
				defer swg.Done()

				// Make a connection to the smtp server prepare for sending txt commands to it.  Panic if there's an issue
				conn, err := textproto.Dial("tcp", host)
				if err != nil {
					l.Fatal().Err(err).Msg("Couldn't initiate connection to host")
				}

				// Process all of the usernames we want
				for user := range userChan {
					// Bail early if the context says to
					if err := ctx.Err(); err != nil {
						return
					}

					// Test the host with the current user
					failed := testUser(user, host, valid, conn)

					// If we're not skipping the chach and we haven't terminated early from the context, push the
					// user onto the appropriate channel depending if the test passed or failed
					if !opts.skip {
						if err := ctx.Err(); err == nil {
							if failed {
								attemptedErrC <- user
							} else {
								attemptedC <- user
							}
						}
					}
				}
			}()
		}

		swg.Wait()
	}()

	<-done
}

// Test the host/user combo against the smtp server.  If it's valid send it to the valid channel
func testUser(host, user string, valid chan string, conn *textproto.Conn) bool {
	l := log.With().Str("Host", host).Str("User", user).Logger()

	// Format the host/user combo we want to verify
	eStr := fmt.Sprintf("%s:%s", host, user)
	if id, err := conn.Cmd("VRFY %s", eStr); err != nil {
		l.Fatal().Err(err).Msg("Error sending the verify message?")
	} else {
		// Do the comms
		conn.StartResponse(id)
		defer conn.EndResponse(id)

		// We'll handle the error code ourselves
		if code, msg, err := conn.ReadResponse(-1); err != nil {
			l.Err(err).Msg("Error verifying user")

			// Valid codes from https://cr.yp.to/smtp/vrfy.html
		} else {
			l = l.With().Int("Code", code).Str("Msg", msg).Logger()

			if code == 250 || code == 251 {
				valid <- eStr
				l.Info().Msg("Valid username")
				return false

				// Unsure; treat as invalid?
			} else if code == 252 {
				l.Warn().Err(err).Msg("Not sure if valid")

			} else if code == 501 {
				l.Err(err).Msg("Invalid username")

				// Other code that we'll treat as invalid
			} else {
				l.Err(err).Msg("Error verifying user")
			}
		}
	}

	// Most branches return true so we'll do so at the end here
	return true
}
