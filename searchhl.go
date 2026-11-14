package main

import (
	"strings"

	"github.com/slzatz/vimango/vim"
)

// Search highlighting in the note editor: the matches of the last search
// pattern, drawn over the text, as vim does with 'hlsearch'.
//
// vim keeps all the state. libvim reports where the matches are
// (Buffer.SearchMatches) and whether they should be showing
// (vim.SearchHighlighting): on after "/", "?", "n", "N", "*", "#" and ":s",
// hidden by ":noh" until the next of those, off altogether with
// ":set nohlsearch". Nothing here tracks those commands; drawSearchMatches
// asks on every redraw, for the visible rows only, so the highlights follow
// edits and scrolling.
//
// What it cannot learn from the text changing is when to redraw: ":noh" and
// "n" change what should be highlighted and leave the buffer alone.
// editorProcessKey compares searchState with what was last drawn.
//
// libvim searches as the pattern is typed, so the matches of "/fo" show while
// the "o" is still to come, like vim's 'incsearch'.

// searchStyle is black on a muted yellow: vim's Search group, more or less.
const searchStyle = "\x1b[30;48;5;179m"

// searchState is everything the highlighting depends on apart from the text:
// the pattern, and whether vim is showing its matches.
func searchState() string {
	if !vim.SearchHighlighting() {
		return ""
	}
	return vim.SearchPattern()
}

// drawSearchMatches draws the visible matches of the search pattern. Called
// from drawText after the rows (and spelling errors) are down.
func (e *Editor) drawSearchMatches(pab *strings.Builder) {
	e.searchDrawn = searchState()
	if e.searchDrawn == "" || len(e.ss) == 0 {
		return
	}

	first := e.firstVisibleRow
	last := min(first+e.screenlines, len(e.ss)) // every row takes at least one screen line
	if first >= last {
		return
	}
	matches := e.vbuf.SearchMatches(first+1, last)
	if len(matches) == 0 {
		return
	}

	// the screen line each visible row starts on, 0-based within the pane
	ys := make([]int, last-first)
	y := e.getScreenYFromRowColWW(first, 0) - e.lineOffset
	for r := first; r < last; r++ {
		ys[r-first] = y
		y += e.getLinesInRowWW(r)
	}

	pab.WriteString(searchStyle)
	for _, m := range matches {
		// one row, or several for a pattern that matches across lines
		for r := m.Line - 1; r <= m.EndLine-1; r++ {
			if r < first || r >= last {
				continue
			}
			raw := e.ss[r]
			lo, hi := 0, len(raw)
			if r == m.Line-1 {
				lo = m.Col
			}
			if r == m.EndLine-1 {
				hi = m.EndCol
			}
			if lo < 0 || hi > len(raw) || lo >= hi {
				continue // an empty match, or e.ss out of step with vim's buffer
			}
			e.highlightRowRange(pab, e.visualRowText(r), expandedOffset(raw, lo), expandedOffset(raw, hi), ys[r-first])
		}
	}
	pab.WriteString(sgrReset)
}
