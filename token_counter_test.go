package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitLongString(t *testing.T) {
	t.Run("Test case 1: Basic test", func(t *testing.T) {
		text := "Short.\n\nThis is a simple paragraph.\n\nAnother paragraph with a longer sentence. This one has more tokens than the previous one. And. A few. Very. Short. Sentences."
		maxTokens := 12
		expected := []string{
			"Short.\n\nThis is a simple paragraph.",
			"Another paragraph with a longer sentence.",
			"This one has more tokens than the previous one. And.",
			"A few. Very. Short. Sentences.",
		}
		actual := splitLongString(text, maxTokens)
		assert.Equal(t, expected, actual)

		// Assert that the number of tokens in each part is less than or equal to the max.
		for _, part := range actual {
			assert.LessOrEqual(t, getNumTokens(part), maxTokens)
		}
	})

	t.Run("Test case 2: Test long sentences", func(t *testing.T) {
		text := "This is a simple paragraph.\n\nAnother paragraph with a longer sentence. This one has more tokens than the previous one."
		maxTokens := 3
		expected := []string{
			"This is",
			"a simple",
			"paragraph.",
			"Another paragraph",
			"with a",
			"longer",
			"sentence.",
			"This one",
			"has more",
			"tokens",
			"than the",
			"previous",
			"one.",
		}
		actual := splitLongString(text, maxTokens)
		// Assert that the number of tokens in each part is less than or equal to the max.
		for _, part := range actual {
			assert.LessOrEqual(t, getNumTokens(part), maxTokens)
		}
		assert.Equal(t, expected, actual)
	})
}
