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
	// trimChars are the characters to be trimmed from a string
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
var IndexCommand = Command{Name: "INDEX", ExactParams: 2}
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
	Index00 *IndexPoint
	Index01 *IndexPoint
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
			// skip empty lines and empty REM commands
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
	case IndexCommand.Name:
		err = c.parseIndex(parameters)
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

// assignValue assigns a value to a field if it is not already set.
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
	if err := FileCommand.validateNrParameters(len(parameters)); err != nil {
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
	if err := PerformerCommand.validateNrParameters(len(parameters)); err != nil {
		return fmt.Errorf("invalid PERFORMER parameters: %w", err)
	}
	if err := parseString(strings.Join(parameters, " "), &c.AlbumPerformer); err != nil {
		return fmt.Errorf("error parsing PERFORMER parameters: %w", err)
	}
	return nil
}

func (c *CueSheet) parseTrack(parameters []string) error {
	if err := TrackCommand.validateNrParameters(len(parameters)); err != nil {
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

func (c *CueSheet) parseIndex(parameters []string) error {
	if err := IndexCommand.validateNrParameters(len(parameters)); err != nil {
		return fmt.Errorf("invalid TRACK INDEX parameters: %w", err)
	}
	indexNrStr := parameters[0]
	indexPointStr := parameters[1]
	indexNr, err := strconv.Atoi(indexNrStr)
	if err != nil {
		return fmt.Errorf("failed to parse index number: %w", err)
	}
	idxToUpdate, err := c.getIndexToUpdate(indexNr)
	if err != nil {
		return fmt.Errorf("failed to get index %d to update: %w", indexNr, err)
	}
	err = idxToUpdate.update(indexPointStr)
	if err != nil {
		return fmt.Errorf("error updating index point: %w", err)
	}
	return nil
}

func (c *CueSheet) getIndexToUpdate(indexNr int) (*IndexPoint, error) {
	lastTrack := c.Tracks[len(c.Tracks)-1]
	newIndex := &IndexPoint{}
	if indexNr == 0 {
		if lastTrack.Index00 != nil {
			return nil, fmt.Errorf("track %d already has INDEX 00", len(c.Tracks)-1)
		}
		lastTrack.Index00 = newIndex
	} else if indexNr == 1 {
		if lastTrack.Index01 != nil {
			return nil, fmt.Errorf("track %d already has INDEX 01", len(c.Tracks)-1)
		}
		lastTrack.Index01 = newIndex
	} else {
		return nil, fmt.Errorf("expected index number 0 or 1, got %d", indexNr)
	}
	return newIndex, nil
}

func (c *CueSheet) parseTitle(parameters []string) error {
	if err := TitleCommand.validateNrParameters(len(parameters)); err != nil {
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
	if err := RemGenreCommand.validateNrParameters(len(parameters)); err != nil {
		return fmt.Errorf("invalid REM GENRE parameters: %w", err)
	}
	if err := parseString(strings.Join(parameters, " "), &c.Genre); err != nil {
		return fmt.Errorf("error parsing REM GENRE parameters: %w", err)
	}
	return nil
}

func (cmd *Command) validateNrParameters(parameters int) error {
	if cmd.ExactParams > 0 && parameters != cmd.ExactParams {
		return fmt.Errorf("expected %d parameters, got %d", cmd.ExactParams, parameters)
	}
	if cmd.MinParams > 0 && parameters < cmd.MinParams {
		return fmt.Errorf("expected at least %d parameters, got %d", cmd.MinParams, parameters)
	}
	return nil
}

// validate checks if the cue sheet has FILE with FORMAT and at least one TRACK command with INDEX 01.
// It also ensures that index points do not overlap.
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
	var indexPoints []*IndexPoint
	for i, track := range c.Tracks {
		if track.Type == "" {
			return fmt.Errorf("missing track TYPE in track %d", i+1)
		}
		if track.Index01 == nil {
			return fmt.Errorf("missing INDEX 01 in track %d", i+1)
		}
		if track.Index00 != nil {
			indexPoints = append(indexPoints, track.Index00)
		}
		indexPoints = append(indexPoints, track.Index01)
	}
	for i := range len(indexPoints) - 2 {
		currIdx := indexPoints[i]
		nextIndex := indexPoints[i+1]
		if nextIndex.before(currIdx) {
			return fmt.Errorf("overlapping index points %v and %v", currIdx, nextIndex)
		}
	}
	return nil
}

func (idx *IndexPoint) before(indexPoint *IndexPoint) bool {
	if idx.Timestamp < indexPoint.Timestamp {
		return true
	}
	if idx.Timestamp == indexPoint.Timestamp {
		return idx.Frame < indexPoint.Frame
	}
	return false
}

func (idx *IndexPoint) update(indexPoint string) error {
	var minutes, seconds, frames int
	if _, err := fmt.Sscanf(indexPoint, "%2d:%2d:%2d", &minutes, &seconds, &frames); err != nil {
		return fmt.Errorf("error parsing timestamp and frame: %w", err)
	}
	duration := time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	index := IndexPoint{Timestamp: duration, Frame: frames}
	*idx = index
	return nil
}
