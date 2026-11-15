package editor

import (
	"testing"

	"github.com/slzatz/vimango/vim"
)

// The editor's copy of the text and the cursor is made in one place,
// resyncFromVim, after anything that touches vim. These are the places that
// used to make a copy of their own, or to skip it.

// vimAndEditorAgree fails unless the editor's text and cursor are vim's.
func vimAndEditorAgree(t *testing.T, e *Editor, when string) {
	t.Helper()
	lines := e.vbuf.Lines()
	if len(lines) != len(e.ss) {
		t.Fatalf("%s: vim has %q, the editor %q", when, lines, e.ss)
	}
	for i := range lines {
		if lines[i] != e.ss[i] {
			t.Fatalf("%s: vim has %q, the editor %q", when, lines, e.ss)
		}
	}
	if pos := vim.GetCursorPosition(); pos[0]-1 != e.fr || pos[1] != e.fc {
		t.Errorf("%s: vim's cursor is on row %d, byte %d and the editor's on row %d, byte %d", when, pos[0]-1, pos[1], e.fr, e.fc)
	}
}

// A mapping can edit and then stop half way through an operator. Until
// 2026-11-15 a key that left vim waiting for a motion returned before the
// copy was made: the edit was not drawn, and the tick that said so was spent.
func TestMappingThatEndsInAnOperatorIsDrawn(t *testing.T) {
	e := newMappingEditor(t, "alpha beta", "nnoremap ,x xd")
	var redraw bool
	captureStdout(t, func() {
		e.ProcessKey(',')
		redraw = e.ProcessKey('x')
	})
	assertLines(t, e, "lpha beta")
	if e.mode != PENDING {
		t.Fatalf("mode %v, want PENDING: the d is waiting for its motion", e.mode)
	}
	if !redraw {
		t.Error("the mapping deleted a character and nothing asked for a redraw")
	}
	vimAndEditorAgree(t, e, "half way through the operator")
	captureStdout(t, func() { e.ProcessKey('w') })
	assertLines(t, e, "beta")
	vimAndEditorAgree(t, e, "after the motion")
}

// ...or on a command line.
func TestMappingThatEndsOnACommandLineIsDrawn(t *testing.T) {
	e := newMappingEditor(t, "alpha beta", "nnoremap ,s x:%s/")
	var redraw bool
	captureStdout(t, func() {
		e.ProcessKey(',')
		redraw = e.ProcessKey('s')
	})
	assertLines(t, e, "lpha beta")
	if e.mode != EX_COMMAND || e.command_line != "%s/" {
		t.Fatalf("mode %v, line %q", e.mode, e.command_line)
	}
	if !redraw {
		t.Error("the mapping deleted a character and nothing asked for a redraw")
	}
	vimAndEditorAgree(t, e, "with the command line open")
	captureStdout(t, func() { e.ProcessKey(esc) })
}

// :checkstale puts a row's new text in the buffer and holds the cursor where
// it was -- inside the new text, which can be shorter. Replacing a buffer's
// lines leaves vim's cursor where it was, on a row that may be gone; and the
// editor used to keep a cursor of its own here, one past the end of a row
// where vim, in NORMAL, has it on the last character.
func TestCheckStaleKeepsTheCursorInTheText(t *testing.T) {
	e, _ := savedNoteEditor(t, "alpha")
	e.vbuf.SetLines(0, -1, []string{"alpha", "bravo", "charlie delta"})
	e.dbText = "alpha\nbravo\ncharlie delta"
	e.ss = e.vbuf.Lines()
	e.saveTick = e.vbuf.GetLastChangedTick() // as read from the database
	captureStdout(t, func() { e.sendKeys("G$") })
	if e.fr != 2 || e.fc != 12 {
		t.Fatalf("the cursor is on row %d, byte %d before the reload", e.fr, e.fc)
	}
	if _, err := e.Store.DB().Exec("UPDATE task SET note='one\ntw' WHERE id=?;", e.id); err != nil {
		t.Fatal(err)
	}

	captureStdout(t, func() { e.exCommand("checkstale") })

	assertLines(t, e, "one", "tw")
	vimAndEditorAgree(t, e, "after the reload")
	if e.fr != 1 || e.fc != 1 {
		t.Errorf("the cursor is on row %d, byte %d, want the last character of the last row", e.fr, e.fc)
	}
	if e.isModified() {
		t.Error("the reload counted as an edit")
	}
	// and the next key is typed where the cursor is shown
	captureStdout(t, func() { e.sendKeys("x") })
	assertLines(t, e, "one", "t")
}

// A spelling suggestion is put in by keys vimango sends, and they are
// followed like the user's: the copy is vim's afterwards.
func TestSpellingChoiceIsFollowed(t *testing.T) {
	e := newSpellEditor(t, "Hello wrold here.")
	vim.SetCursorPosition(1, 6)
	var redraw bool
	captureStdout(t, func() {
		e.sendKeys("z=")
		if e.mode != SPELLING {
			t.Fatalf("mode %v, want SPELLING", e.mode)
		}
		redraw = e.ProcessKey('1')
	})
	assertLines(t, e, "Hello world here.")
	if e.mode != NORMAL || !redraw {
		t.Errorf("after choosing: mode %v, redraw %v", e.mode, redraw)
	}
	vimAndEditorAgree(t, e, "after the choice")
	if e.ProcessKey('l') {
		t.Error("the key after the choice redrew: the change was not accounted for")
	}
}
