package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"

	"github.com/rs/zerolog/log"
)

const (
	Valid              = "valid.txt"
	ValidName          = "valid"
	Attempted          = "attempted.txt"
	AttemptedName      = "attempted"
	AttemptedError     = "attempted_error.txt"
	AttemptedErrorName = "attempted errors"
)

// Given a file path, try to read each line and add it to the set pointer (if it exists)
func readAttempted(name, path string, mp *map[string]bool) {
	m := *mp

	if fh, err := os.Open(path); err != nil {
		log.Info().Str("Name", name).Str("Path", path).Msg("Couldn't read the attempted cache; skipping")
	} else {
		defer safeClose(name, path, fh)

		scanner := bufio.NewScanner(fh)
		for scanner.Scan() {
			m[scanner.Text()] = true
		}
	}
}

// Read the entries we already tried from the Attempted and AttemptedError files
//
// If we want to ignore the cache then we'll skip this altogether
func getAttempted(skip bool) map[string]bool {
	if skip {
		return nil
	}

	m := make(map[string]bool, 1<<14)

	readAttempted(AttemptedName, Attempted, &m)

	// This should only contain dupes but keep it just in case
	readAttempted(AttemptedErrorName, AttemptedError, &m)

	if len(m) == 0 {
		return nil
	} else {
		log.Info().Int("Len", len(m)).Msg("Loaded strs from attempted cache")
		return m
	}
}

// Open up a handle to a cache file and write all of the incoming entries to that file
//
// If skip is true then the cache files are left untouched and all of the incoming
// strings are dropped
func cacheWriter(skip bool, name, path string, hosts int, strs chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Even if we want to skip everything, we must still process any incoming string and just drop them
	if skip {
		for range strs {
		}
		return
	}

	// Populate a logger with helper data
	l := log.With().Str("Name", name).Str("Path", path).Logger()

	// Try opening the cache file in append mode (if it exists); panic otherwise
	if fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		l.Fatal().Err(err).Msg("Couldn't create cache writer")
	} else {
		b := bufio.NewWriter(fh)
		defer func() {
			if err := b.Flush(); err != nil {
				l.Fatal().Err(err).Msg("Couldn't flush cache buffer")
			}
		}()

		counts := make(map[string]int, 100)

		// See how many times we've seen this username... once we've seen it as many times as hosts we're scanning
		// mark it as finished by writing it to the cache file and
		for str := range strs {
			if count, _ := counts[str]; count == hosts {
				delete(counts, str)

				if _, err := fmt.Fprintln(b, str); err != nil {
					l.Fatal().Err(err).Str("Str", str).Msg("Couldn't write cache str to file")
				}
			} else {
				counts[str] = count + 1
			}
		}
	}
}

// Writes the username to the cache file contianing the errored users and forwards the username to the normal
// cache writer
func genWriter(skip bool, name, path string, strs chan string, okWriter chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Even if we want to skip everything, we must still process any incoming string and just drop them
	if skip {
		for range strs {
		}
		return
	}

	// Populate a logger with helper data
	l := log.With().Str("Name", name).Str("Path", path).Logger()

	// Try opening the cache file in append mode (if it exists); panic otherwise
	if fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		l.Fatal().Err(err).Msg("Couldn't create cache writer")
	} else {
		b := bufio.NewWriter(fh)
		defer func() {
			if err := b.Flush(); err != nil {
				l.Fatal().Err(err).Msg("Couldn't flush cache buffer")
			}
		}()

		for str := range strs {
			if _, err := fmt.Fprintln(b, str); err != nil {
				l.Fatal().Err(err).Str("Entry", str).Msg("Couldn't write entry")
			}
			if okWriter != nil {
				okWriter <- str
			}
		}
	}
}

// Get the channels we'll use for persisting the data to txt files
func getCaches(skip bool, hosts int, wg *sync.WaitGroup) (chan string, chan string, chan string) {
	valid := make(chan string, 10)
	attempted := make(chan string, 10)
	attemptedErr := make(chan string, 10)

	wg.Add(3)
	go genWriter(false, ValidName, Valid, valid, nil, wg)
	go cacheWriter(skip, AttemptedName, Attempted, hosts, attempted, wg)
	go genWriter(skip, AttemptedErrorName, AttemptedError, attemptedErr, attempted, wg)

	return valid, attempted, attemptedErr
}
