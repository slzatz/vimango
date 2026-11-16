package editor

import (
	//"bufio"
	"fmt"
	"strings"
	"unicode"

	//	"time"
	"github.com/mattn/go-runewidth"
	"github.com/slzatz/vimango/internal/pane"
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
	if y >= e.ScreenLines {
		return
	}

	match := e.ss[pos[0]-1][pos[1]]

	x := e.getScreenXFromRowColWW(pos[0]-1, pos[1]) + e.LeftMargin + e.left_margin_offset + 1
	fmt.Printf("\x1b[%d;%dH%s%s", y+e.TopMargin, x, shadedStyle, string(match))

	x = e.getScreenXFromRowColWW(e.fr, e.fc-back) + e.LeftMargin + e.left_margin_offset + 1
	y = e.getScreenYFromRowColWW(e.fr, e.fc-back) + e.TopMargin - e.lineOffset // added line offset 12-25-2019
	fmt.Printf("\x1b[%d;%dH%s%s\x1b[0m", y, x, shadedStyle, string(b))
	return
}

// wrapWidth is the number of display columns available to wrapped buffer text.
func (e *Editor) wrapWidth() int {
	return e.ScreenCols - e.left_margin_offset
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
// screen — tabs widened to the four spaces styledRow emits —
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

// DrawAfterKey redraws what a key (or a mapping's timeout) changed. c is the
// key, or NoKey.
//
// In INSERT mode a key that vim is holding -- the "j" of a "jk" mapping,
// until the next key or the timeout says which it is -- is not in the text
// yet. vim shows it at the cursor meanwhile, so that typing does not seem to
// lag, and so does this; whatever happens next redraws the text over it.
func (e *Editor) DrawAfterKey(redraw bool, c int) {
	viewMoved := e.followCursor()
	if redraw || viewMoved || e.heldKeyShown {
		e.drawText()
		e.drawStatusBar()
	}
	e.heldKeyShown = false
	if e.mode == INSERT && c != NoKey && !pane.IsSyntheticKey(c) && vim.InputPending() > 0 {
		if r := rune(c); unicode.IsPrint(r) && runewidth.RuneWidth(r) == 1 {
			fmt.Printf("\x1b[%d;%dH%c", e.cy+e.TopMargin, e.cx+e.LeftMargin+e.left_margin_offset+1, r)
			e.heldKeyShown = true
		}
	}
}

// Draw draws the whole editor: the text, the frame, the status bar, the
// notice over them if one is up, and what its mode keeps on the message
// line. For when it is opened, when its layout changes -- a resize, which
// clears the screen first -- and when its note is reloaded; a key redraws
// only what it changed (DrawAfterKey). Nothing need be erased first: drawText
// clears each line of the text area before it writes it.
//
// The view is worked out again first: a layout of another width wraps the
// rows above the cursor onto other screen lines, and one of another height
// may not hold the cursor's row.
func (e *Editor) Draw() {
	e.followCursor()
	e.drawText()
	e.drawFrame()
	e.drawStatusBar()
	// The text is now over the notice the keys are still scrolling. Show it
	// again: from its top, and laid out for the layout as it is now, which
	// is what changed if this is a resize.
	if e.mode == HELP {
		e.showNotice(e.notice.text, e.notice.hang)
	}
	e.drawModeLine()
}

// drawText draws the text area: the rows, and what is marked on them. It
// draws over a notice, which is how one ends.
func (e *Editor) drawText() {
	e.notice.up = false
	var ab strings.Builder

	fmt.Fprintf(&ab, "\x1b[?25l\x1b[%d;%dH", e.TopMargin, e.LeftMargin+1)
	// \x1b[NC moves cursor forward by n columns
	lf_ret := fmt.Sprintf("\r\n\x1b[%dC", e.LeftMargin)
	erase_chars := fmt.Sprintf("\x1b[%dX", e.ScreenCols)
	for i := 0; i < e.ScreenLines; i++ {
		//time.Sleep(10 * time.Millisecond) //helps test if redraws are happening by slowing down redraws
		ab.WriteString(erase_chars)
		ab.WriteString(lf_ret)
	}
	e.drawRows(&ab) // editor_rows.go
	e.drawnView.top, e.drawnView.skip = e.firstVisibleRow, e.topSkip
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
		if y >= 0 && y < e.ScreenLines {
			start, end := lo, hi
			if start < seg.start {
				start = seg.start
			}
			if end > seg.end {
				end = seg.end
			}
			if start < end {
				x := runewidth.StringWidth(row[seg.start:start]) + e.LeftMargin + e.left_margin_offset + 1
				fmt.Fprintf(pab, "\x1b[%d;%dH", y+e.TopMargin, x)
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
	for r := startRow; r <= endRow && y < e.ScreenLines; r++ {
		row := e.visualRowText(r)
		lo, hi := 0, len(row) // VISUAL_LINE takes the whole row

		switch e.vmode {
		case VISUAL:
			// the columns vim reports are byte offsets and the selection takes
			// in the character under the cursor, so the end has to step over a
			// whole rune
			if r == startRow {
				lo = pane.RuneStart(row, expandedOffset(e.ss[r], e.highlight[0][1]))
			}
			if r == endRow {
				// in VISUAL mode like INSERT mode, the cursor can go beyond end of row
				hi = pane.InclusiveEnd(row, expandedOffset(e.ss[r], e.highlight[1][1]))
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
	pab.WriteString(sgrReset)
}

func (e *Editor) drawStatusBar() {
	// a host with a status bar of its own asks for none (Screen.editorLayout)
	if !e.StatusBar {
		return
	}

	var ab strings.Builder
	fmt.Fprintf(&ab, "\x1b[%d;%dH", e.ScreenLines+e.TopMargin, e.LeftMargin+1)

	//erase from start of an Editor's status bar to the end of the Editor's status bar
	fmt.Fprintf(&ab, "\x1b[%dX", e.ScreenCols)

	ab.WriteString("\x1b[7m ") //switches to inverted colors
	// cut and padded by screen columns, not bytes: a title can hold
	// characters of several bytes, and of two columns
	title, _ := pane.ClipColumns(e.title, 0, 30)
	if e.isModified() {
		title += "[+]"
	}
	status, cols := pane.ClipColumns(fmt.Sprintf("%d - %s ...", e.id, title), 0, e.ScreenCols-1)
	ab.WriteString(status)
	ab.WriteString(strings.Repeat(" ", max(e.ScreenCols-cols, 0)))
	ab.WriteString("\x1b[0m") //switches back to normal formatting
	fmt.Print(ab.String())
}

func (e *Editor) drawFrame() {
	var ab strings.Builder
	ab.WriteString("\x1b(0") // Enter line drawing mode

	for j := 1; j < e.ScreenLines+1; j++ {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", e.TopMargin-1+j, e.LeftMargin+e.ScreenCols+1)
		// below x = 0x78 vertical line (q = 0x71 is horizontal) 37 = white; 1m = bold (note
		// only need one 'm'
		ab.WriteString("\x1b[37;1mx")
	}

	// the top right corner (k), where the frame meets a border above the
	// text. With no such border there is no row for it: a layout whose text
	// starts on row 1 asks for none, or the corner would land on the text.
	if e.Corner {
		fmt.Fprintf(&ab, "\x1b[%d;%dH", e.TopMargin-1, e.LeftMargin+e.ScreenCols+1)
		ab.WriteString("\x1b[37;1mk")
	}

	//exit line drawing mode
	ab.WriteString("\x1b(B")
	ab.WriteString("\x1b[?25h") //shows the cursor
	ab.WriteString("\x1b[0m")   //or else subsequent editors are bold
	fmt.Print(ab.String())
}

// isModified is whether the buffer's text is other than the text the
// database holds, as this editor last read or wrote it (dbText). On it rest
// the quit guard, what :x writes, and what a host is told.
//
// It is the text that is compared, and not whether vim has changed the
// buffer since the save: vim's change tick only goes up, so by the tick
// alone a change undone was a change still -- :q after x and u was refused,
// and :x wrote a row that had not changed. The same text is the same note,
// however it was come by: undone, typed again, or undone past a save and
// done again. And a text the database does not hold is a modified note,
// which is how undoing past a save is one.
//
// The tick says when to look: the text is read from vim and compared once
// for each value of it, and not at all while it is the tick of the save.
func (e *Editor) isModified() bool {
	tick := e.vbuf.GetLastChangedTick()
	if tick == e.saveTick {
		return false
	}
	if tick != e.comparedTick {
		e.comparedTick = tick
		e.differs = strings.Join(e.vbuf.Lines(), "\n") != e.dbText
	}
	return e.differs
}

// noteIs records that the database holds text and vim's buffer holds the
// same: after a note is read into the buffer, and after the buffer is
// written. It is the one place dbText and saveTick are set, since isModified
// is right only while they are set together.
func (e *Editor) noteIs(text string) {
	e.dbText = text
	e.saveTick = e.vbuf.GetLastChangedTick()
}
