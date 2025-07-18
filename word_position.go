package main

import (
	"fmt"
	"unicode"
)

// WordPosition represents the location of a word in the text.
type WordPosition struct {
	Start int    // 0-based start index (character) within the string
	End   int    // 0-based end index (character) within the string
	Word  string // The word itself (without punctuation)
}

func GetWordPositionsSingleLine(text string) []WordPosition {
	var wordPositions []WordPosition

	for i := 0; i < len(text); {
		// Skip whitespace and punctuation at the beginning of a potential word
		for i < len(text) && (unicode.IsSpace(rune(text[i])) || unicode.IsPunct(rune(text[i]))) {
			i++
		}

		// If we reached the end of the string, stop
		if i == len(text) {
			break
		}

		// Find the start of a word
		wordStart := i
		wordStartCharIndex := i

		// Find the end of the word (until space or punctuation)
		for i < len(text) && !unicode.IsSpace(rune(text[i])) && !unicode.IsPunct(rune(text[i])) {
			i++
		}

		wordEnd := i
		wordEndCharIndex := i - 1 // Adjust end index to be inclusive

		// Extract the word
		word := text[wordStart:wordEnd]

		// Add the word position to the list
		if len(word) > 0 {
			wordPositions = append(wordPositions, WordPosition{
				Start: wordStartCharIndex,
				End:   wordEndCharIndex,
				Word:  word,
			})
		}
	}

	return wordPositions
}

// GetWordAtIndex finds the word that contains the given character index.
// It returns the word string and its start and end indices within the input string.
// If the index is not within a word, it returns an empty string and -1, -1.
func GetWordAtIndex(text string, index int) (word string, startIndex int, endIndex int) {
	// Handle invalid index
	if index < 0 || index >= len(text) {
		return "", -1, -1
	}

	// If the character at the index is not a word character (space or punctuation),
	// return empty string and -1, -1
	if unicode.IsSpace(rune(text[index])) || unicode.IsPunct(rune(text[index])) {
		return "", -1, -1
	}

	// Find the start of the word by moving left until space or punctuation
	startIndex = index
	for startIndex > 0 && !unicode.IsSpace(rune(text[startIndex-1])) && !unicode.IsPunct(rune(text[startIndex-1])) {
		startIndex--
	}

	// Find the end of the word by moving right until space or punctuation
	endIndex = index
	for endIndex < len(text)-1 && !unicode.IsSpace(rune(text[endIndex+1])) && !unicode.IsPunct(rune(text[endIndex+1])) {
		endIndex++
	}

	// Extract the word
	word = text[startIndex : endIndex+1]

	return word, startIndex, endIndex
}

func test2() {
	text := "Hello, this is a text with words and some punctuation!"
	index := 15 // Index within the word "text"

	word, startIndex, endIndex := GetWordAtIndex(text, index)

	if word != "" {
		fmt.Printf("At index %d, the word is \"%s\" (from index %d to %d)\n", index, word, startIndex, endIndex)
	} else {
		fmt.Printf("At index %d, no word was found.\n", index)
	}

	// Test with an index within punctuation
	index = 5 // Index at the comma
	word, startIndex, endIndex = GetWordAtIndex(text, index)
	if word != "" {
		fmt.Printf("At index %d, the word is \"%s\" (from index %d to %d)\n", index, word, startIndex, endIndex)
	} else {
		fmt.Printf("At index %d, no word was found.\n", index)
	}
}

func test() {
	text := "Hello, this is a text with words and some punctuation!"
	wordPositions := GetWordPositionsSingleLine(text)

	for _, wp := range wordPositions {
		fmt.Printf("Start: %d, End: %d, Word: \"%s\"\n", wp.Start, wp.End, wp.Word)
	}
}
