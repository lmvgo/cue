package cuesheetgo

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

//go:embed testdata
var testdataFS embed.FS

type testCase struct {
	name        string
	input       io.Reader
	expected    CueSheet
	expectedErr error
}

var minimalCueSheet = CueSheet{
	FileName: "sample.flac",
	Format:   "WAVE",
	Tracks: []Track{
		{
			Type: "AUDIO",
		},
	},
}

var allCueSheet = CueSheet{
	FileName: "sample.flac",
	Format:   "WAVE",
	Tracks: []Track{
		{
			Type: "AUDIO",
			Index01: IndexPoint{
				Frame:     0,
				Timestamp: time.Duration(1) * time.Second,
			},
		},
		{
			Type: "AUDIO",
			Index01: IndexPoint{
				Frame:     0,
				Timestamp: time.Duration(1) * time.Minute,
			},
		},
	},
}

func TestParseCueSheets(t *testing.T) {
	tcs := []testCase{
		{
			name:     "MinimalCueSheet",
			input:    open(t, "minimal.cue"),
			expected: minimalCueSheet,
		},
		{
			name:     "AllFieldsCueSheet",
			input:    open(t, "all.cue"),
			expected: allCueSheet,
		},
		{
			name:        "EmptyCueSheet",
			input:       open(t, "empty.cue"),
			expectedErr: errors.New("missing file name"),
		},
		{
			name:        "UnexpectedCommand",
			input:       open(t, path.Join("command", "unexpected.cue")),
			expectedErr: errors.New("unexpected command: UNSUPPORTED"),
		},
		{
			name:        "InsufficientLineFields",
			input:       open(t, path.Join("command", "insufficient.cue")),
			expectedErr: errors.New("expected at least 2 fields, got 1"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, runTest(tc))
	}
}

func TestParseFileCommand(t *testing.T) {
	tcs := []testCase{
		{
			name:        "RepeatedFileCommand",
			input:       open(t, path.Join("file", "repeated.cue")),
			expectedErr: errors.New("field already set: WAVE"),
		},
		{
			name:        "InsufficientFileParams",
			input:       open(t, path.Join("file", "insufficient.cue")),
			expectedErr: errors.New("expected 2 parameters, got 1"),
		},
		{
			name:        "ExcessiveFileParams",
			input:       open(t, path.Join("file", "excessive.cue")),
			expectedErr: errors.New("expected 2 parameters, got 3"),
		},
		{
			name:        "EmptyFileName",
			input:       open(t, path.Join("file", "empty_name.cue")),
			expectedErr: errors.New("missing file name"),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, runTest(tc))
	}
}

func TestParseTrackCommand(t *testing.T) {
	tcs := []testCase{
		{
			name:        "InsufficientTrackParams",
			input:       open(t, path.Join("track", "insufficient.cue")),
			expectedErr: errors.New("expected 2 parameters, got 1"),
		},
		{
			name:        "ExcessiveTrackParams",
			input:       open(t, path.Join("track", "excessive.cue")),
			expectedErr: errors.New("expected 2 parameters, got 3"),
		},
		{
			name:        "MissingTracks",
			input:       open(t, path.Join("track", "missing.cue")),
			expectedErr: errors.New("missing tracks"),
		},
		{
			name:        "UnorderedTracks",
			input:       open(t, path.Join("track", "unordered.cue")),
			expectedErr: errors.New("expected track number 1, got 2"),
		},
		{
			name:        "NonNumericTrackNumber",
			input:       open(t, path.Join("track", "non_numeric.cue")),
			expectedErr: errors.New("failed to parse track number"),
		},
		{
			name:        "ExceedsMaxTracks",
			input:       strings.NewReader(generateExceedsMaxTracks()),
			expectedErr: errors.New("cannot have more than 99 tracks"),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, runTest(tc))
	}
}

func TestParseIndex(t *testing.T) {
	tcs := []testCase{
		{
			name:        "OverlappingFrames",
			input:       open(t, path.Join("index", "overlapping_frame.cue")),
			expectedErr: errors.New("overlapping indices in tracks 1 and 2"),
		},
		{
			name:        "OverlappingTimestamps",
			input:       open(t, path.Join("index", "overlapping_timestamp.cue")),
			expectedErr: errors.New("overlapping indices in tracks 1 and 2"),
		},
		{
			name:        "NonNumericIndexNumber",
			input:       open(t, path.Join("index", "non_numeric.cue")),
			expectedErr: errors.New("failed to parse index number"),
		},
		{
			name:        "InvalidTimeFormat",
			input:       open(t, path.Join("index", "format.cue")),
			expectedErr: errors.New("error parsing timestamp and frame"),
		},
		{
			name:        "UnorderedIndex",
			input:       open(t, path.Join("index", "unordered.cue")),
			expectedErr: errors.New("expected index number 1, got 2"),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, runTest(tc))
	}
}

func runTest(tc testCase) func(t *testing.T) {
	return func(t *testing.T) {
		cueSheet, err := Parse(tc.input)
		if tc.expectedErr != nil {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr.Error())
			fmt.Println(err)
			return
		}
		require.NoError(t, err)
		require.Equal(t, tc.expected, *cueSheet)
	}
}

func open(t *testing.T, p string) fs.File {
	file, err := testdataFS.Open(path.Join("testdata", p))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, file.Close())
	})
	return file
}

func generateExceedsMaxTracks() string {
	cueSheet := fmt.Sprintf("FILE test.flac WAVE\n")
	for i := range maxTracks + 1 {
		cueSheet += fmt.Sprintf("TRACK %02d AUDIO\n", i+1)
	}
	return cueSheet
}
