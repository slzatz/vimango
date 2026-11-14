package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/slzatz/vimango/vim"
)

// Mappings with a left-hand side of more than one key. The first key waits in
// vim for the second; libvim used to put the second in front of it, so the
// mapping never completed and the first key was never seen.

// newMappingEditor is an editor with no mappings but the ones given, and
// vim's default timeout, whatever a test before it left behind.
func newMappingEditor(t *testing.T, line string, mappings ...string) *Editor {
	t.Helper()
	e := newTestEditor(t, line)
	vim.ExecuteCommand("mapclear | mapclear!")
	vim.ExecuteCommand("set timeout timeoutlen=1000")
	for _, m := range mappings {
		vim.ExecuteCommand(m)
	}
	t.Cleanup(func() {
		vim.SendKey("<esc>") // a key still held goes to this test's buffer
		vim.ExecuteCommand("mapclear | mapclear!")
		vim.ExecuteCommand("set timeout timeoutlen=1000")
	})
	return e
}

func TestTwoKeyInsertMapping(t *testing.T) {
	e := newMappingEditor(t, "alpha", "inoremap jk <Esc>")

	e.sendKeys("iaj")
	if vim.InputPending() != 1 {
		t.Fatalf("after j: %d bytes held, want 1", vim.InputPending())
	}
	assertLines(t, e, "aalpha") // nothing done with the j yet
	out := captureStdout(t, func() { e.sendKeys("k") })
	if erase := fmt.Sprintf("\x1b[%dX", e.message.width); strings.Contains(out, "INSERT") || !strings.Contains(out, erase) {
		t.Errorf("leaving INSERT by a mapping did not clear the message line: %q", out)
	}
	if vim.InputPending() != 0 {
		t.Errorf("after jk: %d bytes still held", vim.InputPending())
	}
	if e.mode != NORMAL || vim.GetCurrentMode() != vim.ModeNormal {
		t.Errorf("after jk: mode %v, want NORMAL", e.mode)
	}
	assertLines(t, e, "aalpha")
}

// A first key that turns out not to begin the mapping is typed text, in the
// order it was typed.
func TestFirstKeyOfAMappingIsKept(t *testing.T) {
	e := newMappingEditor(t, "alpha", "inoremap jk <Esc>")

	e.sendKeys("ijx")
	if e.mode != INSERT {
		t.Errorf("mode %v, want INSERT", e.mode)
	}
	assertLines(t, e, "jxalpha")

	// "jj": the first is text once the second arrives; the second waits
	e.sendKeys("jj")
	assertLines(t, e, "jxjalpha")
	e.sendKeys("k")
	if e.mode != NORMAL {
		t.Errorf("after jjk: mode %v, want NORMAL", e.mode)
	}
	assertLines(t, e, "jxjalpha")
}

func TestTwoKeyNormalMapping(t *testing.T) {
	e := newMappingEditor(t, "alpha", "nnoremap ,d x")

	e.sendKeys(",d")
	assertLines(t, e, "lpha")
	// ",x" is not the mapping: vim's "," (nothing to repeat) and then x
	e.sendKeys(",x")
	assertLines(t, e, "pha")
	if vim.InputPending() != 0 {
		t.Errorf("%d bytes still held", vim.InputPending())
	}
}

// <esc> is sent by vimango itself, with mappings off. A key vim is holding
// must still come before it.
func TestEscTakesTheHeldKeyFirst(t *testing.T) {
	e := newMappingEditor(t, "alpha", "inoremap jk <Esc>")

	e.sendKeys("ij")
	e.editorProcessKey('\x1b')
	if e.mode != NORMAL {
		t.Errorf("mode %v, want NORMAL", e.mode)
	}
	assertLines(t, e, "jalpha")
	if vim.InputPending() != 0 {
		t.Errorf("%d bytes still held", vim.InputPending())
	}
}

// The timeout: vim takes the held key as typed -- or as the shorter mapping
// it completes -- and the editor redraws.
func TestMappingTimeout(t *testing.T) {
	e := newMappingEditor(t, "alpha", "inoremap jk <Esc>")

	if _, waiting := mappingWait(); waiting {
		t.Error("waiting with nothing typed")
	}
	if e.editorMappingTimeout() {
		t.Error("a timeout with nothing held asked for a redraw")
	}

	e.sendKeys("ij")
	if d, waiting := mappingWait(); !waiting || d != time.Second {
		t.Errorf("mappingWait = %v, %v; want 1s, true", d, waiting)
	}
	vim.ExecuteCommand("set timeoutlen=250")
	if d, waiting := mappingWait(); !waiting || d != 250*time.Millisecond {
		t.Errorf("with timeoutlen=250: mappingWait = %v, %v", d, waiting)
	}
	vim.ExecuteCommand("set notimeout")
	if _, waiting := mappingWait(); waiting {
		t.Error("with notimeout vim waits for the next key, however long")
	}

	if !e.editorMappingTimeout() {
		t.Error("the timeout put text in the note and asked for no redraw")
	}
	assertLines(t, e, "jalpha")
	if e.mode != INSERT || e.fc != 1 {
		t.Errorf("mode %v, column %d; want INSERT, 1", e.mode, e.fc)
	}
	if _, waiting := mappingWait(); waiting || vim.InputPending() != 0 {
		t.Error("still waiting after the timeout")
	}
	// the k that comes too late is a k
	e.sendKeys("k")
	assertLines(t, e, "jkalpha")
}

func TestMappingTimeoutUsesTheShorterMapping(t *testing.T) {
	e := newMappingEditor(t, "alpha", "inoremap j X", "inoremap jk Y")

	e.sendKeys("ij")
	assertLines(t, e, "alpha")
	e.editorMappingTimeout()
	assertLines(t, e, "Xalpha")
	e.sendKeys("jk")
	assertLines(t, e, "XYalpha")
}

// A mapping can end on a command line. The editor asks vim what kind of line
// it is and what is on it; it used to go by the key typed, and took any key
// but ':' for the start of a search.
func TestMappingThatEndsOnACommandLine(t *testing.T) {
	e := newMappingEditor(t, "alpha alpha", "nnoremap ,s :s/alpha/", "nnoremap Q /ph")

	captureStdout(t, func() { e.sendKeys(",s") })
	if e.mode != EX_COMMAND || e.command_line != "s/alpha/" {
		t.Fatalf("after ,s: mode %v, command line %q; want EX_COMMAND, \"s/alpha/\"", e.mode, e.command_line)
	}
	captureStdout(t, func() { e.sendKeys("omega/\r") })
	assertLines(t, e, "omega alpha")
	if e.mode != NORMAL {
		t.Errorf("after the command: mode %v", e.mode)
	}

	captureStdout(t, func() { e.sendKeys("0Q") })
	if e.mode != SEARCH || e.searchPrefix != "/" || e.command_line != "ph" {
		t.Fatalf("after Q: mode %v, prefix %q, line %q; want SEARCH, \"/\", \"ph\"", e.mode, e.searchPrefix, e.command_line)
	}
	captureStdout(t, func() { e.sendKeys("a\r") })
	if got := vim.GetCursorPosition(); got != [2]int{1, 8} {
		t.Errorf("after /pha: cursor %v, want [1 8]", got)
	}
}

// In INSERT mode the held key is shown at the cursor, as vim shows it, and
// the next thing to happen redraws the text over it.
func TestHeldKeyIsShownAtTheCursor(t *testing.T) {
	e := newMappingEditor(t, "alpha", "inoremap jk <Esc>")
	e.sendKeys("i")

	out := captureStdout(t, func() { e.drawAfterKey(e.editorProcessKey('j'), 'j') })
	if !e.heldKeyShown || !strings.HasSuffix(out, "j") {
		t.Errorf("held j not drawn: shown=%v, output ends %q", e.heldKeyShown, out[max(0, len(out)-12):])
	}
	// jk changes no text, and must still redraw: the j is on the screen
	out = captureStdout(t, func() { e.drawAfterKey(e.editorProcessKey('k'), 'k') })
	if e.heldKeyShown || !strings.Contains(out, "alpha") {
		t.Errorf("after jk: shown=%v, text redrawn=%v", e.heldKeyShown, strings.Contains(out, "alpha"))
	}

	// a key vim acts on at once is not drawn twice
	e.sendKeys("i")
	out = captureStdout(t, func() { e.drawAfterKey(e.editorProcessKey('x'), 'x') })
	if e.heldKeyShown {
		t.Error("x is in the text, yet counted as held")
	}
	// nor is one held in NORMAL mode, where it would look like text
	e.editorProcessKey('\x1b')
	vim.ExecuteCommand("nnoremap ,d x")
	out = captureStdout(t, func() { e.drawAfterKey(e.editorProcessKey(','), ',') })
	if e.heldKeyShown || strings.HasSuffix(out, ",") {
		t.Error("a key held in NORMAL mode was drawn in the text")
	}
}
