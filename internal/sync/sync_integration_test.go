package sync

// Integration tests: two or more simulated clients (each its own SQLite
// pair in a temp dir) syncing through one throwaway Postgres database.
//
// They need a Postgres the tests may create databases on, e.g.
//
//	VIMANGO_TEST_PG='host=127.0.0.1 port=54329 user=postgres sslmode=disable' \
//	  CGO_ENABLED=1 go test --tags=fts5 ./internal/sync/
//
// and are skipped when VIMANGO_TEST_PG is unset. Every test gets a fresh
// database built from cmd/create_dbs/postgres_init4.sql.

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// clientSchema mirrors sqliteSchema in ~/vimango/init.go, minus
// server_modified, so every test also exercises ensureSyncColumns.
const clientSchema = `
CREATE TABLE context (id INTEGER PRIMARY KEY, tid INTEGER UNIQUE, uuid TEXT NOT NULL UNIQUE, title TEXT NOT NULL UNIQUE,
	star BOOLEAN DEFAULT FALSE, deleted BOOLEAN DEFAULT FALSE, modified TEXT DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE folder (id INTEGER PRIMARY KEY, tid INTEGER UNIQUE, uuid TEXT NOT NULL UNIQUE, title TEXT NOT NULL UNIQUE,
	star BOOLEAN DEFAULT FALSE, deleted BOOLEAN DEFAULT FALSE, modified TEXT DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE keyword (id INTEGER PRIMARY KEY, tid INTEGER UNIQUE, uuid TEXT NOT NULL UNIQUE, title TEXT NOT NULL UNIQUE,
	star BOOLEAN DEFAULT FALSE, deleted BOOLEAN DEFAULT FALSE, modified TEXT DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE task (
	id INTEGER PRIMARY KEY, tid INTEGER UNIQUE, star BOOLEAN DEFAULT FALSE, title TEXT NOT NULL,
	folder_tid INTEGER %[1]s, context_tid INTEGER %[1]s,
	folder_uuid TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000002',
	context_uuid TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001',
	note TEXT, archived BOOLEAN DEFAULT FALSE, deleted BOOLEAN DEFAULT FALSE, added TEXT NOT NULL,
	modified TEXT DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(folder_uuid) REFERENCES folder (uuid), FOREIGN KEY(context_uuid) REFERENCES context (uuid));
CREATE TABLE sync (id INTEGER PRIMARY KEY, machine TEXT NOT NULL UNIQUE, timestamp TEXT DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE task_keyword (task_tid INTEGER NOT NULL, keyword_tid INTEGER, keyword_uuid TEXT NOT NULL,
	PRIMARY KEY (task_tid, keyword_uuid), FOREIGN KEY(task_tid) REFERENCES task (tid),
	FOREIGN KEY(keyword_uuid) REFERENCES keyword (uuid));
INSERT INTO context (title, tid, uuid) VALUES ('none', 1, '00000000-0000-0000-0000-000000000001');
INSERT INTO folder (title, tid, uuid) VALUES ('none', 1, '00000000-0000-0000-0000-000000000002');
INSERT INTO sync (machine, timestamp) VALUES ('server', '1970-01-01 00:00:00'), ('client', '1970-01-01 00:00:00');
`

type testClient struct {
	t    *testing.T
	name string
	s    *Syncer
}

// newServer creates a fresh Postgres database and returns a handle to it.
func newServer(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("VIMANGO_TEST_PG")
	if dsn == "" {
		t.Skip("VIMANGO_TEST_PG not set")
	}
	admin, err := sql.Open("postgres", dsn+" dbname=postgres")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("vimango_test_%d", rand.Int63())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("creating test database: %v", err)
	}
	t.Cleanup(func() {
		admin, err := sql.Open("postgres", dsn+" dbname=postgres")
		if err == nil {
			admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
			admin.Close()
		}
	})

	pg, err := sql.Open("postgres", dsn+" dbname="+name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Close() })
	schema, err := os.ReadFile(filepath.Join("..", "..", "cmd", "create_dbs", "postgres_init4.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Exec(string(schema)); err != nil {
		t.Fatalf("loading postgres schema: %v", err)
	}
	return pg
}

// newClient creates a client database pair. tidDefault is the DEFAULT
// clause for task.context_tid/folder_tid ("" reproduces issue 7).
func newClient(t *testing.T, name string, pg *sql.DB, tidDefault string) *testClient {
	t.Helper()
	dir := t.TempDir()
	mainDB, err := sql.Open("sqlite3", filepath.Join(dir, "vimango.db"))
	if err != nil {
		t.Fatal(err)
	}
	mainDB.SetMaxOpenConns(1)
	t.Cleanup(func() { mainDB.Close() })
	if _, err := mainDB.Exec(fmt.Sprintf(clientSchema, tidDefault)); err != nil {
		t.Fatalf("client schema: %v", err)
	}
	if _, err := mainDB.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		t.Fatal(err)
	}
	ftsDB, err := sql.Open("sqlite3", filepath.Join(dir, "fts5_vimango.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ftsDB.Close() })
	if _, err := ftsDB.Exec("CREATE VIRTUAL TABLE fts USING fts5 (title, note, tag, tid UNINDEXED);"); err != nil {
		t.Fatal(err)
	}
	return &testClient{t: t, name: name, s: New(mainDB, ftsDB, pg, "test", "test")}
}

func (c *testClient) exec(query string, args ...interface{}) sql.Result {
	c.t.Helper()
	res, err := c.s.MainDB.Exec(query, args...)
	if err != nil {
		c.t.Fatalf("%s: %s: %v", c.name, query, err)
	}
	return res
}

func (c *testClient) sync() string {
	c.t.Helper()
	log, err := c.s.Synchronize(false)
	if err != nil {
		c.t.Fatalf("%s sync failed: %v\n%s", c.name, err, log)
	}
	return log
}

func (c *testClient) addNote(title, note string) int {
	c.t.Helper()
	res := c.exec("INSERT INTO task (title, note, added) VALUES (?, ?, datetime('now'));", title, note)
	id, _ := res.LastInsertId()
	return int(id)
}

func (c *testClient) edit(id int, note string) {
	c.t.Helper()
	c.exec("UPDATE task SET note=?, modified=datetime('now') WHERE id=?;", note, id)
}

func (c *testClient) remove(id int) {
	c.t.Helper()
	c.exec("UPDATE task SET deleted=true, modified=datetime('now') WHERE id=?;", id)
}

func (c *testClient) tidOf(id int) int {
	c.t.Helper()
	var tid sql.NullInt64
	if err := c.s.MainDB.QueryRow("SELECT tid FROM task WHERE id=?;", id).Scan(&tid); err != nil {
		c.t.Fatalf("%s: tid of id %d: %v", c.name, id, err)
	}
	return int(tid.Int64)
}

func (c *testClient) idOf(tid int) int {
	c.t.Helper()
	var id int
	if err := c.s.MainDB.QueryRow("SELECT id FROM task WHERE tid=?;", tid).Scan(&id); err != nil {
		c.t.Fatalf("%s: id of tid %d: %v", c.name, tid, err)
	}
	return id
}

// notes returns title -> note for every live local task.
func (c *testClient) notes() map[string]string {
	c.t.Helper()
	rows, err := c.s.MainDB.Query("SELECT title, COALESCE(note, '') FROM task WHERE deleted=false;")
	if err != nil {
		c.t.Fatal(err)
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var title, note string
		rows.Scan(&title, &note)
		m[title] = note
	}
	return m
}

func (c *testClient) count(db *sql.DB, query string, args ...interface{}) int {
	c.t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		c.t.Fatalf("%s: %s: %v", c.name, query, err)
	}
	return n
}

func (c *testClient) probe() int {
	c.t.Helper()
	n, err := c.s.Probe()
	if err != nil {
		c.t.Fatalf("%s probe: %v", c.name, err)
	}
	return n
}

func serverNote(t *testing.T, pg *sql.DB, tid int) (note string, deleted bool, modified string) {
	t.Helper()
	if err := pg.QueryRow("SELECT COALESCE(note, ''), deleted, modified::text FROM task WHERE tid=$1;", tid).
		Scan(&note, &deleted, &modified); err != nil {
		t.Fatalf("server tid %d: %v", tid, err)
	}
	return
}

func conflictTitle(title string) string {
	return fmt.Sprintf("%s (conflict %s)", title, time.Now().Format("2006-01-02"))
}

// The basic round trip, and no ping-pong: a sync with nothing new to do
// must not push anything, or every client would bounce rows forever.
func TestRoundTripWithoutChurn(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

	id := a.addNote("shopping", "milk")
	a.sync()
	tid := a.tidOf(id)
	if tid < 1 {
		t.Fatal("A's note got no tid")
	}

	if n := b.probe(); n == 0 {
		t.Error("B's probe saw nothing to pull")
	}
	b.sync()
	if got := b.notes()["shopping"]; got != "milk" {
		t.Fatalf("B has %q, want milk", got)
	}
	if n := b.probe(); n != 0 {
		t.Errorf("B's probe after syncing = %d, want 0", n)
	}

	b.edit(b.idOf(tid), "milk, eggs")
	b.sync()
	a.sync()
	if got := a.notes()["shopping"]; got != "milk, eggs" {
		t.Fatalf("A has %q after B's edit", got)
	}

	_, _, before := serverNote(t, pg, tid)
	for i := 0; i < 2; i++ {
		for _, c := range []*testClient{a, b} {
			// hybrid shows this count; our own pushes coming back through
			// the watermark overlap must not be in it.
			if log := c.sync(); !strings.Contains(log, "is: **0**") {
				t.Errorf("%s: idle sync reported changes:\n%s", c.name, log)
			}
		}
	}
	if _, _, after := serverNote(t, pg, tid); after != before {
		t.Errorf("idle syncs re-pushed the note: server modified %s -> %s", before, after)
	}
	if n := a.probe(); n != 0 {
		t.Errorf("A's probe after idle syncs = %d, want 0 (own pushes counted as changes?)", n)
	}
}

// Issue 3: the losing edit is kept, not discarded.
func TestConflictKeepsLosingEdit(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

	id := a.addNote("plan", "v1")
	a.sync()
	tid := a.tidOf(id)
	b.sync()

	a.edit(id, "A's edit")
	a.sync()
	b.edit(b.idOf(tid), "B's edit")
	log := b.sync()

	// hybrid opens the sync log on this marker, so a conflict copy is seen.
	if !strings.Contains(log, "Server won:") {
		t.Errorf("conflict not reported with the marker hybrid looks for:\n%s", log)
	}
	notes := b.notes()
	if notes["plan"] != "A's edit" {
		t.Errorf("B's plan = %q, want the server's (A's) edit\n%s", notes["plan"], log)
	}
	if notes[conflictTitle("plan")] != "B's edit" {
		t.Errorf("B lost its edit; notes = %v\n%s", notes, log)
	}

	a.sync()
	if got := a.notes()[conflictTitle("plan")]; got != "B's edit" {
		t.Errorf("the conflict copy did not reach A; A has %v", a.notes())
	}
}

// Both sides making the same change is not a conflict.
func TestIdenticalEditsAreNotAConflict(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

	id := a.addNote("same", "v1")
	a.sync()
	b.sync()
	a.edit(id, "v2")
	b.edit(b.idOf(a.tidOf(id)), "v2")
	a.sync()
	b.sync()
	if len(b.notes()) != 1 {
		t.Errorf("identical edits produced a conflict copy: %v", b.notes())
	}
}

// Issue 1: a server row committed while another client's sync was running
// carries a modified earlier than that sync's end; it must still be pulled.
func TestServerWriteDuringSyncIsPulled(t *testing.T) {
	pg := newServer(t)
	a := newClient(t, "A", pg, "DEFAULT 1")
	a.sync()

	// What a write landing mid-sync looks like afterwards: stamped before
	// the sync finished.
	if _, err := pg.Exec("INSERT INTO task (title, note, added, modified) VALUES ('late', 'x', now(), now() - interval '1 minute');"); err != nil {
		t.Fatal(err)
	}
	a.sync()
	if _, ok := a.notes()["late"]; !ok {
		t.Error("a server row stamped inside the last sync's window was never pulled")
	}
}

// Missed issue A: a local write landing while a sync runs is stamped before
// that sync's end; it must still be pushed.
func TestLocalWriteDuringSyncIsPushed(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")
	a.sync()

	a.exec("INSERT INTO task (title, note, added, modified) VALUES ('mid-sync', 'x', datetime('now'), datetime('now', '-1 second'));")
	a.sync()
	b.sync()
	if _, ok := b.notes()["mid-sync"]; !ok {
		t.Error("a local row stamped inside the last sync's window was never pushed")
	}
}

// Issue 2, push side: a row that fails to push is retried.
func TestFailedPushIsRetried(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")
	if _, err := pg.Exec("ALTER TABLE task ADD CONSTRAINT no_boom CHECK (title <> 'boom');"); err != nil {
		t.Fatal(err)
	}
	a.addNote("boom", "x")
	a.addNote("fine", "y")
	log := a.sync()
	if !strings.Contains(log, "failed to push (retried next sync): 1") {
		t.Errorf("push failure not reported:\n%s", log)
	}

	if _, err := pg.Exec("ALTER TABLE task DROP CONSTRAINT no_boom;"); err != nil {
		t.Fatal(err)
	}
	a.sync()
	b.sync()
	if _, ok := b.notes()["boom"]; !ok {
		t.Errorf("the failed row was never retried; B has %v", b.notes())
	}
}

// Issue 2, pull side: a row that fails to apply is pulled again.
func TestFailedPullIsRetried(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")
	b.exec("CREATE TRIGGER boom BEFORE INSERT ON task WHEN NEW.title='boom' BEGIN SELECT RAISE(ABORT, 'boom'); END;")

	a.addNote("boom", "x")
	a.sync()
	log := b.sync()
	if !strings.Contains(log, "not advanced") {
		t.Errorf("server watermark advanced past a failed pull:\n%s", log)
	}

	b.exec("DROP TRIGGER boom;")
	b.sync()
	if _, ok := b.notes()["boom"]; !ok {
		t.Error("the failed pull was never retried")
	}
}

// Issue 4: a keyword deleted on one client disappears from the others.
func TestKeywordDeletePropagates(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

	a.exec("INSERT INTO keyword (title, uuid) VALUES ('urgent', 'kw-urgent');")
	a.sync()
	b.sync()
	if b.count(b.s.MainDB, "SELECT COUNT(*) FROM keyword WHERE title='urgent';") != 1 {
		t.Fatal("keyword never reached B")
	}

	a.exec("UPDATE keyword SET deleted=true, modified=datetime('now') WHERE title='urgent';")
	a.sync()
	b.sync()
	if b.count(b.s.MainDB, "SELECT COUNT(*) FROM keyword WHERE title='urgent';") != 0 {
		t.Error("keyword delete never reached B")
	}
}

// Issue 6: deletes remove the FTS row on both the deleting and the
// receiving client.
func TestDeleteRemovesFTSRow(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

	id := a.addNote("gone", "x")
	a.sync()
	tid := a.tidOf(id)
	b.sync()
	if b.count(b.s.FtsDB, "SELECT COUNT(*) FROM fts WHERE tid=?;", tid) != 1 {
		t.Fatal("B has no FTS row to begin with")
	}

	a.remove(id)
	a.sync()
	b.sync()
	for _, c := range []*testClient{a, b} {
		if n := c.count(c.s.FtsDB, "SELECT COUNT(*) FROM fts WHERE tid=?;", tid); n != 0 {
			t.Errorf("%s still has %d FTS rows for deleted tid %d", c.name, n, tid)
		}
	}
}

// Issue 7: a database with no DEFAULT on the *_tid columns can push.
func TestPushFromInitSchemaWithoutTidDefaults(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, ""), newClient(t, "B", pg, "DEFAULT 1")

	a.addNote("fresh install", "x")
	log := a.sync()
	b.sync()
	if _, ok := b.notes()["fresh install"]; !ok {
		t.Errorf("note with NULL context_tid/folder_tid never reached the server\n%s", log)
	}
	var ctx string
	pg.QueryRow("SELECT context_uuid FROM task WHERE title='fresh install';").Scan(&ctx)
	if ctx != DefaultContextUUID {
		t.Errorf("server context_uuid = %q, want the 'none' uuid", ctx)
	}
}

// Missed issue D: an edit to a note deleted elsewhere is kept.
func TestEditOfNoteDeletedElsewhereIsKept(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

	id := a.addNote("doomed", "v1")
	a.sync()
	tid := a.tidOf(id)
	b.sync()

	b.remove(b.idOf(tid))
	b.sync()
	a.edit(id, "A's edit")
	a.sync()

	notes := a.notes()
	if _, ok := notes["doomed"]; ok {
		t.Error("the deleted note is still live on A")
	}
	if notes[conflictTitle("doomed")] != "A's edit" {
		t.Errorf("A's edit to a note B deleted was lost; A has %v", notes)
	}
}

// A local delete loses to a server edit: nothing is lost, the note returns.
func TestDeleteOfNoteEditedElsewhereRestoresIt(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

	id := a.addNote("keep", "v1")
	a.sync()
	tid := a.tidOf(id)
	b.sync()

	a.edit(id, "A's edit")
	a.sync()
	b.remove(b.idOf(tid))
	b.sync()

	if got := b.notes()["keep"]; got != "A's edit" {
		t.Errorf("B's notes = %v, want keep restored with A's edit", b.notes())
	}
	if _, deleted, _ := serverNote(t, pg, tid); deleted {
		t.Error("server row was deleted over A's newer edit")
	}
}

// Data finding: a task pointing at a context that doesn't exist is filed
// under "none" instead of failing every run.
func TestMissingContainerFallsBackToNone(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

	a.exec("PRAGMA foreign_keys=OFF;")
	a.exec("INSERT INTO task (title, added, context_uuid) VALUES ('orphan', datetime('now'), 'no-such-context');")
	a.exec("PRAGMA foreign_keys=ON;")
	if _, err := pg.Exec("INSERT INTO task (title, added, context_uuid) VALUES ('server orphan', now(), 'also-missing');"); err != nil {
		t.Fatal(err)
	}
	a.sync()
	b.sync()

	notes := b.notes()
	for _, title := range []string{"orphan", "server orphan"} {
		if _, ok := notes[title]; !ok {
			t.Errorf("%q never reached B; B has %v", title, notes)
		}
	}
	var ctx string
	pg.QueryRow("SELECT context_uuid FROM task WHERE title='orphan';").Scan(&ctx)
	if ctx != DefaultContextUUID {
		t.Errorf("server context_uuid for the local orphan = %q, want 'none'", ctx)
	}
}

// hideServerChanges moves a client's server watermark to now, so its next
// pull misses everything already on the server and a clash surfaces on the
// push side instead — what a create racing another client's sync sees.
func (c *testClient) hideServerChanges() {
	c.t.Helper()
	var now string
	if err := c.s.PG.QueryRow("SELECT localtimestamp::text;").Scan(&now); err != nil {
		c.t.Fatal(err)
	}
	c.exec("UPDATE sync SET timestamp=? WHERE machine='server';", now)
}

func (c *testClient) contextUUIDOf(title string) string {
	c.t.Helper()
	var uuid string
	if err := c.s.MainDB.QueryRow("SELECT context_uuid FROM task WHERE title=?;", title).Scan(&uuid); err != nil {
		c.t.Fatalf("%s: context of %q: %v", c.name, title, err)
	}
	return uuid
}

func serverContexts(t *testing.T, pg *sql.DB) map[string]string {
	t.Helper()
	rows, err := pg.Query("SELECT title, uuid FROM context WHERE deleted=false;")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var title, uuid string
		rows.Scan(&title, &uuid)
		m[title] = uuid
	}
	return m
}

// Issue 8: the same title created on two clients is one container. Both
// the pull side (B sees A's container first) and the push side (B's insert
// collides) must merge, and B's notes must follow.
func TestSameTitleCreatedOnTwoClientsMerges(t *testing.T) {
	for _, side := range []string{"pull", "push"} {
		t.Run(side, func(t *testing.T) {
			pg := newServer(t)
			a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")
			b.sync()

			a.exec("INSERT INTO context (title, uuid) VALUES ('work', 'work-from-A');")
			a.exec("INSERT INTO task (title, added, context_uuid) VALUES ('A note', datetime('now'), 'work-from-A');")
			a.sync()

			if side == "push" {
				b.hideServerChanges()
			}
			b.exec("INSERT INTO context (title, uuid) VALUES ('work', 'work-from-B');")
			b.exec("INSERT INTO task (title, added, context_uuid) VALUES ('B note', datetime('now'), 'work-from-B');")
			log := b.sync()
			if strings.Contains(log, "**Error**") || strings.Contains(log, "not advanced") {
				t.Fatalf("B's sync failed on the clash:\n%s", log)
			}

			if got := b.contextUUIDOf("B note"); got != "work-from-A" {
				t.Errorf("B's note is in context %q, want A's 'work'\n%s", got, log)
			}
			if n := b.count(b.s.MainDB, "SELECT COUNT(*) FROM context WHERE title='work';"); n != 1 {
				t.Errorf("B has %d 'work' contexts", n)
			}
			if got := serverContexts(t, pg)["work"]; got != "work-from-A" {
				t.Errorf("server 'work' uuid = %q", got)
			}

			a.sync()
			if got := a.contextUUIDOf("B note"); got != "work-from-A" {
				t.Errorf("on A, B's note is in context %q", got)
			}
			if log := b.sync(); strings.Contains(log, "**Error**") {
				t.Errorf("the merge left something failing:\n%s", log)
			}
		})
	}
}

// Keywords merge the same way, carrying their task_keyword rows along.
func TestSameKeywordCreatedOnTwoClientsMerges(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

	id := b.addNote("tagged", "x")
	b.sync()
	tid := b.tidOf(id)
	a.sync()

	a.exec("INSERT INTO keyword (title, uuid) VALUES ('urgent', 'kw-A');")
	a.sync()
	b.hideServerChanges()
	b.exec("INSERT INTO keyword (title, uuid, tid) VALUES ('urgent', 'kw-B', NULL);")
	// What addTaskKeywordByUUID would have refused; written directly to
	// exercise moving an existing association.
	b.exec("INSERT INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES (?, NULL, 'kw-B');", tid)
	b.exec("UPDATE task SET modified=datetime('now') WHERE id=?;", id)
	if log := b.sync(); strings.Contains(log, "**Error**") {
		t.Fatalf("B's sync failed on the keyword clash:\n%s", log)
	}
	b.sync()

	var n int
	pg.QueryRow("SELECT COUNT(*) FROM task_keyword tk JOIN keyword k ON k.tid=tk.keyword_tid WHERE tk.task_tid=$1 AND k.title='urgent';", tid).Scan(&n)
	if n != 1 {
		t.Errorf("the keyword association did not reach the server under the merged keyword (rows: %d)", n)
	}
	if n := b.count(b.s.MainDB, "SELECT COUNT(*) FROM task_keyword WHERE keyword_uuid='kw-B';"); n != 0 {
		t.Errorf("B still has %d task_keyword rows on the merged-away keyword", n)
	}
}

// Issue 8, tombstones: a deleted container's title can be used again,
// whether by creating a container or by renaming one.
func TestDeletedTitleCanBeReused(t *testing.T) {
	pg := newServer(t)
	a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

	a.exec("INSERT INTO context (title, uuid) VALUES ('cpp', 'cpp-1'), ('rust', 'rust-1'), ('home', 'home-1');")
	a.sync()
	a.exec("UPDATE context SET deleted=true, modified=datetime('now') WHERE title IN ('cpp', 'rust');")
	a.sync()

	a.exec("INSERT INTO context (title, uuid) VALUES ('cpp', 'cpp-2');")
	a.exec("INSERT INTO task (title, added, context_uuid) VALUES ('new cpp note', datetime('now'), 'cpp-2');")
	a.exec("UPDATE context SET title='rust', modified=datetime('now') WHERE title='home';")
	if log := a.sync(); strings.Contains(log, "**Error**") {
		t.Fatalf("reusing deleted titles failed:\n%s", log)
	}

	ctx := serverContexts(t, pg)
	if ctx["cpp"] != "cpp-2" {
		t.Errorf("server 'cpp' = %q, want the new container", ctx["cpp"])
	}
	if ctx["rust"] != "home-1" {
		t.Errorf("server 'rust' = %q, want the renamed 'home'", ctx["rust"])
	}
	b.sync()
	if got := b.contextUUIDOf("new cpp note"); got != "cpp-2" {
		t.Errorf("on B the note is in %q", got)
	}
}

// Issue 8, renames: renaming onto a title another live container has is
// undone, on whichever side the clash shows up.
func TestRenameOntoExistingTitleIsRejected(t *testing.T) {
	for _, side := range []string{"pull", "push"} {
		t.Run(side, func(t *testing.T) {
			pg := newServer(t)
			a, b := newClient(t, "A", pg, "DEFAULT 1"), newClient(t, "B", pg, "DEFAULT 1")

			a.exec("INSERT INTO context (title, uuid) VALUES ('home', 'home-1');")
			a.sync()
			b.sync()
			a.exec("INSERT INTO context (title, uuid) VALUES ('work', 'work-1');")
			a.sync()

			if side == "push" {
				b.hideServerChanges()
			}
			b.exec("UPDATE context SET title='work', modified=datetime('now') WHERE uuid='home-1';")
			log := b.sync()
			if !strings.Contains(log, "Server won:") {
				t.Errorf("the rejected rename was not reported with the marker hybrid looks for:\n%s", log)
			}
			if strings.Contains(log, "**Error**") || strings.Contains(log, "not advanced") {
				t.Errorf("the clash failed the sync:\n%s", log)
			}

			var title string
			b.s.MainDB.QueryRow("SELECT title FROM context WHERE uuid='home-1';").Scan(&title)
			if title != "home" {
				t.Errorf("B's renamed context is %q, want it back to 'home'", title)
			}
			ctx := serverContexts(t, pg)
			if ctx["home"] != "home-1" || ctx["work"] != "work-1" {
				t.Errorf("server contexts changed: %v", ctx)
			}
		})
	}
}
