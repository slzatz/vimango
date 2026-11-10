package main

import (
	"fmt"
	"testing"

	"github.com/slzatz/vimango/vim"
)

// indexedNotes fills a test database with n synced notes that all match the
// word "common", each in the search index as sync leaves it. Note k (tid k)
// has the word k%7+1 times, so the ranking is not the insertion order.
func indexedNotes(t *testing.T, db *Database, n int) {
	t.Helper()
	tx, err := db.MainDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	ftx, err := db.FtsDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for k := 1; k <= n; k++ {
		note := "filler text"
		for i := 0; i < k%7+1; i++ {
			note += " common"
		}
		title := fmt.Sprintf("note %d", k)
		if _, err := tx.Exec("INSERT INTO task (tid, title, note, added) VALUES (?, ?, ?, datetime('now'));", k, title, note); err != nil {
			t.Fatal(err)
		}
		if _, err := ftx.Exec("INSERT INTO fts (title, note, tag, tid) VALUES (?, ?, '', ?);", title, note, k); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := ftx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// A search that matches more notes than SQLite allows ORDER BY terms (2,000)
// used to fail outright. It returns the best MAX, in the index's rank order.
func TestSearchWithMoreHitsThanSQLiteOrderByTerms(t *testing.T) {
	db := newTestDatabase(t)
	indexedNotes(t, db, 2100)

	rows, err := db.searchEntries("common", "modified", false)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(rows) != MAX {
		t.Fatalf("got %d rows, want the best %d", len(rows), MAX)
	}

	// the order the index ranks them in
	ranked, err := db.FtsDB.Query("SELECT tid FROM fts WHERE fts MATCH 'common' ORDER BY bm25(fts, 2.0, 1.0, 5.0) LIMIT ?;", MAX)
	if err != nil {
		t.Fatal(err)
	}
	defer ranked.Close()
	for i := 0; ranked.Next(); i++ {
		var tid int
		if err := ranked.Scan(&tid); err != nil {
			t.Fatal(err)
		}
		if rows[i].tid != tid {
			t.Fatalf("row %d is tid %d, want tid %d (rank order)", i, rows[i].tid, tid)
		}
	}
	if rows[0].ftsTitle == "" {
		t.Error("rows lost the highlighted title from the index")
	}
}

// Deleted and archived notes are left out before the cut to MAX, so they do
// not take the place of live matches.
func TestSearchSkipsDeletedBeforeTheLimit(t *testing.T) {
	db := newTestDatabase(t)
	indexedNotes(t, db, MAX+50)
	if _, err := db.MainDB.Exec("UPDATE task SET deleted=true WHERE tid <= 40;"); err != nil {
		t.Fatal(err)
	}

	rows, err := db.searchEntries("common", "modified", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != MAX {
		t.Errorf("got %d rows, want %d live ones", len(rows), MAX)
	}
	for _, r := range rows {
		if r.tid <= 40 {
			t.Fatalf("deleted note tid %d is in the results", r.tid)
		}
	}
}

// A search that fails leaves the organizer's list as it was. It used to empty
// o.rows, and the next key indexed into nothing: a panic, in raw mode.
func TestFailedSearchKeepsTheList(t *testing.T) {
	vim.InitializeVim(0)
	sess := &Session{}
	scr := &Screen{screenCols: 120, screenLines: 40, textLines: 30, divider: 40, totaleditorcols: 78, Session: sess}
	db := newTestDatabase(t)
	o := &Organizer{Session: sess, Screen: scr, Database: db, view: TASK, taskview: BY_FOLDER, filter: "none", sort: "modified",
		rows: []Row{{id: 1, title: "one"}, {id: 2, title: "two"}}, fr: 1, vbuf: vim.NewBuffer(0)}
	vim.SetCurrentBuffer(o.vbuf)
	o.readRowsIntoBuffer()

	db.FtsDB.Close() // any search now errors
	o.command_line = "find anything"
	captureStdout(t, func() { o.find(4) })

	if len(o.rows) != 2 || o.fr != 1 {
		t.Fatalf("after a failed search: %d rows, fr=%d; want the 2 rows and fr=1 untouched", len(o.rows), o.fr)
	}
	if o.taskview != BY_FOLDER || o.filter != "none" {
		t.Errorf("a failed search changed the view: taskview=%d filter=%q", o.taskview, o.filter)
	}
	// the key that used to crash
	captureStdout(t, func() { o.organizerProcessKey('j') })
}
