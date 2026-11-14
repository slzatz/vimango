package editor

import (
	"fmt"
	"strings"

	"github.com/slzatz/vimango/internal/frontmatter"
)

// Drawing the note's rows, with markdown highlighting (markdown.go).

// styledRow returns buffer row raw as it goes on screen, each byte in its
// style: tabs as four spaces, and a tab between the screen lines it wraps
// onto (the tab is the separator because no real one is left). Each screen
// line starts and ends in a reset state.
func (e *Editor) styledRow(raw string, styles []mdStyle) string {
	row := strings.ReplaceAll(raw, "\t", "    ")
	if len(row) != len(raw) { // a tab became four spaces: so did its style
		wide := make([]mdStyle, 0, len(row))
		for i := 0; i < len(raw); i++ {
			wide = append(wide, styles[i])
			if raw[i] == '\t' {
				wide = append(wide, styles[i], styles[i], styles[i])
			}
		}
		styles = wide
	}
	var sb strings.Builder
	for i, seg := range wrapRow(row, e.wrapWidth()) {
		if i > 0 {
			sb.WriteByte('\t')
		}
		styledSegment(&sb, row, styles, seg.start, seg.end)
	}
	return sb.String()
}

// visibleRows calls yield for each buffer row from the first visible one down,
// with the row as it is to be drawn (its screen lines separated by tabs),
// until yield returns false.
//
// Only the visible rows are styled. The rows above them are read just for
// where fenced code blocks open and close, which is cheap enough to do on
// every redraw: nothing is cached.
func (e *Editor) visibleRows(yield func(n int, row string) bool) {
	first := e.firstVisibleRow
	if first < 0 || first >= len(e.ss) {
		return
	}
	if !e.highlightSyntax {
		// Every row as body text, in the colour it has with highlighting
		// on. Sent with no colour at all, it took the terminal's default
		// foreground, a brighter white than the body text's grey, and the
		// whole note looked bold.
		for n := first; n < len(e.ss); n++ {
			if !yield(n, e.styledRow(e.ss[n], make([]mdStyle, len(e.ss[n])))) {
				return
			}
		}
		return
	}

	fm := frontmatter.Rows(e.ss)
	var b mdBlock
	for n := fm; n < first; n++ {
		b = b.next(e.ss[n])
	}
	for n := first; n < len(e.ss); n++ {
		var styles []mdStyle
		if n < fm {
			styles = make([]mdStyle, len(e.ss[n])) // frontmatter is plain text
		} else {
			styles, b = rowStyles(e.ss[n], b)
		}
		if !yield(n, e.styledRow(e.ss[n], styles)) {
			return
		}
	}
}

// drawRows writes the visible part of the note: each row's screen lines, and
// its number in the margin if line numbers are on.
func (e *Editor) drawRows(pab *strings.Builder) {
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.LeftMargin)
	fmt.Fprintf(pab, "\x1b[?25l\x1b[%d;%dH", e.TopMargin, e.LeftMargin+1)

	// The numbers are collected separately and written after the text, in a
	// second pass down the margin.
	var numCols strings.Builder
	indent := ""
	if e.numberLines {
		// a rectangle of background for the number column
		fmt.Fprintf(&numCols, "\x1b[2*x\x1b[%d;%d;%d;%d;48;5;235$r\x1b[*x",
			e.TopMargin,
			e.LeftMargin,
			e.TopMargin+e.ScreenLines,
			e.LeftMargin+e.left_margin_offset)
		fmt.Fprintf(&numCols, "\x1b[?25l\x1b[%d;%dH", e.TopMargin, e.LeftMargin+1)
		indent = fmt.Sprintf("\x1b[%dC", e.left_margin_offset)
	}

	y := 0
	e.visibleRows(func(n int, row string) bool {
		if e.numberLines {
			// (both colours are put back: whatever is drawn next -- the status
			// bar, a message -- is not the numbers' grey)
			fmt.Fprintf(&numCols, "\x1b[48;5;235m\x1b[38;5;245m%3d \x1b[39;49m", n+1)
		}
		for _, line := range strings.Split(row, "\t") {
			if y == e.ScreenLines {
				return false // the row runs past the bottom of the pane
			}
			pab.WriteString(indent)
			pab.WriteString(line)
			pab.WriteString(lf_ret)
			if e.numberLines {
				numCols.WriteString(lf_ret)
			}
			y++
		}
		return y < e.ScreenLines
	})
	pab.WriteString(numCols.String())
}
