// Package store is vimango.db as the application uses it: the notes, and the
// contexts, folders and keywords that organize them.
//
// It knows nothing of the screen. Every method returns what the database
// said, as data or as an error, and it is the caller that decides what the
// user is told: the organizer puts a message on its status line, the editor
// keeps the buffer modified, `--render-html` writes to stderr and exits. That
// is the point of the package -- the editor and the renderer use the database
// without an organizer existing.
//
// Three things it leaves to others. How the file is stored (write-ahead
// logging, the search index and its triggers) is internal/localdb's. Nothing
// here writes the search index: a change to a note's title, text or keywords
// is one statement on the row, and the index follows in the same transaction.
// And sync is internal/sync's, which takes the database handle (DB) and never
// goes through this package.
package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrNotFound means the row asked about is not there: no note has that id, no
// context that title. Other errors are the database's own.
var ErrNotFound = errors.New("not found")

// Store is an open vimango.db.
type Store struct {
	db *sql.DB
}

// New wraps a database opened with localdb.Open.
func New(db *sql.DB) *Store { return &Store{db: db} }

// DB is the handle itself, for the migrations and the sync engine.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Validate reports whether the database has the tables vimango needs, which
// one made by `vimango --init` has.
func (s *Store) Validate() error {
	for _, table := range []string{"task", "context", "folder", "keyword", "sync", "task_keyword"} {
		var name string
		err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			return fmt.Errorf("required table '%s' not found in main database", table)
		}
	}
	return nil
}

// Kind is one of the four kinds of row: a note, or a container of notes. Its
// value is the table's name.
type Kind string

const (
	Task    Kind = "task"
	Context Kind = "context"
	Folder  Kind = "folder"
	Keyword Kind = "keyword"
)

// check keeps a Kind that is not one of the four out of the SQL it would be
// formatted into.
func (k Kind) check(containerOnly bool) error {
	switch k {
	case Context, Folder, Keyword:
		return nil
	case Task:
		if !containerOnly {
			return nil
		}
	}
	return fmt.Errorf("store: %q is not a kind of row this can be asked about", string(k))
}

// ---- Notes

// Note is a note's text; "" when it has none. ErrNotFound when no note has
// the id.
func (s *Store) Note(id int) (string, error) {
	var note sql.NullString
	err := s.db.QueryRow("SELECT note FROM task WHERE id=?;", id).Scan(&note)
	if err != nil {
		return "", notFound(err, "note %d", id)
	}
	return note.String, nil
}

// SetNote writes a note's text. An error means the database does not hold
// this text -- including an UPDATE that matched no row, which SQLite reports
// as success: the note was deleted (a sync, another window) while it was
// open. That one is ErrNotFound.
func (s *Store) SetNote(id int, text string) error {
	var nullable sql.NullString // an empty note is stored as NULL
	if text != "" {
		nullable = sql.NullString{String: text, Valid: true}
	}
	res, err := s.db.Exec("UPDATE task SET note=?, modified=datetime('now') WHERE id=?;", nullable, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("note %d is no longer in the database: %w", id, ErrNotFound)
	}
	return nil
}

// Title is a note's title. ErrNotFound when no note has the id.
func (s *Store) Title(id int) (string, error) {
	var title string
	err := s.db.QueryRow("SELECT title FROM task WHERE id=?;", id).Scan(&title)
	return title, notFound(err, "note %d", id)
}

// TitleAndNote is both, read together: what rendering a note needs.
func (s *Store) TitleAndNote(id int) (title, note string, err error) {
	var text sql.NullString
	err = s.db.QueryRow("SELECT title, note FROM task WHERE id=?;", id).Scan(&title, &text)
	return title, text.String, notFound(err, "note %d", id)
}

// TitleForTID is the title of the live note with a server id, for a
// [[tid:N]] link. ErrNotFound when there is none, or it has been deleted.
func (s *Store) TitleForTID(tid int64) (string, error) {
	var title string
	err := s.db.QueryRow("SELECT title FROM task WHERE tid=? AND deleted=0;", tid).Scan(&title)
	return title, notFound(err, "a note with tid %d", tid)
}

// Exists reports whether a note has the id.
func (s *Store) Exists(id int) (bool, error) {
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM task WHERE id=?);", id).Scan(&exists)
	return exists, err
}

// SetTitle renames a note.
func (s *Store) SetTitle(id int, title string) error {
	_, err := s.db.Exec("UPDATE task SET title=?, modified=datetime('now') WHERE id=?", title, id)
	return err
}

// Add makes a new note with a title and no text, in a context and a folder
// (by uuid), and returns its id.
func (s *Store) Add(title string, star bool, contextUUID, folderUUID string) (id int, err error) {
	err = s.db.QueryRow("INSERT INTO task (title, folder_uuid, context_uuid, star, added) "+
		"VALUES (?, ?, ?, ?, datetime('now')) RETURNING id;",
		title, folderUUID, contextUUID, star).Scan(&id)
	return id, err
}

// Copy duplicates a note: the title prefixed with "Copy of ", plus the
// source's text, star, context and folder. It returns the copy's id, or
// ErrNotFound when there is no such note to copy.
//
// One INSERT…SELECT rather than a read-then-write, which is what keeps
// context_uuid and folder_uuid attached to the copy, and carries a NULL note
// through as NULL.
//
// Deliberately not copied: tid (NULL is what makes the next sync insert a
// server row and assign one), folder_tid/context_tid (left at their column
// defaults, as Add does -- sync re-derives them from the uuids),
// archived/deleted, and keywords (task_keyword keys off tid, which the new row
// does not have yet). modified takes its CURRENT_TIMESTAMP default, which is
// what makes the copy a sync candidate.
func (s *Store) Copy(id int) (newID int, err error) {
	err = s.db.QueryRow(
		`INSERT INTO task (title, note, context_uuid, folder_uuid, star, added)
		 SELECT 'Copy of ' || title, note, context_uuid, folder_uuid, star, datetime('now')
		 FROM task WHERE id=?
		 RETURNING id;`,
		id).Scan(&newID)
	if err != nil {
		// no rows: the SELECT matched nothing, so nothing was inserted
		return -1, notFound(err, "note %d", id)
	}
	return newID, nil
}

// Info is a note's row, less its text.
type Info struct {
	ID       int
	TID      int // 0 until the note has synced
	Title    string
	Star     bool
	Archived bool
	Deleted  bool
	Added    string
	Modified string
}

// Info returns a note's row. ErrNotFound when no note has the id.
func (s *Store) Info(id int) (Info, error) {
	var e Info
	var tid sql.NullInt64
	err := s.db.QueryRow("SELECT id, tid, title, star, added, archived, deleted, modified FROM task WHERE id=?;", id).
		Scan(&e.ID, &tid, &e.Title, &e.Star, &e.Added, &e.Archived, &e.Deleted, &e.Modified)
	if err != nil {
		return Info{}, notFound(err, "note %d", id)
	}
	e.TID = int(tid.Int64)
	return e, nil
}

// SetStar, SetDeleted and SetArchived set a flag on a row of any kind (only
// notes are archived).
func (s *Store) SetStar(kind Kind, id int, on bool) error { return s.setFlag(kind, "star", id, on) }

func (s *Store) SetDeleted(kind Kind, id int, on bool) error {
	return s.setFlag(kind, "deleted", id, on)
}

func (s *Store) SetArchived(id int, on bool) error { return s.setFlag(Task, "archived", id, on) }

func (s *Store) setFlag(kind Kind, column string, id int, on bool) error {
	if err := kind.check(false); err != nil {
		return err
	}
	_, err := s.db.Exec(fmt.Sprintf("UPDATE %s SET %s=?, modified=datetime('now') WHERE id=?;", kind, column), on, id)
	return err
}

// ---- A note's containers

// ContextOf and FolderOf are the titles of the context and folder a note is
// in.
func (s *Store) ContextOf(id int) (string, error) {
	var title string
	err := s.db.QueryRow("SELECT context.title FROM task JOIN context on task.context_uuid = context.uuid WHERE task.id=?;", id).Scan(&title)
	return title, notFound(err, "note %d", id)
}

func (s *Store) FolderOf(id int) (string, error) {
	var title string
	err := s.db.QueryRow("SELECT folder.title FROM task JOIN folder on task.folder_uuid = folder.uuid WHERE task.id=?;", id).Scan(&title)
	return title, notFound(err, "note %d", id)
}

// SetContext and SetFolder move a note into the context or folder with a
// uuid.
func (s *Store) SetContext(id int, contextUUID string) error {
	_, err := s.db.Exec("UPDATE task SET context_uuid=?, modified=datetime('now') WHERE id=?;", contextUUID, id)
	return err
}

func (s *Store) SetFolder(id int, folderUUID string) error {
	_, err := s.db.Exec("UPDATE task SET folder_uuid=?, modified=datetime('now') WHERE id=?;", folderUUID, id)
	return err
}

// tid is a note's server id, or 0 when it has not synced (or does not exist).
func (s *Store) tid(id int) (int, error) {
	var tid sql.NullInt64
	err := s.db.QueryRow("SELECT tid FROM task WHERE id=?;", id).Scan(&tid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return int(tid.Int64), err
}

// KeywordsOf is the titles of a note's keywords. A note that has not synced
// has none: task_keyword is keyed by the server's ids.
func (s *Store) KeywordsOf(id int) ([]string, error) {
	tid, err := s.tid(id)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query("SELECT keyword.title FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.uuid=task_keyword.keyword_uuid WHERE task_keyword.task_tid=?;", tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var title sql.NullString // NULL if the keyword's own row is gone
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		titles = append(titles, title.String)
	}
	return titles, rows.Err()
}

// AddKeyword attaches a keyword (by uuid) to a note. task_keyword is keyed by
// server tids, so both the note and the keyword must have synced first: a
// NULL tid would be stored as 0 and the association silently lost on the next
// sync (SYNC_ISSUES.md #5).
func (s *Store) AddKeyword(id int, keywordUUID string) error {
	tid, err := s.tid(id)
	if err != nil {
		return err
	}
	if tid < 1 {
		return fmt.Errorf("entry %d has not been synced yet - sync before adding keywords", id)
	}

	var keywordTID sql.NullInt64
	err = s.db.QueryRow("SELECT tid FROM keyword WHERE uuid=?;", keywordUUID).Scan(&keywordTID)
	if err != nil {
		return fmt.Errorf("looking up keyword tid: %v", err)
	}
	if !keywordTID.Valid || keywordTID.Int64 < 1 {
		return fmt.Errorf("keyword has not been synced yet - sync before using it")
	}

	_, err = s.db.Exec("INSERT OR IGNORE INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES (?, ?, ?);",
		tid, keywordTID.Int64, keywordUUID)
	if err != nil {
		return fmt.Errorf("INSERT OR IGNORE INTO task_keyword: %v", err)
	}
	if _, err = s.db.Exec("UPDATE task SET modified = datetime('now') WHERE id=?;", id); err != nil {
		return fmt.Errorf("updating task modified: %v", err)
	}
	return nil
}

// RemoveKeywords detaches every keyword from a note and returns how many
// there were.
func (s *Store) RemoveKeywords(id int) (int, error) {
	tid, err := s.tid(id)
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec("DELETE FROM task_keyword WHERE task_tid=?;", tid)
	if err != nil {
		return 0, fmt.Errorf("deleting from task_keyword: %w", err)
	}
	n, _ := res.RowsAffected()
	if _, err = s.db.Exec("UPDATE task SET modified=datetime('now') WHERE id=?;", id); err != nil {
		return 0, fmt.Errorf("updating the note's modified time: %w", err)
	}
	return int(n), nil
}

// ---- Lists of notes

// Entry is a note as a list holds it.
type Entry struct {
	ID       int
	TID      int // 0 until the note has synced
	Title    string
	Star     bool
	Deleted  bool
	Archived bool
	Note     string // the text; "" when it has none
	Stamp    string // the column the list is sorted on (Listing.Sort); "" when NULL
	Marked   string // from Search: the title, each matched term between mark and unmark
}

// By is what a list of notes is chosen by.
type By int

const (
	ByContext By = iota // the notes in a context, by its title
	ByFolder            // ...in a folder
	ByKeyword           // ...with a keyword
	Recent              // every note; the name is not used
)

// Listing is how a list is cut and ordered.
type Listing struct {
	Sort         string // the timestamp to order by, newest first, and to return as Stamp: "modified" or "added"
	StarredFirst bool   // starred notes ahead of the rest
	ShowHidden   bool   // include archived and deleted notes
	Limit        int
}

func (l Listing) check() error {
	if l.Sort != "modified" && l.Sort != "added" {
		return fmt.Errorf("store: notes cannot be sorted by %q", l.Sort)
	}
	return nil
}

// Entries lists notes: those in the named context or folder or with the named
// keyword, or (Recent) all of them.
func (s *Store) Entries(by By, name string, l Listing) ([]Entry, error) {
	if err := l.check(); err != nil {
		return nil, err
	}
	stmt := fmt.Sprintf("SELECT task.id, task.tid, task.title, task.star, task.deleted, task.archived, task.note, task.%s FROM task ", l.Sort)
	var arg any = name
	switch by {
	case ByContext:
		stmt += "JOIN context ON context.uuid=task.context_uuid WHERE context.title=?"
	case ByFolder:
		stmt += "JOIN folder ON folder.uuid = task.folder_uuid WHERE folder.title=?"
	case ByKeyword:
		stmt += "JOIN task_keyword ON task.tid=task_keyword.task_tid " +
			"JOIN keyword ON keyword.uuid=task_keyword.keyword_uuid " +
			"WHERE task.tid = task_keyword.task_tid AND " +
			"task_keyword.keyword_uuid = keyword.uuid AND keyword.title=?"
	case Recent:
		stmt += "WHERE 1=?"
		arg = 1
	default:
		return nil, fmt.Errorf("store: no list of notes is chosen by %d", by)
	}
	if !l.ShowHidden {
		stmt += " AND task.archived=false AND task.deleted=false"
	}
	if l.StarredFirst {
		stmt += fmt.Sprintf(" ORDER BY task.star DESC, task.%s DESC LIMIT ?;", l.Sort)
	} else {
		stmt += fmt.Sprintf(" ORDER BY task.%s DESC LIMIT ?;", l.Sort)
	}

	rows, err := s.db.Query(stmt, arg, l.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var tid sql.NullInt64
		var note, stamp sql.NullString
		if err := rows.Scan(&e.ID, &tid, &e.Title, &e.Star, &e.Deleted, &e.Archived, &note, &stamp); err != nil {
			return nil, err
		}
		e.TID, e.Note, e.Stamp = int(tid.Int64), note.String, stamp.String
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// The search index: one row per note, rowid = task.id, columns title, note
// and tag. internal/localdb builds it and its triggers keep it current.
const searchRank = "bm25(task_fts, 2.0, 1.0, 5.0)" // a match in a tag counts most, then the title

// Search lists the notes matching a full-text query, best match first. Notes
// that have not synced yet are found too: the index is keyed by the local id.
// Each Entry's Marked is its title with the matched terms between mark and
// unmark. Listing.StarredFirst does not apply: the order is the match's rank.
//
// The query is FTS5's syntax, and one it cannot parse is an error.
func (s *Store) Search(query string, l Listing, mark, unmark string) ([]Entry, error) {
	if err := l.check(); err != nil {
		return nil, err
	}
	stmt := fmt.Sprintf("SELECT task.id, task.tid, task.title, task.star, task.deleted, task.archived, task.note, task.%s, "+
		"highlight(task_fts, 0, ?, ?) "+
		"FROM task_fts JOIN task ON task.id = task_fts.rowid WHERE task_fts MATCH ?", l.Sort)
	if !l.ShowHidden {
		stmt += " AND task.archived=false AND task.deleted=false"
	}
	stmt += " ORDER BY " + searchRank + " LIMIT ?;"

	rows, err := s.db.Query(stmt, mark, unmark, query, l.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var tid sql.NullInt64
		var note, stamp, marked sql.NullString
		if err := rows.Scan(&e.ID, &tid, &e.Title, &e.Star, &e.Deleted, &e.Archived, &note, &stamp, &marked); err != nil {
			return nil, err
		}
		e.TID, e.Note, e.Stamp, e.Marked = int(tid.Int64), note.String, stamp.String, marked.String
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// MarkedTitle is one note's title with the terms of a query marked, as Search
// returns it. ErrNotFound when the note does not match the query (it was
// edited since the search).
func (s *Store) MarkedTitle(id int, query, mark, unmark string) (string, error) {
	var title string
	err := s.db.QueryRow("SELECT highlight(task_fts, 0, ?, ?) FROM task_fts WHERE task_fts MATCH ? AND rowid=?;",
		mark, unmark, query, id).Scan(&title)
	return title, notFound(err, "a match in note %d", id)
}

// MarkedNote is one note's text with the terms of a query marked. A note that
// no longer matches the query is returned as it is.
func (s *Store) MarkedNote(id int, query, mark, unmark string) (string, error) {
	var note sql.NullString
	err := s.db.QueryRow("SELECT highlight(task_fts, 1, ?, ?) FROM task_fts WHERE rowid=? AND task_fts MATCH ?;",
		mark, unmark, id, query).Scan(&note)
	if errors.Is(err, sql.ErrNoRows) {
		return s.Note(id)
	}
	return note.String, err
}

// ---- Containers: contexts, folders and keywords

// Container is a context, a folder or a keyword.
type Container struct {
	ID       int
	TID      int // 0 until it has synced
	Title    string
	Star     bool
	Deleted  bool
	Modified string
	Count    int // how many notes are in it; set by Container, not by Containers
}

// Containers lists every container of a kind, by title.
func (s *Store) Containers(kind Kind) ([]Container, error) {
	if err := kind.check(true); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(fmt.Sprintf("SELECT id, tid, title, star, deleted, modified FROM %s ORDER BY title COLLATE NOCASE ASC;", kind))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cc []Container
	for rows.Next() {
		var c Container
		var tid sql.NullInt64
		var modified sql.NullString
		if err := rows.Scan(&c.ID, &tid, &c.Title, &c.Star, &c.Deleted, &modified); err != nil {
			return nil, err
		}
		c.TID, c.Modified = int(tid.Int64), modified.String
		cc = append(cc, c)
	}
	return cc, rows.Err()
}

// ContainerTitles is the titles of every container of a kind, in no order.
func (s *Store) ContainerTitles(kind Kind) ([]string, error) {
	if err := kind.check(true); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(fmt.Sprintf("SELECT title FROM %s;", kind))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		titles = append(titles, title)
	}
	return titles, rows.Err()
}

// Container is one container and the number of notes in it. ErrNotFound when
// none of that kind has the id.
func (s *Store) Container(kind Kind, id int) (Container, error) {
	var count string
	switch kind {
	case Context:
		count = "SELECT COUNT(*) FROM task JOIN context ON context.uuid = task.context_uuid WHERE context.id=?;"
	case Folder:
		count = "SELECT COUNT(*) FROM task JOIN folder ON folder.uuid = task.folder_uuid WHERE folder.id=?;"
	case Keyword:
		count = "SELECT COUNT(*) FROM task_keyword WHERE keyword_uuid=(SELECT uuid FROM keyword WHERE id=?);"
	default:
		return Container{}, kind.check(true)
	}

	var c Container
	var tid sql.NullInt64
	var modified sql.NullString
	err := s.db.QueryRow(fmt.Sprintf("SELECT id, tid, title, star, deleted, modified FROM %s WHERE id=?;", kind), id).
		Scan(&c.ID, &tid, &c.Title, &c.Star, &c.Deleted, &modified)
	if err != nil {
		return Container{}, notFound(err, "%s %d", kind, id)
	}
	c.TID, c.Modified = int(tid.Int64), modified.String
	if err := s.db.QueryRow(count, id).Scan(&c.Count); err != nil {
		return Container{}, err
	}
	return c, nil
}

// ContainerUUID is the uuid of the container of a kind with a title: what a
// note is pointed at to put it there. ErrNotFound when there is none.
func (s *Store) ContainerUUID(kind Kind, title string) (string, error) {
	if err := kind.check(true); err != nil {
		return "", err
	}
	var id string
	err := s.db.QueryRow(fmt.Sprintf("SELECT uuid FROM %s WHERE title=?;", kind), title).Scan(&id)
	return id, notFound(err, "a %s titled %q", kind, title)
}

// AddContainer makes a new container, with a uuid of its own (the server
// gives it a tid when it syncs), and returns its id.
func (s *Store) AddContainer(kind Kind, title string, star bool) (id int, err error) {
	if err := kind.check(true); err != nil {
		return 0, err
	}
	stmt := fmt.Sprintf("INSERT INTO %s (title, uuid, star, deleted, modified) "+
		"VALUES (?, ?, ?, False, datetime('now')) RETURNING id;", kind)
	err = s.db.QueryRow(stmt, title, uuid.New().String(), star).Scan(&id)
	return id, err
}

// RenameContainer changes a container's title.
func (s *Store) RenameContainer(kind Kind, id int, title string) error {
	if err := kind.check(true); err != nil {
		return err
	}
	_, err := s.db.Exec(fmt.Sprintf("UPDATE %s SET title=?, modified=datetime('now') WHERE id=?", kind), title, id)
	return err
}

// notFound turns the database's "no rows" into ErrNotFound, saying what was
// not found, and leaves any other error (and nil) as it is.
func notFound(err error, format string, args ...any) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), ErrNotFound)
	}
	return err
}
