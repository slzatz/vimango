package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// Text that is too long for where it is shown is cut by screen columns, at a
// character boundary. It used to be cut at a byte count, which left half of a
// multi-byte character -- shown as garbage, and one more column than there is
// room for when the character was a wide one.
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

	o, _ := newDBOrganizer(t, long)
	for divider := 20; divider <= 50; divider++ {
		screenOf(e).divider = divider
		e.editorLayout = screenOf(e).editorLayout() // as a resize does
		check("editor message", text(captureStdout(t, func() { e.ShowMessage(BL, "%s", long) })), divider)
		check("editor message, bold", text(captureStdout(t, func() { e.ShowMessage(BL, "\x1b[1m%s\x1b[0m", long) })), divider)

		o.Screen.divider = divider
		check("organizer message", text(captureStdout(t, func() { o.showMessage("%s", long) })), divider)
		// (at these widths the view's name takes most of the bar, so the
		// title may be cut to nothing: only validity and width apply)
		bar := text(captureStdout(t, o.drawStatusBar))
		if !utf8.ValidString(bar) || runewidth.StringWidth(stripANSI(bar)) > divider {
			t.Errorf("organizer status bar at %d columns: %q", divider, bar)
		}
	}
}

// A bold message keeps its closing escape when it is cut, so the bold does not
// run on into whatever is drawn next.
func TestCutMessageKeepsItsReset(t *testing.T) {
	e := newTestEditor(t, "alpha")
	screenOf(e).divider = 10
	e.editorLayout = screenOf(e).editorLayout()
	out := captureStdout(t, func() { e.ShowMessage(BL, "\x1b[1m%s\x1b[0m", strings.Repeat("x", 40)) })
	if !strings.HasSuffix(out, "\x1b[0m") || strings.Count(out, "x") != 10 {
		t.Errorf("cut bold message is %q", out)
	}
}

func TestBackspaceOnTheCommandLineRemovesACharacter(t *testing.T) {
	for in, want := range map[string]string{"find café": "find caf", "find 日本": "find 日", "a": "", "": ""} {
		if got := dropLastRune(in); got != want {
			t.Errorf("dropLastRune(%q) = %q, want %q", in, got, want)
		}
	}
	o, _ := newDBOrganizer(t, "one")
	captureStdout(t, func() {
		for _, r := range ":find é" {
			o.organizerProcessKey(int(r))
		}
		o.organizerProcessKey(BACKSPACE)
	})
	if o.command_line != "find " {
		t.Errorf("after typing \"find é\" and Backspace the command line is %q", o.command_line)
	}
}
