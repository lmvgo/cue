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

	maxTracks = 99
)

type Command struct {
	Name        string
	ExactParams int
	MinParams   int
}

var FileCommand = Command{Name: "FILE", ExactParams: 2}
var PerformerCommand = Command{Name: "PERFORMER", MinParams: 1}
var TitleCommand = Command{Name: "TITLE", MinParams: 1}
var TrackCommand = Command{Name: "TRACK", ExactParams: 2}
var TrackIndexCommand = Command{Name: "INDEX", ExactParams: 2}
var RemCommand = Command{Name: "REM", MinParams: 1}
var RemGenreCommand = Command{Name: "GENRE", MinParams: 1}

type IndexPoint struct {
	Frame     int
	Timestamp time.Duration
}

// Track represents a single track in a cue sheet file.
// Required fields: Index01, Type.
type Track struct {
	Title   string
	Type    string
	Index01 IndexPoint
}

// CueSheet represents the contents of a cue sheet file.
// Required fields: FileName, Format, Tracks.
type CueSheet struct {
	AlbumPerformer string
	AlbumTitle     string
	Format         string
	FileName       string
	Genre          string
	Tracks         []*Track
}

// Parse reads the cue sheet data from the provided reader and returns a parsed CueSheet struct.
func Parse(reader io.Reader) (*CueSheet, error) {
	scanner := bufio.NewScanner(reader)
	c := &CueSheet{Tracks: []*Track{}}

	var lineNr int
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), trimChars)
		lineNr++
		if line == "" || line == "REM" {
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
	var err error
	command := fields[0]
	parameters := fields[1:]
	switch strings.ToUpper(command) {
	case FileCommand.Name:
		err = c.parseFile(parameters)
	case PerformerCommand.Name:
		err = c.parsePerformer(parameters)
	case TrackCommand.Name:
		err = c.parseTrack(parameters)
	case TrackIndexCommand.Name:
		err = c.parseTrackIndex01(parameters)
	case TitleCommand.Name:
		err = c.parseTitle(parameters)
	case RemCommand.Name:
		err = c.parseRem(parameters)
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
	if err := FileCommand.validateParameters(len(parameters)); err != nil {
		return fmt.Errorf("invalid FILE parameters: %w", err)
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
	if err := PerformerCommand.validateParameters(len(parameters)); err != nil {
		return fmt.Errorf("invalid PERFORMER parameters: %w", err)
	}
	if err := parseString(strings.Join(parameters, " "), &c.AlbumPerformer); err != nil {
		return fmt.Errorf("error parsing PERFORMER parameters: %w", err)
	}
	return nil
}

func (c *CueSheet) parseTrack(parameters []string) error {
	if err := TrackCommand.validateParameters(len(parameters)); err != nil {
		return fmt.Errorf("invalid TRACK parameters: %w", err)
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
	c.Tracks = append(c.Tracks, &track)
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

func (c *CueSheet) parseTrackIndex01(parameters []string) error {
	if err := TrackIndexCommand.validateParameters(len(parameters)); err != nil {
		return fmt.Errorf("invalid TRACK INDEX parameters: %w", err)
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
	lastTrack := c.Tracks[len(c.Tracks)-1]
	return assignValue(index, &lastTrack.Index01)
}

func (c *CueSheet) parseTitle(parameters []string) error {
	if err := TitleCommand.validateParameters(len(parameters)); err != nil {
		return fmt.Errorf("invalid TITLE parameters: %w", err)
	}
	nrTracks := len(c.Tracks)
	if nrTracks == 0 {
		// no tracks yet - try setting album title
		if err := parseString(strings.Join(parameters, " "), &c.AlbumTitle); err != nil {
			return fmt.Errorf("error parsing album TITLE: %w", err)
		}
		return nil
	}
	currentTrack := c.Tracks[nrTracks-1]
	if err := parseString(strings.Join(parameters, " "), &currentTrack.Title); err != nil {
		// current track title is already set
		return fmt.Errorf("error parsing track %d TITLE: %w", nrTracks-1, err)
	}
	return nil
}

func (c *CueSheet) parseRem(parameters []string) error {
	var err error
	command := parameters[0]
	switch strings.ToUpper(command) {
	case "GENRE":
		err = c.parseGenre(parameters[1:])
	default:
		//TODO: handle REM comments
		return nil
	}
	if err != nil {
		return fmt.Errorf("error parsing REM %q command: %w", command, err)
	}
	return nil
}

func (c *CueSheet) parseGenre(parameters []string) error {
	if err := RemGenreCommand.validateParameters(len(parameters)); err != nil {
		return fmt.Errorf("invalid REM GENRE parameters: %w", err)
	}
	if err := parseString(strings.Join(parameters, " "), &c.Genre); err != nil {
		return fmt.Errorf("error parsing REM GENRE parameters: %w", err)
	}
	return nil
}

func (cmd *Command) validateParameters(parameters int) error {
	if cmd.ExactParams > 0 && parameters != cmd.ExactParams {
		return fmt.Errorf("expected %d parameters, got %d", cmd.ExactParams, parameters)
	}
	if cmd.MinParams > 0 && parameters < cmd.MinParams {
		return fmt.Errorf("expected at least %d parameters, got %d", cmd.MinParams, parameters)
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
	if err := c.validateTracks(); err != nil {
		return fmt.Errorf("invalid tracks: %w", err)
	}
	return nil
}

func (c *CueSheet) validateTracks() error {
	for i, track := range c.Tracks {
		if track.Type == "" {
			return errors.New("missing track type")
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
