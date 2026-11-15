package editor

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// A notice (:help, what a vim command listed) is drawn over the text, and
// HELP is the mode whose keys scroll it. The two go together: no notice on
// screen with the keys going to the note under it, and no HELP mode with the
// notice gone.

// loopKey is a key as the app's loop gives it: processed, then drawn. It
// reports whether the text was drawn.
func loopKey(t *testing.T, e *Editor, c int) (drew bool) {
	t.Helper()
	out := captureStdout(t, func() { e.DrawAfterKey(e.ProcessKey(c), c) })
	return strings.Contains(out, noticeNote)
}

// noticeEditor is an editor on a note no notice mentions, drawn once, as an
// open editor has been.
const noticeNote = "zebra"

func noticeEditor(t *testing.T) *Editor {
	t.Helper()
	e := newTestEditor(t, noticeNote)
	e.resyncFromVim()
	captureStdout(t, e.Draw)
	return e
}

func loopKeys(t *testing.T, e *Editor, s string) (drew bool) {
	t.Helper()
	for _, r := range s {
		drew = loopKey(t, e, int(r)) || drew
	}
	return drew
}

// ':' typed over a notice opens a command line and leaves the notice up, to
// be read while the command is typed. When that line ends -- any way but in
// another notice -- the keys are the note's again, and the note is drawn.
// Until 2026-11-15 it was not: the notice stayed, over a note being edited.
func TestNoticeEndsWithTheCommandLineTypedOverIt(t *testing.T) {
	for name, keys := range map[string]string{
		"<esc>":                     "\x1b",
		"Enter on an empty line":    "\r",
		"Backspace over the colon":  "\x7f",
		"a command of vim's":        "noh\r",
		"a command of vimango's":    "paste\r",
		"a command that is refused": "edit x\r",
		"a typo":                    "zzzz\r",
	} {
		t.Run(name, func(t *testing.T) {
			e := noticeEditor(t)
			if loopKeys(t, e, ":help\r") || e.mode != HELP {
				t.Fatalf(":help: mode %v", e.mode)
			}
			if loopKey(t, e, ':') {
				t.Error("':' drew the text over the notice")
			}
			if e.mode != EX_COMMAND {
				t.Fatalf("after ':' the mode is %v", e.mode)
			}
			last := []rune(keys)[len([]rune(keys))-1]
			if loopKeys(t, e, keys[:len(keys)-len(string(last))]) {
				t.Error("typing on the line drew the text over the notice")
			}
			if !loopKey(t, e, int(last)) {
				t.Errorf("the line ended (mode %v) and the note was not drawn: the notice is still up", e.mode)
			}
			if e.mode != NORMAL || vim.GetCurrentMode() == vim.ModeCmdline {
				t.Errorf("mode %v", e.mode)
			}
			// once, not at every key from here on
			if loopKey(t, e, 'l') {
				t.Error("the next key drew the text again")
			}
		})
	}
}

// One notice straight after another: the text is not drawn in between.
func TestNoticeAfterNotice(t *testing.T) {
	e := noticeEditor(t)
	loopKeys(t, e, ":help\r")
	if loopKeys(t, e, ":help normal\r") {
		t.Error("the text was drawn between two notices")
	}
	if e.mode != HELP || len(hostOf(e).shown) != 2 {
		t.Fatalf("mode %v, %d notices shown", e.mode, len(hostOf(e).shown))
	}
	if !loopKey(t, e, esc) || e.mode != NORMAL {
		t.Errorf("<esc>: mode %v, or the note was not drawn", e.mode)
	}
}

// Drawing the whole editor -- a resize does -- draws the text over a notice.
// The notice is put back, laid out for the new size. Until 2026-11-15 it was
// gone and the mode was still HELP: every key but the ones that scroll did
// nothing, on a screen showing only the note.
func TestDrawKeepsTheNoticeUp(t *testing.T) {
	e := noticeEditor(t)
	loopKeys(t, e, ":help normal\r")
	h := hostOf(e)
	if e.mode != HELP || len(h.shown) != 1 {
		t.Fatalf("mode %v, %d notices shown", e.mode, len(h.shown))
	}
	out := captureStdout(t, e.Draw)
	if e.mode != HELP {
		t.Fatalf("mode %v after a redraw", e.mode)
	}
	if len(h.shown) != 2 || h.shown[1] != h.shown[0] {
		t.Fatalf("after a redraw in HELP mode %d notices have been shown: the text is over the first", len(h.shown))
	}
	if text, notice := strings.Index(out, noticeNote), strings.LastIndex(out, "vimango.vim mappings"); text < 0 || notice < text {
		t.Error("the notice was not drawn after the text")
	}
	// with no notice up, a redraw shows none
	loopKey(t, e, esc)
	captureStdout(t, e.Draw)
	if len(h.shown) != 2 {
		t.Error("a redraw outside HELP mode showed a notice")
	}
}
