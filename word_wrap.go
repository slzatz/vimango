package main

import (
	"strings"
	// "unicode/utf8" // Not strictly needed

	"github.com/mattn/go-runewidth" // Import the key library
)

func visibleWidth(s string) int {
	width := 0
	state := 0 // 0: Normal, 1: Saw ESC, 2: Inside CSI

	for _, r := range s {
		switch state {
		case 0: // Normal text
			if r == '\x1b' {
				state = 1
			} else {
				width += runewidth.RuneWidth(r)
			}
		case 1: // Saw ESC
			if r == '[' {
				state = 2
			} else {
				state = 0 // Assume short escape, reset
			}
		case 2: // Inside CSI sequence
			if r >= '@' && r <= '~' {
				state = 0 // End of sequence
			}
			// Characters inside escape sequence have no width
		}
	}
	return width
}

// breakWord takes a single word potentially containing escape codes and breaks
// it into segments that fit within the limit. Escape codes are preserved
// but do not count towards the width limit.
func breakWord(word string, limit int) []string {
	if visibleWidth(word) <= limit {
		return []string{word}
	}

	var segments []string
	var currentSegment strings.Builder
	currentSegmentWidth := 0
	state := 0 // 0: Normal, 1: Saw ESC, 2: Inside CSI

	for _, r := range word {
		runeWidth := 0 // Width of this specific rune (0 if escape)
		isEscapeChar := false

		// Determine rune width and track escape state
		switch state {
		case 0: // Normal text
			if r == '\x1b' {
				state = 1
				isEscapeChar = true
			} else {
				runeWidth = runewidth.RuneWidth(r)
			}
		case 1: // Saw ESC
			isEscapeChar = true
			if r == '[' {
				state = 2
			} else {
				state = 0 // Assume short escape, reset
			}
		case 2: // Inside CSI sequence
			isEscapeChar = true
			if r >= '@' && r <= '~' {
				state = 0 // End of sequence
			}
		}

		// Check if adding the rune (if it has width) exceeds the limit
		if !isEscapeChar && currentSegmentWidth+runeWidth > limit {
			// Check if the segment is not empty before adding it
            // This prevents adding empty segments if a break happens at the very beginning
            if currentSegment.Len() > 0 {
                segments = append(segments, currentSegment.String())
                currentSegment.Reset()
                currentSegmentWidth = 0
            }
            // Even if the current segment was empty, we reset the width
            // because the new rune starts a new line conceptually.
            currentSegmentWidth = 0

			// If the rune itself is wider than the limit (e.g., large CJK char, limit=1)
            // it will still be placed, potentially exceeding limit for this segment.
		}

		// Add the rune to the current segment
		currentSegment.WriteRune(r)

		// Update width only if it wasn't part of an escape sequence
		if !isEscapeChar {
			currentSegmentWidth += runeWidth
		}
	}

	// Add the final segment
	if currentSegment.Len() > 0 {
		segments = append(segments, currentSegment.String())
	}

	// Handle case where input word was empty or only escape codes
	if len(segments) == 0 && len(word) > 0 {
		segments = append(segments, word) // Preserve escape-only words
	} else if len(segments) == 0 && len(word) == 0 {
        return []string{} // Return empty slice if input word was empty
    }

	return segments
}

// WordWrap wraps the given text to the specified limit, respecting ANSI escape codes,
// preserving existing newline characters, and breaking long words.
func WordWrap(text string, limit int) string {
	if limit <= 0 { // Cannot wrap to zero or negative width
		return text // Or handle as an error
	}

	var finalResult strings.Builder
	originalLines := strings.Split(text, "\n")

	for i, line := range originalLines {
		var currentLineBuilder strings.Builder
		currentLineWidth := 0
		spaceWidth := runewidth.StringWidth(" ") // Usually 1

		words := strings.Fields(line)

		// Use blank identifier '_' as wordIdx is not needed
		for _, word := range words { // <--- FIXED HERE
			wordWidth := visibleWidth(word)

			// --- Check if word needs breaking ---
			if wordWidth > limit {
				// Flush existing line content before breaking the word
				if currentLineBuilder.Len() > 0 {
					finalResult.WriteString(currentLineBuilder.String())
					finalResult.WriteByte('\n')
					currentLineBuilder.Reset()
					// currentLineWidth reset below after breakWord
				}

				// Break the long word
				brokenSegments := breakWord(word, limit)

				// Add segments to result
				for segIdx, segment := range brokenSegments {
					if segIdx > 0 {
						finalResult.WriteByte('\n') // Add newline before subsequent segments
					}
					finalResult.WriteString(segment)
				}

				// Update current line width based on the *last* segment
				if len(brokenSegments) > 0 {
					currentLineWidth = visibleWidth(brokenSegments[len(brokenSegments)-1])
				} else {
					currentLineWidth = 0
				}
				// Reset builder as its content is conceptually transferred to finalResult
				currentLineBuilder.Reset()

				continue // Skip normal fitting logic
			}

			// --- Normal word fitting logic ---
			neededWidth := wordWidth
			// Add space width only if the current line already has content
			if currentLineWidth > 0 {
				neededWidth += spaceWidth
			}

			if currentLineWidth+neededWidth <= limit {
				// Word fits
				if currentLineWidth > 0 {
					currentLineBuilder.WriteByte(' ')
					currentLineWidth += spaceWidth
				}
				currentLineBuilder.WriteString(word)
				currentLineWidth += wordWidth
			} else {
				// Word doesn't fit, finalize this wrapped line and start a new one
				finalResult.WriteString(currentLineBuilder.String())
				finalResult.WriteByte('\n') // Wrap break
				currentLineBuilder.Reset()
				currentLineBuilder.WriteString(word)
				currentLineWidth = wordWidth
			}
		} // End of loop over words in the line

		// Add any remaining content from the currentLineBuilder for this original line
		if currentLineBuilder.Len() > 0 {
			finalResult.WriteString(currentLineBuilder.String())
		}

		// Re-insert the original newline delimiter
		if i < len(originalLines)-1 {
			finalResult.WriteByte('\n')
		}

	} // End of loop over original lines

	return finalResult.String()
}
