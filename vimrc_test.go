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

// Mappings apply to what the user types and to nothing vimango sends itself.
// Ctrl-B decorates with a counted "s" (decorateSpan); with "s" mapped to
// something destructive, a typed s must be remapped while Ctrl-B still works.
func TestVimrcMappingsDoNotTouchInternalKeys(t *testing.T) {
	e := newVimrcEditor(t, "alpha bravo charlie", "second")
	vim.ExecuteCommand("nnoremap s dd")

	vim.SetCursorPosition(1, 6)
	e.ss = e.vbuf.Lines()
	e.fr, e.fc = 0, 6
	e.editorProcessKey(ctrlKey('b'))
	assertLines(t, e, "alpha **bravo** charlie", "second")

	e.sendKeys("s")
	assertLines(t, e, "second")

	// and expansion is off again once the typed key has been handled
	// plain vim "s" on "second" deletes the 's' and enters INSERT; the
	// mapping would have deleted the line
	vim.SendInput("s")
	vim.SendKey("<esc>")
	assertLines(t, e, "econd")
}

// A missing vimango.vim is normal and must not disturb anything.
func TestVimrcMissingFileIsSilent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	e := newVimrcEditor(t, "alpha")
	e.sendKeys("V-")
	assertLines(t, e, "alpha")
}
