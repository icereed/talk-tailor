package main

import (
	"strings"

	gptTokenizer "github.com/wbrown/gpt_bpe"
)

const (
	ParagraphSeparator = "\n\n"
)

func getNumTokens(sentence string) int {
	tokenizer := gptTokenizer.NewGPT2Encoder()
	return len(*tokenizer.Encode(&sentence))
}

// splitParagraphs takes an input text and splits it into paragraphs when it sees two newlines.
// This function is used to divide the input text into smaller, more manageable units for further processing.
//
// Parameters:
//   - text: The input text to be split into paragraphs.
//
// Returns:
//   - A slice of strings representing the paragraphs in the input text.
func splitParagraphs(text string) []string {
	return strings.Split(text, ParagraphSeparator)
}

// splitSentences takes a paragraph and splits it into sentences based on full stops.
// This function is used to divide a paragraph into smaller, more manageable units for further processing.
//
// Parameters:
//   - paragraph: The paragraph to be split into sentences.
//
// Returns:
//   - A slice of strings representing the sentences in the paragraph.
func splitSentences(paragraph string) []string {
	trimmed := strings.TrimSuffix(paragraph, ".")
	return strings.Split(trimmed, ".")
}

// splitSentence takes a single sentence and splits it into words based on whitespace characters.
// This function is used to divide a sentence into smaller, more manageable units for further processing.
//
// Parameters:
//   - sentence: The sentence to be split into words.
//
// Returns:
//   - A slice of strings representing the words in the sentence.
func splitSingleSentence(sentence string) []string {
	return strings.Fields(sentence)
}

// appendPart adds a part to the parts slice if it is not an empty or whitespace-only string.
// This function ensures that only non-empty parts are included in the resulting parts slice.
//
// Parameters:
//   - parts: The existing slice of strings representing the previously generated parts.
//   - part: The part to append to the parts slice.
//
// Returns:
//   - The updated parts slice with the new part appended if the part is non-empty and contains non-whitespace characters;
//     otherwise, returns the original parts slice unchanged.
func appendPart(parts []string, part string) []string {
	trimmed := strings.TrimSpace(part)
	if trimmed != "" {
		return append(parts, trimmed)
	}
	return parts
}

// appendSentenceToPart concatenates a sentence to an existing part, ensuring that the sentence ends with a period.
// This function is used to build a part from multiple sentences while maintaining proper punctuation.
//
// Parameters:
//   - part: The existing part to which the sentence should be appended.
//   - sentence: The sentence to append to the part.
//
// Returns:
//   - A string representing the combined part and sentence, with the sentence ending in a period.
func appendSentenceToPart(part, sentence string) string {
	if !strings.HasSuffix(sentence, ".") {
		sentence += "."
	}
	return part + sentence
}

// handleLongSentence is responsible for splitting a long sentence into shorter parts based on the maxTokens limit.
// The function ensures that the resulting parts maintain proper punctuation and that each part does not exceed the
// token limit. It is needed for cases when a single sentence is too long to fit within the token limit on its own.
//
// Parameters:
//   - parts: A slice of strings representing the previously generated parts from the input text.
//   - currentPart: The current part being processed.
//   - sentence: The long sentence to be split into shorter parts.
//   - maxTokens: The maximum allowed token count for each part.
//
// Returns:
//   - A slice of strings representing the parts split from the input text.
func handleLongSentence(parts []string, currentPart, sentence string, maxTokens int) ([]string, string) {
	maxTokens -= 1 // Account for the period at the end of the sentence.

	words := splitSingleSentence(sentence)
	var currentSentence string

	for index, word := range words {
		word = addPeriodToLastWordIfNeeded(words, index, word)
		tempSentence := currentSentence + " " + word

		if getNumTokens(tempSentence) <= maxTokens {
			currentSentence = tempSentence
		} else {
			parts = appendPart(parts, currentSentence)
			currentSentence = word
		}
	}

	return appendPart(parts, currentSentence), ""
}

// addPeriodToLastWordIfNeeded appends a period to the last word of a sentence if it is missing.
// This function is needed to ensure that the resulting sentence parts retain proper punctuation
// after splitting the long sentence based on token limits.
//
// Parameters:
//   - words: A slice of strings representing the individual words in a sentence.
//   - index: The index of the current word in the 'words' slice.
//   - word: The current word being processed in the sentence.
//
// Returns:
//   - The input 'word' with a period appended if it is the last word in the sentence and
//     does not already have a period; otherwise, returns the input 'word' unmodified.
func addPeriodToLastWordIfNeeded(words []string, index int, word string) string {
	if index == len(words)-1 && !strings.HasSuffix(word, ".") {
		return word + "."
	}
	return word
}

// splitLongString takes a long input text and splits it into parts with a token count less than the specified maxTokens.
// The function first attempts to split the text based on paragraphs, and if a paragraph is too long, it splits it based
// on sentences. If a single sentence is still too long, the function splits the sentence at word boundaries while
// maintaining proper punctuation.
//
// Parameters:
//   - text: The input text to be split into parts.
//   - maxTokens: The maximum allowed token count for each part.
//
// Returns:
//   - A slice of strings representing the split parts of the input text, each with a token count less than maxTokens.
func splitLongString(text string, maxTokens int) []string {
	paragraphs := splitParagraphs(text)

	var parts []string
	currentPart := ""

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		paragraphTokens := getNumTokens(paragraph)

		if paragraphTokens > maxTokens {
			sentences := splitSentences(paragraph)

			for _, sentence := range sentences {
				sentence = strings.TrimSuffix(sentence, ".")
				sentenceTokens := getNumTokens(sentence)

				if sentenceTokens > maxTokens {
					parts, currentPart = handleLongSentence(parts, currentPart, sentence, maxTokens)
				} else {
					if getNumTokens(appendSentenceToPart(currentPart, sentence)) <= maxTokens {
						currentPart = appendSentenceToPart(currentPart, sentence)
					} else {
						parts = appendPart(parts, currentPart)
						currentPart = appendSentenceToPart("", sentence)
					}
				}
			}
		} else {
			if getNumTokens(currentPart+ParagraphSeparator+paragraph) <= maxTokens {
				currentPart += ParagraphSeparator + paragraph
			} else {
				parts = appendPart(parts, currentPart)
				currentPart = paragraph
			}
		}
	}

	return appendPart(parts, currentPart)
}
