package localdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// The search index: an FTS5 table inside vimango.db, one row per note with
// rowid = task.id, kept current by triggers.
//
// Until 2026-11-10 the index was a table in a second file, fts5_vimango.db,
// keyed by the server's tid, and every program that changed a note had to
// remember to change the index as well, in a separate, non-atomic write. Not
// all of them did (vimango_hybrid edits titles and never wrote it), a note
// had no index row until its first sync gave it a tid, and the two files
// drifted: titles and bodies in the index that no longer matched the note,
// rows for notes that were gone.
//
// With triggers the index changes in the same transaction as the row, for
// every writer -- vimango, vimango-sync, hybrid's GRDB connection, the sqlite3
// shell -- with no code in any of them. The one requirement is that the
// writer's SQLite has FTS5, since the triggers run in its process; all of
// those do.
//
// Columns, and so bm25's weights, are as before: title, note, tag, where tag
// is the note's keywords joined with commas. Keywords hang off the server tid
// (task_keyword.task_tid), so a note that has not synced has none.

// SearchTable is the name of the index. Search it with
//
//	SELECT ... FROM task_fts JOIN task ON task.id = task_fts.rowid
//	WHERE task_fts MATCH ? ORDER BY bm25(task_fts, 2.0, 1.0, 5.0)
const SearchTable = "task_fts"

// tagOf is SQL for the tag column of the note whose server id is tid (an SQL
// expression). keyword_uuid is the reference on current schemas; databases
// from before it existed may still hold rows that only have keyword_tid.
func tagOf(tid string) string {
	return `(SELECT coalesce(group_concat(k.title, ','), '') FROM task_keyword tk ` +
		`JOIN keyword k ON k.uuid = tk.keyword_uuid OR (tk.keyword_uuid IS NULL AND k.tid = tk.keyword_tid) ` +
		`WHERE tk.task_tid = ` + tid + `)`
}

// reindexNote is the body of the triggers on task: write the note's row
// afresh. Delete-then-insert rather than UPDATE or INSERT OR REPLACE so it
// does the same thing whatever conflict policy the statement that fired it
// carries, and repairs a missing row instead of silently updating nothing.
var reindexNote = `
	DELETE FROM task_fts WHERE rowid = new.id;
	INSERT INTO task_fts(rowid, title, note, tag) VALUES (new.id, new.title, new.note, ` + tagOf("new.tid") + `);`

// retagNote is SQL to recompute the tag of the note with server id tid.
func retagNote(tid string) string {
	return `
	UPDATE task_fts SET tag = ` + tagOf(tid) + ` WHERE rowid IN (SELECT id FROM task WHERE tid = ` + tid + `);`
}

// retagKeyword is SQL to recompute the tag of every note carrying the keyword
// row kw ("new" or "old"), after the keyword was renamed or removed.
func retagKeyword(kw string) string {
	return `
	UPDATE task_fts SET tag = (SELECT ` + tagOf("t.tid") + ` FROM task t WHERE t.id = task_fts.rowid)
	WHERE rowid IN (SELECT t.id FROM task t JOIN task_keyword tk ON tk.task_tid = t.tid
		WHERE tk.keyword_uuid = ` + kw + `.uuid OR tk.keyword_tid = ` + kw + `.tid);`
}

// searchObjects is the index and its triggers: name -> the statement that
// creates it. The database is current when sqlite_master holds exactly these,
// with exactly this text, so changing a statement here rebuilds the index on
// every machine at its next start.
var searchObjects = map[string]string{
	SearchTable: `CREATE VIRTUAL TABLE task_fts USING fts5(title, note, tag)`,

	// a note is added, or its text or tid (and so its keywords) changes
	"task_fts_task_ai": `CREATE TRIGGER task_fts_task_ai AFTER INSERT ON task BEGIN` + reindexNote + `
END`,
	"task_fts_task_au": `CREATE TRIGGER task_fts_task_au AFTER UPDATE OF title, note, tid ON task BEGIN` + reindexNote + `
END`,
	"task_fts_task_ad": `CREATE TRIGGER task_fts_task_ad AFTER DELETE ON task BEGIN
	DELETE FROM task_fts WHERE rowid = old.id;
END`,

	// a keyword is attached to or removed from a note
	"task_fts_tk_ai": `CREATE TRIGGER task_fts_tk_ai AFTER INSERT ON task_keyword BEGIN` + retagNote("new.task_tid") + `
END`,
	"task_fts_tk_au": `CREATE TRIGGER task_fts_tk_au AFTER UPDATE ON task_keyword BEGIN` + retagNote("old.task_tid") + retagNote("new.task_tid") + `
END`,
	"task_fts_tk_ad": `CREATE TRIGGER task_fts_tk_ad AFTER DELETE ON task_keyword BEGIN` + retagNote("old.task_tid") + `
END`,

	// a keyword is renamed or removed
	"task_fts_kw_au": `CREATE TRIGGER task_fts_kw_au AFTER UPDATE OF title ON keyword BEGIN` + retagKeyword("new") + `
END`,
	"task_fts_kw_ad": `CREATE TRIGGER task_fts_kw_ad AFTER DELETE ON keyword BEGIN` + retagKeyword("old") + `
END`,
}

// searchTriggerOrder creates the triggers in a fixed order (map order is not).
var searchTriggerOrder = []string{
	"task_fts_task_ai", "task_fts_task_au", "task_fts_task_ad",
	"task_fts_tk_ai", "task_fts_tk_au", "task_fts_tk_ad",
	"task_fts_kw_au", "task_fts_kw_ad",
}

type execQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// existingSearchObjects reads the index table and every task_fts_* trigger
// from sqlite_master: name -> sql.
func existingSearchObjects(ctx context.Context, q execQuerier) (map[string]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT name, coalesce(sql, '') FROM sqlite_master
		WHERE (type = 'table' AND name = ?) OR (type = 'trigger' AND name LIKE 'task\_fts\_%' ESCAPE '\')`, SearchTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := make(map[string]string)
	for rows.Next() {
		var name, stmt string
		if err := rows.Scan(&name, &stmt); err != nil {
			return nil, err
		}
		found[name] = stmt
	}
	return found, rows.Err()
}

func sameSQL(a, b string) bool {
	return strings.Join(strings.Fields(a), " ") == strings.Join(strings.Fields(b), " ")
}

func searchIndexCurrent(found map[string]string) bool {
	if len(found) != len(searchObjects) {
		return false
	}
	for name, want := range searchObjects {
		if got, ok := found[name]; !ok || !sameSQL(got, want) {
			return false
		}
	}
	return true
}

// ensureSearchIndex creates the index and its triggers and fills the index
// from the notes, unless the database already has exactly that. It reports
// how many notes it indexed, or -1 if there was nothing to do.
//
// Everything happens in one write transaction, taken up front (BEGIN
// IMMEDIATE): if two programs start at once, the second waits for the first
// and then finds the work done. The index is derived from the notes, so
// rebuilding it loses nothing.
func ensureSearchIndex(db *sql.DB) (indexed int, err error) {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	found, err := existingSearchObjects(ctx, conn)
	if err != nil {
		return -1, err
	}
	if searchIndexCurrent(found) {
		return -1, nil
	}

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return -1, err
	}
	defer func() {
		if err != nil {
			conn.ExecContext(ctx, "ROLLBACK")
		}
	}()

	// look again, now that nothing else can be changing it
	if found, err = existingSearchObjects(ctx, conn); err != nil {
		return -1, err
	}
	if searchIndexCurrent(found) {
		_, err = conn.ExecContext(ctx, "COMMIT")
		return -1, err
	}

	for name := range found {
		if name != SearchTable {
			if _, err = conn.ExecContext(ctx, "DROP TRIGGER IF EXISTS "+name); err != nil {
				return -1, err
			}
		}
	}
	if _, err = conn.ExecContext(ctx, "DROP TABLE IF EXISTS "+SearchTable); err != nil {
		return -1, err
	}
	if _, err = conn.ExecContext(ctx, searchObjects[SearchTable]); err != nil {
		return -1, fmt.Errorf("creating %s (does this SQLite have FTS5?): %w", SearchTable, err)
	}
	res, err := conn.ExecContext(ctx, `INSERT INTO task_fts(rowid, title, note, tag)
		SELECT t.id, t.title, t.note, `+tagOf("t.tid")+` FROM task t`)
	if err != nil {
		return -1, fmt.Errorf("filling %s: %w", SearchTable, err)
	}
	n, _ := res.RowsAffected()
	for _, name := range searchTriggerOrder {
		if _, err = conn.ExecContext(ctx, searchObjects[name]); err != nil {
			return -1, fmt.Errorf("creating trigger %s: %w", name, err)
		}
	}
	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		return -1, err
	}
	return int(n), nil
}
