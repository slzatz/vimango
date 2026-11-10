package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	//"github.com/lib/pq"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// Database is the local SQLite database (vimango.db) and, when sync is
// configured, the PostgreSQL server.
//
// The full-text index is the task_fts table inside vimango.db, kept current
// by triggers (internal/localdb): nothing here writes it. A change to a
// note's title, text or keywords is one statement on the row, and the index
// changes in the same transaction.
type Database struct {
	MainDB *sql.DB
	PG     *sql.DB
}

func (db *Database) entryTidFromId(id int) int {
	var tid int
	_ = db.MainDB.QueryRow("SELECT tid FROM task WHERE id=?;", id).Scan(&tid)
	return tid
}

func (db *Database) keywordExists(title string) (string, bool) {
	var uuid string
	err := db.MainDB.QueryRow("SELECT uuid FROM keyword WHERE title=?;", title).Scan(&uuid)
	if err != nil {
		return "", false
	}
	return uuid, true
}

func (db *Database) contextExists(title string) (string, bool) {
	var uuid string
	err := db.MainDB.QueryRow("SELECT uuid FROM context WHERE title=?;", title).Scan(&uuid)
	if err != nil {
		return "", false
	}
	return uuid, true
}

func (db *Database) folderExists(title string) (string, bool) {
	var uuid string
	err := db.MainDB.QueryRow("SELECT uuid FROM folder WHERE title=?;", title).Scan(&uuid)
	if err != nil {
		return "", false
	}
	return uuid, true
}

func (db *Database) contextList() map[string]struct{} {
	rows, _ := db.MainDB.Query("SELECT title FROM context;")
	defer rows.Close()

	contexts := make(map[string]struct{})
	for rows.Next() {
		var title string
		_ = rows.Scan(&title)
		contexts[title] = struct{}{}
	}
	return contexts
}

func (db *Database) folderList() map[string]struct{} {
	rows, _ := db.MainDB.Query("SELECT title FROM folder;")
	defer rows.Close()

	folders := make(map[string]struct{})
	for rows.Next() {
		var title string
		_ = rows.Scan(&title)
		folders[title] = struct{}{}
	}
	return folders
}

func (db *Database) toggleStar(id int, state bool, table string) error {
	s := fmt.Sprintf("UPDATE %s SET star=?, modified=datetime('now') WHERE id=?;",
		table)
	_, err := db.MainDB.Exec(s, !state, id)
	return err
}

func (db *Database) toggleDeleted(id int, state bool, table string) error {
	s := fmt.Sprintf("UPDATE %s SET deleted=?, modified=datetime('now') WHERE id=?;", table)
	_, err := db.MainDB.Exec(s, !state, id)
	return err
}

func (db *Database) toggleArchived(id int, state bool, table string) error {
	s := fmt.Sprintf("UPDATE %s SET archived=?, modified=datetime('now') WHERE id=?;", table)
	_, err := db.MainDB.Exec(s, !state, id)
	return err
}

func (db *Database) updateTaskContextByUUID(uuid string, id int) error {
	_, err := db.MainDB.Exec("UPDATE task SET context_uuid=?, modified=datetime('now') WHERE id=?;",
		uuid, id)
	return err
}

func (db *Database) updateTaskFolderByUUID(uuid string, id int) error {
	_, err := db.MainDB.Exec("UPDATE task SET folder_uuid=?, modified=datetime('now') WHERE id=?;",
		uuid, id)
	return err
}

// updateNote writes a note's text. An error means the database does not hold
// this text -- including an UPDATE that matched no row, which SQLite reports
// as success: the note was deleted (a sync, another window) while it was open.
func (db *Database) updateNote(id int, text string) error {

	var nullableText sql.NullString
	if len(text) != 0 {
		nullableText.String = text
		nullableText.Valid = true
	}

	res, err := db.MainDB.Exec("UPDATE task SET note=?, modified=datetime('now') WHERE id=?;",
		nullableText, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("note %d is no longer in the database", id)
	}
	return nil
}

func (db *Database) filterEntries(taskView int, filter interface{}, showDeleted bool, sort string, sortPriority bool, max int) ([]Row, error) {

	s := fmt.Sprintf("SELECT task.id, task.title, task.star, task.deleted, task.archived, task.note, task.%s FROM task ", sort)

	switch taskView {
	case BY_CONTEXT:
		s += "JOIN context ON context.uuid=task.context_uuid WHERE context.title=?"
	case BY_FOLDER:
		s += "JOIN folder ON folder.uuid = task.folder_uuid WHERE folder.title=?"
	case BY_KEYWORD:
		s += "JOIN task_keyword ON task.tid=task_keyword.task_tid " +
			"JOIN keyword ON keyword.uuid=task_keyword.keyword_uuid " +
			"WHERE task.tid = task_keyword.task_tid AND " +
			"task_keyword.keyword_uuid = keyword.uuid AND keyword.title=?"
	case BY_RECENT:
		//s += "WHERE 1=1"
		s += "WHERE 1=?"
		filter = 1
	default:
		app.Organizer.ShowMessage(BL, "You asked for an unsupported db query")
		return []Row{}, nil //FIXME
	}

	if !showDeleted {
		s += " AND task.archived=false AND task.deleted=false"
	}
	if sortPriority {
		s += fmt.Sprintf(" ORDER BY task.star DESC, task.%s DESC LIMIT %d;", sort, max)
	} else {
		s += fmt.Sprintf(" ORDER BY task.%s DESC LIMIT %d;", sort, max) //01162022
	}
	rows, err := db.MainDB.Query(s, filter)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error in getItems: %v", err)
		return []Row{}, err
	}
	defer rows.Close()

	var orgRows []Row
	for rows.Next() {
		var row Row
		var sort sql.NullString
		var note sql.NullString

		err = rows.Scan(
			&row.id,
			&row.title,
			&row.star,
			&row.deleted,
			&row.archived,
			&note,
			&sort,
		)

		if err != nil {
			app.Organizer.ShowMessage(BL, "Error in filterEntries: %v", err)
			//return orgRows
			return orgRows, err
		}

		if note.Valid && note.String != "" {
			row.hasImage = containsGoogleDriveImage(note.String)
		}

		if sort.Valid {
			row.sort = timeDelta(sort.String)
		} else {
			row.sort = ""
		}

		orgRows = append(orgRows, row)

	}
	return orgRows, nil
}

func (db *Database) updateTitle(row *Row) error {
	_, err := db.MainDB.Exec("UPDATE task SET title=?, modified=datetime('now') WHERE id=?", row.title, row.id)
	return err
}

func (db *Database) insertTitle(row *Row, context_uuid, folder_uuid string) error { // should return err

	var id int
	err := db.MainDB.QueryRow("INSERT INTO task (title, folder_uuid, context_uuid, star, added) "+
		"VALUES (?, ?, ?, ?, datetime('now')) RETURNING id;",
		row.title, folder_uuid, context_uuid, row.star).Scan(&id)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error inserting into DB: %v", err)
		//return -1
		return err
	}
	row.id = id
	row.dirty = false

	return nil
}

func (db *Database) readNoteIntoString(id int) string {
	if id == -1 {
		return "" // id given to new and unsaved entries
	}

	row := db.MainDB.QueryRow("SELECT note FROM task WHERE id=?;", id)
	var note sql.NullString
	err := row.Scan(&note)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error retrieving note for id %d: %v", id, err)
		return ""
	}
	return note.String
}

func (db *Database) getEntryInfo(id int) NewEntry {
	if id == -1 {
		return NewEntry{}
	}
	// Note: folder_tid and context_tid are deprecated in favor of uuid-based lookups
	// We no longer query them here - use taskFolder() and taskContext() instead
	row := db.MainDB.QueryRow("SELECT id, tid, title, star, added, archived, deleted, modified FROM task WHERE id=?;", id)

	var e NewEntry
	var tid sql.NullInt64
	err := row.Scan(
		&e.id,
		&tid,
		&e.title,
		&e.star,
		&e.added,
		&e.archived,
		&e.deleted,
		&e.modified,
	)
	e.tid = int(tid.Int64)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error in getEntryInfo for id %d: %v", id, err)
		return NewEntry{}
	}
	return e
}

func (db *Database) taskFolder(id int) string {
	// Query folder title by joining on uuid
	row := db.MainDB.QueryRow("SELECT folder.title FROM task JOIN folder on task.folder_uuid = folder.uuid WHERE task.id=?;", id)
	var title string
	err := row.Scan(&title)
	if err != nil {
		return ""
	}
	return title
}

func (db *Database) taskContext(id int) string {
	row := db.MainDB.QueryRow("SELECT context.title FROM task JOIN context on task.context_uuid = context.uuid WHERE task.id=?;", id)
	var title string
	err := row.Scan(&title)
	if err != nil {
		return ""
	}
	return title
}

func (db *Database) entryExists(id int) bool {
	var exists bool
	err := db.MainDB.QueryRow("SELECT EXISTS(SELECT 1 FROM task WHERE id=?);", id).Scan(&exists)
	return err == nil && exists
}

func (db *Database) getTitle(id int) string {
	row := db.MainDB.QueryRow("SELECT title FROM task WHERE id=?;", id)
	var title string
	err := row.Scan(&title)
	if err != nil {
		return ""
	}
	return title
}

func (db *Database) getTaskKeywords(id int) string {

	entry_tid := db.entryTidFromId(id) /////////////////////////////////////////////////////

	//rows, err := db.Query("SELECT keyword.name FROM task_keyword LEFT OUTER JOIN keyword ON "+
	//	"keyword.id=task_keyword.keyword_id WHERE task_keyword.task_id=?;", id)

	rows, err := db.MainDB.Query("SELECT keyword.title FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.uuid=task_keyword.keyword_uuid WHERE task_keyword.task_tid=?;", entry_tid)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error in getTaskKeywords for entry id %d: %v", id, err)
		return ""
	}
	defer rows.Close()

	kk := []string{}
	for rows.Next() {
		var title string

		err = rows.Scan(&title)
		kk = append(kk, title)
	}
	if len(kk) == 0 {
		return ""
	}
	return strings.Join(kk, ",")
}

// The escape sequences highlight() wraps a matched term in, in the title
// column of a search result.
const ftsMark, ftsUnmark = "\x1b[48;5;31m", "\x1b[49m"

// updateFtsTitle refreshes a search-result row's highlighted title after the
// title was edited. An error (sql.ErrNoRows) means the row no longer matches
// the search.
func (db *Database) updateFtsTitle(st string, row *Row) error {
	var ftsTitle string
	err := db.MainDB.QueryRow("SELECT highlight(task_fts, 0, ?, ?) FROM task_fts WHERE task_fts MATCH ? AND rowid=?;",
		ftsMark, ftsUnmark, st, row.id).Scan(&ftsTitle)
	if err != nil {
		return err
	}
	row.ftsTitle = ftsTitle
	return nil
}

// searchEntries returns the notes matching a full-text query, best match
// first, and at most MAX of them, like every other list. Notes that have not
// synced yet are found too: the index is keyed by the local id.
func (db *Database) searchEntries(st, sort string, showDeleted bool) ([]Row, error) {
	stmt := fmt.Sprintf("SELECT task.id, task.tid, task.title, task.star, task.deleted, task.archived, task.note, task.%s, "+
		"highlight(task_fts, 0, ?, ?) "+
		"FROM task_fts JOIN task ON task.id = task_fts.rowid WHERE task_fts MATCH ?", sort)
	if !showDeleted {
		stmt += " AND task.archived=false AND task.deleted=false"
	}
	stmt += " ORDER BY bm25(task_fts, 2.0, 1.0, 5.0) LIMIT ?;"

	rows, err := db.MainDB.Query(stmt, ftsMark, ftsUnmark, st, MAX)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgRows []Row
	for rows.Next() {
		var row Row
		var tid sql.NullInt64
		var stamp, note, ftsTitle sql.NullString
		if err := rows.Scan(&row.id, &tid, &row.title, &row.star, &row.deleted, &row.archived, &note, &stamp, &ftsTitle); err != nil {
			return nil, err
		}
		row.tid = int(tid.Int64)
		if note.Valid && note.String != "" {
			row.hasImage = containsGoogleDriveImage(note.String)
		}
		if stamp.Valid {
			row.sort = timeDelta(stamp.String)
		}
		row.ftsTitle = ftsTitle.String
		orgRows = append(orgRows, row)
	}
	return orgRows, rows.Err()
}

func (db *Database) getContainers(view View) []Row {
	stmt := fmt.Sprintf("SELECT id, title, star, deleted, modified FROM %s ORDER BY title COLLATE NOCASE ASC;", view)
	rows, err := db.MainDB.Query(stmt)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error SELECTING id, title, star, deleted, modified FROM %s", view)
		return []Row{}
	}
	defer rows.Close()

	var orgRows []Row
	for rows.Next() {
		var r Row
		var sort string
		rows.Scan(
			&r.id,
			&r.title,
			&r.star,
			&r.deleted,
			&sort,
		)
		r.sort = timeDelta(sort)
		orgRows = append(orgRows, r)
	}
	return orgRows
}

func (db *Database) getContainerInfo(id int, view View) Container {

	if id == -1 {
		return Container{}
	}

	var countQuery string
	switch view {
	case CONTEXT:
		// Join on uuid for context
		countQuery = "SELECT COUNT(*) FROM task JOIN context ON context.uuid = task.context_uuid WHERE context.id=?;"
	case FOLDER:
		// Join on uuid for folder
		countQuery = "SELECT COUNT(*) FROM task JOIN folder ON folder.uuid = task.folder_uuid WHERE folder.id=?;"
	case KEYWORD:
		// Join on uuid for keyword
		countQuery = "SELECT COUNT(*) FROM task_keyword WHERE keyword_uuid=(SELECT uuid FROM keyword WHERE id=?);"
	default:
		app.Organizer.ShowMessage(BL, "Somehow you are in a view I can't handle")
		return Container{}
	}

	var c Container

	row := db.MainDB.QueryRow(countQuery, id)
	err := row.Scan(&c.count)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error in getContainerInfo: %v", err)
		return Container{}
	}

	//stmt := fmt.Sprintf("SELECT %s FROM %s WHERE id=?;", columns, table)
	stmt := fmt.Sprintf("SELECT id, tid, title, star, deleted, modified FROM %s WHERE id=?;", view)
	row = db.MainDB.QueryRow(stmt, id)
	var tid sql.NullInt64
	err = row.Scan(
		&c.id,
		&tid,
		&c.title,
		&c.star,
		&c.deleted,
		&c.modified,
	)
	c.tid = int(tid.Int64)

	if err != nil {
		app.Organizer.ShowMessage(BL, "Error in getContainerInfo: %v", err)
		return Container{}
	}
	return c
}

// addTaskKeywordByUUID attaches a keyword to a task. task_keyword is keyed
// by server tids, so both the task and the keyword must have synced first:
// a NULL tid would be stored as 0 and the association silently lost on the
// next sync (SYNC_ISSUES.md #5).
func (db *Database) addTaskKeywordByUUID(keyword_uuid string, entry_id int) error {
	entry_tid := db.entryTidFromId(entry_id)
	if entry_tid < 1 {
		return fmt.Errorf("entry %d has not been synced yet - sync before adding keywords", entry_id)
	}

	var keyword_tid sql.NullInt64
	err := db.MainDB.QueryRow("SELECT tid FROM keyword WHERE uuid=?;", keyword_uuid).Scan(&keyword_tid)
	if err != nil {
		return fmt.Errorf("looking up keyword tid: %v", err)
	}
	if !keyword_tid.Valid || keyword_tid.Int64 < 1 {
		return fmt.Errorf("keyword has not been synced yet - sync before using it")
	}

	_, err = db.MainDB.Exec("INSERT OR IGNORE INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES (?, ?, ?);",
		entry_tid, keyword_tid.Int64, keyword_uuid)
	if err != nil {
		return fmt.Errorf("INSERT OR IGNORE INTO task_keyword: %v", err)
	}

	_, err = db.MainDB.Exec("UPDATE task SET modified = datetime('now') WHERE id=?;", entry_id)
	if err != nil {
		return fmt.Errorf("updating task modified: %v", err)
	}

	return nil
}

func (db *Database) updateContainerTitle(row *Row, view View) error {
	if row.id == -1 {
		err := db.insertContainer(row, view)
		return err
	}
	stmt := fmt.Sprintf("UPDATE %s SET title=?, modified=datetime('now') WHERE id=?", view)
	_, err := db.MainDB.Exec(stmt, row.title, row.id)
	return err
}

func (db *Database) insertContainer(row *Row, view View) error {
	newUUID := generateUUID()
	stmt := fmt.Sprintf("INSERT INTO %s (title, uuid, star, deleted, modified) ", view)
	stmt += "VALUES (?, ?, ?, False, datetime('now')) RETURNING id;"
	var id int
	err := db.MainDB.QueryRow(stmt, row.title, newUUID, row.star).Scan(&id)
	if err != nil {
		//sess.showOrgMessage("Error in insertContainer: %v", err)
		return err
	}
	row.id = id
	row.dirty = false

	return nil
}

func (db *Database) deleteKeywords(id int) int {
	entry_tid := db.entryTidFromId(id)
	res, err := db.MainDB.Exec("DELETE FROM task_keyword WHERE task_tid=?;", entry_tid)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error deleting from task_keyword: %v", err)
		return -1
	}
	rowsAffected, _ := res.RowsAffected()
	_, err = db.MainDB.Exec("UPDATE task SET modified=datetime('now') WHERE id=?;", id)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error updating entry modified column in deleteKeywords: %v", err)
		return -1
	}
	return int(rowsAffected)
}

// current method in use
func (db *Database) highlightTerms2(id int) string {
	if id == -1 {
		return "" // id given to new and unsaved entries
	}

	// qx and qy mark the matched terms; the renderer turns them into colour
	row := db.MainDB.QueryRow("SELECT highlight(task_fts, 1, 'qx', 'qy') FROM task_fts WHERE rowid=? AND task_fts MATCH ?;",
		id, app.Session.fts_search_terms)

	var note sql.NullString
	err := row.Scan(&note)
	if errors.Is(err, sql.ErrNoRows) {
		// the note no longer matches the search (it was edited since): show it plain
		return db.readNoteIntoString(id)
	}
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error highlighting search terms: %v", err)
		return ""
	}
	return note.String
}

// copyEntry duplicates a task row: the title prefixed with "Copy of ",
// plus the source's note, star, context and folder.
//
// One INSERT…SELECT rather than a read-then-write. That is what keeps
// context_uuid and folder_uuid — the authoritative container references
// since the uuid migration — attached to the copy. The previous version
// read the containers through getEntryInfo, which stopped selecting the
// deprecated *_tid columns during that migration, so entry.folder_tid /
// entry.context_tid were always 0 and every copy silently landed in
// context/folder "none" instead of the source's. Copying the columns in
// SQL also carries a NULL note through as NULL, matching updateNote.
//
// Deliberately not copied: tid (NULL is what makes the next sync insert
// a server row and assign one), folder_tid/context_tid (left at their
// column defaults, as insertTitle does — sync re-derives them from the
// uuids), archived/deleted, and keywords (task_keyword keys off tid,
// which the new row does not have yet). The search index needs nothing
// here: its trigger indexes the new row. modified takes its
// CURRENT_TIMESTAMP default, which is what makes the copy a sync candidate.
func (db *Database) copyEntry(sourceID int) (int, error) {
	var newID int
	err := db.MainDB.QueryRow(
		`INSERT INTO task (title, note, context_uuid, folder_uuid, star, added)
		 SELECT 'Copy of ' || title, note, context_uuid, folder_uuid, star, datetime('now')
		 FROM task WHERE id=?
		 RETURNING id;`,
		sourceID).Scan(&newID)
	if err == sql.ErrNoRows {
		// The SELECT matched nothing, so nothing was inserted.
		return -1, fmt.Errorf("source entry not found")
	}
	if err != nil {
		return -1, err
	}

	return newID, nil
}
