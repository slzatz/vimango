package main

import (
	"testing"

	"github.com/slzatz/vimango/vim"
)

// newVimrcEditor is newTestEditor with vimango.vim sourced, as startup does.
// vim is a process-wide singleton and vimInit does not clear mappings, so
// every test starts from mapclear.
func newVimrcEditor(t *testing.T, lines ...string) *Editor {
	t.Helper()
	e := newTestEditor(t, lines...)
	vim.ExecuteCommand("mapclear | mapclear!")
	sourceVimrc()
	return e
}

func assertLines(t *testing.T, e *Editor, want ...string) {
	t.Helper()
	got := e.vbuf.Lines()
	for i, w := range want {
		if i >= len(got) || got[i] != w {
			t.Fatalf("buffer = %q, want %q", got, want)
		}
	}
}

// The bullet mapping from vimango.vim, typed through editorProcessKey.
func TestVimrcBulletMappingLinewise(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "  bravo", "", "charlie", "delta")

	e.sendKeys("Vjjj-")
	assertLines(t, e, "- alpha", "  - bravo", "", "- charlie", "delta")
	// the mapping's :s ends VISUAL; NORMAL_BUSY is what editorProcessKey
	// records for vim's normal mode after any forwarded key
	if e.mode != NORMAL && e.mode != NORMAL_BUSY {
		t.Errorf("mode after mapping = %v, want normal", e.mode)
	}
	// drawText renders e.ss, so the editor's copy must be current
	if e.ss[0] != "- alpha" {
		t.Errorf("e.ss[0] = %q, the change would not be drawn", e.ss[0])
	}

	// one undo takes the whole list back
	e.sendKeys("u")
	assertLines(t, e, "alpha", "  bravo", "", "charlie", "delta")
}

func TestVimrcBulletMappingBlockwise(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "bravo", "charlie")

	e.sendKeys("\x16jj-") // Ctrl-V
	assertLines(t, e, "- alpha", "- bravo", "- charlie")
}

// Pressing - again on an all-bulleted selection takes the bullets off; a
// mixed selection is completed rather than double-bulleted.
func TestVimrcBulletMappingToggles(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "  bravo", "", "charlie")

	e.sendKeys("Vjjj-")
	assertLines(t, e, "- alpha", "  - bravo", "", "- charlie")

	e.sendKeys("gv-")
	assertLines(t, e, "alpha", "  bravo", "", "charlie")

	// mixed: some lines already bulleted -> bullet the rest, nest nothing
	e2 := newVimrcEditor(t, "- alpha", "bravo", "  - charlie")
	e2.sendKeys("Vjj-")
	assertLines(t, e2, "- alpha", "- bravo", "  - charlie")

	// all blank: nothing to do
	e3 := newVimrcEditor(t, "", "  ", "alpha")
	e3.sendKeys("Vj-")
	assertLines(t, e3, "", "  ", "alpha")

	// a toggle off is one undo step too
	e4 := newVimrcEditor(t, "- alpha", "- bravo")
	e4.sendKeys("Vj-")
	assertLines(t, e4, "alpha", "bravo")
	e4.sendKeys("u")
	assertLines(t, e4, "- alpha", "- bravo")
}

// # is the numbered twin of -: every line gets "1. " (markdown numbers them
// when rendering), and pressing it again takes them off.
func TestVimrcNumberMappingToggles(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "  bravo", "", "charlie")
	e.sendKeys("Vjjj#")
	assertLines(t, e, "1. alpha", "  1. bravo", "", "1. charlie")

	e.sendKeys("gv#")
	assertLines(t, e, "alpha", "  bravo", "", "charlie")

	// any existing number marker counts, not just "1."; mixed is completed
	e2 := newVimrcEditor(t, "3. alpha", "bravo", "12) charlie")
	e2.sendKeys("Vjj#")
	assertLines(t, e2, "3. alpha", "1. bravo", "12) charlie")
	e2.sendKeys("gv#")
	assertLines(t, e2, "alpha", "bravo", "charlie")

	e3 := newVimrcEditor(t, "alpha", "bravo")
	e3.sendKeys("\x16j#") // Ctrl-V
	assertLines(t, e3, "1. alpha", "1. bravo")
	e3.sendKeys("u")
	assertLines(t, e3, "alpha", "bravo")
}

// Each key on a list of the other kind switches it rather than nesting
// markers, and a selection mixing plain, bulleted and numbered lines all ends
// up the requested kind.
func TestVimrcListKindSwitching(t *testing.T) {
	e := newVimrcEditor(t, "- alpha", "  - bravo", "", "- charlie")
	e.sendKeys("Vjjj#")
	assertLines(t, e, "1. alpha", "  1. bravo", "", "1. charlie")

	e.sendKeys("gv-")
	assertLines(t, e, "- alpha", "  - bravo", "", "- charlie")

	// "*" and "+" are bullets too; *italic* at the start of a line is not
	e2 := newVimrcEditor(t, "* alpha", "+ bravo", "*charlie* here", "2. delta")
	e2.sendKeys("Vjjj#")
	assertLines(t, e2, "1. alpha", "1. bravo", "1. *charlie* here", "2. delta")

	e3 := newVimrcEditor(t, "* alpha", "1. bravo", "charlie")
	e3.sendKeys("Vjj-")
	assertLines(t, e3, "* alpha", "- bravo", "- charlie")

	// task items keep their checkbox across a switch
	e4 := newVimrcEditor(t, "- [ ] alpha", "- [x] bravo")
	e4.sendKeys("Vj#")
	assertLines(t, e4, "1. [ ] alpha", "1. [x] bravo")
}

// bulleting must not replace the pattern n searches for.
func TestVimrcBulletMappingKeepsSearchPattern(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "bravo")
	vim.ExecuteCommand("let @/ = 'bravo'")

	e.sendKeys("Vj-")
	if got := vim.EvaluateExpression("@/"); got != "bravo" {
		t.Errorf("@/ = %q after bulleting, want %q", got, "bravo")
	}
}

// Mappings apply to what the user types and to nothing vimango sends itself:
// with "s" mapped to something destructive, a typed s is remapped while an s
// vimango sends through the vim API keeps vim's meaning.
func TestVimrcMappingsDoNotTouchInternalKeys(t *testing.T) {
	e := newVimrcEditor(t, "first", "second")
	vim.ExecuteCommand("nnoremap s dd")

	e.sendKeys("s")
	assertLines(t, e, "second")

	// expansion is off again once the typed key has been handled: plain vim
	// "s" on "second" deletes the 's' and enters INSERT; the mapping would
	// have deleted the line
	vim.SendInput("s")
	vim.SendKey("<esc>")
	assertLines(t, e, "econd")
}

// ---- Ctrl-B / Ctrl-I / Ctrl-E (VimangoDecorate in vimango.vim) ----

// at puts vim's cursor and the editor's copy of it on line 1, byte col.
func (e *Editor) at(col int) *Editor {
	vim.SetCursorPosition(1, col)
	e.ss = e.vbuf.Lines()
	e.fr, e.fc = 0, col
	return e
}

func TestVimrcDecorateWordNormalMode(t *testing.T) {
	e := newVimrcEditor(t, "alpha bravo charlie").at(6) // on "bravo"

	e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "alpha **bravo** charlie")

	// same key again toggles it off -- the cursor was parked on the word
	e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "alpha bravo charlie")

	// a different style replaces rather than nests
	e.editorProcessKey(ctrlKey('b'))
	e.editorProcessKey(ctrlKey('i'))
	assertLines(t, e, "alpha *bravo* charlie")
	e.editorProcessKey(ctrlKey('e'))
	assertLines(t, e, "alpha `bravo` charlie")
	e.editorProcessKey(ctrlKey('e'))
	assertLines(t, e, "alpha bravo charlie")
}

// The cases the old Go version (ciw / <cword>) got wrong: the cursor sitting
// on a marker rather than on the word.
func TestVimrcDecorateCursorOnMarker(t *testing.T) {
	for _, col := range []int{6, 7, 13, 14} { // each * of "**bravo**"
		e := newVimrcEditor(t, "alpha **bravo** charlie").at(col)
		e.editorProcessKey(ctrlKey('b'))
		assertLines(t, e, "alpha bravo charlie")
	}

	e := newVimrcEditor(t, "say `x`, then").at(4) // on the opening `
	e.editorProcessKey(ctrlKey('e'))
	assertLines(t, e, "say x, then")
}

// Sentence punctuation ends the word; whitespace or punctuation under the
// cursor means there is nothing to decorate.
func TestVimrcDecorateWordBoundaries(t *testing.T) {
	e := newVimrcEditor(t, "(alpha), bravo.").at(2)
	e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "(**alpha**), bravo.")

	e2 := newVimrcEditor(t, "alpha bravo").at(5) // the space
	e2.editorProcessKey(ctrlKey('b'))
	assertLines(t, e2, "alpha bravo")

	// non-ASCII letters are word characters
	e3 := newVimrcEditor(t, "un café noir").at(4)
	e3.editorProcessKey(ctrlKey('i'))
	assertLines(t, e3, "un *café* noir")
}

func TestVimrcDecorateVisualSelection(t *testing.T) {
	e := newVimrcEditor(t, "alpha bravo charlie").at(6)
	e.sendKeys("ve") // select "bravo"
	if e.mode != VISUAL {
		t.Fatalf("expected VISUAL, got %v", e.mode)
	}
	redraw := e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "alpha **bravo** charlie")
	if !redraw {
		t.Error("visual Ctrl-B returned redraw=false")
	}
	if e.mode == VISUAL {
		t.Error("still VISUAL after the mapping")
	}

	// a phrase, then toggled off by reselecting just the inner text: the
	// markers outside the selection are found
	e2 := newVimrcEditor(t, "say hello world now").at(4)
	e2.sendKeys("vee")
	e2.editorProcessKey(ctrlKey('b'))
	assertLines(t, e2, "say **hello world** now")
	e2.at(6).sendKeys("vee") // "hello world" inside the markers
	e2.editorProcessKey(ctrlKey('b'))
	assertLines(t, e2, "say hello world now")

	// selecting the markers too works the same
	e3 := newVimrcEditor(t, "a *b c* d").at(2)
	e3.sendKeys("v4l")
	e3.editorProcessKey(ctrlKey('i'))
	assertLines(t, e3, "a b c d")

	// V decorates the line's text, not its indentation
	e4 := newVimrcEditor(t, "  whole line  ")
	e4.sendKeys("V")
	e4.editorProcessKey(ctrlKey('e'))
	assertLines(t, e4, "  `whole line`  ")

	// a selection over two lines is left alone
	e5 := newVimrcEditor(t, "alpha", "bravo")
	e5.sendKeys("vj")
	e5.editorProcessKey(ctrlKey('b'))
	assertLines(t, e5, "alpha", "bravo")
}

// One decoration is one undo step.
func TestVimrcDecorateUndo(t *testing.T) {
	e := newVimrcEditor(t, "alpha bravo").at(0)
	e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "**alpha** bravo")
	e.sendKeys("u")
	assertLines(t, e, "alpha bravo")
}

// The Go intercepts are gone: nothing in normalCmds claims Ctrl-B/E/I, so
// they reach vim and its mappings.
func TestDecorationKeysAreNotInterceptedInGo(t *testing.T) {
	e := newTestEditor(t, "x")
	for _, k := range []int{ctrlKey('b'), ctrlKey('e'), ctrlKey('i')} {
		if _, found := e.normalCmds[string(rune(k))]; found {
			t.Errorf("normalCmds still intercepts %s", keyToDisplayName(string(rune(k))))
		}
	}
}

// A key that ends VISUAL without changing text must still redraw, or the
// selection highlight stays on screen.
func TestLeavingVisualWithoutChangeRedraws(t *testing.T) {
	e := newVimrcEditor(t, "alpha", "bravo")
	e.sendKeys("vj")
	if redraw := e.editorProcessKey(ctrlKey('b')); !redraw { // two lines: no-op
		t.Error("no-op mapping out of VISUAL returned redraw=false")
	}

	e2 := newTestEditor(t, "alpha bravo")
	e2.sendKeys("ve")
	if redraw := e2.editorProcessKey('y'); !redraw {
		t.Error("visual y returned redraw=false")
	}
}

// A missing vimango.vim is normal and must not disturb anything.
func TestVimrcMissingFileIsSilent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	e := newVimrcEditor(t, "alpha")
	e.sendKeys("V-")
	assertLines(t, e, "alpha")
}
