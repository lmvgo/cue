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
	"time"
)

const (
	// trimChars contains the characters to be trimmed from a string,
	// which are: space, double quote, tab, newline.
	trimChars = " " + `"` + "\t" + "\n"

	minLineFields = 2

	fileParams  = 2
	trackParams = 2
	indexParams = 2

	maxTracks = 99
)

type IndexPoint struct {
	Frame     int
	Timestamp time.Duration
}

// Track represents a single track in a cue sheet file.
// Required fields: Index01, Type.
type Track struct {
	Type    string
	Index01 IndexPoint
}

// CueSheet represents the contents of a cue sheet file.
// Required fields: FileName, Format, Tracks.
type CueSheet struct {
	AlbumPerformer string
	Format         string
	FileName       string
	Tracks         []Track
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
			return nil, fmt.Errorf("line %d:\t%s:\n\t%w", lineNr, line, err)
		}
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("invalid cue sheet: %w", err)
	}
	slog.Info("cue sheet parsed correctly", "lines", lineNr, "file", c.FileName, "format", c.Format, "tracks", len(c.Tracks))
	return c, nil
}

func (c *CueSheet) parseLine(line string) error {
	fields := strings.Fields(line)
	if len(fields) < minLineFields {
		return fmt.Errorf("expected at least %d fields, got %d", minLineFields, len(fields))
	}

	var err error
	command := fields[0]
	parameters := fields[1:]
	switch command {
	case "FILE":
		err = c.parseFile(parameters)
	case "PERFORMER":
		err = c.parsePerformer(parameters)
	case "TRACK":
		err = c.parseTrack(parameters)
	case "INDEX":
		err = c.parseIndex(parameters)
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
	if *field != zero {
		return fmt.Errorf("field already set: %v", *field)
	}
	*field = val
	return nil
}

func parseString(val string, field *string) error {
	val = strings.Trim(val, trimChars)
	return assignValue(val, field)
}

func (c *CueSheet) parseFile(parameters []string) error {
	if len(parameters) != fileParams {
		return fmt.Errorf("FILE: expected %d parameters, got %d", fileParams, len(parameters))
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

func (c *CueSheet) parsePerformer(parameters []string) error {
	if err := parseString(strings.Join(parameters, " "), &c.AlbumPerformer); err != nil {
		return fmt.Errorf("error parsing PERFORMER parameters")
	}
	return nil
}

func (c *CueSheet) parseTrack(parameters []string) error {
	if len(parameters) != trackParams {
		return fmt.Errorf("TRACK: expected %d parameters, got %d", 2, len(parameters))
	}
	nr := parameters[0]
	typ := parameters[1]

	if err := c.isNextTrack(nr); err != nil {
		return fmt.Errorf("invalid track number: %w", err)
	}

	var track Track
	if err := parseString(typ, &track.Type); err != nil {
		return fmt.Errorf("error parsing track type: %w", err)
	}
	c.Tracks = append(c.Tracks, track)
	return nil
}

func (c *CueSheet) isNextTrack(nr string) error {
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

func (c *CueSheet) parseIndex(parameters []string) error {
	if len(parameters) != indexParams {
		return fmt.Errorf("INDEX: expected %d parameters, got %d", 2, len(parameters))
	}
	nr := parameters[0]
	indexPoint := parameters[1]

	indexNr, err := strconv.Atoi(nr)
	if err != nil {
		return fmt.Errorf("failed to parse index number: %w", err)
	}
	if indexNr != 1 {
		return fmt.Errorf("expected index number 1, got %d", indexNr)
	}

	var minutes, seconds, frames int
	if _, err = fmt.Sscanf(indexPoint, "%2d:%2d:%2d", &minutes, &seconds, &frames); err != nil {
		return fmt.Errorf("error parsing timestamp and frame: %w", err)
	}
	duration := time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	index := IndexPoint{Timestamp: duration, Frame: frames}
	c.Tracks[len(c.Tracks)-1].Index01 = index
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
	if err := c.validateTracks(); err != nil {
		return fmt.Errorf("invalid tracks: %w", err)
	}
	return nil
}

func (c *CueSheet) validateTracks() error {
	for i, track := range c.Tracks {
		if track.Type == "" {
			return errors.New("missing type")
		}
		if i < len(c.Tracks)-1 {
			var (
				timestamp = track.Index01.Timestamp
				frame     = track.Index01.Frame

				nextTrack     = c.Tracks[i+1]
				nextTimestamp = nextTrack.Index01.Timestamp
				nextFrame     = nextTrack.Index01.Frame
			)
			if timestamp > nextTimestamp || (timestamp == nextTimestamp && frame >= nextFrame) {
				return fmt.Errorf("overlapping indices in tracks %d and %d", i+1, i+2)
			}
		}
	}
	return nil
}
