package main

import (
	"bufio"
	"context"
	"strings"
)

// Remove all spaces
func trimUser(user string) string {
	return strings.Replace(strings.TrimSpace(user), " ", "", -1)
}

// Parses the file that contains the user names to tests.  Each user is sent to the chans provided unless
// the hosts exists in the already scanned cache (in the opts struct)
//
// This closes all of the channels upon termination
func getUsers(ctx context.Context, chans []chan string, o opts) {
	// Cleanup the filehandle
	defer o.close()

	// Close all of the outgoing channels when we're done sending things
	defer func() {
		for _, c := range chans {
			close(c)
		}
	}()

	// So we can easily terminate early (from a ctrl-c or whatever) we need to spawn 2
	// goroutines so we can return without waiting for the usersC to have space available
	done := make(chan interface{}, 0)

	// Notify we should exit early if the context ends early
	go func() {
		<-ctx.Done()
		done <- nil
	}()

	go func() {
		defer func() { done <- nil }()

		s := bufio.NewScanner(o.input)
		for s.Scan() {
			// terminate if the context has ended while we were waiting
			if err := ctx.Err(); err != nil {
				return
			}

			t := trimUser(s.Text())

			for _, c := range chans {
				c <- t
			}
		}
	}()

	<-done
}
