// Package localdb keeps a machine's vimango.db in the shape the current code
// expects. The database is local to each machine -- sync carries rows, not
// the file -- so a change to how it is stored has to be made once on every
// machine. Migrate is that change: every step checks the database's actual
// state and does nothing if it is already there, so it is safe to run at
// every startup of every program that opens the file (vimango in all its
// modes, vimango-sync), and `vimango --migrate` runs it by hand and says what
// it did.
package localdb

import (
	"database/sql"
	"fmt"
	"strings"
)

// Migrate brings the database up to date and returns one line per change it
// made; none means it was already current. An error means a step could not
// be done now -- typically because another process was in the middle of a
// transaction -- and nothing is left half-done: run it again.
func Migrate(db *sql.DB) (changes []string, err error) {
	switched, err := ensureWAL(db)
	if err != nil {
		return changes, fmt.Errorf("switching to write-ahead logging: %w", err)
	}
	if switched {
		changes = append(changes, "journal mode: delete -> wal (write-ahead log; readers and the writer no longer block each other)")
	}

	indexed, err := ensureSearchIndex(db)
	if err != nil {
		return changes, fmt.Errorf("building the search index: %w", err)
	}
	if indexed >= 0 {
		changes = append(changes, fmt.Sprintf("search index: built %s from %d notes, with triggers that keep it current (fts5_vimango.db is no longer used)", SearchTable, indexed))
	}
	return changes, nil
}

// ensureWAL puts the database in write-ahead-log mode.
//
// Several processes share this file: the TUI, an `--editor` per pop-out, a
// `--render-html` per preview, vimango-sync, and vimango_hybrid's own
// connection. In SQLite's default rollback-journal mode a reader blocks the
// writer's commit and a committing writer blocks readers, so a sync and a
// preview render wait on each other, up to the busy timeout. In WAL mode
// readers see a snapshot and never block or wait for the writer.
//
// The mode is recorded in the database file, so setting it once is enough
// for every program and every later run; connections already open elsewhere
// pick it up on their next transaction. The switch needs the file to itself
// for an instant: it fails if another connection is inside a transaction for
// longer than the busy timeout, and succeeds if the others are merely open.
//
// Two files appear next to the database while it is open, <db>-wal and
// <db>-shm. Recent commits are in the -wal file until a checkpoint, so a
// backup made by copying the .db file alone can be stale or torn: use
// `sqlite3 <db> ".backup <copy>"`.
func ensureWAL(db *sql.DB) (switched bool, err error) {
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		return false, err
	}
	if strings.EqualFold(mode, "wal") {
		return false, nil
	}
	if err := db.QueryRow("PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		return false, err
	}
	if !strings.EqualFold(mode, "wal") {
		// SQLite answers with the mode in force when it could not change it
		return false, fmt.Errorf("database is in use; journal mode is still %q", mode)
	}
	return true, nil
}
