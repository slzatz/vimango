package main

import (
	//"bufio"
	"fmt"
	"strings"
	"unicode"

	//	"time"
	"github.com/mattn/go-runewidth"
	"github.com/slzatz/vimango/vim"
)

func (e *Editor) highlightInfo() { // [2][4]int {
	e.highlight = vim.GetVisualRange() //[]line col []line col
}

// shadedStyle is how the visual selection and a matched pair of braces are
// drawn: grey text on a darker ground. The grey was the line numbers', left
// set by accident until 2026-11-14 -- so a selection was grey only with
// numbers on and nothing else drawn in between. It is now said here.
const shadedStyle = "\x1b[48;5;237m\x1b[38;5;245m"

// highlight matched braces in NORMAL and INSERT modes
func (e *Editor) drawHighlightedBraces() {

	if len(e.ss) == 0 || len(e.ss[e.fr]) == 0 {
		return
	}

	var b byte
	var back int
	//if below handles case when in insert mode and brace is last char
	//in a row and cursor is beyond that last char (which is a brace)
	if e.fc == len(e.ss[e.fr]) {
		b = e.ss[e.fr][e.fc-1]
		back = 1
	} else {
		b = e.ss[e.fr][e.fc]
		back = 0
	}

	if !strings.ContainsAny(string(b), "{}()") {
		return
	}

	pos := vim.GetMatchingPair()
	if pos == [2]int{0, 0} {
		return
	}

	y := e.getScreenYFromRowColWW(pos[0]-1, pos[1]) - e.lineOffset
	if y >= e.screenlines {
		return
	}

	match := e.ss[pos[0]-1][pos[1]]

	x := e.getScreenXFromRowColWW(pos[0]-1, pos[1]) + e.left_margin + e.left_margin_offset + 1
	fmt.Printf("\x1b[%d;%dH%s%s", y+e.top_margin, x, shadedStyle, string(match))

	x = e.getScreenXFromRowColWW(e.fr, e.fc-back) + e.left_margin + e.left_margin_offset + 1
	y = e.getScreenYFromRowColWW(e.fr, e.fc-back) + e.top_margin - e.lineOffset // added line offset 12-25-2019
	fmt.Printf("\x1b[%d;%dH%s%s\x1b[0m", y, x, shadedStyle, string(b))
	return
}

// used by updateNote
func (e *Editor) bufferToString() string {
	return strings.Join(e.ss, "\n")
}

// wrapWidth is the number of display columns available to wrapped buffer text.
func (e *Editor) wrapWidth() int {
	return e.screencols - e.left_margin_offset
}

// wrapRowText returns buffer row r with tabs expanded and byte offset c
// adjusted to match the expansion, ready to hand to wrapRow. The four-character
// "$$$$" stand-in keeps byte offsets and display width in step with the four
// spaces the drawing code substitutes.
func (e *Editor) wrapRowText(r, c int) (string, int) {
	row := e.ss[r]
	return strings.ReplaceAll(row, "\t", "$$$$"), expandedOffset(row, c)
}

// visualRowText returns buffer row r exactly as the drawing code puts it on
// screen — tabs widened to the four spaces wrappedRow emits —
// so a highlight drawn over it lines up with the text underneath.
func (e *Editor) visualRowText(r int) string {
	return strings.ReplaceAll(e.ss[r], "\t", strings.Repeat(" ", tabWidth))
}

// displayCol returns the display column of byte offset c in buffer row r,
// measured from the start of the row.
func (e *Editor) displayCol(r, c int) int {
	row := e.ss[r]
	return runewidth.StringWidth(e.visualRowText(r)[:expandedOffset(row, c)])
}

// wrapSegmentAt returns the segment holding byte offset c, or the last segment
// when c is past the end of the row (the cursor may sit there in INSERT mode).
func wrapSegmentAt(segs []wrapSegment, c int) wrapSegment {
	for _, seg := range segs {
		if c < seg.end {
			return seg
		}
	}
	return segs[len(segs)-1]
}

// getScreenXFromRowColWW returns the screen column (0-based, in display
// columns) of byte offset c within buffer row r.
func (e *Editor) getScreenXFromRowColWW(r, c int) int {
	row, c := e.wrapRowText(r, c)
	seg := wrapSegmentAt(wrapRow(row, e.wrapWidth()), c)
	if c > seg.end {
		c = seg.end
	}
	return runewidth.StringWidth(row[seg.start:c])
}

func (e *Editor) getScreenYFromRowColWW(r, c int) int {
	screenLine := 0

	for n := 0; n < r; n++ {
		screenLine += e.getLinesInRowWW(n)
	}

	screenLine = screenLine + e.getLineInRowWW(r, c) - 1
	return screenLine
}

// getLinesInRowWW returns how many screen lines buffer row r occupies.
func (e *Editor) getLinesInRowWW(r int) int {
	row, _ := e.wrapRowText(r, 0)
	return len(wrapRow(row, e.wrapWidth()))
}

// getLineInRowWW returns which screen line of buffer row r (1-based) contains
// byte offset c.
func (e *Editor) getLineInRowWW(r, c int) int {
	row, c := e.wrapRowText(r, c)
	segs := wrapRow(row, e.wrapWidth())
	for i, seg := range segs {
		if c < seg.end {
			return i + 1
		}
	}
	return len(segs)
}

// drawAfterKey redraws what a key (or a mapping's timeout) changed. c is the
// key, or noKey.
//
// In INSERT mode a key that vim is holding -- the "j" of a "jk" mapping,
// until the next key or the timeout says which it is -- is not in the text
// yet. vim shows it at the cursor meanwhile, so that typing does not seem to
// lag, and so does this; whatever happens next redraws the text over it.
func (e *Editor) drawAfterKey(redraw bool, c int) {
	offsetChanged := e.scroll()
	if redraw || offsetChanged || e.heldKeyShown {
		e.drawText()
		e.drawStatusBar()
	}
	e.heldKeyShown = false
	if e.mode == INSERT && c != noKey && !isSyntheticKey(c) && vim.InputPending() > 0 {
		if r := rune(c); unicode.IsPrint(r) && runewidth.RuneWidth(r) == 1 {
			fmt.Printf("\x1b[%d;%dH%c", e.cy+e.top_margin, e.cx+e.left_margin+e.left_margin_offset+1, r)
			e.heldKeyShown = true
		}
	}
}

// draw draws the whole editor: the text, the frame, the status bar. For when
// it is opened, when its layout changes, and when its note is reloaded; a key
// redraws only what it changed (drawAfterKey). Nothing need be erased first:
// drawText clears each line of the text area before it writes it.
func (e *Editor) draw() {
	e.drawText()
	e.drawFrame()
	e.drawStatusBar()
}

func (e *Editor) drawText() {
	var ab strings.Builder

	fmt.Fprintf(&ab, "\x1b[?25l\x1b[%d;%dH", e.top_margin, e.left_margin+1)
	// \x1b[NC moves cursor forward by n columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.left_margin)
	erase_chars := fmt.Sprintf("\x1b[%dX", e.screencols)
	for i := 0; i < e.screenlines; i++ {
		//time.Sleep(10 * time.Millisecond) //helps test if redraws are happening by slowing down redraws
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}
	e.drawRows(&ab)          // editor_rows.go
	e.drawSpellErrors(&ab)   // no-op unless :spell is on
	e.drawSearchMatches(&ab) // no-op unless a search is being highlighted
	if e.mode == VISUAL {
		e.drawVisual(&ab)
	}
	fmt.Print(ab.String())
	e.drawHighlightedBraces() //has to come after drawing rows
}

// highlightRowRange draws bytes [lo, hi) of row — already tab-expanded — in
// reverse video, split across the screen lines the row wraps onto. y is the
// screen line, 0-based within the pane, that the row's first wrapped line sits
// on; the line the row after it starts on is returned. Lines scrolled off the
// top or bottom are counted but not drawn.
func (e *Editor) highlightRowRange(pab *strings.Builder, row string, lo, hi, y int) int {
	for _, seg := range wrapRow(row, e.wrapWidth()) {
		if y >= 0 && y < e.screenlines {
			start, end := lo, hi
			if start < seg.start {
				start = seg.start
			}
			if end > seg.end {
				end = seg.end
			}
			if start < end {
				x := runewidth.StringWidth(row[seg.start:start]) + e.left_margin + e.left_margin_offset + 1
				fmt.Fprintf(pab, "\x1b[%d;%dH", y+e.top_margin, x)
				pab.WriteString(row[start:end])
			}
		}
		y++
	}
	return y
}

func (e *Editor) drawVisual(pab *strings.Builder) {
	if len(e.ss) == 0 {
		return
	}

	// note the rows vim reports are 1-based
	startRow, endRow := e.highlight[0][0]-1, e.highlight[1][0]-1
	if startRow < 0 {
		startRow = 0
	}
	if endRow > len(e.ss)-1 {
		endRow = len(e.ss) - 1
	}

	// vim reports each VISUAL_BLOCK corner as a byte offset in its own row, but
	// a block is a rectangle on screen: the edges only compare once both are
	// display columns, and each row then needs its own byte range.
	var leftCol, rightCol int
	if e.vmode == VISUAL_BLOCK {
		leftCol = e.displayCol(startRow, e.highlight[0][1])
		rightCol = e.displayCol(endRow, e.highlight[1][1])
		if rightCol < leftCol {
			leftCol, rightCol = rightCol, leftCol
		}
	}

	y := e.getScreenYFromRowColWW(startRow, 0) - e.lineOffset

	pab.WriteString(shadedStyle)
	for r := startRow; r <= endRow && y < e.screenlines; r++ {
		row := e.visualRowText(r)
		lo, hi := 0, len(row) // VISUAL_LINE takes the whole row

		switch e.vmode {
		case VISUAL:
			// the columns vim reports are byte offsets and the selection takes
			// in the character under the cursor, so the end has to step over a
			// whole rune
			if r == startRow {
				lo = runeStart(row, expandedOffset(e.ss[r], e.highlight[0][1]))
			}
			if r == endRow {
				// in VISUAL mode like INSERT mode, the cursor can go beyond end of row
				hi = inclusiveEnd(row, expandedOffset(e.ss[r], e.highlight[1][1]))
			}
			if hi < lo {
				hi = lo
			}
		case VISUAL_BLOCK:
			// a block clips to each screen line the row occupies
			lo = byteAtDisplayCol(row, leftCol)
			hi = byteAfterDisplayCol(row, rightCol)
		}

		y = e.highlightRowRange(pab, row, lo, hi, y)
	}
	pab.WriteString(RESET)
}

func (e *Editor) drawStatusBar() {
	// a host with a status bar of its own asks for none (Screen.editorLayout)
	if !e.statusBar {
		return
	}

	var ab strings.Builder
	fmt.Fprintf(&ab, "\x1b[%d;%dH", e.screenlines+e.top_margin, e.left_margin+1)

	//erase from start of an Editor's status bar to the end of the Editor's status bar
	fmt.Fprintf(&ab, "\x1b[%dX", e.screencols)

	ab.WriteString("\x1b[7m ") //switches to inverted colors
	// cut and padded by screen columns, not bytes: a title can hold
	// characters of several bytes, and of two columns
	title, _ := clipColumns(e.title, 0, 30)
	if e.isModified() {
		title += "[+]"
	}
	status, cols := clipColumns(fmt.Sprintf("%d - %s ...", e.id, title), 0, e.screencols-1)
	ab.WriteString(status)
	ab.WriteString(strings.Repeat(" ", max(e.screencols-cols, 0)))
	ab.WriteString("\x1b[0m") //switches back to normal formatting
	fmt.Print(ab.String())
}

func (e *Editor) drawFrame() {
	var ab strings.Builder
	ab.WriteString("\x1b(0") // Enter line drawing mode

	for j := 1; j < e.screenlines+1; j++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", e.top_margin-1+j, e.left_margin+e.screencols+1)
		// below x = 0x78 vertical line (q = 0x71 is horizontal) 37 = white; 1m = bold (note
		// only need one 'm'
		ab.WriteString("\x1b[37;1mx")
	}

	// the top right corner (k), where the frame meets a border above the
	// text. With no such border there is no row for it: a layout whose text
	// starts on row 1 asks for none, or the corner would land on the text.
	if e.corner {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", e.top_margin-1, e.left_margin+e.screencols+1)
		ab.WriteString("\x1b[37;1mk")
	}

	//exit line drawing mode
	ab.WriteString("\x1b(B")
	ab.WriteString("\x1b[?25h") //shows the cursor
	ab.WriteString("\x1b[0m")   //or else subsequent editors are bold
	fmt.Print(ab.String())
}

// scroll sets cx and cy, which cursor() turns into the terminal's row and column
func (e *Editor) scroll() (offset_changed bool) {

	prevOffset := e.lineOffset

	if e.fc == 0 && e.fr == 0 {
		e.cy, e.cx, e.lineOffset, e.firstVisibleRow = 0, 0, 0, 0
		return prevOffset != e.lineOffset
	}

	e.cx = e.getScreenXFromRowColWW(e.fr, e.fc)
	cy := e.getScreenYFromRowColWW(e.fr, e.fc)

	//deal with scroll insufficient to include the current line
	if cy > e.screenlines+e.lineOffset-1 {
		e.lineOffset = cy - e.screenlines + 1 ////
		e.adjustFirstVisibleRow()             //can also change e.lineOffset
	}

	if cy < e.lineOffset {
		e.lineOffset = cy
		e.adjustFirstVisibleRow()
	}

	if e.lineOffset == 0 {
		e.firstVisibleRow = 0
	}

	e.cy = cy - e.lineOffset

	return prevOffset != e.lineOffset
}

// e.lineOffset determines the first
// visible row but we want the whole row
// visible so that can change e.lineOffset
func (e *Editor) adjustFirstVisibleRow() {

	if e.lineOffset == 0 {
		e.firstVisibleRow = 0
		return
	}

	rowNum := 0
	lines := 0

	for {
		lines += e.getLinesInRowWW(rowNum)
		rowNum++

		/*
			there is no need to adjust line_offset
			if it happens that we start
			on the first line of the first visible row
		*/
		if lines == e.lineOffset {
			break
		}

		/*
			need to adjust line_offset
			so we can start on the first
			line of the top row
		*/
		if lines > e.lineOffset {
			e.lineOffset = lines
			break
		}
	}
	e.firstVisibleRow = rowNum
}

// this func is reason that we are writing notes to file
// allows easy testing if a file is modified with BufferOption
func (e *Editor) isModified() bool {
	//return e.vbuf.IsModified()

	tick := e.vbuf.GetLastChangedTick()
	if tick > e.saveTick {
		//e.bufferTick = tick
		return true
	}
	return false
}
