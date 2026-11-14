package main

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// How a row of the note is laid out on screen lines: where it wraps, and
// which byte of it is at which column. (The organizer's preview is wrapped
// by WordWrap, word_wrap.go, which works on rendered text.)

// wrapSegment is one screen line produced by wrapping a buffer row. start and
// end are byte offsets into the row (end exclusive) and always fall on rune
// boundaries.
type wrapSegment struct {
	start int
	end   int
}

// wrapRow breaks row into screen lines no wider than width *display columns*,
// breaking after the last space that fits when there is one. Measuring in
// display columns rather than bytes is what keeps multi-byte content — box
// drawing, arrows, accented text — from wrapping several times too often and
// from being sliced in the middle of a rune.
//
// A row that fits returns a single segment; an empty row returns a single
// empty segment, so the result is never empty.
func wrapRow(row string, width int) []wrapSegment {
	if width < 1 {
		width = 1
	}

	var segs []wrapSegment
	start := 0
	for start < len(row) {
		w := 0
		end := -1       // byte offset where this segment must stop, -1 = rest fits
		lastSpace := -1 // byte offset just past the last space that fits
		for i, r := range row[start:] {
			rw := runewidth.RuneWidth(r)
			if w+rw > width {
				end = start + i
				break
			}
			w += rw
			if r == ' ' {
				lastSpace = start + i + utf8.RuneLen(r)
			}
		}

		if end == -1 { // remainder of the row fits on this line
			segs = append(segs, wrapSegment{start, len(row)})
			return segs
		}

		if lastSpace > start {
			end = lastSpace
		}
		if end <= start { // a single rune wider than the whole line
			_, size := utf8.DecodeRuneInString(row[start:])
			end = start + size
		}
		segs = append(segs, wrapSegment{start, end})
		start = end
	}

	if len(segs) == 0 {
		segs = append(segs, wrapSegment{0, 0})
	}
	return segs
}

// tabWidth is the number of columns a tab occupies in the editor; the drawing
// code substitutes four spaces and the width helpers a four-character
// stand-in, so a tab never changes a row's length in bytes.
const tabWidth = 4

// expandedOffset maps byte offset c in row to its offset in the tab-expanded
// row, clamping c to the row.
func expandedOffset(row string, c int) int {
	if c > len(row) {
		c = len(row)
	}
	if c < 0 {
		c = 0
	}
	return c + (tabWidth-1)*strings.Count(row[:c], "\t")
}

// byteAtDisplayCol returns the byte offset of the rune covering display column
// col, or len(row) if the row ends before it. A wide rune straddling col
// counts as covering it.
func byteAtDisplayCol(row string, col int) int {
	w := 0
	for i, r := range row {
		rw := runewidth.RuneWidth(r)
		if w+rw > col {
			return i
		}
		w += rw
	}
	return len(row)
}

// byteAfterDisplayCol returns the byte offset just past the last rune that
// begins at or before display column col, so a wide rune straddling col is
// included whole.
func byteAfterDisplayCol(row string, col int) int {
	w := 0
	for i, r := range row {
		if w > col {
			return i
		}
		w += runewidth.RuneWidth(r)
	}
	return len(row)
}
