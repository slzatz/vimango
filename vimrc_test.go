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

// keeppatterns: bulleting must not replace the pattern n searches for.
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
