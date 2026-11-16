package editor

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/localdb"
	"github.com/slzatz/vimango/vim"
)

// oldStamp is a modified time no test run can produce, so a write shows up as
// a change to it.
const oldStamp = "2000-01-01 00:00:00"

// savedNoteEditor is an editor on a synced note holding text -- in the
// database and in the buffer, as opening it leaves them -- with the layout a
// host app gives, the way vimango_hybrid runs it. The host says whether the
// editor has closed: to vimango_hybrid that is the process ending, its only
// "saved" signal.
func savedNoteEditor(t *testing.T, text string) (*Editor, *testHost) {
	t.Helper()
	db := newTestStore(t)
	const tid = 7
	var id int
	err := db.DB().QueryRow(
		"INSERT INTO task (tid, title, note, added, modified) VALUES (?, 'a note', ?, datetime('now'), ?) RETURNING id;",
		tid, text, oldStamp).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}

	e := newTestEditor(t, text)
	e.Store = db
	e.id = id
	e.dbText = text
	e.Layout = hostAppLayout
	return e, hostOf(e)
}

func noteInDB(t *testing.T, e *Editor) (note, modified string) {
	t.Helper()
	if err := e.Store.DB().QueryRow("SELECT note, modified FROM task WHERE id=?;", e.id).Scan(&note, &modified); err != nil {
		t.Fatalf("reading the note back: %v", err)
	}
	return note, modified
}

// :x on a modified note writes it -- text and search index -- and closes.
func TestExitSavesAndQuits(t *testing.T) {
	e, h := savedNoteEditor(t, "alpha")

	e.sendKeys("x") // "lpha"
	e.exCommand("x")

	if note, modified := noteInDB(t, e); note != "lpha" || modified == oldStamp {
		t.Errorf("after :x the database holds %q (modified %s), want \"lpha\" freshly stamped", note, modified)
	}
	// the index follows by trigger: the new text is found, the old is not
	var found, stale int
	e.Store.DB().QueryRow("SELECT count(*) FROM task_fts WHERE task_fts MATCH 'lpha' AND rowid=?;", e.id).Scan(&found)
	e.Store.DB().QueryRow("SELECT count(*) FROM task_fts WHERE task_fts MATCH 'alpha' AND rowid=?;", e.id).Scan(&stale)
	if found != 1 || stale != 0 {
		t.Errorf("after :x the search index finds the new text %d times and the old %d; want 1 and 0", found, stale)
	}
	if h.open() {
		t.Error(":x saved but did not close the note")
	}
}

// The regression: when the write fails, :x must leave the editor open and the
// buffer modified. A host that injected ":x" reads a process exit as "saved".
func TestExitStaysOpenWhenTheWriteFails(t *testing.T) {
	failures := map[string]func(t *testing.T, e *Editor){
		"database error": func(t *testing.T, e *Editor) {
			e.Store.DB().Close()
		},
		// SQLite calls an UPDATE that matches no row a success
		"note deleted underneath": func(t *testing.T, e *Editor) {
			if _, err := e.Store.DB().Exec("DELETE FROM task WHERE id=?;", e.id); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, breakIt := range failures {
		t.Run(name, func(t *testing.T) {
			e, h := savedNoteEditor(t, "alpha")
			e.sendKeys("x")
			breakIt(t, e)

			e.exCommand("x")

			if !h.open() {
				t.Fatal(":x closed the note although it was not saved")
			}
			if !e.isModified() {
				t.Error("a failed :x left the buffer marked clean")
			}
			if e.line(0) != "lpha" {
				t.Errorf("buffer is %q after a failed :x, want the edit intact", e.line(0))
			}

			// and the refusal holds for :q; only :q! may discard the edit
			e.exCommand("q")
			if !h.open() {
				t.Error(":q closed a note whose save had failed")
			}
			e.exCommand("q!")
			if h.open() {
				t.Error(":q! did not close the note")
			}
		})
	}
}

// As in vim, :x does not write a clean buffer: no new modified time, and a row
// that changed underneath (a sync) keeps its newer text.
func TestExitLeavesACleanNoteAlone(t *testing.T) {
	e, h := savedNoteEditor(t, "alpha")
	if _, err := e.Store.DB().Exec("UPDATE task SET note='newer, from a sync' WHERE id=?;", e.id); err != nil {
		t.Fatal(err)
	}

	e.exCommand("x")

	if note, modified := noteInDB(t, e); note != "newer, from a sync" || modified != oldStamp {
		t.Errorf(":x on a clean buffer wrote the row: note %q, modified %s", note, modified)
	}
	if h.open() {
		t.Error(":x on a clean buffer did not close the note")
	}
}

// :w that fails leaves the buffer modified, so the quit guard still protects it.
func TestFailedWriteLeavesTheBufferModified(t *testing.T) {
	e, h := savedNoteEditor(t, "alpha")
	e.sendKeys("x")
	e.Store.DB().Close()

	e.exCommand("w")
	if !e.isModified() {
		t.Fatal("a failed :w marked the buffer clean")
	}
	e.exCommand("q")
	if !h.open() {
		t.Error(":q closed a note whose :w had failed")
	}
}

// :checkstale is what the host sends after a sync. A clean buffer takes the
// row's new text, and is still clean.
func TestCheckStaleReloadsACleanBuffer(t *testing.T) {
	e, _ := savedNoteEditor(t, "alpha")
	if _, err := e.Store.DB().Exec("UPDATE task SET note='newer\nfrom a sync', title='renamed' WHERE id=?;", e.id); err != nil {
		t.Fatal(err)
	}

	out := stripANSI(captureStdout(t, func() { e.exCommand("checkstale") }))

	if got := strings.Join(e.vbuf.Lines(), "|"); got != "newer|from a sync" {
		t.Errorf("after :checkstale the buffer is %q, want the row's text", got)
	}
	// the editor draws the new text itself: it has no screen to ask
	if !strings.Contains(out, "newer") || !strings.Contains(out, "from a sync") {
		t.Errorf(":checkstale did not draw the reloaded text: %q", out)
	}
	if e.isModified() {
		t.Error("the reload counted as an edit")
	}
	if e.title != "renamed" || e.dbText != "newer\nfrom a sync" {
		t.Errorf("after :checkstale title %q, baseline %q", e.title, e.dbText)
	}
}

// The text a reload brings is not an edit to undo, and what was there to undo
// before it was about the old text. With an edit saved, the note changed by a
// sync and reloaded, u put line 1 of the old text over line 1 of the new, and
// :x wrote the mixture to the database -- from where it went to every client
// as the newest version.
func TestCheckStaleReloadForgetsUndo(t *testing.T) {
	e, h := savedNoteEditor(t, "alpha one\nbravo two\ncharlie three")
	e.vbuf.SetLines(0, -1, strings.Split(e.dbText, "\n")) // as Open reads a note
	e.ss = e.vbuf.Lines()
	e.saveTick = e.vbuf.GetLastChangedTick()
	captureStdout(t, func() {
		e.sendKeys("ggx")
		e.exCommand("w")
	})
	assertLines(t, e, "lpha one", "bravo two", "charlie three")
	const synced = "rewritten elsewhere\nsecond\nthird\nfourth"
	if _, err := e.Store.DB().Exec("UPDATE task SET note=? WHERE id=?;", synced, e.id); err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() { e.exCommand("checkstale") })

	captureStdout(t, func() { e.sendKeys("u") })
	if got := strings.Join(e.vbuf.Lines(), "\n"); got != synced {
		t.Fatalf("u after the reload left %q, want the row's text", got)
	}
	vimAndEditorAgree(t, e, "after u")
	if e.isModified() {
		t.Error("u after the reload counts as an edit")
	}

	// what is typed after the reload is undone, back to the reloaded text
	captureStdout(t, func() { e.sendKeys("ddx") })
	assertLines(t, e, "econd", "third", "fourth")
	captureStdout(t, func() { e.sendKeys("uuu") })
	if got := strings.Join(e.vbuf.Lines(), "\n"); got != synced {
		t.Errorf("undoing what was typed after the reload left %q", got)
	}

	captureStdout(t, func() { e.exCommand("x") })
	if h.open() {
		t.Error(":x did not close the note")
	}
	if note, _ := noteInDB(t, e); note != synced {
		t.Errorf("the row holds %q after :x, want what the sync wrote", note)
	}
}

// Both sides changed: only the user can choose, so it says so and touches
// nothing. A note deleted elsewhere is said too.
func TestCheckStaleWarnsAndTouchesNothing(t *testing.T) {
	e, _ := savedNoteEditor(t, "alpha")
	e.sendKeys("x") // "lpha", unsaved
	if _, err := e.Store.DB().Exec("UPDATE task SET note='newer' WHERE id=?;", e.id); err != nil {
		t.Fatal(err)
	}
	if msg := exMessage(t, e, "checkstale"); !strings.Contains(msg, "changed in the database and you have unsaved changes") {
		t.Errorf(":checkstale with both sides changed said %q", msg)
	}
	if e.line(0) != "lpha" || !e.isModified() || e.dbText != "alpha" {
		t.Errorf("the buffer is %q (modified %v, baseline %q); it should be untouched", e.line(0), e.isModified(), e.dbText)
	}

	if _, err := e.Store.DB().Exec("DELETE FROM task WHERE id=?;", e.id); err != nil {
		t.Fatal(err)
	}
	if msg := exMessage(t, e, "checkstale"); !strings.Contains(msg, "was deleted elsewhere") {
		t.Errorf(":checkstale on a deleted note said %q", msg)
	}
	if e.line(0) != "lpha" {
		t.Errorf("the buffer is %q", e.line(0))
	}
}

// Stale is what :checkstale found and could not settle, for a host to show:
// the row changed under unsaved changes, or is gone. It ends when the two
// are the same again -- a save that goes through, a reload, or the row back
// as it was read.
func TestStaleIsWhatCheckStaleCouldNotSettle(t *testing.T) {
	e, _ := savedNoteEditor(t, "alpha")
	setRow := func(text string) {
		t.Helper()
		if _, err := e.Store.DB().Exec("UPDATE task SET note=? WHERE id=?;", text, e.id); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want bool, when string) {
		t.Helper()
		captureStdout(t, func() { e.exCommand("checkstale") })
		if e.Stale() != want {
			t.Fatalf("%s: Stale() is %v", when, e.Stale())
		}
	}

	check(false, "the row as it was read")
	setRow("newer")
	captureStdout(t, func() { e.sendKeys("x") })
	check(true, "the row changed under unsaved changes")
	check(true, "and asked again")
	setRow("alpha")
	check(false, "the row back as it was read")

	setRow("newer still")
	check(true, "changed again")
	captureStdout(t, func() { e.exCommand("w") })
	if e.Stale() {
		t.Error("stale after :w put the buffer in the row")
	}

	// a clean buffer is reloaded, and that is not stale
	setRow("from a sync")
	check(false, "a clean buffer, reloaded")
	if e.line(0) != "from a sync" {
		t.Fatalf("the buffer is %q", e.line(0))
	}

	// a read that fails says nothing either way
	captureStdout(t, func() { e.sendKeys("x") })
	setRow("and again")
	check(true, "changed under unsaved changes")
	if _, err := e.Store.DB().Exec("ALTER TABLE task RENAME TO task_gone;"); err != nil {
		t.Fatal(err)
	}
	check(true, "the row cannot be read")
	if _, err := e.Store.DB().Exec("ALTER TABLE task_gone RENAME TO task;"); err != nil {
		t.Fatal(err)
	}

	// a deleted row is stale, and :w, which cannot write it, leaves it so
	if _, err := e.Store.DB().Exec("DELETE FROM task WHERE id=?;", e.id); err != nil {
		t.Fatal(err)
	}
	check(true, "the row deleted")
	captureStdout(t, func() { e.exCommand("w") })
	if !e.Stale() {
		t.Error("not stale after a :w that had no row to write")
	}
}

// Not being able to read the row is not "the row is now empty". A read that
// failed used to come back as empty text, which a clean buffer was then
// reloaded with: the note blanked on screen.
func TestCheckStaleThatCannotReadChangesNothing(t *testing.T) {
	e, _ := savedNoteEditor(t, "alpha")
	e.Store.DB().Close()

	msg := exMessage(t, e, "checkstale")

	if !strings.Contains(msg, "Could not check note") {
		t.Errorf(":checkstale against a database it cannot read said %q", msg)
	}
	if got := strings.Join(e.vbuf.Lines(), "|"); got != "alpha" || e.dbText != "alpha" {
		t.Errorf("the buffer is %q and the baseline %q; both should still be the note", got, e.dbText)
	}
}

// Closing is all the editor does about it: what follows is whoever opened it's
// to say, through OnClose. The editor closes when and only when the note's
// text is in the database or was thrown away.
func TestEditorClosesThroughOnClose(t *testing.T) {
	e := newTestEditor(t, "alpha")
	e.Store = newTestStore(t)
	id, err := e.Store.Add("a note", false, localdb.DefaultContextUUID, localdb.DefaultFolderUUID)
	if err != nil {
		t.Fatal(err)
	}
	e.id = id
	e.saveTick = e.vbuf.GetLastChangedTick()
	closed := 0
	e.OnClose = func() { closed++ }

	steps := []struct {
		keys, cmd string
		want      int // times closed, after the command
		why       string
	}{
		{"", "q", 1, ":q on a clean note closes it"},
		{"x", "q", 1, ":q on a modified note does not"},
		{"", "q!", 2, ":q! does"},
		{"", "x", 3, ":x writes the note and closes it"},
		{"", "x", 4, ":x on the note just written closes it"},
	}
	captureStdout(t, func() {
		for _, s := range steps {
			e.sendKeys(s.keys)
			e.exCommand(s.cmd)
			if closed != s.want {
				t.Errorf("%s: closed %d times, want %d", s.why, closed, s.want)
			}
		}
	})
	if text, _ := e.Store.Note(id); text != "lpha" {
		t.Errorf(":x left %q in the database, want \"lpha\"", text)
	}
}

// Modified is that the buffer's text is not the text the database holds. It
// used to go by vim's change tick, which only goes up: after x and u the
// note was "modified" -- :q was refused, :x wrote a row that had not changed
// and gave it a new modified time for a sync to push, and a host was told of
// unsaved changes there were none of.
func TestUndoneChangeIsNotAModifiedNote(t *testing.T) {
	e, h := savedNoteEditor(t, "alpha")

	e.sendKeys("x")
	if e.line(0) != "lpha" || !e.Modified() {
		t.Fatalf("after x the note is %q, modified %v", e.line(0), e.Modified())
	}
	e.sendKeys("u")
	if e.line(0) != "alpha" || e.Modified() {
		t.Errorf("after x and u the note is %q, modified %v: it is the note as saved", e.line(0), e.Modified())
	}

	// typed back and not undone: the same text is the same note
	e.sendKeys("xia\x1b")
	if e.line(0) != "alpha" || e.Modified() {
		t.Errorf("with the letter typed back the note is %q, modified %v", e.line(0), e.Modified())
	}

	e.exCommand("x")
	if note, modified := noteInDB(t, e); note != "alpha" || modified != oldStamp {
		t.Errorf(":x wrote a note that had not changed: %q, modified %s", note, modified)
	}
	if h.open() {
		t.Error(":x did not close the note")
	}
}

// :q after x and u closes: there is nothing to lose.
func TestQuitAfterUndoCloses(t *testing.T) {
	e, h := savedNoteEditor(t, "alpha")
	e.sendKeys("xu")
	e.exCommand("q")
	if h.open() {
		t.Error(":q after x and u was refused")
	}
}

// A save moves what "as saved" is. Undo from there goes to a text the
// database no longer holds, and that is a change like any other: :q must not
// close on it. Redo comes back to the saved text.
func TestUndoPastASaveIsAModifiedNote(t *testing.T) {
	e, h := savedNoteEditor(t, "alpha")
	e.sendKeys("x")
	e.exCommand("w")
	if note, _ := noteInDB(t, e); note != "lpha" || e.Modified() {
		t.Fatalf("after x and :w the row is %q and the note modified %v", note, e.Modified())
	}

	e.sendKeys("u")
	if e.line(0) != "alpha" || !e.Modified() {
		t.Fatalf("after u the note is %q, modified %v: the database holds \"lpha\"", e.line(0), e.Modified())
	}
	if msg := exMessage(t, e, "q"); !strings.Contains(msg, "No write since last change") || !h.open() {
		t.Fatalf(":q on a note undone past its save said %q, and left it open: %v", msg, h.open())
	}

	e.ProcessKey(ctrlKey('r'))
	if e.line(0) != "lpha" || e.Modified() {
		t.Errorf("after the redo the note is %q, modified %v", e.line(0), e.Modified())
	}
	e.exCommand("q")
	if h.open() {
		t.Error(":q on the note as saved was refused")
	}
}

// A reload is a new "as saved" too: undo has nothing to go back to, and a
// change undone is the reloaded text again.
func TestReloadedNoteUndoneChangeIsNotModified(t *testing.T) {
	e, _ := savedNoteEditor(t, "alpha")
	if _, err := e.Store.DB().Exec("UPDATE task SET note='bravo' WHERE id=?;", e.id); err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() { e.exCommand("checkstale") })
	if e.line(0) != "bravo" || e.Modified() {
		t.Fatalf("after the reload the note is %q, modified %v", e.line(0), e.Modified())
	}
	e.sendKeys("x")
	if !e.Modified() {
		t.Error("after x the reloaded note is not modified")
	}
	e.sendKeys("u")
	if e.line(0) != "bravo" || e.Modified() {
		t.Errorf("after x and u the reloaded note is %q, modified %v", e.line(0), e.Modified())
	}
}

// What asking costs on a long note, 900 rows: the text is read from vim and
// compared once for each change to it, which is once for a key that changes
// the text. Asked again without a change it costs a tick read from vim.
func BenchmarkModifiedAfterAChange(b *testing.B) {
	e := newTestEditor(b, longNote()...)
	vim.SendInput("x")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.comparedTick = -1 // as after a change
		if !e.isModified() {
			b.Fatal("a changed note is not modified")
		}
	}
}

func BenchmarkModifiedAskedAgain(b *testing.B) {
	e := newTestEditor(b, longNote()...)
	vim.SendInput("x")
	for i := 0; i < b.N; i++ {
		e.isModified()
	}
}
