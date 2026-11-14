package pane

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// ClipColumns returns the part of s that falls in display columns
// [from, from+width) and how many columns it fills. Titles are measured in
// display columns on screen but stored as UTF-8, so slicing them by titlecols
// cut multi-byte characters in half. CSI escapes (the fts highlight markers)
// take no columns and are all kept, so a highlight the clip cuts off is still
// closed. A wide rune straddling an edge becomes spaces for its visible part.
func ClipColumns(s string, from, width int) (string, int) {
	var b strings.Builder
	end := from + width
	w, cols := 0, 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			j = min(j+1, len(s))
			b.WriteString(s[i:j])
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := runewidth.RuneWidth(r)
		switch {
		case w >= from && w+rw <= end:
			b.WriteString(s[i : i+size])
			cols += rw
		case w < end && w+rw > from:
			n := min(w+rw, end) - max(w, from)
			b.WriteString(strings.Repeat(" ", n))
			cols += n
		}
		w += rw
		i += size
	}
	return b.String(), cols
}

// RuneStart backs c up to the first byte of the rune it lands inside, so a
// column that arrives mid-rune cannot produce a torn slice.
func RuneStart(row string, c int) int {
	if c >= len(row) {
		return len(row)
	}
	if c < 0 {
		return 0
	}
	for c > 0 && !utf8.RuneStart(row[c]) {
		c--
	}
	return c
}

// InclusiveEnd returns the byte offset just past the rune covering c. A visual
// selection includes the character under the cursor and vim reports that
// cursor as a byte offset, so the end of the selection has to step over a
// whole rune rather than a single byte.
func InclusiveEnd(row string, c int) int {
	if c >= len(row) {
		return len(row)
	}
	c = RuneStart(row, c)
	_, size := utf8.DecodeRuneInString(row[c:])
	return c + size
}
