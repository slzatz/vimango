package editor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// <esc> is a key like any other to vim, and until 2026-11-15 it was not one
// to the editor: handleKey sent it and copied back what it thought an <esc>
// could change. What vim did with it beyond that -- a key it was holding,
// which goes in ahead of the <esc> -- was not drawn, not reported and not
// checked. These are the cases that showed.

const esc = '\x1b'

// ':' typed in VISUAL ends the selection, in vim and so here: the line opens
// on "'<,'>" and the text is drawn without it. Until 2026-11-15 the selection
// stayed on screen, and an <esc> on that line left it there, in NORMAL, with
// nothing selected.
func TestColonEndsTheSelectionOnScreen(t *testing.T) {
	e := newTestEditor(t, "alpha beta", "gamma")
	var atColon bool
	captureStdout(t, func() {
		e.sendKeys("vl")
		atColon = e.ProcessKey(':')
	})
	if e.mode != EX_COMMAND || e.command_line != "'<,'>" {
		t.Fatalf("after v l : the mode is %v and the line %q", e.mode, e.command_line)
	}
	if !atColon {
		t.Error("':' ended VISUAL and nothing asked for the text to be redrawn")
	}
	if out := captureStdout(t, e.drawText); strings.Contains(out, shadedStyle) {
		t.Error("the selection is still drawn while the ':' line is open")
	}
	captureStdout(t, func() { e.ProcessKey(esc) })
	if e.mode != NORMAL || vim.GetCurrentMode() == vim.ModeCmdline {
		t.Errorf("after <esc>: mode %v", e.mode)
	}
}

// A key vim is holding for a mapping goes in ahead of the <esc>. If it
// edits, the edit has to be drawn.
func TestEscDrawsWhatAHeldKeyDid(t *testing.T) {
	e := newMappingEditor(t, "alpha", "nnoremap xx dd")
	var redraw bool
	captureStdout(t, func() {
		e.ProcessKey('x')
		if vim.InputPending() != 1 {
			t.Fatalf("after x: %d bytes held, want 1", vim.InputPending())
		}
		redraw = e.ProcessKey(esc)
	})
	assertLines(t, e, "lpha")
	if !redraw {
		t.Error("the held x deleted a character and nothing asked for a redraw")
	}
	if len(e.ss) != 1 || e.ss[0] != "lpha" {
		t.Errorf("the editor's copy of the text is %q", e.ss)
	}
	// and the next key has nothing to catch up on
	if e.ProcessKey('l') {
		t.Error("the key after the <esc> redrew: the <esc> left the change unaccounted for")
	}
}

// ...and what vim says about it has to be shown.
func TestEscShowsWhatAHeldKeySaid(t *testing.T) {
	e := newMappingEditor(t, "alpha", "nnoremap uu :echo 1<CR>")
	var redraw bool
	out := captureStdout(t, func() {
		e.sendKeys("xu")
		redraw = e.ProcessKey(esc)
	})
	assertLines(t, e, "alpha")
	if !redraw {
		t.Error("the held u undid the change and nothing asked for a redraw")
	}
	if !strings.Contains(out, "1 change; before #1") {
		t.Errorf("vim's report of the undo was not shown: %q", stripANSI(out))
	}
}

// A held key can take vim off the note's buffer; the <esc> behind it must
// bring it back, as any other key does.
func TestEscAfterAHeldKeyLeavesVimOnTheNote(t *testing.T) {
	e := newMappingEditor(t, "alpha")
	titles := vim.NewBuffer(0)
	vim.SetCurrentBuffer(titles)
	titles.SetLines(0, -1, []string{"a title", "another title"})
	vim.SetCurrentBuffer(e.vbuf)
	vim.ExecuteCommand("nnoremap <C-^><C-^> :echo 1<CR>")

	out := captureStdout(t, func() {
		e.ProcessKey(ctrlKey('^'))
		if vim.InputPending() == 0 {
			t.Fatal("vim is not holding the Ctrl-^")
		}
		e.ProcessKey(esc)
	})
	if vim.GetCurrentBuffer().GetID() != e.vbuf.GetID() {
		t.Error("after the <esc> vim is not on the note's buffer")
	}
	if strings.Contains(out, "line 1 of 2") {
		t.Errorf("vim's report on the other buffer was shown: %q", stripANSI(out))
	}
	captureStdout(t, func() {
		e.sendKeys("ix")
		e.ProcessKey(esc)
	})
	assertLines(t, e, "xalpha")
	if got := titles.Lines(); got[0] != "a title" || got[1] != "another title" {
		t.Errorf("the other buffer is now %q", got)
	}
}

// What an <esc> has always done, on the path it now shares.

// <esc> clears the message line: vim does not announce NORMAL.
func TestEscClearsTheMessageLine(t *testing.T) {
	e := newTestEditor(t, "alpha", "bravo", "charlie")
	captureStdout(t, func() { e.sendKeys("yj") }) // "2 lines yanked"... or nothing: either way
	clear := fmt.Sprintf("\x1b[%d;%dH\x1b[%dX", e.Message.Row, e.Message.Col, e.Message.Width)
	for _, keys := range []string{"", "i", "v", "d", "/al", ":se"} {
		var out string
		captureStdout(t, func() { e.sendKeys(keys) })
		out = captureStdout(t, func() { e.ProcessKey(esc) })
		if strings.Count(out, clear) != 1 || stripANSI(out) != "" {
			t.Errorf("<esc> after %q wrote %q, want the message line cleared once and nothing else", keys, out)
		}
		if e.mode != NORMAL || vim.GetCurrentMode() == vim.ModeCmdline {
			t.Errorf("<esc> after %q: mode %v", keys, e.mode)
		}
	}
}

// An insert with a count is carried out by the <esc>: the text it adds is
// drawn then, and is not news to the key after.
func TestEscThatEditsIsAccountedFor(t *testing.T) {
	e := newTestEditor(t, "alpha")
	var redraw bool
	captureStdout(t, func() {
		e.sendKeys("3ix")
		redraw = e.ProcessKey(esc)
	})
	assertLines(t, e, "xxxalpha")
	if !redraw {
		t.Error("3ix<esc> did not ask for a redraw")
	}
	if e.ProcessKey('l') {
		t.Error("the key after 3ix<esc> redrew")
	}
}

// Leaving INSERT redraws, however it is left: the word under the cursor is
// not checked for spelling while it is being typed, and is once it is not.
func TestLeavingInsertRedraws(t *testing.T) {
	e := newMappingEditor(t, "alpha", "inoremap jk <Esc>")
	var byEsc, byMapping bool
	captureStdout(t, func() {
		e.sendKeys("i")
		byEsc = e.ProcessKey(esc)
		e.sendKeys("ij")
		byMapping = e.ProcessKey('k')
	})
	if e.mode != NORMAL {
		t.Fatalf("mode %v, want NORMAL", e.mode)
	}
	assertLines(t, e, "alpha")
	if !byEsc || !byMapping {
		t.Errorf("redraw on leaving INSERT: by <esc> %v, by a mapping %v; want both", byEsc, byMapping)
	}
}
