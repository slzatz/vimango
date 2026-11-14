package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/editor"
	"github.com/slzatz/vimango/internal/localdb"
	"github.com/slzatz/vimango/internal/store"
	"github.com/slzatz/vimango/vim"
)

// The app with a note open in its editor: what a close comes to in the TUI
// and in --editor, and that the editor's :sync reaches neither a sync nor the
// organizer. What the editor itself does is tested with the editor.

// testApp is a vimango with notes to open: a store with one note per text
// ("note A", "note B", ...), and a screen 40 rows by 120 columns. Started with
// --editor it has no organizer and the editor has the whole screen;
// otherwise it is the TUI, the editor in its right pane, with an organizer
// that has a vim buffer of its own and no rows -- so going back to it draws
// nothing that needs a list.
func testApp(t *testing.T, editorOnly bool, texts ...string) (a *App, ids []int) {
	t.Helper()
	vim.InitializeVim(0)
	vim.SendKey("<esc>")
	db := newTestStore(t)
	for i, text := range texts {
		id, err := db.Add("note "+string(rune('A'+i)), false, localdb.DefaultContextUUID, localdb.DefaultFolderUUID)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.SetNote(id, text); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	sess := &Session{editorOnly: editorOnly}
	scr := &Screen{screenCols: 120, screenLines: 40, textLines: 37, divider: 48, totaleditorcols: 70, edPct: 60, Session: sess}
	a = &App{Session: sess, Screen: scr, Store: db, Run: true}
	if editorOnly {
		scr.divider, scr.totaleditorcols, scr.edPct = 1, 118, 100
	} else {
		a.Organizer = &Organizer{Session: sess, Screen: scr, Store: db, vbuf: vim.NewBuffer(0)}
		vim.SetCurrentBuffer(a.Organizer.vbuf)
	}
	prev := app
	app = a
	t.Cleanup(func() { app = prev })
	return a, ids
}

// newTestStore is a throwaway vimango.db as --init makes one: the schema, the
// default containers, and the migrations (WAL, the search index and its
// triggers).
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := localdb.Open(filepath.Join(t.TempDir(), "vimango.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := localdb.Create(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

// tuiApp is the TUI; editorOnlyApp is a vimango started with --editor, before
// its note is open.
func tuiApp(t *testing.T, texts ...string) (*App, []int) { return testApp(t, false, texts...) }

func editorOnlyApp(t *testing.T, texts ...string) (*App, []int) { return testApp(t, true, texts...) }

// openIn opens a note in the app's editor, as :e and --open do.
func openIn(t *testing.T, a *App, id int) *editor.Editor {
	t.Helper()
	captureStdout(t, func() {
		if err := a.openEditor(id, ""); err != nil {
			t.Fatal(err)
		}
	})
	return a.Session.editor
}

// pressKeys types s in the editor, and runEx an ex command and Enter. said is
// what the editor wrote for an ex command, without the escape sequences.
func pressKeys(e *editor.Editor, s string) {
	for _, r := range s {
		e.ProcessKey(int(r))
	}
}

func runEx(e *editor.Editor, cmd string) { pressKeys(e, ":"+cmd+"\r") }

func said(t *testing.T, e *editor.Editor, cmd string) string {
	t.Helper()
	return stripANSI(captureStdout(t, func() { runEx(e, cmd) }))
}

// noteLines is the text of the note vim is on.
func noteLines() string { return strings.Join(vim.GetCurrentBuffer().Lines(), "|") }

// One note at a time: :q closes the note and hands focus back to the
// organizer -- no editor left open behind it, vim on the organizer's buffer.
func TestQuitReturnsToOrganizer(t *testing.T) {
	a, ids := tuiApp(t, "alpha")
	e := openIn(t, a, ids[0])

	captureStdout(t, func() { runEx(e, "q") })

	if a.Session.editor != nil {
		t.Error(":q left an editor open")
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
	a, ids := tuiApp(t, "alpha")
	e := openIn(t, a, ids[0])

	captureStdout(t, func() {
		pressKeys(e, "x")
		runEx(e, "q")
	})
	if a.Session.editor != e {
		t.Fatal(":q closed a note with unsaved changes")
	}

	captureStdout(t, func() { runEx(e, "q!") })
	if a.Session.editor != nil {
		t.Error(":q! did not close the note")
	}
	if text, _ := a.Store.Note(ids[0]); text != "alpha" {
		t.Errorf(":q! left %q in the database", text)
	}
}

// :sync is not the editor's: typed there -- as :test and :sync?, the dry
// runs, too -- it is refused, and the note stays open as it was. It used to
// hand over to the organizer with the note still open and its edits unsaved,
// where :q then quit without a word about them.
func TestSyncIsRefusedInTheEditor(t *testing.T) {
	a, ids := tuiApp(t, "alpha")
	e := openIn(t, a, ids[0])
	orgMode := a.Organizer.mode

	captureStdout(t, func() { pressKeys(e, "x") }) // an unsaved edit
	for _, cmd := range []string{"sync", "test", "sync?"} {
		if msg := said(t, e, cmd); !strings.Contains(msg, "close the note") {
			t.Errorf(":%s said %q", cmd, msg)
		}
		if a.Session.editor != e {
			t.Fatalf(":%s left the editor", cmd)
		}
		if a.Organizer.mode != orgMode {
			t.Errorf(":%s reached the organizer: its mode is %v", cmd, a.Organizer.mode)
		}
		if e.Mode() != editor.NORMAL || !e.Modified() || noteLines() != "lpha" {
			t.Errorf("after :%s: mode %v, modified %v, text %q", cmd, e.Mode(), e.Modified(), noteLines())
		}
	}

	// in --editor the answer is the host's
	a, ids = editorOnlyApp(t, "alpha")
	e = openIn(t, a, ids[0])
	if msg := said(t, e, "sync"); !strings.Contains(msg, "sync is handled by the app") {
		t.Errorf(":sync in --editor said %q", msg)
	}
}

// --editor has no organizer to return to: :q ends the app instead, leaving the
// editor in place for the main loop to finish its iteration.
func TestQuitInEditorOnlyModeEndsTheApp(t *testing.T) {
	a, ids := editorOnlyApp(t, "alpha")
	e := openIn(t, a, ids[0])

	captureStdout(t, func() { runEx(e, "q") })
	if a.Run {
		t.Error(":q in --editor mode did not end the app")
	}
	if a.Session.editor != e {
		t.Error(":q in --editor mode dropped the editor before the loop finished")
	}
}

// A write that fails keeps the note open, in the TUI as in --editor: the
// editor does not say it closed, so the app does nothing.
func TestFailedSaveKeepsTheNoteOpen(t *testing.T) {
	for _, editorOnly := range []bool{false, true} {
		a, ids := testApp(t, editorOnly, "alpha")
		e := openIn(t, a, ids[0])
		captureStdout(t, func() { pressKeys(e, "x") })
		a.Store.DB().Close()

		captureStdout(t, func() { runEx(e, "x") })

		if a.Session.editor != e || !a.Run || !e.Modified() {
			t.Errorf("--editor %v: after a failed :x the editor is open %v, the app running %v, the buffer modified %v",
				editorOnly, a.Session.editor == e, a.Run, e.Modified())
		}
	}
}
