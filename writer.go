package main

import (
	"bufio"
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/lespea/smtpBrute/util"
)

const (
	HeaderIdxServer = iota
	HeaderIdxUser
	HeaderIdxCode
	HeaderIdxMsg
	HeaderIdxErr
)

const (
	HeaderArrayLen = HeaderIdxErr + 1

	HeaderServer = "Server"
	HeaderUser   = "User"
	HeaderCode   = "Code"
	HeaderMsg    = "Msg"
	HeaderErr    = "Err"
)

var (
	CsvHeaders = [HeaderArrayLen]string{}
)

func init() {
	CsvHeaders[HeaderIdxServer] = HeaderServer
	CsvHeaders[HeaderIdxUser] = HeaderUser
	CsvHeaders[HeaderIdxCode] = HeaderCode
	CsvHeaders[HeaderIdxMsg] = HeaderMsg
	CsvHeaders[HeaderIdxErr] = HeaderErr
}

func getCachePath(path string) map[string]*map[string]bool {
	if path == "" {
		log.Info().Msg("Skipping the cache")
		return make(map[string]*map[string]bool)
	}

	fh, err := os.Open(path)
	if err != nil {
		log.Err(err).Msg("Error opening old cache for reading; skipping")
		return make(map[string]*map[string]bool)
	}
	defer func() {
		util.SafeClose("write cache", path, fh)
	}()

	return getCacheFH(fh)
}

// Tries reading past output csvs so we don't re-test a server/user combo.  If there is any error we log it but
// just return an empty map as we'll recreate the output csv if the returned map is empty.
//
// Disable getting the by providing an empty string as the path
func getCacheFH(fh io.Reader) map[string]*map[string]bool {
	// By making this a map pointer, we don't have to continually update the map after getting the sub map.  Normally
	// this would be worse when later using it, but we'll only be passing the relevant sub-map to each server parser
	// we won't pay the double pointer penalty later
	m := make(map[string]*map[string]bool)

	buf := bufio.NewReader(fh)
	c := csv.NewReader(buf)

	if firstRow, err := c.Read(); err != nil {
		if err == io.EOF {
			log.Info().Msg("Skipping the cache")
			return m
		} else {
			log.Err(err).Msg("Couldn't get the csv header; no cache will be used")
			return m
		}
	} else {
		if len(firstRow) != len(CsvHeaders) {
			log.Err(err).Strs("Row", firstRow).Msg("First row of output has incorrect headers; not using")
			return m
		} else {
			for i, n := range firstRow {
				if n != CsvHeaders[i] {
					log.Err(err).Strs("Row", firstRow).Msg("First row of output has incorrect headers; not using")
					return m
				}
			}
		}
	}

	// Put the contents of what we've scanned so far into the map and return when we're done
	for {
		if row, err := c.Read(); err != nil {
			// We should be done normally when we get the EOF err
			if err == io.EOF {
				return m
			} else {
				log.Err(err).Msg("Error reading csv; returning the cache we've parsed so far")

				// Return a new map here?
				return m
			}
		} else {
			server := row[HeaderIdxServer]

			// Validate the row
			if len(row) != len(CsvHeaders) {
				if len(row) > 0 {
					log.Err(err).Strs("Row", row).Msg("Row has incorrect len; skipping")
				} else {
					log.Err(err).Msg("Empty row found?  Quiting early")
					return m
				}
			}

			// Update the maps with our info
			if mpointer, ok := m[server]; ok {
				// You can't update the map pointer without dereferencing it first
				sm := *mpointer
				sm[row[HeaderIdxUser]] = true
			} else {
				sm := make(map[string]bool)
				sm[row[HeaderIdxUser]] = true
				m[server] = &sm
			}
		}
	}
}

type scanResults struct {
	server string
	user   string
	code   int64
	msg    string
	err    error
}

func startWriterPath(path string, append bool, wg *sync.WaitGroup) chan scanResults {
	name := "result writer"
	l := log.With().Str("Name", name).Str("Path", path).Logger()

	var fh io.WriteCloser

	{
		openp := os.O_APPEND | os.O_WRONLY | os.O_CREATE
		if !append {
			openp |= os.O_TRUNC
		}

		var err error
		if fh, err = os.OpenFile(path, openp, 0644); err != nil {
			l.Fatal().Err(err).Msg("Couldn't create output file")
		}
	}

	return startWriter(fh, util.SafeClose(name, path, fh), l, append, wg)
}

func startWriter(fh io.Writer, closer func(), l zerolog.Logger, append bool, wg *sync.WaitGroup) chan scanResults {
	resultsChan := make(chan scanResults, 50)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer closer()

		c := csv.NewWriter(fh)
		defer func() {
			c.Flush()
			if err := c.Error(); err != nil {
				l.Err(err).Msg("Error flushing csv output")
			}
		}()

		if !append {
			if err := c.Write(CsvHeaders[:]); err != nil {
				l.Fatal().Err(err).Msg("Error writing output headers")
			}
		}

		row := [HeaderArrayLen]string{}

		for sr := range resultsChan {
			var estr string
			if sr.err != nil {
				estr = sr.err.Error()
			}

			CsvHeaders[HeaderIdxServer] = sr.server
			CsvHeaders[HeaderIdxUser] = sr.user
			CsvHeaders[HeaderIdxCode] = strconv.FormatInt(sr.code, 10)
			CsvHeaders[HeaderIdxMsg] = sr.msg
			CsvHeaders[HeaderIdxErr] = estr

			if err := c.Write(row[:]); err != nil {
				l.Fatal().Err(err).Interface("Info", sr).Msg("Error writing output info")
			}
		}
	}()

	return resultsChan
}
