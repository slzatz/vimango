package editor

import (
	"strconv"
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// The test editor is 50 columns wide, so a row of 25 "word "s is three
// screen lines: bytes 0-49, 50-99 and 100-124.
var threeScreenLines = strings.Repeat("word ", 25)

func expectCursor(t *testing.T, after string, line, col int) {
	t.Helper()
	if got := vim.GetCursorPosition(); got != [2]int{line, col} {
		t.Errorf("after %s: cursor at %v, want [%d %d]", after, got, line, col)
	}
}

func TestScreenLineMotions(t *testing.T) {
	e := newTestEditor(t, threeScreenLines, threeScreenLines)
	vim.SetCursorPosition(1, 7)

	e.sendKeys("gj")
	expectCursor(t, "gj", 1, 57)
	e.sendKeys("gj")
	expectCursor(t, "gj gj", 1, 107)
	e.sendKeys("gj")
	expectCursor(t, "a third gj, onto the next row", 2, 7)
	e.sendKeys("gk")
	expectCursor(t, "gk, back onto the first row's last screen line", 1, 107)
	e.sendKeys("2gk")
	expectCursor(t, "2gk", 1, 7)
	e.sendKeys("gk")
	expectCursor(t, "gk on the first screen line", 1, 7)
	e.sendKeys("9gj")
	expectCursor(t, "9gj: as far as it goes", 2, 107)
}

// A short screen line takes the cursor left of the column it is in; the next
// gj or gk goes back to that column.
func TestScreenLineMotionsKeepTheColumn(t *testing.T) {
	e := newTestEditor(t, threeScreenLines, threeScreenLines)
	vim.SetCursorPosition(1, 40)

	e.sendKeys("gjgj")
	expectCursor(t, "gj gj, onto a 25-column screen line", 1, 124)
	e.sendKeys("gj")
	expectCursor(t, "gj from there", 2, 40)

	// and j onto a short line, then gj: the column j wanted
	// (40l, not SetCursorPosition: that leaves vim's wanted column as it was)
	e = newTestEditor(t, threeScreenLines, "word", threeScreenLines)
	e.sendKeys("40lj")
	expectCursor(t, "j onto a short line", 2, 3)
	e.sendKeys("gj")
	expectCursor(t, "gj after it", 3, 40)
}

// After $, gj and gk go to the end of each screen line.
func TestScreenLineMotionsAfterDollar(t *testing.T) {
	e := newTestEditor(t, threeScreenLines)
	// "$" as the first key in the buffer: libvim used to work the wanted
	// column out after the first command, which undid its "the end"
	e.sendKeys("$")
	expectCursor(t, "$", 1, 124)
	e.sendKeys("gk")
	expectCursor(t, "$ gk", 1, 99)
	e.sendKeys("gk")
	expectCursor(t, "$ gk gk", 1, 49)
	e.sendKeys("gj")
	expectCursor(t, "$ gk gk gj", 1, 99)
}

// A tab is four columns on screen and up to eight in vim's count: the column
// kept is the screen's, and vim is told the character's.
func TestScreenLineMotionsOverTabs(t *testing.T) {
	e := newTestEditor(t, "\tabc", "abcdefgh")
	vim.SetCursorPosition(2, 5)
	e.sendKeys("gk")
	expectCursor(t, "gk from column 5 onto \"\\tabc\"", 1, 2)

	vim.SetCursorPosition(2, 2)
	e.sendKeys("gk")
	expectCursor(t, "gk from column 2: the tab", 1, 0)
}

func TestScreenLineMotionWithAnOperator(t *testing.T) {
	e := newTestEditor(t, threeScreenLines)
	e.sendKeys("dgj")
	if got, want := e.line(0), strings.Repeat("word ", 15); got != want {
		t.Errorf("dgj left %q, want %q", got, want)
	}
}

// H, M and L are the lines on screen, wrapped rows taking their height.
func TestScreenTopMiddleBottom(t *testing.T) {
	lines := []string{"one", "two", "three", threeScreenLines, "five", "six", "seven"}
	e := newTestEditor(t, lines...)
	e.ScreenLines = 5
	e.firstVisibleRow, e.lineOffset = 2, 2 // rows 3, 4 (three screen lines) and 5
	vim.SetCursorPosition(4, 0)
	e.fr, e.fc = 3, 0

	for _, c := range []struct {
		keys string
		line int
	}{
		{"H", 3},
		{"L", 5},
		{"M", 4},
		{"2H", 4},
		{"2L", 4},
		{"9H", 5},
	} {
		e.sendKeys(c.keys)
		expectCursor(t, c.keys, c.line, 0)
		e.firstVisibleRow, e.lineOffset = 2, 2
	}
}

// g0, g^, gm and g$ are the start, the first non-blank, the middle and the
// end of the screen line the cursor is on, not of the row.
func TestScreenColumnMotions(t *testing.T) {
	e := newTestEditor(t, threeScreenLines, threeScreenLines)
	for _, c := range []struct {
		from int
		keys string
		col  int
	}{
		{57, "g0", 50},
		{57, "g$", 99}, // the space the row is wrapped after
		{57, "gm", 75}, // half of 50 columns along
		{57, "g^", 50},
		{7, "g0", 0},
		{7, "g$", 49},
		{7, "gm", 25},
		{110, "g0", 100},
		{110, "g$", 124},
		{110, "gm", 124}, // a screen line of 25 columns: as far as it goes
		{50, "g0", 50},   // already there
		{99, "g$", 99},
	} {
		vim.SetCursorPosition(1, c.from)
		e.sendKeys(c.keys)
		expectCursor(t, c.keys+" from column "+strconv.Itoa(c.from), 1, c.col)
		if e.fr != 0 || e.fc != c.col {
			t.Errorf("after %s the editor has the cursor at row %d, column %d; vim has it at column %d", c.keys, e.fr, e.fc, c.col)
		}
	}
}

// A screen line can start with a blank -- the row is wrapped after the last
// space that fits, and more can follow it -- and g^ goes past those.
func TestScreenColumnFirstNonBlank(t *testing.T) {
	// 49 x's and a space fill the first screen line; "  abc" is the second
	e := newTestEditor(t, strings.Repeat("x", 49)+"   abc")
	vim.SetCursorPosition(1, 54)
	e.sendKeys("g0")
	expectCursor(t, "g0", 1, 50)
	e.sendKeys("g^")
	expectCursor(t, "g^", 1, 52)
}

// g$ with a count is the end of a screen line further down, on the next row
// when this one has no more.
func TestScreenColumnEndWithACount(t *testing.T) {
	e := newTestEditor(t, threeScreenLines, threeScreenLines)
	vim.SetCursorPosition(1, 7)
	e.sendKeys("2g$")
	expectCursor(t, "2g$", 1, 99)
	e.sendKeys("3g$")
	expectCursor(t, "3g$ from there", 2, 49)
}

// After g$ the column wanted is the one it went to: j and gj keep it, where
// after $ they would go to the end of each line.
func TestScreenColumnEndKeepsTheColumn(t *testing.T) {
	e := newTestEditor(t, threeScreenLines, threeScreenLines)
	vim.SetCursorPosition(1, 57)
	e.sendKeys("g$j")
	expectCursor(t, "g$ j", 2, 99)

	vim.SetCursorPosition(1, 110)
	e.sendKeys("g0gk")
	expectCursor(t, "g0 gk", 1, 50)
}

func TestScreenColumnMotionsWithAnOperator(t *testing.T) {
	// g$ takes the screen line's last character, the space, with it
	e := newTestEditor(t, threeScreenLines)
	vim.SetCursorPosition(1, 55)
	e.sendKeys("dg$")
	if got, want := e.line(0), strings.Repeat("word ", 11)+strings.Repeat("word ", 5); got != want {
		t.Errorf("dg$ left %q, want %q", got, want)
	}

	// g0 stops short of the cursor
	e = newTestEditor(t, threeScreenLines)
	vim.SetCursorPosition(1, 55)
	e.sendKeys("dg0")
	if got, want := e.line(0), strings.Repeat("word ", 24); got != want {
		t.Errorf("dg0 left %q, want %q", got, want)
	}
	expectCursor(t, "dg0", 1, 50)

	e = newTestEditor(t, threeScreenLines)
	vim.SetCursorPosition(1, 55)
	e.sendKeys("vg$d")
	if got, want := e.line(0), strings.Repeat("word ", 11)+strings.Repeat("word ", 5); got != want {
		t.Errorf("vg$d left %q, want %q", got, want)
	}
}

// A tab is one byte and four columns, and a character can be several bytes
// and one column: the screen line is found in columns and answered in bytes.
func TestScreenColumnMotionsOverTabsAndWideBytes(t *testing.T) {
	// the tab and nine words are 49 columns, bytes 0-45; the rest is 46-60
	e := newTestEditor(t, "\t"+strings.Repeat("word ", 12))
	for _, c := range []struct {
		from int
		keys string
		col  int
	}{
		{3, "g$", 45},
		{50, "g0", 46},
		{10, "g0", 0},
		{0, "gm", 22}, // column 25 is byte 22: the tab is columns 0-3
	} {
		vim.SetCursorPosition(1, c.from)
		e.sendKeys(c.keys)
		expectCursor(t, c.keys+" from byte "+strconv.Itoa(c.from)+" of a row with a tab", 1, c.col)
	}

	// "été " is six bytes and four columns: twelve fill the first screen
	// line, bytes 0-71, and eight the second, 72-119
	e = newTestEditor(t, strings.Repeat("été ", 20))
	for _, c := range []struct {
		from int
		keys string
		col  int
	}{
		{80, "g0", 72},
		{80, "g$", 119},
		{0, "g$", 71},
		{0, "gm", 38}, // column 25: the t of the seventh word
	} {
		vim.SetCursorPosition(1, c.from)
		e.sendKeys(c.keys)
		expectCursor(t, c.keys+" from byte "+strconv.Itoa(c.from)+" of a row of two-byte characters", 1, c.col)
	}
}

// With no layout -- no note open: the organizer's title -- they are vim's
// own, worked out from libvim's window of 80 columns: 0 and $ on one line.
func TestScreenColumnMotionsWithoutALayout(t *testing.T) {
	e := newTestEditor(t, "a title")
	vim.SetScreenLayout(nil)
	defer vim.SetScreenLayout(e)
	vim.SetCursorPosition(1, 3)
	e.sendKeys("g$")
	expectCursor(t, "g$ with no layout", 1, 6)
	e.sendKeys("g0")
	expectCursor(t, "g0 with no layout", 1, 0)
}
