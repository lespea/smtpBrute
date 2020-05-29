package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/akamensky/argparse"
	log "github.com/rs/zerolog/log"
)

const (
	DefaultConnsPerHost = 10
	DefaultUserList     = "users.txt"
)

var (
	validHostRe = regexp.MustCompile(`^(?:[a-zA-Z0-9_\-]{1,255}\.)(?::\d{1,5})?$`)
)

// A host shouldn't be greater than 255 chars and match the validHostRe regex
func validHost(host string) error {
	if len(host) <= (255+1+5) && validHostRe.MatchString(host) {
		return nil
	} else {
		return fmt.Errorf("invalid host: %s", host)
	}
}

// Valid hostnames must contain at least one entry and match the validHost function test
func validHostname(hosts []string) (err error) {
	if len(hosts) == 0 {
		return errors.New("must include at least one host to scan")
	}

	for _, host := range hosts {
		if herr := validHost(host); herr != nil {
			err = fmt.Errorf("%v: %w", herr, err)
		}
	}

	return
}

// The count should be a uint8.  Seems sensible but we could bump this later
func validConns(count []string) error {
	if len(count) != 1 {
		return errors.New("count should be a len 1")
	}

	_, err := strconv.ParseUint(count[0], 10, 8)
	return err
}

// Info we want from the user when running
type opts struct {
	// Holds the previously scanned entries
	cache map[string]bool

	// How many simultaneous outgoing connections we should try per host
	connsPerHost uint8

	// The list of hosts we should scan
	hosts []string

	// If we should skip the cache altogether
	skip bool

	// The filehandle to the users input file
	fh *os.File

	// Cleanup function (this needs to be called for cleanup
	close func()
}

// Parse all of the command line arguments and upt the values into a common struct we can use everywhere
func getOpts() opts {
	parser := argparse.NewParser("smtpBrue", "brute for an smtp server")

	skipCache := parser.Flag("", "nocache", &argparse.Options{
		Help:    "don't use any cache files (do a full scan",
		Default: false,
	})

	connsPerHost := parser.Int("c", "conns-per-host", &argparse.Options{
		Help:     "max connection attempts per host",
		Default:  DefaultConnsPerHost,
		Validate: validConns,
	})

	hosts := parser.StringList("h", "hosts", &argparse.Options{
		Help:     "the hosts we should try brute forcing",
		Required: true,
		Validate: validHostname,
	})

	usersFH := parser.File("u", "users", os.O_RDONLY, 0444, &argparse.Options{
		Help:     "file containing the list of usernames to try",
		Required: true,
		Default:  DefaultUserList,
	})

	// Log what we're using
	log.
		Info().
		Int("Conns Per Host", *connsPerHost).
		Strs("Hosts", *hosts).
		Msg("Starting smtp brute")

	conns := uint8(*connsPerHost)
	if conns == 0 {
		n := runtime.NumCPU()

		if n < 1 {
			conns = 1
		} else if n > math.MaxUint8 {
			conns = math.MaxUint8
		} else {
			conns = uint8(runtime.NumCPU())
		}
	}

	return opts{
		cache:        getAttempted(*skipCache),
		connsPerHost: conns,
		hosts:        *hosts,
		skip:         *skipCache,
		fh:           usersFH,
		close:        safeClose("users", usersFH.Name(), usersFH),
	}
}
