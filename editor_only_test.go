package main

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/editor"
)

// --editor opens its note from the database by itself. It used to build an
// organizer, load a list of notes into it, and ask the organizer to open one.
func TestEditorOpensANoteWithNoOrganizer(t *testing.T) {
	a, ids := editorOnlyApp(t, "first line\nsecond line", "another")

	captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
	})
	e := a.Session.editor
	if e == nil {
		t.Fatal("no editor is open")
	}
	if e.ID() != ids[0] || e.Title() != "note A" {
		t.Errorf("the editor holds note %d, %q", e.ID(), e.Title())
	}
	// vim is on the note's buffer, and that holds the note
	if got := noteLines(); got != "first line|second line" {
		t.Errorf("the buffer is %q", got)
	}
	if e.Modified() {
		t.Error("a note just opened counts as modified")
	}
	if a.Organizer != nil {
		t.Error("opening a note made an organizer")
	}

	// and it edits, saves and closes like any editor
	captureStdout(t, func() {
		pressKeys(e, "x")
		runEx(e, "x")
	})
	if text, _ := a.Store.Note(ids[0]); text != "irst line\nsecond line" {
		t.Errorf("after x and :x the note is %q", text)
	}
	if a.Run {
		t.Error(":x did not end the process")
	}
}

// A note that cannot be read is not opened. It used to open as an empty
// buffer, which looks like an empty note, and saving that would make it one.
func TestEditorDoesNotOpenWhatItCannotRead(t *testing.T) {
	a, ids := editorOnlyApp(t, "text")

	if err := a.openEditor(ids[0]+1000, ""); err == nil {
		t.Error("a note that is not there was opened")
	}
	a.Store.DB().Close()
	if err := a.openEditor(ids[0], ""); err == nil {
		t.Error("a note was opened from a database that cannot be read")
	}
	if a.Session.editor != nil {
		t.Error("an editor is open after a failed read")
	}
}

// With no --open, the note modified last.
func TestEditorOpensTheMostRecentNoteByDefault(t *testing.T) {
	a, ids := editorOnlyApp(t, "one", "two", "three")
	for i, day := range []string{"2026-01-03", "2026-01-05", "2026-01-04"} {
		if _, err := a.Store.DB().Exec("UPDATE task SET modified=? WHERE id=?;", day+" 10:00:00", ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	if id, ok := a.mostRecentNote(); !ok || id != ids[1] {
		t.Errorf("the most recent note is %d (found %v), want %d", id, ok, ids[1])
	}
	if _, err := a.Store.DB().Exec("DELETE FROM task;"); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.mostRecentNote(); ok {
		t.Error("a database with no notes has a most recent one")
	}
}

// Everything the editor shows in a box over itself -- :help, the keys that
// scroll it, a vim command's listing -- works with no organizer. The box was
// the organizer's.
func TestEditorHelpWithNoOrganizer(t *testing.T) {
	a, ids := editorOnlyApp(t, "alpha\nbravo\ncharlie")
	captureStdout(t, func() { a.openEditor(ids[0], "") })
	e := a.Session.editor

	out := said(t, e, "help")
	if e.Mode() != editor.HELP || len(a.Screen.notice) < 20 || !strings.Contains(out, "checkstale") {
		t.Fatalf(":help: mode %v, a notice of %d lines", e.Mode(), len(a.Screen.notice))
	}
	captureStdout(t, func() {
		for _, key := range []int{' ', ctrlKey('j'), ctrlKey('j'), 'b', PAGE_DOWN, PAGE_UP, ctrlKey('k'), HOME_KEY} {
			e.ProcessKey(key)
		}
	})
	if e.Mode() != editor.HELP || a.Screen.noticeTop != 0 {
		t.Errorf("after scrolling down and back: mode %v, top line %d", e.Mode(), a.Screen.noticeTop)
	}
	captureStdout(t, func() { e.ProcessKey(' ') })
	if a.Screen.noticeTop == 0 {
		t.Error("Space did not page the help")
	}
	captureStdout(t, func() { e.ProcessKey('\x1b') })
	if e.Mode() != editor.NORMAL && e.Mode() != editor.NORMAL_BUSY { // vim's two names for it
		t.Errorf("Escape left mode %v", e.Mode())
	}

	// a second notice starts at its own top, not where the last one was left
	captureStdout(t, func() { runEx(e, "help normal") })
	if a.Screen.noticeTop != 0 {
		t.Errorf(":help normal opened at line %d of its text", a.Screen.noticeTop)
	}
	captureStdout(t, func() { e.ProcessKey('\x1b') })

	// a listing from vim
	captureStdout(t, func() {
		pressKeys(e, "ma")
		runEx(e, "marks")
	})
	if e.Mode() != editor.HELP || !strings.Contains(strings.Join(a.Screen.notice, "\n"), "mark") {
		t.Errorf(":marks: mode %v, notice %q", e.Mode(), a.Screen.notice)
	}
	captureStdout(t, func() { e.ProcessKey('\x1b') })

	// and what is the host's to do says so
	if msg := said(t, e, "sync"); !strings.Contains(msg, "sync is handled by the app") {
		t.Errorf(":sync said %q", msg)
	}
	if got := noteLines(); got != "alpha|bravo|charlie" || e.Modified() {
		t.Errorf("the note is %q (modified %v) after only looking at help", got, e.Modified())
	}
}

// A resize redraws the editor, and only the editor.
func TestResizeWithNoOrganizer(t *testing.T) {
	a, ids := editorOnlyApp(t, "alpha\nbravo")
	captureStdout(t, func() { a.openEditor(ids[0], "") })
	for _, size := range [][2]int{{30, 80}, {12, 30}, {60, 200}} {
		a.Screen.screenLines, a.Screen.screenCols = size[0], size[1]
		out := captureStdout(t, func() { a.moveDividerPct(a.Screen.edPct) })
		if !strings.Contains(out, "alpha") {
			t.Errorf("at %dx%d the editor was not redrawn", size[0], size[1])
		}
		if strings.Contains(out, "rows:") {
			t.Errorf("at %dx%d the resize wrote the TUI's size report over the editor", size[0], size[1])
		}
	}
	captureStdout(t, func() { a.returnCursor() })
}
