package editor

import (
	"strings"
	"testing"
)

// A control character in a note is drawn as vim draws it, ^ and its letter,
// two columns wide, and never written to the terminal as it is: there it is
// an instruction, and ^K (a vertical tab) moved the terminal's cursor down a
// line, so that the screen no longer showed the note. vim's Ctrl-K a Ctrl-K
// inserts a ^K, as here, and that is how it was found (2026-11-20).

// rawControls reports the control characters other than the ones the drawing
// itself uses (escape sequences, line ends) in what was written.
func rawControls(out string) []byte {
	var found []byte
	for i := 0; i < len(out); i++ {
		c := out[i]
		if (c < 0x20 && c != 0x1b && c != '\r' && c != '\n') || c == 0x7f {
			found = append(found, c)
		}
	}
	return found
}

func TestControlCharactersAreDrawnAsCaretLetters(t *testing.T) {
	e := newTestEditor(t, "one", "four\vx", "a\x0eb\x7f", "\ttab\x01")
	out := captureStdout(t, e.Draw)
	if raw := rawControls(out); len(raw) != 0 {
		t.Errorf("control characters written to the terminal: %q", raw)
	}
	seen := stripANSI(out)
	for _, want := range []string{"four^Kx", "a^Nb^?", "    tab^A"} {
		if !strings.Contains(seen, want) {
			t.Errorf("%q is not drawn; the screen shows %q", want, seen)
		}
	}
	if !strings.Contains(out, mdSpecialColour+"^K") {
		t.Error("^K is not drawn in its own colour")
	}
}

// What is drawn, where the cursor goes and where a row wraps are measured on
// the same layout: the character after a ^K is three columns in, not one.
func TestControlCharactersTakeTwoColumns(t *testing.T) {
	e := newTestEditor(t, "a\vb\tc")
	for _, c := range []struct{ byteOff, col int }{{0, 0}, {1, 1}, {2, 3}, {3, 4}, {4, 8}} {
		if got := e.getScreenXFromRowColWW(0, c.byteOff); got != c.col {
			t.Errorf("byte %d is drawn at column %d, want %d", c.byteOff, got, c.col)
		}
		if got := rawOffset(e.ss[0], expandedOffset(e.ss[0], c.byteOff)); got != c.byteOff {
			t.Errorf("byte %d comes back from the expanded row as %d", c.byteOff, got)
		}
	}
	// inside the two columns of ^K is the ^K
	if got := rawOffset(e.ss[0], 2); got != 1 {
		t.Errorf("the K of ^K is byte %d, want 1", got)
	}
}

// The way it was found: Ctrl-K a Ctrl-K in INSERT puts a ^K in the note, and
// the screen shows ^K, with the lines above it where they were.
func TestCtrlKACtrlKIsDrawnAsCaretK(t *testing.T) {
	e := newRegisterEditor(t, "one", "two", "three")
	e.Layout = hostAppLayout
	typeKeys(t, e, "G")
	typeKeys(t, e, "A"+ctrlK+"a"+ctrlK)
	assertLines(t, e, "one", "two", "three\v")
	out := captureStdout(t, e.Draw)
	if raw := rawControls(out); len(raw) != 0 {
		t.Errorf("control characters written to the terminal: %q", raw)
	}
	seen := stripANSI(out)
	if !strings.Contains(seen, "one") || !strings.Contains(seen, "two") || !strings.Contains(seen, "three^K") {
		t.Errorf("the screen shows %q", seen)
	}
	row, col, _ := e.Cursor()
	if wantCol := e.LeftMargin + e.left_margin_offset + len("three^K") + 1; col != wantCol || row != e.TopMargin+2 {
		t.Errorf("cursor at %d,%d, want %d,%d", row, col, e.TopMargin+2, wantCol)
	}
	typeKeys(t, e, "\x1b")
}
