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

	_ "github.com/mattn/go-sqlite3"
)

// Open opens the vimango.db at path, with foreign keys enforced. Every
// program that opens the database opens it here.
//
// SQLite's foreign_keys setting belongs to a connection, not to the file, and
// is off until asked for; a *sql.DB is a pool that opens connections as it
// needs them. So it is asked for in the DSN, which the driver applies to each
// connection it opens. Running "PRAGMA foreign_keys=ON" on the pool, as
// vimango and vimango-sync did until 2026-11-10, reached whichever one
// connection ran it: a statement that landed on another -- any query made
// while a result set was still being read, anything from a second goroutine --
// was not checked.
//
// Like sql.Open, this validates nothing and creates the file if it is missing.
func Open(path string) (*sql.DB, error) {
	// not a "file:" URI, where a path with %, # or a space would need
	// escaping: the driver takes its own parameters off after the "?" and
	// hands SQLite the path as written
	return sql.Open("sqlite3", path+"?_foreign_keys=on")
}

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
