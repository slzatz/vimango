package localdb

import (
	"database/sql"
	"fmt"
	"slices"
	"sync"
	"testing"
)

// migrated is a database with the search index in place.
func migrated(t *testing.T) *sql.DB {
	t.Helper()
	db, _ := newDB(t)
	if _, err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// search returns the titles of the notes matching q, the way the clients ask.
func search(t *testing.T, db *sql.DB, q string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT task.title FROM task_fts JOIN task ON task.id = task_fts.rowid
		WHERE task_fts MATCH ? ORDER BY bm25(task_fts, 2.0, 1.0, 5.0), task.id`, q)
	if err != nil {
		t.Fatalf("search %q: %v", q, err)
	}
	defer rows.Close()
	var titles []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			t.Fatal(err)
		}
		titles = append(titles, title)
	}
	return titles
}

func wantHits(t *testing.T, db *sql.DB, q string, want ...string) {
	t.Helper()
	if got := search(t, db, q); !slices.Equal(got, want) {
		t.Errorf("search %q found %q, want %q", q, got, want)
	}
}

// checkIndex asks FTS5 to verify the index against the text it stores, and
// checks there is exactly one index row per note.
func checkIndex(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO task_fts(task_fts) VALUES ('integrity-check')"); err != nil {
		t.Errorf("FTS5 integrity-check: %v", err)
	}
	var stray int
	err := db.QueryRow(`SELECT (SELECT count(*) FROM task WHERE id NOT IN (SELECT rowid FROM task_fts))
		+ (SELECT count(*) FROM task_fts WHERE rowid NOT IN (SELECT id FROM task))`).Scan(&stray)
	if err != nil || stray != 0 {
		t.Errorf("notes without an index row plus index rows without a note: %d (%v)", stray, err)
	}
}

// The migration indexes what is already there: text, and keywords as tags.
func TestMigrateIndexesExistingNotes(t *testing.T) {
	db, _ := newDB(t)
	exec(t, db, "INSERT INTO task (tid, title, note) VALUES (10, 'Garden plan', 'tomatoes and basil')")
	exec(t, db, "INSERT INTO task (title, note) VALUES ('Never synced', 'so it has no tid')")
	exec(t, db, "INSERT INTO keyword (tid, uuid, title) VALUES (7, 'kw-7', 'outdoors')")
	exec(t, db, "INSERT INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES (10, 7, 'kw-7')")
	// a row from before keyword_uuid existed
	exec(t, db, "INSERT INTO keyword (tid, uuid, title) VALUES (8, 'kw-8', 'summer')")
	exec(t, db, "INSERT INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES (10, 8, NULL)")

	if _, err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	wantHits(t, db, "basil", "Garden plan")
	wantHits(t, db, "garden", "Garden plan")
	wantHits(t, db, "outdoors", "Garden plan")
	wantHits(t, db, "summer", "Garden plan")
	// the old index, keyed by tid, could not hold this one
	wantHits(t, db, "synced", "Never synced")
	checkIndex(t, db)
}

// Every change to a note reaches the index, with no help from the writer.
func TestTriggersFollowNotes(t *testing.T) {
	db := migrated(t)

	exec(t, db, "INSERT INTO task (title, note) VALUES ('Draft', 'about herons')")
	wantHits(t, db, "herons", "Draft")

	exec(t, db, "UPDATE task SET note = 'about egrets' WHERE title = 'Draft'")
	wantHits(t, db, "herons")
	wantHits(t, db, "egrets", "Draft")

	exec(t, db, "UPDATE task SET title = 'Birds' WHERE title = 'Draft'") // hybrid's title edit
	wantHits(t, db, "draft")
	wantHits(t, db, "birds", "Birds")

	exec(t, db, "UPDATE task SET note = NULL WHERE title = 'Birds'") // an emptied note is stored as NULL
	wantHits(t, db, "egrets")
	wantHits(t, db, "birds", "Birds")

	exec(t, db, "UPDATE task SET star = true WHERE title = 'Birds'") // not indexed; must not disturb it
	wantHits(t, db, "birds", "Birds")

	exec(t, db, "DELETE FROM task WHERE title = 'Birds'")
	wantHits(t, db, "birds")
	checkIndex(t, db)
}

// The triggers do the same thing whatever conflict policy the statement that
// fires them carries.
func TestTriggersUnderConflictClauses(t *testing.T) {
	db := migrated(t)
	exec(t, db, "INSERT INTO task (id, title, note) VALUES (50, 'Original', 'walrus')")

	exec(t, db, "INSERT OR REPLACE INTO task (id, title, note) VALUES (50, 'Replacement', 'narwhal')")
	wantHits(t, db, "walrus")
	wantHits(t, db, "narwhal", "Replacement")

	exec(t, db, "INSERT OR IGNORE INTO task (id, title, note) VALUES (50, 'Ignored', 'manatee')")
	wantHits(t, db, "manatee")
	wantHits(t, db, "narwhal", "Replacement")

	exec(t, db, "INSERT OR IGNORE INTO task (id, title, note) VALUES (51, 'Fresh', 'manatee')")
	wantHits(t, db, "manatee", "Fresh")
	checkIndex(t, db)
}

// Keywords are the tag column: attaching, removing, renaming and deleting one
// all show up, and a note picks up its keywords when it gets its tid.
func TestTriggersFollowKeywords(t *testing.T) {
	db := migrated(t)
	exec(t, db, "INSERT INTO task (tid, title, note) VALUES (10, 'Trip', 'packing list')")
	exec(t, db, "INSERT INTO keyword (tid, uuid, title) VALUES (7, 'kw-7', 'travel')")
	exec(t, db, "INSERT INTO keyword (tid, uuid, title) VALUES (8, 'kw-8', 'italy')")

	exec(t, db, "INSERT INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES (10, 7, 'kw-7')")
	exec(t, db, "INSERT INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES (10, 8, 'kw-8')")
	wantHits(t, db, "travel", "Trip")
	wantHits(t, db, "italy", "Trip")

	exec(t, db, "UPDATE keyword SET title = 'journeys' WHERE uuid = 'kw-7'")
	wantHits(t, db, "travel")
	wantHits(t, db, "journeys", "Trip")

	exec(t, db, "DELETE FROM task_keyword WHERE task_tid = 10 AND keyword_uuid = 'kw-8'")
	wantHits(t, db, "italy")
	wantHits(t, db, "journeys", "Trip")

	exec(t, db, "DELETE FROM keyword WHERE uuid = 'kw-7'") // without removing its task_keyword row first
	wantHits(t, db, "journeys")

	// sync writes a pulled note's keywords and its row in either order
	exec(t, db, "INSERT INTO keyword (tid, uuid, title) VALUES (9, 'kw-9', 'recipes')")
	exec(t, db, "INSERT INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES (20, 9, 'kw-9')")
	exec(t, db, "INSERT INTO task (tid, title, note) VALUES (20, 'Pesto', 'pine nuts')")
	wantHits(t, db, "recipes", "Pesto")

	// a local note gets its tid at its first sync
	exec(t, db, "INSERT INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES (30, 9, 'kw-9')")
	exec(t, db, "INSERT INTO task (title, note) VALUES ('Focaccia', 'olive oil')")
	wantHits(t, db, "recipes", "Pesto")
	exec(t, db, "UPDATE task SET tid = 30 WHERE title = 'Focaccia'")
	wantHits(t, db, "recipes", "Pesto", "Focaccia")
	checkIndex(t, db)
}

// A second Migrate finds nothing to do. One that finds the index damaged --
// a trigger missing, so the index has gone stale -- rebuilds it from the notes.
func TestMigrateRepairsTheIndex(t *testing.T) {
	db := migrated(t)
	exec(t, db, "INSERT INTO task (title, note) VALUES ('Tides', 'spring and neap')")

	if changes, err := Migrate(db); err != nil || len(changes) != 0 {
		t.Fatalf("second Migrate: changes %q, err %v; want none", changes, err)
	}

	exec(t, db, "DROP TRIGGER task_fts_task_au")
	exec(t, db, "UPDATE task SET note = 'ebb and flow' WHERE title = 'Tides'")
	wantHits(t, db, "ebb") // stale: the trigger that would have indexed it is gone

	changes, err := Migrate(db)
	if err != nil || len(changes) != 1 {
		t.Fatalf("Migrate with a trigger missing: changes %q, err %v; want the index rebuilt", changes, err)
	}
	wantHits(t, db, "ebb", "Tides")
	wantHits(t, db, "neap")
	exec(t, db, "UPDATE task SET note = 'slack water' WHERE title = 'Tides'")
	wantHits(t, db, "slack", "Tides")
	checkIndex(t, db)
}

// Several programs start at once on a database that needs the index (hybrid
// launching: previews, an editor, a sync). Each runs Migrate; all must come
// out with a working index and none with an error.
func TestMigrateFromSeveralProcessesAtOnce(t *testing.T) {
	db, path := newDB(t)
	for i := 0; i < 500; i++ {
		exec(t, db, "INSERT INTO task (title, note) VALUES (?, 'common text')", fmt.Sprintf("note %d", i))
	}
	if _, err := ensureWAL(db); err != nil {
		t.Fatal(err)
	}

	const starters = 6
	var wg sync.WaitGroup
	built := make([]int, starters)
	errs := make([]error, starters)
	for i := 0; i < starters; i++ {
		handle, err := sql.Open("sqlite3", "file:"+path+"?_busy_timeout=5000")
		if err != nil {
			t.Fatal(err)
		}
		defer handle.Close()
		wg.Add(1)
		go func() {
			defer wg.Done()
			built[i], errs[i] = ensureSearchIndex(handle)
		}()
	}
	wg.Wait()

	builders := 0
	for i := range errs {
		if errs[i] != nil {
			t.Errorf("starter %d: %v", i, errs[i])
		}
		if built[i] >= 0 {
			builders++
		}
	}
	if builders != 1 {
		t.Errorf("%d starters built the index, want exactly 1", builders)
	}
	if got := len(search(t, db, "common")); got != 500 {
		t.Errorf("search finds %d of the 500 notes", got)
	}
	checkIndex(t, db)
}
