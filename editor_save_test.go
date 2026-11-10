package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/slzatz/vimango/internal/localdb"
)

// newTestDatabase is a throwaway vimango.db as --init makes one: the schema,
// then the migrations (WAL, the search index and its triggers).
func newTestDatabase(t *testing.T) *Database {
	t.Helper()
	dir := t.TempDir()
	mainDB, err := sql.Open("sqlite3", filepath.Join(dir, "vimango.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mainDB.Close() })
	if _, err := mainDB.Exec(sqliteSchema); err != nil {
		t.Fatalf("creating schema: %v", err)
	}
	for _, stmt := range []struct{ sql, uuid string }{
		{"INSERT INTO context (title, tid, uuid) VALUES ('none', 1, ?);", DefaultContextUUID},
		{"INSERT INTO folder (title, tid, uuid) VALUES ('none', 1, ?);", DefaultFolderUUID},
	} {
		if _, err := mainDB.Exec(stmt.sql, stmt.uuid); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := localdb.Migrate(mainDB); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return &Database{MainDB: mainDB}
}

// oldStamp is a modified time no test run can produce, so a write shows up as
// a change to it.
const oldStamp = "2000-01-01 00:00:00"

// savedNoteEditor opens a synced note holding text in an editor -- in the
// database and in the buffer, as editNote leaves it --
// with the app in --editor mode, the way vimango_hybrid runs it.
func savedNoteEditor(t *testing.T, text string) (*Editor, *App) {
	t.Helper()
	db := newTestDatabase(t)
	const tid = 7
	var id int
	err := db.MainDB.QueryRow(
		"INSERT INTO task (tid, title, note, added, modified) VALUES (?, 'a note', ?, datetime('now'), ?) RETURNING id;",
		tid, text, oldStamp).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}

	e := newTestEditor(t, text)
	e.Database = db
	e.id = id
	e.dbText = text
	a := withOrganizer(t, e)
	e.Session.editorOnly = true
	return e, a
}

func (e *Editor) exCommand(s string) {
	e.sendKeys(":" + s)
	e.editorProcessKey('\r')
}

func noteInDB(t *testing.T, e *Editor) (note, modified string) {
	t.Helper()
	if err := e.Database.MainDB.QueryRow("SELECT note, modified FROM task WHERE id=?;", e.id).Scan(&note, &modified); err != nil {
		t.Fatalf("reading the note back: %v", err)
	}
	return note, modified
}

// :x on a modified note writes it -- text and search index -- and closes.
func TestExitSavesAndQuits(t *testing.T) {
	e, a := savedNoteEditor(t, "alpha")

	e.sendKeys("x") // "lpha"
	e.exCommand("x")

	if note, modified := noteInDB(t, e); note != "lpha" || modified == oldStamp {
		t.Errorf("after :x the database holds %q (modified %s), want \"lpha\" freshly stamped", note, modified)
	}
	// the index follows by trigger: the new text is found, the old is not
	var found, stale int
	e.Database.MainDB.QueryRow("SELECT count(*) FROM task_fts WHERE task_fts MATCH 'lpha' AND rowid=?;", e.id).Scan(&found)
	e.Database.MainDB.QueryRow("SELECT count(*) FROM task_fts WHERE task_fts MATCH 'alpha' AND rowid=?;", e.id).Scan(&stale)
	if found != 1 || stale != 0 {
		t.Errorf("after :x the search index finds the new text %d times and the old %d; want 1 and 0", found, stale)
	}
	if a.Run {
		t.Error(":x saved but did not end the app")
	}
}

// The regression: when the write fails, :x must leave the editor open and the
// buffer modified. A host that injected ":x" reads a process exit as "saved".
func TestExitStaysOpenWhenTheWriteFails(t *testing.T) {
	failures := map[string]func(t *testing.T, e *Editor){
		"database error": func(t *testing.T, e *Editor) {
			e.Database.MainDB.Close()
		},
		// SQLite calls an UPDATE that matches no row a success
		"note deleted underneath": func(t *testing.T, e *Editor) {
			if _, err := e.Database.MainDB.Exec("DELETE FROM task WHERE id=?;", e.id); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, breakIt := range failures {
		t.Run(name, func(t *testing.T) {
			e, a := savedNoteEditor(t, "alpha")
			e.sendKeys("x")
			breakIt(t, e)

			e.exCommand("x")

			if !a.Run {
				t.Fatal(":x ended the app although the note was not saved")
			}
			if a.Session.editor != e {
				t.Error(":x dropped the editor although the note was not saved")
			}
			if !e.isModified() {
				t.Error("a failed :x left the buffer marked clean")
			}
			if e.line(0) != "lpha" {
				t.Errorf("buffer is %q after a failed :x, want the edit intact", e.line(0))
			}

			// and the refusal holds for :q; only :q! may discard the edit
			e.exCommand("q")
			if !a.Run {
				t.Error(":q closed a note whose save had failed")
			}
			e.exCommand("q!")
			if a.Run {
				t.Error(":q! did not close the note")
			}
		})
	}
}

// The same in the TUI, where closing means wiping the buffer and returning to
// the organizer.
func TestExitStaysOpenWhenTheWriteFailsInTheTUI(t *testing.T) {
	e, a := savedNoteEditor(t, "alpha")
	e.Session.editorOnly = false
	e.sendKeys("x")
	e.Database.MainDB.Close()

	e.exCommand("x")

	if a.Session.editor != e || !a.Session.editorMode {
		t.Fatal(":x closed the note although it was not saved")
	}
	if e.line(0) != "lpha" {
		t.Errorf("buffer is %q after a failed :x, want the edit intact", e.line(0))
	}
}

// As in vim, :x does not write a clean buffer: no new modified time, and a row
// that changed underneath (a sync) keeps its newer text.
func TestExitLeavesACleanNoteAlone(t *testing.T) {
	e, a := savedNoteEditor(t, "alpha")
	if _, err := e.Database.MainDB.Exec("UPDATE task SET note='newer, from a sync' WHERE id=?;", e.id); err != nil {
		t.Fatal(err)
	}

	e.exCommand("x")

	if note, modified := noteInDB(t, e); note != "newer, from a sync" || modified != oldStamp {
		t.Errorf(":x on a clean buffer wrote the row: note %q, modified %s", note, modified)
	}
	if a.Run {
		t.Error(":x on a clean buffer did not end the app")
	}
}

// :w that fails leaves the buffer modified, so the quit guard still protects it.
func TestFailedWriteLeavesTheBufferModified(t *testing.T) {
	e, a := savedNoteEditor(t, "alpha")
	e.sendKeys("x")
	e.Database.MainDB.Close()

	e.exCommand("w")
	if !e.isModified() {
		t.Fatal("a failed :w marked the buffer clean")
	}
	e.exCommand("q")
	if !a.Run {
		t.Error(":q closed a note whose :w had failed")
	}
}
