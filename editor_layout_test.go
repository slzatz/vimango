package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// The editor draws by the layout it was given and asks nothing else: not
// which program it is in, not the screen's size. Each editor here is given
// the layout of the *other* program, and draws that. (That the screen makes
// these two layouts is TestEditorLayouts, with the screen.)
func TestEditorDrawsByItsLayout(t *testing.T) {
	// on a terminal of 40 rows by 100 columns: the whole of it, as a host
	// app's window; and its right pane from column 41, as in the TUI
	hostLayout := editorLayout{
		top_margin: 1, left_margin: 1, screenlines: 38, screencols: 98,
		message: messageLine{row: 40, col: 2, width: 99},
		aside:   messageLine{row: 40, col: 2, width: 99},
	}
	paneLayout := editorLayout{
		top_margin: 2, left_margin: 40, screenlines: 37, screencols: 59,
		statusBar: true, corner: true,
		message: messageLine{row: 40, col: 41, width: 60},
		aside:   messageLine{row: 40, col: 1, width: 40},
	}

	inTUI := newTestEditor(t, "alpha")
	inTUI.id, inTUI.title = 7, "a note"
	inTUI.editorLayout = hostLayout

	inHost := newTestEditor(t, "alpha")
	inHost.id, inHost.title = 7, "a note"
	inHost.editorLayout = paneLayout

	for _, c := range []struct {
		name          string
		e             *Editor
		bar, corner   bool
		aside, cursor string
	}{
		{"a TUI editor with the host's layout", inTUI, false, false, "\x1b[40;2H", "40;4"},
		{"a host's editor with the TUI's layout", inHost, true, true, "\x1b[40;1H", "40;43"},
	} {
		e := c.e
		if bar := captureStdout(t, e.drawStatusBar); strings.Contains(bar, "a note") != c.bar {
			t.Errorf("%s: status bar drawn %v, want %v", c.name, !c.bar, c.bar)
		}
		// the corner is on the row above the text, at the frame's column
		corner := fmt.Sprintf("\x1b[%d;%dH\x1b[37;1mk", e.top_margin-1, e.left_margin+e.screencols+1)
		if frame := captureStdout(t, e.drawFrame); strings.Contains(frame, corner) != c.corner {
			t.Errorf("%s: corner drawn %v, want %v", c.name, !c.corner, c.corner)
		}
		if out := captureStdout(t, func() { e.ShowAside("saved") }); !strings.HasPrefix(out, c.aside) {
			t.Errorf("%s: what it says beside the note went to %q, want it to start %q", c.name, out, c.aside)
		}
		// the cursor on a command line: after the ':' and the one character typed
		captureStdout(t, func() { e.sendKeys(":s") })
		row, col, show := e.cursor()
		if got := fmt.Sprintf("%d;%d", row, col); !show || got != c.cursor {
			t.Errorf("%s: cursor on the command line at %s (shown %v), want %s", c.name, got, show, c.cursor)
		}
		captureStdout(t, func() { e.editorProcessKey('\x1b') })
	}
}

// A status bar or a message with no room for all of its text is cut at a
// character boundary. It used to be cut at a byte count, which left half of a
// multi-byte character -- shown as garbage, and one more column than there is
// room for when the character was a wide one. (The organizer's half of this
// is TestLongTitleIsCutAtACharacterBoundary.)
func TestLongTextIsCutAtACharacterBoundary(t *testing.T) {
	// every character here is more than one byte; 日本語 are two columns each
	const long = "café déjà vu — 日本語のノート — żółć — café déjà vu — 日本語のノート — żółć — café déjà vu — 日本語のノート"

	check := func(what, out string, room int) {
		t.Helper()
		if !utf8.ValidString(out) {
			t.Errorf("%s: output is not valid UTF-8: %q", what, out)
		}
		if w := runewidth.StringWidth(stripANSI(out)); w > room {
			t.Errorf("%s: %d columns shown, room for %d", what, w, room)
		}
		if !strings.Contains(out, "caf") {
			t.Errorf("%s: the text is not there at all: %q", what, out)
		}
	}
	// text is what a draw wrote after its last cursor movement
	text := func(out string) string {
		if i := strings.LastIndex(out, "H"); i >= 0 && strings.Contains(out[:i], "\x1b[") {
			return out[i+1:]
		}
		return out
	}

	e := newTestEditor(t, "alpha")
	e.title = long
	e.id = 7
	for cols := 20; cols <= 50; cols++ { // every width, so the cut lands inside each kind of character
		e.screencols = cols
		out := text(captureStdout(t, e.drawStatusBar))
		// the bar is a space and then the row, as it always was
		check("editor status bar", out, cols+1)
		if w := runewidth.StringWidth(stripANSI(out)); w != cols+1 {
			t.Errorf("editor status bar at %d columns is %d wide, want %d: cut or padded by bytes?", cols, w, cols+1)
		}
	}
	for width := 20; width <= 50; width++ { // as a resize changes it
		e.aside.width = width
		check("editor message", text(captureStdout(t, func() { e.ShowAside("%s", long) })), width)
		check("editor message, bold", text(captureStdout(t, func() { e.ShowAside("\x1b[1m%s\x1b[0m", long) })), width)
	}
}

// A bold message keeps its closing escape when it is cut, so the bold does not
// run on into whatever is drawn next.
func TestCutMessageKeepsItsReset(t *testing.T) {
	e := newTestEditor(t, "alpha")
	e.aside.width = 10
	out := captureStdout(t, func() { e.ShowAside("\x1b[1m%s\x1b[0m", strings.Repeat("x", 40)) })
	if !strings.HasSuffix(out, "\x1b[0m") || strings.Count(out, "x") != 10 {
		t.Errorf("cut bold message is %q", out)
	}
}
