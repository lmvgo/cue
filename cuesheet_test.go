package cuesheetgo

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"path"
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
	expectedErr string
}

var minimalCueSheet = CueSheet{
	FileName: "sample.flac",
	Format:   "WAVE",
	Tracks: []*Track{
		{
			Type: "AUDIO",
		},
	},
}

var allCueSheet = CueSheet{
	AlbumPerformer: "Sample Album Artist",
	AlbumTitle:     "Sample Album Title",
	Date:           "1989",
	FileName:       "sample.flac",
	Format:         "WAVE",
	Genre:          "Heavy Metal",
	Tracks: []*Track{
		{
			Title: "Track 1",
			Type:  "AUDIO",
			Index01: IndexPoint{
				Frame:     0,
				Timestamp: time.Duration(1) * time.Second,
			},
		},
		{
			Title: "Track 2",
			Type:  "AUDIO",
			Index01: IndexPoint{
				Frame:     0,
				Timestamp: time.Duration(1) * time.Minute,
			},
		},
	},
}

var cueSheetWithTrackTitleAndNoAlbumTitle = CueSheet{
	FileName: "sample.flac",
	Format:   "WAVE",
	Tracks: []*Track{
		{
			Title: "Track 1",
			Type:  "AUDIO",
			Index01: IndexPoint{
				Frame:     0,
				Timestamp: time.Duration(1) * time.Second,
			},
		},
		{
			Title: "Track 2",
			Type:  "AUDIO",
			Index01: IndexPoint{
				Frame:     0,
				Timestamp: time.Duration(1) * time.Minute,
			},
		},
	},
}

var cueSheetWithInterleavedTrackTitles = CueSheet{
	FileName: "sample.flac",
	Format:   "WAVE",
	Tracks: []*Track{
		{
			Type: "AUDIO",
			Index01: IndexPoint{
				Frame:     0,
				Timestamp: time.Duration(1) * time.Second,
			},
		},
		{
			Title: "Track 2",
			Type:  "AUDIO",
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
			expectedErr: "missing file name",
		},
		{
			name:        "UnexpectedCommand",
			input:       open(t, "unexpected.cue"),
			expectedErr: "unexpected command: UNSUPPORTED",
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
			expectedErr: "field already set: WAVE",
		},
		{
			name:        "InsufficientFileParams",
			input:       open(t, path.Join("file", "insufficient.cue")),
			expectedErr: "expected 2 parameters, got 1",
		},
		{
			name:        "ExcessiveFileParams",
			input:       open(t, path.Join("file", "excessive.cue")),
			expectedErr: "expected 2 parameters, got 3",
		},
		{
			name:        "EmptyFileName",
			input:       open(t, path.Join("file", "empty_name.cue")),
			expectedErr: "missing file name",
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
			expectedErr: "expected 2 parameters, got 1",
		},
		{
			name:        "ExcessiveTrackParams",
			input:       open(t, path.Join("track", "excessive.cue")),
			expectedErr: "expected 2 parameters, got 3",
		},
		{
			name:        "MissingTracks",
			input:       open(t, path.Join("track", "missing.cue")),
			expectedErr: "missing tracks",
		},
		{
			name:        "UnorderedTracks",
			input:       open(t, path.Join("track", "unordered.cue")),
			expectedErr: "expected track number 1, got 2",
		},
		{
			name:        "RepeatedTracks",
			input:       open(t, path.Join("track", "repeated.cue")),
			expectedErr: "expected track number 2, got 1",
		},
		{
			name:        "NonNumericTrackNumber",
			input:       open(t, path.Join("track", "non_numeric.cue")),
			expectedErr: "failed to parse track number",
		},
		{
			name:        "ZeroTrackNumber",
			input:       open(t, path.Join("track", "zero.cue")),
			expectedErr: "expected track number 1, got 0",
		},
		{
			name:        "NegativeTrackNumber",
			input:       open(t, path.Join("track", "negative.cue")),
			expectedErr: "expected track number 1, got -1",
		},
		{
			name:        "SingleDigit",
			input:       open(t, path.Join("track", "single_digit.cue")),
			expectedErr: "expected 2 digits, got 1",
		},
		{
			name:        "MoreThanTwoDigits",
			input:       open(t, path.Join("track", "more_than_two_digits.cue")),
			expectedErr: "expected 2 digits, got 3",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, runTest(tc))
	}
}

func TestParseTrackIndexCommand(t *testing.T) {
	tcs := []testCase{
		{
			name:        "OverlappingFrames",
			input:       open(t, path.Join("index", "overlapping_frame.cue")),
			expectedErr: "overlapping indices in tracks 1 and 2",
		},
		{
			name:        "OverlappingTimestamps",
			input:       open(t, path.Join("index", "overlapping_timestamp.cue")),
			expectedErr: "overlapping indices in tracks 1 and 2",
		},
		{
			name:        "NonNumericIndexNumber",
			input:       open(t, path.Join("index", "non_numeric.cue")),
			expectedErr: "failed to parse index number",
		},
		{
			name:        "InvalidTimeFormat",
			input:       open(t, path.Join("index", "format.cue")),
			expectedErr: "error parsing timestamp and frame",
		},
		{
			name:        "UnorderedIndex",
			input:       open(t, path.Join("index", "unordered.cue")),
			expectedErr: "expected index number 1, got 2",
		},
		{
			name:        "InsufficientIndexParams",
			input:       open(t, path.Join("index", "insufficient.cue")),
			expectedErr: "expected 2 parameters, got 1",
		},
		{
			name:        "ExcessiveIndexParams",
			input:       open(t, path.Join("index", "excessive.cue")),
			expectedErr: "expected 2 parameters, got 3",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, runTest(tc))
	}
}

func TestParsePerformerCommand(t *testing.T) {
	tcs := []testCase{
		{
			name:        "RepeatedPerformer",
			input:       open(t, path.Join("performer", "repeated.cue")),
			expectedErr: "field already set: Sample Album Artist",
		},
		{
			name:        "EmptyPerformer",
			input:       open(t, path.Join("performer", "empty.cue")),
			expectedErr: "expected at least 1 parameters, got 0",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, runTest(tc))
	}
}

func TestParseTitleCommand(t *testing.T) {
	tcs := []testCase{
		{
			name:        "RepeatedAlbumTitle",
			input:       open(t, path.Join("title", "repeated.cue")),
			expectedErr: "field already set: Sample Album Title",
		},
		{
			name:        "EmptyAlbumTitle",
			input:       open(t, path.Join("title", "empty.cue")),
			expectedErr: "expected at least 1 parameters, got 0",
		},
		{
			name:        "RepeatedTrackTitleWithoutAlbumTitle",
			input:       open(t, path.Join("track", "title", "repeated_wo_album.cue")),
			expectedErr: "field already set: Sample track title",
		},
		{
			name:        "RepeatedTrackTitleWithAlbumTitle",
			input:       open(t, path.Join("track", "title", "repeated_w_album.cue")),
			expectedErr: "field already set: Sample track title",
		},
		{
			name:        "EmptyTrackTitle",
			input:       open(t, path.Join("track", "title", "empty.cue")),
			expectedErr: "expected at least 1 parameters, got 0",
		},
		{
			name:     "TrackTitleWithoutAlbumTitle",
			input:    open(t, path.Join("track", "title", "without_album.cue")),
			expected: cueSheetWithTrackTitleAndNoAlbumTitle,
		},
		{
			name:     "InterleavedTrackTitles",
			input:    open(t, path.Join("track", "title", "interleaved.cue")),
			expected: cueSheetWithInterleavedTrackTitles,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, runTest(tc))
	}
}

func TestParseRemGenreCommand(t *testing.T) {
	tcs := []testCase{
		{
			name:        "RepeatedGenre",
			input:       open(t, path.Join("genre", "repeated.cue")),
			expectedErr: "field already set: Rock",
		},
		{
			name:        "EmptyGenre",
			input:       open(t, path.Join("genre", "empty.cue")),
			expectedErr: "expected at least 1 parameters, got 0",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, runTest(tc))
	}
}

func TestParseRemDateCommand(t *testing.T) {
	tcs := []testCase{
		{
			name:        "RepeatedDate",
			input:       open(t, path.Join("date", "repeated.cue")),
			expectedErr: "field already set: 1974-01",
		},
		{
			name:        "EmptyDate",
			input:       open(t, path.Join("date", "empty.cue")),
			expectedErr: "expected at least 1 parameters, got 0",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, runTest(tc))
	}
}

func runTest(tc testCase) func(t *testing.T) {
	return func(t *testing.T) {
		cueSheet, err := Parse(tc.input)
		if tc.expectedErr != "" {
			require.ErrorContains(t, err, tc.expectedErr)
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
