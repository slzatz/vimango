package localdb

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// open returns a handle on the database file at path that gives up on a lock
// after 100ms, so a test that expects "busy" does not wait five seconds.
func open(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+path+"?_busy_timeout=100")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// testSchema is the part of vimango's schema the migrations touch.
const testSchema = `
CREATE TABLE keyword (id INTEGER PRIMARY KEY, tid INTEGER UNIQUE, uuid TEXT UNIQUE, title TEXT NOT NULL UNIQUE);
CREATE TABLE task (id INTEGER PRIMARY KEY, tid INTEGER UNIQUE, title TEXT NOT NULL, note TEXT, star BOOLEAN DEFAULT FALSE);
CREATE TABLE task_keyword (task_tid INTEGER NOT NULL, keyword_tid INTEGER, keyword_uuid TEXT);
`

// newDB is a fresh database in SQLite's default journal mode, like every
// vimango.db made before this package existed.
func newDB(t *testing.T) (db *sql.DB, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "vimango.db")
	db = open(t, path)
	if _, err := db.Exec(testSchema + "INSERT INTO task (title, note) VALUES ('first', 'one');"); err != nil {
		t.Fatal(err)
	}
	if got := journalMode(t, db); got != "delete" {
		t.Fatalf("a new database starts in %q, expected delete", got)
	}
	return db, path
}

func journalMode(t *testing.T, db *sql.DB) string {
	t.Helper()
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	return mode
}

// holdReadTransaction leaves a read transaction open on its own handle -- a
// preview render in the middle of reading -- until the returned func is called.
func holdReadTransaction(t *testing.T, path string) (release func()) {
	t.Helper()
	tx, err := open(t, path).Begin()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRow("SELECT count(*) FROM task").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return func() { tx.Rollback() }
}

func TestMigrateSwitchesToWALOnce(t *testing.T) {
	db, _ := newDB(t)

	changes, err := Migrate(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || journalMode(t, db) != "wal" {
		t.Fatalf("first Migrate: changes %q, mode %q; want two changes (wal, search index) and wal", changes, journalMode(t, db))
	}

	changes, err = Migrate(db)
	if err != nil || len(changes) != 0 {
		t.Errorf("second Migrate: changes %q, err %v; want nothing to do", changes, err)
	}
}

// vimango_hybrid keeps a connection open the whole time it runs. The switch
// must go through with it there, and that connection must keep working.
func TestWALSwitchWithAnotherConnectionOpen(t *testing.T) {
	db, path := newDB(t)
	other := open(t, path)
	var n int
	if err := other.QueryRow("SELECT count(*) FROM task").Scan(&n); err != nil {
		t.Fatal(err)
	}

	if _, err := Migrate(db); err != nil {
		t.Fatalf("Migrate with an idle connection open: %v", err)
	}
	if _, err := other.Exec("INSERT INTO task (title, note) VALUES ('second', 'from the other connection')"); err != nil {
		t.Fatalf("the connection that was already open cannot write after the switch: %v", err)
	}
	if err := db.QueryRow("SELECT count(*) FROM task").Scan(&n); err != nil || n != 2 {
		t.Errorf("rows visible after the other connection's write: %d (%v), want 2", n, err)
	}
}

// If another process is mid-transaction the switch cannot happen now. Migrate
// says so and leaves the database as it was; the next run does it.
func TestWALSwitchFailsCleanlyWhenBusy(t *testing.T) {
	db, path := newDB(t)
	release := holdReadTransaction(t, path)

	if _, err := Migrate(db); err == nil {
		t.Fatal("Migrate succeeded while another connection held a read transaction")
	}
	if got := journalMode(t, db); got != "delete" {
		t.Fatalf("after a failed switch the mode is %q, want delete", got)
	}

	release()
	if changes, err := Migrate(db); err != nil || len(changes) != 2 {
		t.Errorf("Migrate after the reader finished: changes %q, err %v", changes, err)
	}
}

// What the switch buys: a write no longer waits for a reader to finish.
func TestWriterIsNotBlockedByAReaderInWAL(t *testing.T) {
	db, path := newDB(t)

	release := holdReadTransaction(t, path)
	if _, err := db.Exec("INSERT INTO task (title) VALUES ('blocked')"); err == nil {
		t.Fatal("in rollback-journal mode a write went through with a reader open; the premise of this test is wrong")
	}
	release()

	if _, err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	release = holdReadTransaction(t, path)
	defer release()
	if _, err := db.Exec("INSERT INTO task (title) VALUES ('not blocked')"); err != nil {
		t.Errorf("in WAL mode a write still waited on a reader: %v", err)
	}
}

// Every connection in the pool enforces foreign keys, not just the first.
// "PRAGMA foreign_keys=ON" run once on the pool set it on one connection; the
// control below shows the others were left unchecked.
func TestOpenEnforcesForeignKeysOnEveryConnection(t *testing.T) {
	// a directory name the DSN must not misread: space, %, #
	dir := filepath.Join(t.TempDir(), "my notes 100% #1")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vimango.db")

	enforcing := func(db *sql.DB) (on []bool) {
		t.Helper()
		ctx := context.Background()
		// held at the same time, so the pool has to open three
		for range 3 {
			conn, err := db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			var fk bool
			if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys;").Scan(&fk); err != nil {
				t.Fatal(err)
			}
			on = append(on, fk)
		}
		return on
	}

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := enforcing(db); !slices.Equal(got, []bool{true, true, true}) {
		t.Errorf("foreign keys on the pool's connections: %v, want all true", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Open did not use the path as written: %v", err)
	}

	// and a violation is refused whichever connection it lands on
	if _, err := db.Exec(`CREATE TABLE parent (uuid TEXT UNIQUE);
		CREATE TABLE child (parent_uuid TEXT REFERENCES parent (uuid));`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := range 3 {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if _, err := conn.ExecContext(ctx, "INSERT INTO child VALUES ('nobody');"); err == nil {
			t.Errorf("connection %d accepted a row that breaks a foreign key", i)
		}
	}

	// the control: the old way reaches one connection
	old := open(t, filepath.Join(t.TempDir(), "old.db"))
	if _, err := old.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		t.Fatal(err)
	}
	if got := enforcing(old); !slices.Contains(got, false) {
		t.Errorf("the control is wrong: a PRAGMA on the pool reached every connection (%v)", got)
	}
}
