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
func TestLongTitleIsCutAtACharacterBoundary(t *testing.T) {
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

	o, _ := newDBOrganizer(t, long)
	for divider := 20; divider <= 50; divider++ {
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
