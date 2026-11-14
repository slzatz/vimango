package editor

import (
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
	// "0" first: in libvim the first command in a new buffer finishes with
	// vim recomputing the wanted column, which would undo $'s "the end"
	e.sendKeys("0$")
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
