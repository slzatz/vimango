package editor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// Drawing the whole editor -- which is what a resize does, after the screen
// has been cleared -- writes the message line too, where the editor's mode
// keeps something on it: the command line being typed, "-- INSERT --", the
// spelling suggestions. Until 2026-11-15 it did not, and a command half
// typed when the window changed size was gone from the screen until the
// next key.

// messageLineOf is what Draw wrote on the message line, without escape
// sequences, and whether it wrote there at all.
func messageLineOf(t *testing.T, e *Editor) (text string, written bool) {
	t.Helper()
	out := captureStdout(t, e.Draw)
	at := fmt.Sprintf("\x1b[%d;%dH\x1b[%dX", e.Message.Row, e.Message.Col, e.Message.Width)
	i := strings.LastIndex(out, at)
	if i < 0 {
		return "", false
	}
	rest := out[i+len(at):]
	if j := strings.Index(rest, "\x1b["+fmt.Sprint(e.Message.Row)+";"); j >= 0 {
		rest = rest[:j]
	}
	return stripANSI(rest), true
}

func TestDrawWritesTheCommandLine(t *testing.T) {
	for _, layout := range []Layout{tuiLayout, hostAppLayout} {
		e := newTestEditor(t, "alpha beta")
		e.Layout = layout
		for keys, want := range map[string]string{
			":s/alpha/x":  ":s/alpha/x",
			"/bet":        "/bet",
			"?al":         "?al",
			"V:":          ":'<,'>",
			"i\x12=1+1":   "=1+1", // the expression line of CTRL-R =
			":echo \x12=": "=",    // ... over a ':' line
		} {
			captureStdout(t, func() { e.sendKeys(keys) })
			if vim.GetCurrentMode() != vim.ModeCmdline {
				t.Fatalf("after %q vim is not on a command line", keys)
			}
			if got, written := messageLineOf(t, e); !written || got != want {
				t.Errorf("after %q a redraw left %q on the message line (written: %v), want %q", keys, got, written, want)
			}
			// the line is still there to type on
			captureStdout(t, func() { e.sendKeys("z") })
			if !strings.HasSuffix(e.command_line, "z") {
				t.Errorf("after %q and a redraw, a key typed gave the line %q", keys, e.command_line)
			}
			for i := 0; i < 3 && e.mode != NORMAL; i++ {
				captureStdout(t, func() { e.ProcessKey(esc) })
			}
			if e.mode != NORMAL {
				t.Fatalf("after %q: could not get back to NORMAL (%v)", keys, e.mode)
			}
		}
	}
}

func TestDrawWritesInsert(t *testing.T) {
	e := newTestEditor(t, "alpha")
	captureStdout(t, func() { e.sendKeys("i") })
	if got, written := messageLineOf(t, e); !written || got != "-- INSERT --" {
		t.Errorf("a redraw in INSERT left %q on the message line (written: %v)", got, written)
	}
	captureStdout(t, func() { e.ProcessKey(esc) })
}

func TestDrawWritesTheSpellingSuggestions(t *testing.T) {
	e := newSpellEditor(t, "Hello wrold here.")
	vim.SetCursorPosition(1, 6)
	shown := stripANSI(captureStdout(t, func() { e.sendKeys("z=") }))
	if e.mode != SPELLING || !strings.Contains(shown, "world") {
		t.Fatalf("z=: mode %v, shown %q", e.mode, shown)
	}
	got, written := messageLineOf(t, e)
	if !written || !strings.HasPrefix(got, `"wrold" ->  1 world`) {
		t.Errorf("a redraw while choosing left %q on the message line (written: %v)", got, written)
	}
	// and the choice still works
	captureStdout(t, func() { e.ProcessKey('1') })
	assertLines(t, e, "Hello world here.")
}

// In NORMAL, and with a notice up, the message line is not the mode's: a
// redraw leaves it as it is.
func TestDrawLeavesOtherMessagesAlone(t *testing.T) {
	e := newTestEditor(t, "alpha")
	if got, written := messageLineOf(t, e); written {
		t.Errorf("a redraw in NORMAL wrote %q on the message line", got)
	}
	captureStdout(t, func() { e.sendKeys("v") })
	if got, written := messageLineOf(t, e); written {
		t.Errorf("a redraw in VISUAL wrote %q on the message line", got)
	}
	captureStdout(t, func() { e.ProcessKey(esc) })
}
