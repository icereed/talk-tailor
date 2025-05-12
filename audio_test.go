package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// test for func splitAudio(file multipart.File) ([]string, error)
// setup: use the file from the testdata folder: test/fixtures/15mins.mp3
// test: check that the function returns 2 chunks
// test: check that the chunks are not empty
// test: check that the chunks are not the same
// test: check that the chunks are not the same as the original file

func TestSplitAudio(t *testing.T) {
	file, err := os.Open("test/fixtures/15mins.mp3")
	require.NoError(t, err)
	defer file.Close()

	chunks, err := splitAudio(file)
	require.NoError(t, err)
	require.Len(t, chunks, 2)

	for _, chunk := range chunks {
		assert.NotEmpty(t, chunk)
		assert.NotEqual(t, "testdata/15mins.mp3", chunk)
	}

	// check that the first chunk is longer than 8 minutes
	duration1, err := getAudioDuration(chunks[0])
	require.NoError(t, err)
	assert.Greater(t, duration1, 8*time.Minute)

	// check that the second chunk is longer than 3 minutes
	duration2, err := getAudioDuration(chunks[1])
	require.NoError(t, err)
	assert.Greater(t, duration2, 3*time.Minute)

	// check that all file sizes are lower than 25MB (the limit for the OpenAI Whisper API)
	for _, chunk := range chunks {
		fi, err := os.Stat(chunk)
		require.NoError(t, err)
		// get the size
		size := fi.Size()
		assert.Less(t, size, int64(25*1024*1024))
	}
}

func TestSplitAudio_Short(t *testing.T) {
	file, err := os.Open("test/fixtures/short.mp3")
	require.NoError(t, err)
	defer file.Close()

	chunks, err := splitAudio(file)
	require.NoError(t, err)
	require.Len(t, chunks, 1)

	for _, chunk := range chunks {
		assert.NotEmpty(t, chunk)
		assert.NotEqual(t, "testdata/short.mp3", chunk)
	}

	duration1, err := getAudioDuration(chunks[0])
	require.NoError(t, err)
	assert.Greater(t, duration1, 2*time.Second)
}

func TestParseSilenceTimestamps(t *testing.T) {
	testCases := []struct {
		name           string
		input          string
		expectedOutput []time.Duration
	}{
		{
			name: "no silence",
			input: `
			[graph 0 input from stream 0:0 @ 0x7fa422d101c0] tb:1/44100 samplefmt:s16 samplerate:44100 chlayout:0x3
			[silencedetect @ 0x7fa422d10d20] Setting 'n' to value '-50dB'
			[silencedetect @ 0x7fa422d10d20] Setting 'd' to value '0.5'
			[format_out_0_0 @ 0x7fa422d101c0] auto-inserting filter 'auto_resampler_0' between the filter 'Parsed_anull_0' and the filter 'format_out_0_0'
			[auto_resampler_0 @ 0x7fa422d11640] ch:2 chl:stereo fmt:s16 r:44100Hz -> ch:2 chl:stereo fmt:dbl r:44100Hz`,
			expectedOutput: []time.Duration{},
		},
		{
			name: "one silence",
			input: `
			[silencedetect @ 0x7fa422d10d20] silence_start: 2.5731
			[silencedetect @ 0x7fa422d10d20] silence_end: 3.5731 | silence_duration: 1`,
			expectedOutput: []time.Duration{time.Duration(2573*time.Millisecond + 100*time.Microsecond)},
		},
		{
			name: "multiple silences",
			input: `
			[silencedetect @ 0x7fa422d10d20] silence_start: 2.5731
			[silencedetect @ 0x7fa422d10d20] silence_end: 3.5731 | silence_duration: 1
			[silencedetect @ 0x7fa422d10d20] silence_start: 6.893
			[silencedetect @ 0x7fa422d10d20] silence_end: 7.893 | silence_duration: 1`,
			expectedOutput: []time.Duration{
				time.Duration(2573*time.Millisecond + 100*time.Microsecond),
				time.Duration(6893 * time.Millisecond),
			},
		},
		{
			name: "irregular input",
			input: `
			[silencedetect @ 0x7fa422d10d20] silence_start: 4.1234
			[silencedetect @ 0x7fa422d10d20] silence_end: 5.1234 | silence_duration: 1
			invalid_line
			[silencedetect @ 0x7fa422d10d20] silence_start: 8.6789
			[silencedetect @ 0x7fa422d10d20] silence_end: 9.6789 | silence_duration: 1`,
			expectedOutput: []time.Duration{
				time.Duration(4123*time.Millisecond + 400*time.Microsecond),
				time.Duration(8678*time.Millisecond + 900*time.Microsecond),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := parseSilenceTimestamps(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, output)
		})
	}
}
