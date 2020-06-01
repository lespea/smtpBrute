package main

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"runtime"
	"strconv"

	"github.com/akamensky/argparse"
	log "github.com/rs/zerolog/log"

	"github.com/lespea/smtpBrute/util"
)

const (
	DefaultConnsPerHost = 10
	DefaultOutName      = "findings.csv"
	DefaultInput        = "users.txt"
)

var (
	validHostRe = regexp.MustCompile(`^(?:[a-zA-Z0-9_\-]{1,255}\.)*[a-zA-Z0-9_\-]{1,255}(?::\d{1,5})?$`)
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
			if err != nil {
				err = fmt.Errorf("%v: %w", herr, err)
			} else {
				err = herr
			}
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
	// How many simultaneous outgoing connections we should try per host
	connsPerHost uint8

	// Fresh start
	fresh bool

	// The list of hosts we should scan
	hosts []string

	// If we should skip the cache altogether
	skip bool

	// The filehandle to the users input file
	input io.Reader

	// Cleanup function (this needs to be called for cleanup
	close func()

	// The cache filename
	outName string
}

// Parse all of the command line arguments and upt the values into a common struct we can use everywhere
func getOpts() opts {
	parser := argparse.NewParser("smtpBrue", "brute for an smtp server")

	fresh := parser.Flag("", "fresh", &argparse.Options{
		Help:    "deletes the existing output csv and starts fresh",
		Default: false,
	})

	connsPerHost := parser.Int("c", "conns-per-host", &argparse.Options{
		Help:     "max connection attempts per host",
		Default:  DefaultConnsPerHost,
		Validate: validConns,
	})

	targets := parser.StringList("t", "targets", &argparse.Options{
		Help:     "the targets we should try brute forcing",
		Required: true,
		Validate: validHostname,
	})

	inputFH := parser.File("i", "input", os.O_RDONLY, 0444, &argparse.Options{
		Help:    "file containing the list of usernames to try",
		Default: DefaultInput,
	})

	outNameP := parser.String("o", "output", &argparse.Options{
		Help:    "the csv to write the findings to",
		Default: DefaultOutName,
	})

	if err := parser.Parse(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Error parsing args")
	}

	outName := *outNameP
	if len(outName) == 0 {
		log.Fatal().Msg("The csv file must be a non-empty string")
	}

	// Log what we're using
	log.
		Info().
		Int("Conns Per Host", *connsPerHost).
		Strs("Hosts", *targets).
		Str("Output CSV", outName).
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
		connsPerHost: conns,
		fresh:        *fresh,
		hosts:        *targets,
		outName:      outName,
		input:        inputFH,
		close:        util.SafeClose("input", inputFH.Name(), inputFH),
	}
}
