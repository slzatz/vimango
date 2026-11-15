package editor

import (
	"math"

	"github.com/slzatz/vimango/vim"
)

// The view: which part of the note is on screen. It is a row -- the first one
// shown, firstVisibleRow, vim's 'topline' -- and the rows after it for as far
// as the pane goes, each taking the screen lines it wraps onto. A row is
// shown from its first screen line, with one exception: a row taller than
// the whole pane with the cursor in it, which is shown from whichever of its
// screen lines keeps the cursor on screen (topSkip).
//
// Two things move the view, and both are in this file. The cursor does, when
// it leaves what is shown (followCursor, after every key). And the commands
// that scroll do: libvim carries none of them out, having no window that
// matches the pane, and hands each to the editor (Scroll), which moves the
// view and then, by vim's rules, the cursor.
//
// The rules of both are vim's, from its move.c, with rows where vim has
// lines and with 'scrolloff' 0 -- and they are vim 9.1's, not those of the
// vim 8.1 that libvim is. The two agree wherever rows take one screen line.
// Where rows wrap, which in a note is everywhere, 9.1 pages and moves the
// cursor by screen lines and 8.1 by lines, and 9.1 is the vim there is to
// compare with: view_vim_test.go holds what vim 9.1.1752 answered, key
// for key.
//
// One thing of 9.1's is left out. When half a page ends two or more screen
// lines into a wrapped row, vim brings the view back to the start of that
// row and the cursor back one screen line fewer (an off-by-one in its
// scroll_with_sms), so its cursor creeps a line down the window. Here the
// cursor goes exactly as far as the view, which is what vim says it means
// to do; the two can then differ by a row.

// paneLines is the height of the text area in screen lines; at least one,
// for the arithmetic's sake, before the host has given a layout.
func (e *Editor) paneLines() int {
	return max(e.ScreenLines, 1)
}

// rowHeight is the number of screen lines row n takes, and more than any
// pane holds for a row the note does not have (vim's MAXCOL, for a line
// before the first or after the last).
func (e *Editor) rowHeight(n int) int {
	if n < 0 || n >= len(e.ss) {
		return math.MaxInt32
	}
	return e.getLinesInRowWW(n)
}

// linesAbove is the number of screen lines the rows before row take.
func (e *Editor) linesAbove(row int) int {
	lines := 0
	for n := 0; n < row && n < len(e.ss); n++ {
		lines += e.getLinesInRowWW(n)
	}
	return lines
}

// setView makes top the first row on screen, shown from its screen line
// skip. lineOffset, which everything drawn is placed by, is the number of
// screen lines above the pane's first.
func (e *Editor) setView(top, skip int) {
	e.firstVisibleRow, e.topSkip = top, skip
	e.lineOffset = e.linesAbove(top) + skip
}

// lastWholeRow is the last row shown whole when top is the first on screen:
// the one before vim's 'botline'. A top row taller than the pane is its own.
func (e *Editor) lastWholeRow(top int) int {
	last, used := top, 0
	for n := top; n < len(e.ss); n++ {
		used += e.getLinesInRowWW(n)
		if used > e.paneLines() {
			break
		}
		last = n
	}
	return last
}

// topForBottom is the top row that puts row last on screen, with as many
// rows above it as fit: zb, and where the view goes when the cursor leaves
// by the bottom (vim's scroll_cursor_bot).
func (e *Editor) topForBottom(row int) int {
	top, used := row, e.rowHeight(row)
	for top > 0 {
		h := e.rowHeight(top - 1)
		if used+h > e.paneLines() {
			break
		}
		used += h
		top--
	}
	return top
}

// topForMiddle is the top row that puts row in the middle of the pane: as
// many screen lines above it as below, a row at a time, and when they cannot
// be equal the extra one below it, or with preferAbove above. atEnd says
// whether the empty lines after the note's last row count as lines below --
// they do for zz, which centres a row near the end with nothing under it,
// and do not when the view is following the cursor, where G leaves the last
// row at the bottom. (vim's scroll_cursor_halfway.)
func (e *Editor) topForMiddle(row int, atEnd, preferAbove bool) int {
	top, bottom := row, row
	used := e.rowHeight(row)
	above, below := 0, 0
	fits := func(h int) bool {
		used += h
		return used <= e.paneLines()
	}
	addAbove := func() bool {
		h := e.rowHeight(top - 1)
		if !fits(h) {
			return false
		}
		above += h
		top--
		return true
	}
	addBelow := func() bool {
		if bottom == len(e.ss)-1 {
			below++ // an empty line after the note
			if atEnd {
				used++
			}
			return true
		}
		bottom++
		h := e.rowHeight(bottom)
		if !fits(h) {
			return false
		}
		below += h
		return true
	}
	for top > 0 {
		if preferAbove {
			if below >= above && !addAbove() {
				break
			}
			if below < above && !addBelow() {
				break
			}
		} else {
			if below <= above && !addBelow() {
				break
			}
			if below > above && !addAbove() {
				break
			}
		}
	}
	return top
}

// followCursor brings the view to the cursor if the cursor has left it, and
// sets cx and cy, the cursor's place in the pane, which Cursor() turns into
// the terminal's row and column. It reports whether the view is no longer
// the one last drawn -- because it moved it, or because Scroll did.
//
// Where the view goes is vim's rule (update_topline). The cursor's row is
// shown whole. Just above the view, it becomes the top row; just below, the
// bottom one, so j and k scroll a row at a time. Further off than that --
// about half a pane: G, a search, a mark -- it goes to the middle, with the
// rows around it.
func (e *Editor) followCursor() (moved bool) {
	if len(e.ss) == 0 {
		return false
	}
	height := e.paneLines()
	top := min(max(e.firstVisibleRow, 0), len(e.ss)-1)
	switch last := e.lastWholeRow(top); {
	case e.fr < top:
		if top-e.fr >= max(height/2-1, 2) {
			top = e.topForMiddle(e.fr, false, false)
		} else {
			top = e.fr
		}
	case e.fr > last:
		// How far below is "just below": vim counts the rows from the
		// cursor's back up to the view, and a row under the cursor for
		// each, and if they do not all fit in a pane it is far
		// (scroll_cursor_bot). So about half a pane, and near the end of
		// the note, where there are no rows under the cursor, a whole one.
		used := e.rowHeight(e.fr)
		for above, below := e.fr, e.fr; above > last+1 && used <= height; {
			above--
			used += e.rowHeight(above)
			if used <= height && below < len(e.ss)-1 {
				below++
				used += e.rowHeight(below)
			}
		}
		switch {
		case e.fr-last > height+1: // more than a pane below, which vim asks first
			top = e.topForMiddle(e.fr, false, false)
		case used > height:
			top = e.topForMiddle(e.fr, false, true)
		default:
			top = e.topForBottom(e.fr)
		}
	}

	// A row taller than the pane cannot be shown whole: show the part the
	// cursor is in, moving over the row no further than the cursor makes it.
	skip := 0
	if top == e.fr && e.rowHeight(top) > height {
		if top == e.firstVisibleRow {
			skip = e.topSkip
		}
		line := e.getLineInRowWW(e.fr, e.fc) - 1
		skip = max(min(skip, line), line-height+1)
	}
	e.setView(top, skip)

	e.cx = e.getScreenXFromRowColWW(e.fr, e.fc)
	e.cy = e.getLineInRowWW(e.fr, e.fc) - 1 - skip
	for n := top; n < e.fr; n++ {
		e.cy += e.getLinesInRowWW(n)
	}
	return e.firstVisibleRow != e.drawnView.top || e.topSkip != e.drawnView.skip
}

// halfPage is how far CTRL-D and CTRL-U go, in screen lines: half the pane,
// or the count last given to either, which vim keeps (in 'scroll') until the
// window changes height.
func (e *Editor) halfPage(count int) int {
	height := e.paneLines()
	if count > 0 {
		e.scrollCount.lines, e.scrollCount.pane = min(count, height), height
	}
	if e.scrollCount.pane == height {
		return e.scrollCount.lines
	}
	return max(height/2, 1)
}

// pageLines is how far CTRL-F (down) or CTRL-B goes from top, in screen
// lines: a pane less two, so that the two rows at the edge it is leaving by
// are still on screen after; a pane less one, or all of it, when those rows
// are tall enough to take much of the pane, and all of it when the note's
// last row is showing. (vim's get_scroll_overlap.)
func (e *Editor) pageLines(top int, down bool) int {
	height := e.paneLines()
	room := height - 2
	// the first row off the pane at that edge, and back onto the pane from it
	row, step := top-1, 1
	if down {
		row, step = e.lastWholeRow(top)+1, -1
		if row >= len(e.ss) {
			return height
		}
	}
	h1, h2 := e.rowHeight(row), e.rowHeight(row+step)
	h3, h4 := e.rowHeight(row+2*step), e.rowHeight(row+3*step)
	switch {
	case h1 > room, h2+h1 > room, h3+h2 > room:
		return height
	case h4+h3+h2 > room, h3+h2+h1 > room:
		return height - 1
	}
	return height - 2
}

// scrolledTop is the top row after the view has gone lines screen lines
// down the note from top, or up it: by whole rows, as many as take no more
// screen lines than that, and one row at least. rounded is how many screen
// lines further than asked that came to (fewer, if negative), and is 0 when
// the view stopped at the start of the note or with its last row at the
// top. (What vim's scroll_with_sms does without 'smoothscroll'.)
func (e *Editor) scrolledTop(top, lines int, down bool) (newTop, rounded int) {
	moved := 0
	for moved < lines {
		next := top - 1 // the row that goes off the pane, or comes onto it
		if down {
			next = top
		}
		if (down && top >= len(e.ss)-1) || (!down && top == 0) {
			return top, 0
		}
		h := e.rowHeight(next)
		if moved+h > lines && moved > 0 {
			break
		}
		moved += h
		if down {
			top++
		} else {
			top--
		}
	}
	return top, moved - lines
}

// rowAtScreenLine is the row that screen line n of the whole note, from 0,
// is one of; the first or the last row for a line the note does not have.
func (e *Editor) rowAtScreenLine(n int) int {
	lines := 0
	for row := range e.ss {
		lines += e.getLinesInRowWW(row)
		if n < lines {
			return row
		}
	}
	return len(e.ss) - 1
}

// Scroll is one of vim's scrolling commands, which libvim hands to the
// editor (vim.ScreenLayout):
//
//   - zt, zz and zb put the cursor's row at the top, the middle or the
//     bottom of the pane, and leave the cursor alone.
//   - CTRL-E and CTRL-Y move the view by rows, and the cursor only if its
//     row would leave the pane: to the nearest row that is whole in it.
//   - CTRL-D and CTRL-U move the view half a pane of screen lines (or the
//     count, which is then kept), and the cursor as many screen lines as the
//     view went, so that it stays where it was in the pane. At the end of
//     the note the view stops with the last row at the bottom, and at the
//     start with the first at the top, and the cursor goes the distance
//     alone.
//   - CTRL-F and CTRL-B (here <PageDown> and <PageUp>: CTRL-B is bold) move
//     the view a pane less two screen lines, and put the cursor on the top
//     row of the page after, the bottom row of the page before.
//
// The view moves by whole rows: as many as the distance covers, and always
// one. After the last four the cursor is on the first non-blank of its row,
// as in vim (in the column vim wants instead, with 'nostartofline').
//
// libvim calls this in the middle of a key, so it starts from vim -- the
// text and the cursor as they are now, and the view that goes with them: a
// mapping can edit, move and scroll in one key -- and leaves the view and
// the cursor as the command leaves them, for whatever the key does next.
// The key's own redraw sees that the view is not the one drawn.
func (e *Editor) Scroll(s vim.Scroll, count int) {
	if !e.onItsBuffer() {
		return
	}
	e.copyFromVim()
	e.followCursor()

	top := e.firstVisibleRow
	switch s {
	case vim.ScrollCursorTop:
		e.setView(e.fr, 0)
	case vim.ScrollCursorMiddle:
		e.setView(e.topForMiddle(e.fr, true, false), 0)
	case vim.ScrollCursorBottom:
		e.setView(e.topForBottom(e.fr), 0)

	case vim.ScrollLineUp: // CTRL-E; the last row can go to the top, no further
		top = min(top+max(count, 1), len(e.ss)-1)
		e.setView(top, 0)
		e.scrollCursorTo(max(e.fr, top), false)
	case vim.ScrollLineDown: // CTRL-Y
		top = max(top-max(count, 1), 0)
		e.setView(top, 0)
		e.scrollCursorTo(min(e.fr, e.lastWholeRow(top)), false)

	case vim.ScrollHalfPageDown, vim.ScrollHalfPageUp:
		down := s == vim.ScrollHalfPageDown
		lines := e.halfPage(count)
		cursorLines := lines
		if down {
			// no further than brings the note's last row to the bottom
			toEnd := e.linesAbove(len(e.ss)) - e.linesAbove(top)
			lines = min(lines, toEnd-e.paneLines())
		}
		if lines > 0 {
			var rounded int
			top, rounded = e.scrolledTop(top, lines, down)
			cursorLines += rounded
			e.setView(top, 0)
		}
		if !down {
			cursorLines = -cursorLines
		}
		at := e.linesAbove(e.fr) + e.getLineInRowWW(e.fr, e.fc) - 1
		e.scrollCursorTo(e.rowAtScreenLine(max(at+cursorLines, 0)), true)

	case vim.ScrollPageDown, vim.ScrollPageUp:
		down := s == vim.ScrollPageDown
		newTop, _ := e.scrolledTop(top, max(count, 1)*e.pageLines(top, down), down)
		if newTop == top {
			return // the first page, or the last row at the top: vim beeps
		}
		e.setView(newTop, 0)
		if down {
			e.scrollCursorTo(newTop, true)
		} else {
			e.scrollCursorTo(e.lastWholeRow(newTop), true)
		}
	}
}

// scrollCursorTo has vim move the cursor to row, as its scrolling commands
// do (vim.ScrollCursorTo), and brings the editor's copy of the cursor along.
// Not startOfLine, a cursor already on the row is left where it is.
func (e *Editor) scrollCursorTo(row int, startOfLine bool) {
	if row == e.fr && !startOfLine {
		return
	}
	vim.ScrollCursorTo(row+1, startOfLine)
	e.copyFromVim()
}
