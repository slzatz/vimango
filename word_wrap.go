package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	// "unicode/utf8" // Not strictly needed

	"github.com/mattn/go-runewidth" // Import the key library
)

// osc66Regex matches OSC 66 sequences and captures the metadata and text content
// Format: \x1b]66;metadata;text\x07
var osc66Regex = regexp.MustCompile(`\x1b\]66;([^;]*);([^\x07]*)\x07`)

// parseOSC66Line parses a line containing an OSC 66 sequence and returns its components.
// Returns: ansiPrefix, osc66Meta, text, ansiSuffix, scale, ok
func parseOSC66Line(line string) (ansiPrefix, osc66Meta, text, ansiSuffix string, scale int, ok bool) {
	// Find the OSC 66 sequence
	match := osc66Regex.FindStringSubmatchIndex(line)
	if match == nil {
		return "", "", "", "", 0, false
	}

	// Extract components
	osc66Start := match[0]
	osc66End := match[1]
	osc66Meta = line[match[2]:match[3]]
	text = line[match[4]:match[5]]

	ansiPrefix = line[:osc66Start]
	ansiSuffix = line[osc66End:]

	// Parse scale from metadata (e.g., "s=2" or "s=2:w=0")
	scale = 1 // default
	for _, part := range strings.Split(osc66Meta, ":") {
		if strings.HasPrefix(part, "s=") {
			if s, err := strconv.Atoi(part[2:]); err == nil && s > 0 {
				scale = s
			}
		}
	}

	return ansiPrefix, osc66Meta, text, ansiSuffix, scale, true
}

// wrapOSC66Line wraps a line containing an OSC 66 sequence at the effective width.
// Each wrapped segment becomes its own OSC 66 sequence with the same styling.
// Extra newlines are added between segments to account for the vertical space
// consumed by scaled text (scale=2 means each line takes 2 rows).
func wrapOSC66Line(line string, limit int) []string {
	ansiPrefix, osc66Meta, text, ansiSuffix, scale, ok := parseOSC66Line(line)
	if !ok {
		// Not a valid OSC 66 line, return as-is
		return []string{line}
	}

	// Calculate effective width accounting for scale
	effectiveLimit := limit / scale
	if effectiveLimit < 1 {
		effectiveLimit = 1
	}

	// Calculate visible width of text
	textWidth := 0
	for _, r := range text {
		textWidth += runewidth.RuneWidth(r)
	}

	// If it fits, return as-is
	// Note: HeadingElement.Finish() already adds trailing newlines for scale height
	// so we don't need to add them here for the no-wrap case
	if textWidth <= effectiveLimit {
		return []string{line}
	}

	// Wrap the text at effective limit
	var wrappedSegments []string
	var currentLine strings.Builder
	currentWidth := 0

	for _, r := range text {
		rw := runewidth.RuneWidth(r)

		// Check if adding this rune would exceed the limit
		if currentWidth+rw > effectiveLimit && currentLine.Len() > 0 {
			// Finish current segment
			wrappedSegments = append(wrappedSegments, currentLine.String())
			currentLine.Reset()
			currentWidth = 0
		}

		currentLine.WriteRune(r)
		currentWidth += rw
	}

	// Don't forget the last segment
	if currentLine.Len() > 0 {
		wrappedSegments = append(wrappedSegments, currentLine.String())
	}

	// Build final lines with proper spacing for scale
	var wrappedLines []string
	for i, segment := range wrappedSegments {
		wrappedLine := fmt.Sprintf("%s\x1b]66;%s;%s\x07%s", ansiPrefix, osc66Meta, segment, ansiSuffix)

		// Add extra newlines after each line to account for scale height
		// (scale-1 extra newlines because one newline comes from the line break itself)
		// But don't add extra newlines after the very last segment
		if i < len(wrappedSegments)-1 {
			for j := 1; j < scale; j++ {
				wrappedLine += "\n"
			}
		}
		wrappedLines = append(wrappedLines, wrappedLine)
	}

	return wrappedLines
}

// SegmentType indicates whether a text segment contains words or spaces
type SegmentType int

const (
	SegmentWord SegmentType = iota
	SegmentSpace
)

// TextSegment represents a portion of text that is either words or spaces
type TextSegment struct {
	Type    SegmentType
	Content string
}

// parseWordsWithSpaces parses a line into alternating word and space segments,
// preserving the exact spacing between words
func parseWordsWithSpaces(line string) []TextSegment {
	if len(line) == 0 {
		return []TextSegment{}
	}

	var segments []TextSegment
	var currentSegment strings.Builder
	var currentType SegmentType
	var inWord bool

	// Determine initial state based on first character
	firstRune := rune(line[0])
	if firstRune == ' ' || firstRune == '\t' {
		currentType = SegmentSpace
		inWord = false
	} else {
		currentType = SegmentWord
		inWord = true
	}

	for _, r := range line {
		isSpace := (r == ' ' || r == '\t')

		if isSpace && inWord {
			// Transition from word to space
			if currentSegment.Len() > 0 {
				segments = append(segments, TextSegment{
					Type:    currentType,
					Content: currentSegment.String(),
				})
				currentSegment.Reset()
			}
			currentType = SegmentSpace
			inWord = false
		} else if !isSpace && !inWord {
			// Transition from space to word
			if currentSegment.Len() > 0 {
				segments = append(segments, TextSegment{
					Type:    currentType,
					Content: currentSegment.String(),
				})
				currentSegment.Reset()
			}
			currentType = SegmentWord
			inWord = true
		}

		currentSegment.WriteRune(r)
	}

	// Add the final segment
	if currentSegment.Len() > 0 {
		segments = append(segments, TextSegment{
			Type:    currentType,
			Content: currentSegment.String(),
		})
	}

	return segments
}

func visibleWidth(s string) int {
	width := 0
	// States: 0=Normal, 1=Saw ESC, 2=Inside CSI, 3=Inside OSC, 4=Inside APC
	state := 0
	var prevRune rune

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
				state = 2 // CSI sequence
			} else if r == ']' {
				state = 3 // OSC sequence (e.g., OSC 66 text sizing)
			} else if r == '_' {
				state = 4 // APC sequence (e.g., Kitty graphics)
			} else {
				state = 0 // Unknown/short escape, reset
			}
		case 2: // Inside CSI sequence - ends with letter @ through ~
			if r >= '@' && r <= '~' {
				state = 0
			}
		case 3: // Inside OSC sequence - ends with BEL (\x07) or ST (\x1b\)
			if r == '\x07' {
				state = 0
			} else if prevRune == '\x1b' && r == '\\' {
				state = 0
			}
		case 4: // Inside APC sequence - ends with ST (\x1b\)
			if prevRune == '\x1b' && r == '\\' {
				state = 0
			}
		}
		prevRune = r
	}
	return width
}

// breakWord takes a string potentially containing escape codes and breaks
// it into segments that fit within the limit. Escape codes are preserved
// but do not count towards the width limit.
func breakWord(word string, limit int) []string {
	if visibleWidth(word) <= limit {
		return []string{word}
	}

	var segments []string
	var currentSegment strings.Builder
	currentSegmentWidth := 0
	// States: 0=Normal, 1=Saw ESC, 2=Inside CSI, 3=Inside OSC, 4=Inside APC
	state := 0
	var prevRune rune

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
				state = 2 // CSI sequence
			} else if r == ']' {
				state = 3 // OSC sequence
			} else if r == '_' {
				state = 4 // APC sequence
			} else {
				state = 0 // Unknown/short escape, reset
			}
		case 2: // Inside CSI sequence
			isEscapeChar = true
			if r >= '@' && r <= '~' {
				state = 0 // End of sequence
			}
		case 3: // Inside OSC sequence - ends with BEL or ST
			isEscapeChar = true
			if r == '\x07' {
				state = 0
			} else if prevRune == '\x1b' && r == '\\' {
				state = 0
			}
		case 4: // Inside APC sequence - ends with ST
			isEscapeChar = true
			if prevRune == '\x1b' && r == '\\' {
				state = 0
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

		prevRune = r
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

// detectListItemIndent analyzes a line to detect if it's a list item and returns
// the hanging indent width (visible width up to and including the space after bullet/number).
// Returns 0 if not a list item.
func detectListItemIndent(line string) int {
	// Strip ANSI escape codes to analyze visible content
	visibleLine := stripANSI(line)
	if len(visibleLine) == 0 {
		return 0
	}

	// Count leading spaces
	leadingSpaces := 0
	for _, r := range visibleLine {
		if r == ' ' {
			leadingSpaces++
		} else if r == '\t' {
			leadingSpaces += 4 // Assume tab = 4 spaces
		} else {
			break
		}
	}

	// Get content after leading spaces
	content := visibleLine[leadingSpaces:]
	if len(content) == 0 {
		return 0
	}

	// Check for unordered list bullets: •, -, *, ◦, ▪, ▸, ►
	// These are typically followed by a space
	bullets := []rune{'•', '-', '*', '◦', '▪', '▸', '►', '○', '●'}
	contentRunes := []rune(content)
	firstRune := contentRunes[0]
	for _, bullet := range bullets {
		if firstRune == bullet {
			// Check if followed by space (use runes, not bytes)
			if len(contentRunes) > 1 && contentRunes[1] == ' ' {
				// Hanging indent = leading spaces + bullet width + space
				return leadingSpaces + runewidth.RuneWidth(bullet) + 1
			}
		}
	}

	// Check for ordered list: digits followed by . or ) and space
	// e.g., "1. ", "10. ", "1) "
	digitCount := 0
	for _, r := range content {
		if r >= '0' && r <= '9' {
			digitCount++
		} else {
			break
		}
	}

	if digitCount > 0 && digitCount < len(content) {
		afterDigits := content[digitCount:]
		// Check for ". " or ") " pattern
		if len(afterDigits) >= 2 && (afterDigits[0] == '.' || afterDigits[0] == ')') && afterDigits[1] == ' ' {
			// Hanging indent = leading spaces + digits + delimiter + space
			return leadingSpaces + digitCount + 2
		}
	}

	// Check for task list items: [ ] or [x] or [✓]
	if len(content) >= 4 && content[0] == '[' {
		// Look for closing bracket
		closeBracket := strings.Index(content, "] ")
		if closeBracket > 0 && closeBracket <= 3 {
			// Hanging indent = leading spaces + [x] + space
			return leadingSpaces + closeBracket + 2
		}
	}

	return 0
}

// stripANSI removes ANSI escape sequences from a string
// Handles CSI (\x1b[), OSC (\x1b]), and APC (\x1b_) sequences
func stripANSI(s string) string {
	var result strings.Builder
	// States: 0=Normal, 1=Saw ESC, 2=Inside CSI, 3=Inside OSC, 4=Inside APC
	state := 0
	var prevRune rune

	for _, r := range s {
		switch state {
		case 0: // Normal text
			if r == '\x1b' {
				state = 1
			} else {
				result.WriteRune(r)
			}
		case 1: // Saw ESC
			if r == '[' {
				state = 2 // CSI sequence
			} else if r == ']' {
				state = 3 // OSC sequence
			} else if r == '_' {
				state = 4 // APC sequence
			} else {
				state = 0 // Unknown escape, reset
			}
		case 2: // Inside CSI sequence - ends with letter @ through ~
			if r >= '@' && r <= '~' {
				state = 0
			}
		case 3: // Inside OSC sequence - ends with BEL (\x07) or ST (\x1b\)
			if r == '\x07' {
				state = 0
			} else if prevRune == '\x1b' && r == '\\' {
				state = 0
			}
		case 4: // Inside APC sequence - ends with ST (\x1b\)
			if prevRune == '\x1b' && r == '\\' {
				state = 0
			}
		}
		prevRune = r
	}
	return result.String()
}

// WordWrap wraps the given text to the specified limit, respecting ANSI escape codes,
// preserving existing newline characters, breaking long words, and applying hanging
// indents for list items with an additional offset (used for :help).
func WordWrap(text string, limit int, hangingIndentOffset int) string {
	if limit <= 0 { // Cannot wrap to zero or negative width
		return text // Or handle as an error
	}

	// DEBUG: Log input to WordWrap
	if strings.Contains(text, "\x1b]66;") {
		if f, err := os.OpenFile("/tmp/osc66_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			fmt.Fprintf(f, "DEBUG WordWrap: found OSC66 in input, limit=%d\n", limit)
			logLen := 500
			if len(text) < logLen {
				logLen = len(text)
			}
			fmt.Fprintf(f, "DEBUG WordWrap input: %q\n", text[:logLen])
			f.Close()
		}
	}

	var finalResult strings.Builder
	originalLines := strings.Split(text, "\n")

	for i, line := range originalLines {
		// Skip wrapping for lines containing kitty Unicode placeholders (U+10EEEE)
		// These are image placeholder grids that must not be broken
		if strings.Contains(line, string(rune(0x10EEEE))) {
			finalResult.WriteString(line)
			if i < len(originalLines)-1 {
				finalResult.WriteByte('\n')
			}
			continue
		}

		// Handle OSC 66 text sizing sequences specially - wrap at effective width
		if strings.Contains(line, "\x1b]66;") {
			wrappedOSC66 := wrapOSC66Line(line, limit)
			for j, wrappedLine := range wrappedOSC66 {
				finalResult.WriteString(wrappedLine)
				if j < len(wrappedOSC66)-1 {
					finalResult.WriteByte('\n')
				}
			}
			if i < len(originalLines)-1 {
				finalResult.WriteByte('\n')
			}
			continue
		}

		// Detect if this is a list item and get hanging indent width
		hangingIndent := detectListItemIndent(line) + hangingIndentOffset
		hangingIndentStr := ""
		if hangingIndent > 0 {
			hangingIndentStr = strings.Repeat(" ", hangingIndent)
		}

		var currentLineBuilder strings.Builder
		currentLineWidth := 0
		isFirstLineOfParagraph := true

		segments := parseWordsWithSpaces(line)

		for _, segment := range segments {
			segmentWidth := visibleWidth(segment.Content)

			// Handle space segments
			if segment.Type == SegmentSpace {
				// Include spaces if they fit within the limit
				// This preserves both inter-word spacing and leading indentation
				if currentLineWidth+segmentWidth <= limit {
					currentLineBuilder.WriteString(segment.Content)
					currentLineWidth += segmentWidth
				}
				// If spaces don't fit, skip them (trailing spaces at line breaks)
				continue
			}

			// Handle word segments (original word logic)
			word := segment.Content
			wordWidth := segmentWidth

			// Calculate effective limit for continuation lines (accounting for hanging indent)
			effectiveLimit := limit
			if !isFirstLineOfParagraph && hangingIndent > 0 {
				effectiveLimit = limit - hangingIndent
			}

			// --- Check if word needs breaking ---
			if wordWidth > effectiveLimit {
				// Flush existing line content before breaking the word
				if currentLineBuilder.Len() > 0 {
					finalResult.WriteString(currentLineBuilder.String())
					finalResult.WriteByte('\n')
					currentLineBuilder.Reset()
					isFirstLineOfParagraph = false
				}

				// Break the long word
				brokenSegments := breakWord(word, effectiveLimit)

				// Add segments to result
				for segIdx, segment := range brokenSegments {
					if segIdx > 0 {
						finalResult.WriteByte('\n')
						if hangingIndent > 0 {
							finalResult.WriteString(hangingIndentStr)
						}
					} else if !isFirstLineOfParagraph && hangingIndent > 0 {
						// First segment of broken word on continuation line
						finalResult.WriteString(hangingIndentStr)
					}
					finalResult.WriteString(segment)
				}

				// Update current line width based on the *last* segment
				if len(brokenSegments) > 0 {
					currentLineWidth = visibleWidth(brokenSegments[len(brokenSegments)-1])
					if hangingIndent > 0 {
						currentLineWidth += hangingIndent
					}
				} else {
					currentLineWidth = 0
				}
				isFirstLineOfParagraph = false
				currentLineBuilder.Reset()

				continue // Skip normal fitting logic
			}

			// --- Normal word fitting logic ---
			if currentLineWidth+wordWidth <= limit {
				// Word fits
				currentLineBuilder.WriteString(word)
				currentLineWidth += wordWidth
			} else {
				// Word doesn't fit, finalize this wrapped line and start a new one
				finalResult.WriteString(currentLineBuilder.String())
				finalResult.WriteByte('\n') // Wrap break
				currentLineBuilder.Reset()
				isFirstLineOfParagraph = false

				// Apply hanging indent to continuation line
				if hangingIndent > 0 {
					currentLineBuilder.WriteString(hangingIndentStr)
					currentLineWidth = hangingIndent + wordWidth
				} else {
					currentLineWidth = wordWidth
				}
				currentLineBuilder.WriteString(word)
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
