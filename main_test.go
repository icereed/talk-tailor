package main

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"

	openai "github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementation of the OpenAI Client
type mockOpenAIClient struct{}

func (m *mockOpenAIClient) CreateTranscription(ctx context.Context, request openai.AudioRequest) (response openai.AudioResponse, err error) {
	return openai.AudioResponse{
		Text: "mock transcription",
	}, nil
}

func (m *mockOpenAIClient) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error) {
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{Content: "mock corrected transcription"},
			},
		},
	}, nil
}

func TestSaveFile(t *testing.T) {
	file := createDummyMP3File(t)
	defer os.Remove(file.Name())

	tmpFilePath := "test_save_file.mp3"
	defer os.Remove(tmpFilePath)

	err := saveFile(file, tmpFilePath)
	assert.NoError(t, err)
	assert.FileExists(t, tmpFilePath)
}

func TestTranscribeChunks(t *testing.T) {
	chunks := []string{"chunk1.mp3", "chunk2.mp3"}
	mockClient := &mockOpenAIClient{}
	transcriptions := transcribeChunks(mockClient, chunks)
	assert.Len(t, transcriptions, len(chunks))

	for _, transcription := range transcriptions {
		assert.Equal(t, "mock transcription", transcription)
	}
}

func TestCorrectTranscription(t *testing.T) {
	transcription := "mock transcription"
	mockClient := &mockOpenAIClient{}
	correctedTranscription, err := correctTranscription(mockClient, transcription, tokensForCompletion)
	assert.NoError(t, err)
	assert.Equal(t, "mock corrected transcription", strings.TrimSpace(correctedTranscription))
}

func createDummyMP3File(t *testing.T) *os.File {
	data := []byte("ID3\x02\x00\x00\x00\x00\x00\x0A") // Minimal ID3v2 header
	file, err := ioutil.TempFile("", "test-*.mp3")
	require.NoError(t, err)

	_, err = file.Write(data)
	require.NoError(t, err)

	return file
}

func TestProcessTextInParallel(t *testing.T) {
	testCases := []struct {
		name           string
		options        TextProcessingOptions
		expectedResult string
		expectedErr    error
	}{
		{
			name: "process text in parallel successfully",
			options: TextProcessingOptions{
				Text:      "This is a test. This is only a test.",
				MaxTokens: 10,
				JoinSep:   " ",
				Processor: func(part string) (string, error) {
					return strings.ToUpper(part), nil
				},
			},
			expectedResult: "THIS IS A TEST. THIS IS ONLY A TEST.",
			expectedErr:    nil,
		},
		{
			name: "process text in parallel with error",
			options: TextProcessingOptions{
				Text:      "This is a test. This is only a test.",
				MaxTokens: 10,
				JoinSep:   " ",
				Processor: func(part string) (string, error) {
					return "", errors.New("an error occurred")
				},
			},
			expectedResult: "",
			expectedErr:    errors.New("an error occurred"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := processTextInParallel(tc.options)
			assert.Equal(t, tc.expectedResult, result)
			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProcessTextInParallelMultipleCalls(t *testing.T) {
	var callCounter int
	var counterMutex sync.Mutex

	// TextProcessor function that increments the call counter
	testProcessor := func(part string) (string, error) {
		counterMutex.Lock()
		callCounter++
		counterMutex.Unlock()
		return strings.ToUpper(part), nil
	}

	options := TextProcessingOptions{
		Text:      "This is a test. This is only a test.",
		MaxTokens: 5, // This will ensure multiple calls to the TextProcessor
		JoinSep:   " ",
		Processor: testProcessor,
	}

	_, err := processTextInParallel(options)
	assert.NoError(t, err)

	assert.Greater(t, callCounter, 1, "TextProcessor should be called multiple times")
}
