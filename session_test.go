package main

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// withOrganizer makes e the session's one open editor, with a minimal app
// behind it -- an organizer with its own vim buffer and no rows, so returning
// to it draws nothing that needs a database (displayNote with no rows only
// clears the pane).
func withOrganizer(t *testing.T, e *Editor) *App {
	t.Helper()
	orgBuf := vim.NewBuffer(0)
	a := &App{
		Session:   e.Session,
		Screen:    e.Screen,
		Organizer: &Organizer{Session: e.Session, Screen: e.Screen, vbuf: orgBuf},
		Run:       true,
	}
	prev := app
	app = a
	t.Cleanup(func() { app = prev })

	vim.SetCurrentBuffer(e.vbuf)
	e.Session.editor = e
	e.Session.editorMode = true
	// the note as loaded is the saved state, as editNote records it
	e.saveTick = e.vbuf.GetLastChangedTick()
	return a
}

// editorOnly makes e the editor of a vimango started with --editor, as
// vimango_hybrid starts one: a session in editor-only mode and an app with no
// organizer at all. Anything on the editor's path that reaches for one fails
// here, as it would there.
func editorOnly(t *testing.T, e *Editor) *App {
	t.Helper()
	a := &App{Session: e.Session, Screen: e.Screen, Store: e.Store, Run: true}
	prev := app
	app = a
	t.Cleanup(func() { app = prev })

	vim.SetCurrentBuffer(e.vbuf)
	e.Session.editorOnly = true
	e.Session.editor = e
	e.Session.editorMode = true
	e.saveTick = e.vbuf.GetLastChangedTick()
	return a
}

// One note at a time: :q closes the note and hands focus back to the
// organizer -- no editor left open behind it, vim on the organizer's buffer.
func TestQuitReturnsToOrganizer(t *testing.T) {
	e := newTestEditor(t, "alpha")
	a := withOrganizer(t, e)

	e.sendKeys(":q")
	e.editorProcessKey('\r')

	if a.Session.editor != nil {
		t.Error(":q left an editor open")
	}
	if a.Session.editorMode {
		t.Error(":q left the session in editor mode")
	}
	if got := vim.GetCurrentBuffer(); got.GetID() != a.Organizer.vbuf.GetID() {
		t.Error("vim is not on the organizer's buffer after :q")
	}
	if !a.Run {
		t.Error(":q in the TUI ended the app")
	}
}

// A modified note is not closed by :q, and is by :q!.
func TestQuitRefusesUnsavedChanges(t *testing.T) {
	e := newTestEditor(t, "alpha")
	a := withOrganizer(t, e)

	e.sendKeys("x")
	e.sendKeys(":q")
	e.editorProcessKey('\r')
	if a.Session.editor != e || !a.Session.editorMode {
		t.Fatal(":q closed a note with unsaved changes")
	}

	e.sendKeys(":q!")
	e.editorProcessKey('\r')
	if a.Session.editor != nil || a.Session.editorMode {
		t.Error(":q! did not close the note")
	}
}

// :sync is not the editor's: typed there -- as :test and :sync?, the dry
// runs, too -- it is refused, and the note stays open as it was. It used to
// hand over to the organizer with the note still open and its edits unsaved,
// where :q then quit without a word about them.
func TestSyncIsRefusedInTheEditor(t *testing.T) {
	e := newTestEditor(t, "alpha")
	a := withOrganizer(t, e)
	orgMode := a.Organizer.mode

	captureStdout(t, func() { e.sendKeys("x") }) // an unsaved edit
	for _, cmd := range []string{"sync", "test", "sync?"} {
		if msg := exMessage(t, e, cmd); !strings.Contains(msg, "close the note") {
			t.Errorf(":%s said %q", cmd, msg)
		}
		if a.Session.editor != e || !a.Session.editorMode {
			t.Fatalf(":%s left the editor", cmd)
		}
		if vim.GetCurrentBuffer().GetID() != e.vbuf.GetID() {
			t.Fatalf(":%s took vim off the note's buffer", cmd)
		}
		if a.Organizer.mode != orgMode {
			t.Errorf(":%s reached the organizer: its mode is %v", cmd, a.Organizer.mode)
		}
		if e.mode != NORMAL || !e.isModified() || e.line(0) != "lpha" {
			t.Errorf("after :%s: mode %v, modified %v, text %q", cmd, e.mode, e.isModified(), e.line(0))
		}
	}
}

// --editor has no organizer to return to: :q ends the app instead, leaving the
// editor in place for the main loop to finish its iteration.
func TestQuitInEditorOnlyModeEndsTheApp(t *testing.T) {
	e := newTestEditor(t, "alpha")
	a := editorOnly(t, e)

	e.sendKeys(":q")
	e.editorProcessKey('\r')
	if a.Run {
		t.Error(":q in --editor mode did not end the app")
	}
	if a.Session.editor != e {
		t.Error(":q in --editor mode dropped the editor before the loop finished")
	}
}
