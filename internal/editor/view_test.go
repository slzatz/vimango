package editor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// The test editor's pane is 30 screen lines of 50 columns. Lines and rows
// are counted from 1 in these tests, as vim counts them.

// numberedRows is n rows that each take one screen line.
func numberedRows(n int) []string {
	rows := make([]string, n)
	for i := range rows {
		rows[i] = fmt.Sprintf("row %d of the note", i+1)
	}
	return rows
}

// tallRows is n rows that each take lines screen lines.
func tallRows(n, lines int) []string {
	rows := make([]string, n)
	for i := range rows {
		rows[i] = strings.Repeat("word ", 10*lines-5)
	}
	return rows
}

// typeKeys sends keys as the app does: after each one the view follows the
// cursor (DrawAfterKey, without the drawing).
func (e *Editor) typeKeys(keys ...int) {
	for _, k := range keys {
		e.ProcessKey(k)
		e.followCursor()
	}
}

func (e *Editor) typeString(s string) {
	for _, r := range s {
		e.typeKeys(int(r))
	}
}

// expectView checks the first row on screen and the line the cursor is on.
func expectView(t *testing.T, e *Editor, after string, top, line int) {
	t.Helper()
	if got := vim.GetCursorPosition()[0]; e.firstVisibleRow+1 != top || got != line {
		t.Errorf("after %s: row %d at the top and the cursor on line %d, want row %d and line %d",
			after, e.firstVisibleRow+1, got, top, line)
	}
	if e.cy < 0 || e.cy >= e.ScreenLines {
		t.Errorf("after %s: the cursor is on line %d of a pane of %d", after, e.cy, e.ScreenLines)
	}
}

const (
	ctrlD = 'd' & 0x1f
	ctrlE = 'e' & 0x1f
	ctrlU = 'u' & 0x1f
	ctrlY = 'y' & 0x1f
)

// CTRL-E and CTRL-Y move the view by rows. The cursor stays on its row, and
// goes with the view only when its row leaves the pane.
func TestScrollByRows(t *testing.T) {
	e := newTestEditor(t, numberedRows(100)...)
	e.typeString("4l")

	e.typeKeys(ctrlE)
	expectView(t, e, "CTRL-E", 2, 2)
	expectCursor(t, "CTRL-E, in the column it was in", 2, 4)
	e.typeKeys('3', ctrlE)
	expectView(t, e, "3 CTRL-E", 5, 5)
	e.typeKeys(ctrlY)
	expectView(t, e, "CTRL-Y, the cursor's row still showing", 4, 5)
	e.typeString("L")
	expectView(t, e, "L", 4, 33)
	e.typeKeys(ctrlY)
	expectView(t, e, "CTRL-Y from the bottom row", 3, 32)
	e.typeString("9")
	e.typeKeys(ctrlY)
	expectView(t, e, "9 CTRL-Y: as far as it goes", 1, 30)

	// the last row can go to the top, and no further
	e.typeString("200")
	e.typeKeys(ctrlE)
	expectView(t, e, "200 CTRL-E", 100, 100)
	e.typeKeys(ctrlE)
	expectView(t, e, "CTRL-E with the last row at the top", 100, 100)
}

// CTRL-D and CTRL-U move the view half a pane and the cursor as many rows,
// or by the count given, which is remembered.
func TestScrollHalfPages(t *testing.T) {
	e := newTestEditor(t, numberedRows(100)...)
	e.typeKeys(ctrlD)
	expectView(t, e, "CTRL-D", 16, 16)
	e.typeString("jj")
	e.typeKeys(ctrlD)
	expectView(t, e, "CTRL-D from two rows down the pane", 31, 33)
	e.typeKeys(ctrlU)
	expectView(t, e, "CTRL-U", 16, 18)

	e.typeKeys('5', ctrlD)
	expectView(t, e, "5 CTRL-D", 21, 23)
	e.typeKeys(ctrlD)
	expectView(t, e, "CTRL-D after it: five again", 26, 28)
	e.typeKeys(ctrlU)
	expectView(t, e, "CTRL-U after it", 21, 23)

	// At the ends of the note the view stays and the cursor goes on.
	e = newTestEditor(t, numberedRows(100)...)
	e.typeString("10G")
	e.typeKeys(ctrlU)
	expectView(t, e, "CTRL-U with the first row showing", 1, 1)
	e.typeString("G")
	expectView(t, e, "G", 71, 100)
	e.typeKeys(ctrlU)
	expectView(t, e, "CTRL-U from the end", 56, 85)
	e.typeKeys(ctrlD)
	expectView(t, e, "CTRL-D back to it", 71, 100)
	e.typeString("5k")
	e.typeKeys(ctrlD)
	expectView(t, e, "CTRL-D with the last row showing", 71, 100)
}

// CTRL-D leaves the cursor on the first non-blank of its row, as vim does,
// and in its column with 'nostartofline'.
func TestScrollStartOfLine(t *testing.T) {
	rows := numberedRows(100)
	rows[15] = "    row 16, indented"
	e := newTestEditor(t, rows...)
	e.typeString("7l")
	e.typeKeys(ctrlD)
	expectCursor(t, "CTRL-D", 16, 4)

	e = newTestEditor(t, rows...)
	e.exCommand("set nostartofline")
	defer e.exCommand("set startofline")
	e.typeString("7l")
	e.typeKeys(ctrlD)
	expectCursor(t, "CTRL-D with nostartofline", 16, 7)
}

// <PageDown> and <PageUp> (vim's CTRL-F and CTRL-B) move the view a pane,
// less two rows that stay on screen; the cursor goes to the top row of the
// page after, and the bottom row of the page before.
func TestScrollPages(t *testing.T) {
	e := newTestEditor(t, numberedRows(100)...)
	e.typeKeys(terminal.KeyPageDown)
	expectView(t, e, "<PageDown>", 29, 29)
	e.typeKeys(terminal.KeyPageDown)
	expectView(t, e, "<PageDown> twice", 57, 57)
	e.typeKeys(terminal.KeyPageUp)
	expectView(t, e, "<PageUp>", 29, 58)
	e.typeKeys(terminal.KeyPageUp)
	expectView(t, e, "<PageUp> twice", 1, 30)
	e.typeKeys(terminal.KeyPageUp)
	expectView(t, e, "<PageUp> on the first page", 1, 30)

	e.typeKeys('g', 'g', '2', terminal.KeyPageDown)
	expectView(t, e, "2 <PageDown>", 57, 57)

	// with the end of the note showing, its last row goes to the top
	e.typeString("G")
	e.typeKeys(terminal.KeyPageDown)
	expectView(t, e, "<PageDown> on the last page", 100, 100)
	e.typeKeys(terminal.KeyPageDown)
	expectView(t, e, "<PageDown> past it", 100, 100)

	// in INSERT too, which it does not end
	e = newTestEditor(t, numberedRows(100)...)
	e.typeKeys('i', terminal.KeyPageDown)
	expectView(t, e, "<PageDown> in INSERT", 29, 29)
	if e.mode != INSERT {
		t.Errorf("<PageDown> in INSERT left the editor in %v", e.mode)
	}
	e.typeKeys('\x1b')
}

// zt, zz and zb put the cursor's row at the top, the middle and the bottom
// of the pane, and leave the cursor where it is.
func TestScrollCursorRow(t *testing.T) {
	rows := numberedRows(100)
	rows[59] = "    row 60, indented"
	e := newTestEditor(t, rows...)
	e.typeString("50G4l")

	e.typeString("zt")
	expectView(t, e, "zt", 50, 50)
	e.typeString("zz")
	expectView(t, e, "zz", 36, 50)
	if e.cy != 14 {
		t.Errorf("after zz: the cursor is on line %d of the pane, want 14", e.cy)
	}
	e.typeString("zb")
	expectView(t, e, "zb", 21, 50)
	expectCursor(t, "zt zz zb", 50, 4)

	// with a count, the line to do it to; z<CR>, z. and z- also go to the
	// first non-blank
	e.typeString("5zt")
	expectView(t, e, "5zt", 5, 5)
	e.typeKeys('6', '0', 'z', '\r')
	expectView(t, e, "60z<CR>", 60, 60)
	expectCursor(t, "60z<CR>", 60, 4)
	e.typeString("0z.")
	expectView(t, e, "z.", 46, 60)
	expectCursor(t, "z.", 60, 4)
	e.typeString("0z-")
	expectView(t, e, "z-", 31, 60)

	// near the end of the note there is nothing under the last row
	e.typeString("98Gzz")
	expectView(t, e, "zz on line 98 of 100", 84, 98)
	e.typeString("Gzt")
	expectView(t, e, "zt on the last line", 100, 100)
	// near the start, nothing above the first
	e.typeString("3Gzz")
	expectView(t, e, "zz on line 3", 1, 3)
	e.typeString("zb")
	expectView(t, e, "zb on line 3", 1, 3)
}

// A row counts for the screen lines it takes.
func TestScrollCountsScreenLines(t *testing.T) {
	// ten rows of three screen lines fill the pane
	e := newTestEditor(t, tallRows(40, 3)...)
	e.typeKeys(ctrlE)
	expectView(t, e, "CTRL-E", 2, 2)
	if e.lineOffset != 3 {
		t.Errorf("after CTRL-E: %d screen lines above the pane, want 3", e.lineOffset)
	}
	e.typeKeys('g', 'g', ctrlD)
	expectView(t, e, "CTRL-D: fifteen screen lines is five rows", 6, 6)
	e.typeKeys(ctrlU)
	expectView(t, e, "CTRL-U", 1, 1)
	e.typeKeys(terminal.KeyPageDown)
	expectView(t, e, "<PageDown>: 28 screen lines is nine rows", 10, 10)
	e.typeKeys(terminal.KeyPageUp)
	expectView(t, e, "<PageUp>", 1, 10)
	e.typeString("20Gzz")
	expectView(t, e, "zz", 16, 20)
	if e.cy != 12 {
		t.Errorf("after zz: the cursor is on line %d of the pane, want 12", e.cy)
	}
	e.typeString("zb")
	expectView(t, e, "zb", 11, 20)

	// rows of two: seven of them are the nearest to half a pane
	e = newTestEditor(t, tallRows(40, 2)...)
	e.typeKeys(ctrlD)
	expectView(t, e, "CTRL-D over rows of two screen lines", 8, 8)
}

// The view follows the cursor: a row at a time when the cursor steps off
// it, and to put the cursor in the middle when it goes far.
func TestViewFollowsTheCursor(t *testing.T) {
	e := newTestEditor(t, numberedRows(100)...)
	e.typeString("29j")
	expectView(t, e, "29j", 1, 30)
	e.typeString("j")
	expectView(t, e, "j off the bottom", 2, 31)
	e.typeString("L10j")
	expectView(t, e, "ten rows below", 12, 41)
	e.typeString("ggG")
	expectView(t, e, "G", 71, 100)
	e.typeString("5k30k")
	expectView(t, e, "k off the top", 65, 65)
	e.typeString("10k")
	expectView(t, e, "ten rows above", 55, 55)
	e.typeString("25G")
	expectView(t, e, "thirty rows above", 11, 25)
	e.typeString("gg")
	expectView(t, e, "gg", 1, 1)
	e.typeString("60G")
	expectView(t, e, "thirty rows below", 45, 60)
	if e.cy != 15 {
		t.Errorf("after 60G: the cursor is on line %d of the pane, want 15", e.cy)
	}
	e.typeString("gg/row 77\r")
	expectView(t, e, "a search", 63, 77)
}

// The cursor's row is shown whole. Going up into the second screen line of
// a wrapped row used to leave the cursor above the pane, and the row not
// drawn (before 2026-11-15).
func TestViewShowsTheCursorsRowWhole(t *testing.T) {
	rows := numberedRows(60)
	rows[21] = threeScreenLines
	rows[23] = threeScreenLines
	e := newTestEditor(t, rows...)
	e.typeString("G24G60l") // on the second screen line of row 24, the top row
	expectView(t, e, "24G", 24, 24)
	e.typeString("k")
	expectView(t, e, "k onto a short row", 23, 23)
	e.typeString("k")
	expectView(t, e, "k into the second screen line of a wrapped row", 22, 22)
	expectCursor(t, "k", 22, 60)
	if e.cy != 1 {
		t.Errorf("the cursor is on line %d of the pane, want 1", e.cy)
	}

	// and coming down onto one that wraps
	e = newTestEditor(t, rows...)
	e.typeString("21G")
	expectView(t, e, "21G", 1, 21)
	e.typeString("ztL") // rows 21 to 46 are the pane's thirty screen lines
	expectView(t, e, "L", 21, 46)
	e.typeString("j")
	expectView(t, e, "j off the bottom: row 21 goes, a screen line", 22, 47)
	rows = append(numberedRows(30), threeScreenLines)
	e = newTestEditor(t, rows...)
	e.typeString("G")
	expectView(t, e, "G onto a wrapped row", 4, 31)
}

// A row taller than the pane is shown from whichever screen line of it keeps
// the cursor on screen.
func TestViewOfARowTallerThanThePane(t *testing.T) {
	e := newTestEditor(t, "before", strings.Repeat("word ", 400), "after") // forty screen lines
	e.typeString("j")
	expectView(t, e, "j onto it", 2, 2)
	if e.cy != 0 || e.topSkip != 0 {
		t.Errorf("at its start: line %d of the pane, %d screen lines left out", e.cy, e.topSkip)
	}
	e.typeString("$")
	if e.cy != 29 || e.topSkip != 10 {
		t.Errorf("at its end: line %d of the pane with %d screen lines left out, want 29 and 10", e.cy, e.topSkip)
	}
	out := stripANSI(captureStdout(t, e.drawText))
	if strings.Contains(out, "before") || strings.Contains(out, "after") {
		t.Errorf("the rows around it are drawn:\n%s", out)
	}
	e.typeString("gk")
	if e.cy != 28 || e.topSkip != 10 {
		t.Errorf("gk: line %d of the pane with %d left out, want 28 and 10", e.cy, e.topSkip)
	}
	e.typeString("0")
	if e.cy != 0 || e.topSkip != 0 {
		t.Errorf("back at its start: line %d of the pane, %d left out", e.cy, e.topSkip)
	}
	e.typeString("j")
	expectView(t, e, "j off it", 3, 3)
}

// A scroll is carried out when vim asks, in the middle of the key: a mapping
// or :normal can move, scroll and edit in one go.
func TestScrollInTheMiddleOfAKey(t *testing.T) {
	e := newTestEditor(t, numberedRows(100)...)
	e.exCommand("nnoremap ,z 50Gzt")
	defer e.exCommand("nunmap ,z")
	e.typeString(",z")
	expectView(t, e, "a mapping to 50Gzt", 50, 50)

	e.typeString("gg")
	e.exCommand(`execute "normal \<C-d>dd"`)
	e.followCursor()
	expectView(t, e, `:normal CTRL-D dd`, 16, 16)
	if got := e.line(15); got != "row 17 of the note" || len(e.ss) != 99 {
		t.Errorf("dd after CTRL-D deleted something else: line 16 is %q of %d", got, len(e.ss))
	}

	// in VISUAL the selection goes with the cursor
	e = newTestEditor(t, numberedRows(100)...)
	e.typeKeys('V', ctrlD, 'd')
	if got := e.line(0); got != "row 17 of the note" {
		t.Errorf("V CTRL-D d: the first line is %q, want row 17", got)
	}
}

// A scroll changes what is on screen and nothing in the text, so the key's
// redraw has to notice it.
func TestScrollIsDrawn(t *testing.T) {
	e := newTestEditor(t, numberedRows(100)...)
	captureStdout(t, e.Draw)
	out := stripANSI(captureStdout(t, func() {
		e.DrawAfterKey(e.ProcessKey(ctrlE), ctrlE)
	}))
	if !strings.Contains(out, "row 31 of the note") || strings.Contains(out, "row 1 of the note") {
		t.Errorf("CTRL-E drew:\n%s", out)
	}
	// and a key that moves nothing draws nothing
	if out := captureStdout(t, func() { e.DrawAfterKey(e.ProcessKey('l'), 'l') }); strings.Contains(out, "row 31") {
		t.Errorf("l drew the text again: %q", out)
	}
}

// A layout of another width wraps the rows above the cursor differently, and
// Draw, which a resize calls, places everything again.
func TestDrawAfterAResizePlacesTheView(t *testing.T) {
	e := newTestEditor(t, tallRows(40, 3)...)
	e.typeString("G")
	expectView(t, e, "G", 31, 40)
	e.ScreenCols = 100 // each row is now two screen lines
	captureStdout(t, e.Draw)
	if want := e.linesAbove(e.firstVisibleRow); e.lineOffset != want {
		t.Errorf("after the resize: %d screen lines counted above the pane, and there are %d", e.lineOffset, want)
	}
	expectView(t, e, "the resize", 31, 40)
	if e.cy != 18 {
		t.Errorf("after the resize: the cursor is on line %d of the pane, want 18", e.cy)
	}
}
