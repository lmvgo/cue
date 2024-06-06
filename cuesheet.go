package cuesheetgo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
)

const (
	// trimChars contains the characters to be trimmed from a string.
	// These are: space, double quote, tab, newline.
	trimChars = ` ` + `"` + `\t` + `\n`

	fileParams = 2

	maxTracks = 99
)

// Track represents a single track in a cue sheet file.
// Required fields: Type.
type Track struct {
	Type string
}

// CueSheet represents the contents of a cue sheet file.
// Required fields: FileName, Format, Tracks.
type CueSheet struct {
	Format   string
	FileName string
	Tracks   []Track
}

// Parse reads the cue sheet data from the provided reader and returns a parsed CueSheet struct.
func Parse(reader io.Reader) (*CueSheet, error) {
	scanner := bufio.NewScanner(reader)
	c := &CueSheet{Tracks: []Track{}}

	var lineNr int
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), trimChars)
		lineNr++
		if line == "" {
			continue
		}
		if err := c.parseLine(line); err != nil {
			return nil, fmt.Errorf("error: line %d:\t%s:\n\t%w", lineNr, line, err)
		}
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("invalid cue sheet: %w", err)
	}
	slog.Info("cue sheet parsed correctly", "file", c.FileName, "format", c.Format, "lines", lineNr, "tracks", len(c.Tracks))
	return c, nil
}

func (c *CueSheet) parseLine(line string) error {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return fmt.Errorf("expected at least %d fields, got %d", 2, len(fields))
	}

	var err error
	command := fields[0]
	parameters := fields[1:]
	switch command {
	case "FILE":
		err = c.parseFile(parameters)
	case "TRACK":
		err = c.parseTrack(parameters)
	default:
		return fmt.Errorf("unexpected command: %s", command)
	}
	if err != nil {
		return fmt.Errorf("error parsing %q command: %w", command, err)
	}
	return nil
}

func assignValue[T comparable](val T, field *T) error {
	zero := reflect.Zero(reflect.TypeOf(*field)).Interface()
	if *field == zero {
		*field = val
		return nil
	}
	return fmt.Errorf("field already set: %v", *field)
}

func parseString(val string, field *string) error {
	val = strings.Trim(val, trimChars)
	return assignValue(val, field)
}

func (c *CueSheet) parseFile(parameters []string) error {
	if len(parameters) != fileParams {
		return fmt.Errorf("expected %d parameters, got %d", fileParams, len(parameters))
	}
	last := len(parameters) - 1
	if err := parseString(parameters[last], &c.Format); err != nil {
		return fmt.Errorf("error parsing FILE format: %w", err)
	}
	if err := parseString(strings.Join(parameters[:last], " "), &c.FileName); err != nil {
		return fmt.Errorf("error parsing FILE name: %w", err)
	}
	return nil
}

func (c *CueSheet) parseTrack(parameters []string) error {
	if len(parameters) != 2 {
		return fmt.Errorf("expected %d parameters, got %d", 2, len(parameters))
	}
	nr := parameters[0]
	typ := parameters[1]

	if err := c.isNextTrackNr(nr); err != nil {
		return fmt.Errorf("invalid track number: %w", err)
	}

	var track Track
	if err := parseString(typ, &track.Type); err != nil {
		return fmt.Errorf("error parsing track type: %w", err)
	}
	c.Tracks = append(c.Tracks, track)
	return nil
}

func (c *CueSheet) isNextTrackNr(nr string) error {
	trackNr, err := strconv.Atoi(nr)
	if err != nil {
		return fmt.Errorf("failed to parse track number: %w", err)
	}
	nextTrackNr := len(c.Tracks) + 1
	if trackNr != nextTrackNr {
		return fmt.Errorf("expected track number %d, got %d", nextTrackNr, trackNr)
	}
	if trackNr > maxTracks {
		return fmt.Errorf("cannot have more than %d tracks", maxTracks)
	}
	return nil
}

// validate checks if the cue sheet has FILE and at least one TRACK command with INDEX 01.
func (c *CueSheet) validate() error {
	if c.FileName == "" {
		return errors.New("missing file name")
	}
	if c.Format == "" {
		return errors.New("missing file format")
	}
	if len(c.Tracks) == 0 {
		return errors.New("missing tracks")
	}
	return nil
}