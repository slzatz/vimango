package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/slzatz/vimango/vim"
)

func getId() int {
	return org.rows[org.fr].id
}

/***********************************new*********************************/
func entryTidFromId(id int) int {
	var tid int
	_ = db.QueryRow("SELECT tid FROM task WHERE id=?;", id).Scan(&tid)
	return tid
}

func keywordTidFromId(id int) int {
	var tid int
	_ = db.QueryRow("SELECT tid FROM keyword WHERE id=?;", id).Scan(&tid)
	return tid
}

/***********************************new*********************************/

func timeDelta(t string) string {
	var t0 time.Time
	if strings.Contains(t, "T") {
		t0, _ = time.Parse("2006-01-02T15:04:05Z", t)
	} else {
		t0, _ = time.Parse("2006-01-02 15:04:05", t)
	}
	diff := time.Since(t0)

	diff = diff / 1000000000
	if diff <= 120 {
		return fmt.Sprintf("%d seconds ago", diff)
	} else if diff <= 60*120 {
		return fmt.Sprintf("%d minutes ago", diff/60) // <120 minutes we report minute
	} else if diff <= 48*60*60 {
		return fmt.Sprintf("%d hours ago", diff/3600) // <48 hours report hours
	} else if diff <= 24*60*60*60 {
		return fmt.Sprintf("%d days ago", diff/3600/24) // <60 days report days
	} else if diff <= 24*30*24*60*60 {
		return fmt.Sprintf("%d months ago", diff/3600/24/30) // <24 months rep
	} else {
		return fmt.Sprintf("%d years ago", diff/3600/24/30/12)
	}
}

func keywordId(title string) int {
	var id int
	_ = db.QueryRow("SELECT id FROM keyword WHERE title=?;", title).Scan(&id)
	return id
}

func keywordExists(title string) (int, bool) {
	var tid sql.NullInt64
	err := db.QueryRow("SELECT tid FROM keyword WHERE title=?;", title).Scan(&tid)
	if err != nil {
		return 0, false
	}
	return int(tid.Int64), true
}

func entryTid(id int) int {
	var tid int
	_ = db.QueryRow("SELECT tid FROM task WHERE id=?;", id).Scan(&tid)
	return tid
}

/*
func keywordName(id int) string {
	row := db.QueryRow("SELECT name FROM keyword WHERE id=?;", id)
	var name string
	err := row.Scan(&name)
	if err != nil {
		return ""
	}
	return name
}
*/

func filterTitle(filter string, tid int) string {
	row := db.QueryRow(fmt.Sprintf("SELECT title FROM %s WHERE tid=?;", filter), tid)
	var title string
	err := row.Scan(&title)
	if err != nil {
		return ""
	}
	return title
}

func contextExists(title string) (int, bool) {
	var tid sql.NullInt64
	err := db.QueryRow("SELECT tid FROM context WHERE title=?;", title).Scan(&tid)
	if err != nil {
		return 0, false
	}
	return int(tid.Int64), true
}

func folderExists(title string) (int, bool) {
	var tid sql.NullInt64
	err := db.QueryRow("SELECT tid FROM folder WHERE title=?;", title).Scan(&tid)
	if err != nil {
		return 0, false
	}
	return int(tid.Int64), true
}

func contextList() map[string]struct{} {
	rows, _ := db.Query("SELECT title FROM context;")
	defer rows.Close()

	contexts := make(map[string]struct{})
	for rows.Next() {
		var title string
		_ = rows.Scan(&title)
		contexts[title] = struct{}{}
	}
	return contexts
}

func folderList() map[string]struct{} {
	rows, _ := db.Query("SELECT title FROM folder;")
	defer rows.Close()

	folders := make(map[string]struct{})
	for rows.Next() {
		var title string
		_ = rows.Scan(&title)
		folders[title] = struct{}{}
	}
	return folders
}

func keywordList() map[string]struct{} {
	rows, _ := db.Query("SELECT title FROM keyword;")
	defer rows.Close()

	keywords := make(map[string]struct{})
	for rows.Next() {
		var title string
		_ = rows.Scan(&title)
		keywords[title] = struct{}{}
	}
	return keywords
}

func toggleStar() {
	id := getId()

	s := fmt.Sprintf("UPDATE %s SET star=?, modified=datetime('now') WHERE id=?;",
		org.view) //table
	_, err := db.Exec(s, !org.rows[org.fr].star, id)

	if err != nil {
		sess.showOrgMessage("Error in toggleStar for id %d: %v", id, err)
	}

	org.rows[org.fr].star = !org.rows[org.fr].star
	sess.showOrgMessage("Toggle star succeeded")
}

func toggleDeleted() {
	id := getId()

	s := fmt.Sprintf("UPDATE %s SET deleted=?, modified=datetime('now') WHERE id=?;", org.view)
	_, err := db.Exec(s, !org.rows[org.fr].deleted, id)
	if err != nil {
		sess.showOrgMessage("Error toggling %s id %d to deleted: %v", org.view, id, err)
		return
	}

	org.rows[org.fr].deleted = !org.rows[org.fr].deleted
	sess.showOrgMessage("Toggle deleted for %s id %d succeeded", org.view, id)
}

func toggleArchived() {
	id := getId()
	org.rows[org.fr].archived = !org.rows[org.fr].archived

	_, err := db.Exec("UPDATE task SET archived=?, modified=datetime('now') WHERE id=?;",
		org.rows[org.fr].archived, id)

	if err != nil {
		sess.showOrgMessage("Error toggling archived for entry id %d: %v", id, err)
		return
	}

	sess.showOrgMessage("Toggle archived for entry %d succeeded", id)
}

func updateTaskContextByTid(tid, id int) {
	_, err := db.Exec("UPDATE task SET context_tid=?, modified=datetime('now') WHERE id=?;",
		tid, id)

	if err != nil {
		sess.showOrgMessage("Error updating context for entry %d to tid %d: %v", id, tid, err)
		return
	}
}

func updateTaskFolderByTid(tid, id int) {
	_, err := db.Exec("UPDATE task SET folder_tid=?, modified=datetime('now') WHERE id=?;",
		tid, id)

	if err != nil {
		sess.showOrgMessage("Error updating folder for entry %d to tid %d: %v", id, tid, err)
		return
	}
}

func updateNote(id int, text string) {

	var nullableText sql.NullString
	if len(text) != 0 {
		nullableText.String = text
		nullableText.Valid = true
	}

	_, err := db.Exec("UPDATE task SET note=?, modified=datetime('now') WHERE id=?;",
		nullableText, id)
	if err != nil {
		sess.showOrgMessage("Error in updateNote for entry with id %d: %v", id, err)
		return
	}

	/***************fts virtual table update*********************/

	entry_tid := entryTidFromId(id) /////////////////////////////////////////////////////

	//_, err = fts_db.Exec("UPDATE fts SET note=? WHERE lm_id=?;", text, id)
	_, err = fts_db.Exec("UPDATE fts SET note=? WHERE tid=?;", text, entry_tid)
	if err != nil {
		sess.showOrgMessage("Error in updateNote updating fts for entry with id %d: %v", id, err)
	}

	sess.showOrgMessage("Updated note and fts entry for entry %d", id)
}

func getSyncItems(max int) {
	rows, err := db.Query(fmt.Sprintf("SELECT id, title, %s FROM sync_log ORDER BY %s DESC LIMIT %d", org.sort, org.sort, max))
	if err != nil {
		sess.showOrgMessage("Error in getSyncItems: %v", err)
		return
	}

	defer rows.Close()

	org.rows = nil
	for rows.Next() {
		var row Row
		var sort string

		err = rows.Scan(&row.id, &row.title, &sort)

		if err != nil {
			sess.showOrgMessage("Error in getSyncItems: %v", err)
			return
		}

		row.sort = timeDelta(sort)
		org.rows = append(org.rows, row)

	}
}

func deleteSyncItem(id int) {
	_, err := db.Exec("DELETE FROM sync_log  WHERE id=?;", id)
	if err != nil {
		sess.showOrgMessage("Error deleting sync_log entry with id %d: %v", id, err)
		return
	}
	sess.showOrgMessage("Deleted sync_log entry with id %d", id)
}

func filterEntries(taskView int, filter interface{}, showDeleted bool, sort string, sortPriority bool, max int) []Row {

	s := fmt.Sprintf("SELECT task.id, task.title, task.star, task.deleted, task.archived, task.%s FROM task ", sort)

	switch taskView {
	case BY_CONTEXT:
		s += "JOIN context ON context.tid=task.context_tid WHERE context.title=?"
	case BY_FOLDER:
		s += "JOIN folder ON folder.tid = task.folder_tid WHERE folder.title=?"
	case BY_KEYWORD:
		s += "JOIN task_keyword ON task.tid=task_keyword.task_tid " +
			"JOIN keyword ON keyword.tid=task_keyword.keyword_tid " +
			"WHERE task.tid = task_keyword.task_tid AND " +
			"task_keyword.keyword_tid = keyword.tid AND keyword.title=?"
	case BY_RECENT:
		//s += "WHERE 1=1"
		s += "WHERE 1=?"
		filter = 1
	default:
		sess.showOrgMessage("You asked for an unsupported db query")
		return []Row{}
	}

	if !showDeleted {
		s += " AND task.archived=false AND task.deleted=false"
	}
	if sortPriority {
		s += fmt.Sprintf(" ORDER BY task.star DESC, task.%s DESC LIMIT %d;", sort, max)
	} else {
		s += fmt.Sprintf(" ORDER BY task.%s DESC LIMIT %d;", sort, max) //01162022
	}
	var rows *sql.Rows
	var err error
	/*
		if filter == "" { //Recent
			rows, err = db.Query(s)
		} else {
			rows, err = db.Query(s, filter)
		}
	*/
	rows, err = db.Query(s, filter)
	if err != nil {
		sess.showOrgMessage("Error in getItems: %v", err)
		return []Row{}
	}

	defer rows.Close()

	var orgRows []Row
	for rows.Next() {
		var row Row
		var sort sql.NullString

		err = rows.Scan(
			&row.id,
			&row.title,
			&row.star,
			&row.deleted,
			&row.archived,
			&sort,
		)

		if err != nil {
			sess.showOrgMessage("Error in filterEntries: %v", err)
			return orgRows
		}

		if sort.Valid {
			row.sort = timeDelta(sort.String)
		} else {
			row.sort = ""
		}

		orgRows = append(orgRows, row)

	}
	return orgRows
}

func (o *Organizer) readRowsIntoBuffer() {
	var ss []string
	for _, row := range o.rows {
		ss = append(ss, row.title)
	}
	vim.BufferSetLines(o.vbuf, 0, -1, ss, len(ss))
	vim.BufferSetCurrent(o.vbuf)
}

func updateTitle() {

	row := org.rows[org.fr]

	if row.id == -1 {
		// send pointer to insertRowinDB because updating row struct with new id
		err := insertRowInDB(&org.rows[org.fr])
		if err != nil {
			sess.showOrgMessage("Error inserting into DB: %v", err)
		} else {
			sess.showOrgMessage("New entry written to db with id: %d", row.id)
		}
		return
	}

	_, err := db.Exec("UPDATE task SET title=?, modified=datetime('now') WHERE id=?", row.title, row.id)
	if err != nil {
		sess.showOrgMessage("Error in updateTitle for id %d: %v", row.id, err)
		return
	}

	/***************fts virtual table update*********************/
	entry_tid := entryTidFromId(row.id) /////////////////////////////////////////////////////

	//_, err = fts_db.Exec("UPDATE fts SET title=? WHERE lm_id=?;", row.title, row.id)
	_, err = fts_db.Exec("UPDATE fts SET title=? WHERE tid=?;", row.title, entry_tid)
	if err != nil {
		sess.showOrgMessage("Error in updateTitle update fts for id %d: %v", row.id, err)
		return
	}
}

func updateRows() {
	var updated_rows []int

	for fr, row := range org.rows {
		if !row.dirty {
			continue
		}

		if row.id == -1 {
			// send pointer to insertRowinDB because updating row struct with new id
			err := insertRowInDB(&org.rows[fr])
			if err != nil {
				sess.showOrgMessage("Error inserting into DB: %v", err) //could be overwritten
			} else {
				updated_rows = append(updated_rows, row.id)
			}
			continue
		}

		_, err := db.Exec("UPDATE task SET title=?, modified=datetime('now') WHERE id=?", row.title, row.id)
		if err != nil {
			sess.showOrgMessage("Error in updateRows for id %d: %v", row.id, err)
			return
		}

		row.dirty = false
		updated_rows = append(updated_rows, row.id)
	}

	if len(updated_rows) == 0 {
		sess.showOrgMessage("There were no rows to update")
		return
	}
	sess.showOrgMessage("These ids were updated: %v", updated_rows)
}

func insertRowInDB(row *Row) error { // should return err

	folder_tid := 1
	context_tid := 1
	// if org.taskview is BY_KEYWORD or BY_RECENT then new task gets context=1, folder=1
	switch org.taskview {
	case BY_CONTEXT:
		context_tid, _ = contextExists(org.filter)
	case BY_FOLDER:
		folder_tid, _ = folderExists(org.filter)
	}
	var id int
	err := db.QueryRow("INSERT INTO task (title, folder_tid, context_tid, star, added) "+
		"VALUES (?, ?, ?, ?, datetime('now')) RETURNING id;",
		row.title, folder_tid, context_tid, row.star).Scan(&id)
	if err != nil {
		sess.showOrgMessage("Error inserting into DB: %v", err)
		//return -1
		return err
	}
	row.id = id
	row.dirty = false
	return nil

	/***************fts virtual table update*********************/
	/*
		// this is an issue; in current mode can't index in fts until sync and get a tid
		entry_tid := entryTidFromId(row.id) /////////////////////////////////////////////////////

		_, err = fts_db.Exec("INSERT INTO fts (title, note, tag, tid) VALUES (?, ?, ?, ?);", row.title, "", "", entry_tid)
		if err != nil {
			sess.showOrgMessage("Error in insertRowInDB inserting into fts for %s: %v", row.title, err)
			return row.id
		}
		sess.showOrgMessage("Successfully inserted new row with id %d and indexed it (new version)", row.id)
		//return row.id
		return nil
	*/
}

func insertSyncEntry(title, note string) {
	_, err := db.Exec("INSERT INTO sync_log (title, note, modified) VALUES (?, ?, datetime('now'));",
		title, note)
	if err != nil {
		sess.showOrgMessage("Error inserting sync log into db: %v", err)
	} else {
		sess.showOrgMessage("Wrote sync log to db")
	}
}

func readNoteIntoString(id int) string {
	if id == -1 {
		return "" // id given to new and unsaved entries
	}

	row := db.QueryRow("SELECT note FROM task WHERE id=?;", id)
	var note sql.NullString
	err := row.Scan(&note)
	if err != nil {
		sess.showOrgMessage("Error retrieving note for id %d: %v", id, err)
		return ""
	}
	return note.String
}

func readNoteIntoBuffer(e *Editor, id int) {
	if id == -1 {
		return // id given to new and unsaved entries
	}

	row := db.QueryRow("SELECT note FROM task WHERE id=?;", id)
	var note sql.NullString
	err := row.Scan(&note)
	if err != nil {
		sess.showOrgMessage("Error opening note for editing: %v", err)
		return
	}
	e.ss = strings.Split(note.String, "\n")
	//e.ss = strings.Split(note, "\n")
	e.vbuf = vim.BufferNew(0)
	vim.BufferSetCurrent(e.vbuf)
	vim.BufferSetLines(e.vbuf, 0, -1, e.ss, len(e.ss))
}

func readSyncLogIntoAltRows(id int) {
	row := db.QueryRow("SELECT note FROM sync_log WHERE id=?;", id)
	var note string
	err := row.Scan(&note)
	if err != nil {
		return
	}
	org.altRows = nil
	for _, line := range strings.Split(note, "\n") {
		var r AltRow
		r.title = line
		org.altRows = append(org.altRows, r)
	}

}
func readSyncLog(id int) string {
	row := db.QueryRow("SELECT note FROM sync_log WHERE id=?;", id)
	var note string
	err := row.Scan(&note)
	if err != nil {
		return ""
	}
	return note
}

func getEntryInfo(id int) NewEntry {
	if id == -1 {
		return NewEntry{}
	}
	row := db.QueryRow("SELECT id, tid, title, folder_tid, context_tid, star, added, archived, deleted, modified FROM task WHERE id=?;", id)

	var e NewEntry
	var tid sql.NullInt64
	err := row.Scan(
		&e.id,
		&tid,
		&e.title,
		&e.folder_tid,
		&e.context_tid,
		&e.star,
		&e.added,
		&e.archived,
		&e.deleted,
		&e.modified,
	)
	e.tid = int(tid.Int64)
	if err != nil {
		sess.showOrgMessage("Error in getEntryInfo for id %d: %v", id, err)
		return NewEntry{}
	}
	return e
}

func taskFolder(id int) string {
	//row := db.QueryRow("SELECT folder.title FROM folder JOIN task on task.folder_tid = folder.tid WHERE task.id=?;", id)
	// below seems better because where clause is on task
	row := db.QueryRow("SELECT folder.title FROM task JOIN folder on task.folder_tid = folder.tid WHERE task.id=?;", id)
	var title string
	err := row.Scan(&title)
	if err != nil {
		return ""
	}
	return title
}

func taskContext(id int) string {
	row := db.QueryRow("SELECT context.title FROM task JOIN context on task.context_tid = context.tid WHERE task.id=?;", id)
	var title string
	err := row.Scan(&title)
	if err != nil {
		return ""
	}
	return title
}

/*
func getContextTid(id int) int {
	row := db.QueryRow("SELECT context_tid FROM task WHERE id=?;", id)
	var tid int
	err := row.Scan(&tid)
	if err != nil {
		return -1
	}
	return tid
}
*/

func getTitle(id int) string {
	row := db.QueryRow("SELECT title FROM task WHERE id=?;", id)
	var title string
	err := row.Scan(&title)
	if err != nil {
		return ""
	}
	return title
}

func getTaskKeywords(id int) string {

	entry_tid := entryTidFromId(id) /////////////////////////////////////////////////////

	//rows, err := db.Query("SELECT keyword.name FROM task_keyword LEFT OUTER JOIN keyword ON "+
	//	"keyword.id=task_keyword.keyword_id WHERE task_keyword.task_id=?;", id)

	rows, err := db.Query("SELECT keyword.title FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.tid=task_keyword.keyword_tid WHERE task_keyword.task_tid=?;", entry_tid)
	if err != nil {
		sess.showOrgMessage("Error in getTaskKeywords for entry id %d: %v", id, err)
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

func getTaskKeywordTids(id int) []int {

	entry_tid := entryTidFromId(id) /////////////////////////////////////////////////////

	keyword_tids := []int{}
	rows, err := db.Query("SELECT keyword_tid FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.tid=task_keyword.keyword_tid WHERE task_keyword.task_tid=?;", entry_tid)
	if err != nil {
		sess.showOrgMessage("Error in getTaskKeywordIds for entry id %d: %v", id, err)
		return keyword_tids
	}
	defer rows.Close()

	for rows.Next() {
		var tid int
		err = rows.Scan(&tid)
		keyword_tids = append(keyword_tids, tid)
	}
	return keyword_tids
}

func searchEntries(st string, showDeleted, help bool) []Row {

	rows, err := fts_db.Query("SELECT tid, highlight(fts, 0, '\x1b[48;5;31m', '\x1b[49m') "+
		"FROM fts WHERE fts MATCH ? ORDER BY bm25(fts, 2.0, 1.0, 5.0);", st)

	defer rows.Close()

	var ftsTids []int
	var ftsTitles = make(map[int]string)

	for rows.Next() {
		var ftsTid int
		var ftsTitle string

		err = rows.Scan(
			&ftsTid,
			&ftsTitle,
		)

		if err != nil {
			sess.showOrgMessage("Error trying to retrieve search info from fts_db - term: %s; %v", st, err)
			return []Row{}
		}
		ftsTids = append(ftsTids, ftsTid)
		ftsTitles[ftsTid] = ftsTitle
	}

	if len(ftsTids) == 0 {
		return []Row{}
	}

	// As noted above, if the item is deleted (gone) from the db it's id will not be found if it's still in fts
	stmt := fmt.Sprintf("SELECT task.id, task.tid, task.title, task.star, task.deleted, task.archived, task.%s FROM task WHERE ", org.sort)
	if help {
		stmt += "task.context_tid = 16 and task.tid IN ("
	} else {
		stmt += "task.tid IN ("
	}

	max := len(ftsTids) - 1
	for i := 0; i < max; i++ {
		stmt += strconv.Itoa(ftsTids[i]) + ", "
	}

	stmt += strconv.Itoa(ftsTids[max]) + ")"
	if showDeleted {
		stmt += " ORDER BY "
	} else {
		stmt += " AND task.archived=false AND task.deleted=false ORDER BY "
	}

	for i := 0; i < max; i++ {
		stmt += "task.tid = " + strconv.Itoa(ftsTids[i]) + " DESC, "
	}
	stmt += "task.tid = " + strconv.Itoa(ftsTids[max]) + " DESC"

	rows, err = db.Query(stmt)
	if err != nil {
		sess.showOrgMessage("Error in Find query %q: %v", stmt[:10], err)
		return []Row{}
	}
	var orgRows []Row
	for rows.Next() {
		var row Row
		var sort string

		err = rows.Scan(
			&row.id,
			&row.tid,
			&row.title,
			&row.star,
			&row.deleted,
			&row.archived,
			&sort,
		)

		if err != nil {
			sess.showOrgMessage("Error in searchEntries reading rows")
			return []Row{}
		}

		row.sort = timeDelta(sort)
		row.ftsTitle = ftsTitles[row.tid]

		orgRows = append(orgRows, row)
	}
	return orgRows
}

func getContainers() {
	org.rows = nil
	org.sort = "modified" //only time column that all containers have

	/*
		var table string
		var columns string
		var orderBy string //only needs to be change for keyword
		switch org.view {
		case CONTEXT:
			table = "context"
			columns = "id, title, star, deleted, modified"
			orderBy = "title"
		case FOLDER:
			table = "folder"
			columns = "id, title, star, deleted, modified"
			orderBy = "title"
		case KEYWORD:
			table = "keyword"
			columns = "id, title, star, deleted, modified"
			orderBy = "titltitltitle"
		default:
			sess.showOrgMessage("Somehow you are in a view I can't handle")
			return
		}
	*/

	//stmt := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s COLLATE NOCASE ASC;", columns, table, orderBy)
	stmt := fmt.Sprintf("SELECT id, title, star, deleted, modified FROM %s ORDER BY title COLLATE NOCASE ASC;", org.view)
	rows, err := db.Query(stmt)
	if err != nil {
		sess.showOrgMessage("Error SELECTING id, title, star, deleted, modified FROM %s", org.view)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var r Row
		//var modified string
		var sort string
		rows.Scan(
			&r.id,
			&r.title,
			&r.star,
			&r.deleted,
			//&modified,
			&sort,
		)

		//r.modified = timeDelta(modified)
		r.sort = timeDelta(sort)
		org.rows = append(org.rows, r)
	}
	if len(org.rows) == 0 {
		sess.showOrgMessage("No results were returned")
		org.mode = NO_ROWS
	}

	// below should be somewhere else
	org.fc, org.fr, org.rowoff = 0, 0, 0
	org.filter = ""

}

func getAltContainers() {
	org.altRows = nil

	/*
		var table string
		var columns string
		var orderBy string //only needs to be change for keyword

		switch org.altView {
		case CONTEXT:
			table = "context"
			//columns = "id, title, \"default\""
			columns = "id, title, star"
			orderBy = "title"
		case FOLDER:
			table = "folder"
			//columns = "id, title, private"
			columns = "id, title, star"
			orderBy = "title"
		case KEYWORD:
			table = "keyword"
			columns = "id, title, star"
			orderBy = "title"
		default:
			sess.showOrgMessage("Somehow you are in a view I can't handle")
			return
		}
	*/

	//stmt := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s COLLATE NOCASE ASC;", columns, table, orderBy)
	stmt := fmt.Sprintf("SELECT id, title, star FROM %s ORDER BY title COLLATE NOCASE ASC;", org.altView)
	rows, err := db.Query(stmt)
	if err != nil {
		sess.showOrgMessage("Error SELECTING id, title, star FROM %s", org.altView)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var r AltRow
		rows.Scan(
			&r.id,
			&r.title,
			&r.star,
		)

		org.altRows = append(org.altRows, r)
	}
	/*
		if len(org.altRows) == 0 {
			sess.showOrgMessage("No results were returned")
		}
	*/

	// below should ? be somewhere else
	org.altFr = 0

}

func getContainerInfo(id int) Container {

	/*
		type Container struct {
			id       int
			tid      int
			title    string
			star     bool
			created  string
			deleted  bool
			modified string
			count    int
		}
	*/

	if id == -1 {
		return Container{}
	}

	var countQuery string
	switch org.view {
	case CONTEXT:
		//table = "context"
		// Note: the join for context and folder is on the context/folder *tid*
		countQuery = "SELECT COUNT(*) FROM task JOIN context ON context.tid = task.context_tid WHERE context.id=?;"
		//columns = "id, tid, title, star, created, deleted, modified"
	case FOLDER:
		//table = "folder"
		countQuery = "SELECT COUNT(*) FROM task JOIN folder ON folder.tid = task.folder_tid WHERE folder.id=?;"
		//columns = "id, tid, title, star, created, deleted, modified"
	case KEYWORD:
		//table = "keyword"
		//countQuery = "SELECT COUNT(*) FROM task_keyword WHERE keyword_tid=?;"
		countQuery = "SELECT COUNT(*) FROM task_keyword WHERE keyword_tid=(SELECT tid FROM keyword WHERE id=?);"
		//columns = "id, tid, name, star, deleted, modified"
	default:
		sess.showOrgMessage("Somehow you are in a view I can't handle")
		return Container{}
	}

	var c Container

	row := db.QueryRow(countQuery, id)
	err := row.Scan(&c.count)
	if err != nil {
		sess.showOrgMessage("Error in getContainerInfo: %v", err)
		return Container{}
	}

	//stmt := fmt.Sprintf("SELECT %s FROM %s WHERE id=?;", columns, table)
	stmt := fmt.Sprintf("SELECT id, tid, title, star, deleted, modified FROM %s WHERE id=?;", org.view)
	row = db.QueryRow(stmt, id)
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
		sess.showOrgMessage("Error in getContainerInfo: %v", err)
		return Container{}
	}
	return c
}

func addTaskKeywordByTid(keyword_tid, entry_id int, update_fts bool) {
	entry_tid := entryTidFromId(entry_id) /////////////////////////////////////////////////////

	_, err := db.Exec("INSERT OR IGNORE INTO task_keyword (task_tid, keyword_tid) VALUES (?, ?);",
		entry_tid, keyword_tid)

	if err != nil {
		sess.showOrgMessage("Error in addTaskKeywordByTid = INSERT or IGNORE INTO task_keyword: %v", err)
		return
	}

	_, err = db.Exec("UPDATE task SET modified = datetime('now') WHERE id=?;", entry_id)
	if err != nil {
		sess.showOrgMessage("Error in addTaskKeywordByTid - Update task modified: %v", err)
		return
	}

	// *************fts virtual table update**********************
	if !update_fts {
		return
	}
	s := getTaskKeywords(entry_id)
	//_, err = fts_db.Exec("UPDATE fts SET tag=? WHERE lm_id=?;", s, entry_id)
	_, err = fts_db.Exec("UPDATE fts SET tag=? WHERE tid=?;", s, entry_tid)
	if err != nil {
		sess.showOrgMessage("Error in addTaskKeywordByTid - fts Update: %v", err)
	}
}

// not in use but worked
func getNoteSearchPositions__(id int) [][]int {
	row := fts_db.QueryRow("SELECT rowid FROM fts WHERE lm_id=?;", id)
	var rowid int
	err := row.Scan(&rowid)
	if err != nil {
		return [][]int{}
	}
	var word_positions [][]int
	for i, term := range strings.Split(sess.fts_search_terms, " ") {
		word_positions = append(word_positions, []int{})
		rows, err := fts_db.Query("SELECT offset FROM fts_v WHERE doc=? AND term=? AND col='note';", rowid, term)
		if err != nil {
			sess.showOrgMessage("Error in getNoteSearchPositions - 'SELECT offset FROM fts_v': %v", err)
			return [][]int{}
		}
		defer rows.Close()

		for rows.Next() {
			var offset int
			err = rows.Scan(&offset)
			if err != nil {
				sess.showOrgMessage("Error in getNoteSearchPositions - 'rows.Scan(&offset)': %v", err)
				continue
			}
			word_positions[i] = append(word_positions[i], offset)
		}
	}
	return word_positions
}

func updateContainerTitle() {
	row := &org.rows[org.fr]
	if !row.dirty {
		sess.showOrgMessage("Row has not been changed")
		return
	}
	if row.id == -1 {
		insertContainer(row)
		return
	}
	stmt := fmt.Sprintf("UPDATE %s SET title=?, modified=datetime('now') WHERE id=?", org.view)
	_, err := db.Exec(stmt, row.title, row.id)
	if err != nil {
		sess.showOrgMessage("Error updating %s title for %d", org.view, row.id)
	}
}

func insertContainer(row *Row) int {
	stmt := fmt.Sprintf("INSERT INTO %s (title, star, deleted, modified) ", org.view)
	stmt += "VALUES (?, ?, False, datetime('now')) RETURNING id;"
	var id int
	err := db.QueryRow(stmt, row.title, row.star).Scan(&id)
	if err != nil {
		sess.showOrgMessage("Error in insertContainer: %v", err)
		return -1
	}
	row.id = id
	row.dirty = false

	return id
}

func deleteKeywords(id int) int {
	entry_tid := entryTidFromId(id) /////////////////////////////////////////////////////
	//res, err := db.Exec("DELETE FROM task_keyword WHERE task_id=?;", id)
	res, err := db.Exec("DELETE FROM task_keyword WHERE task_tid=?;", entry_tid)
	if err != nil {
		sess.showOrgMessage("Error deleting from task_keyword: %v", err)
		return -1
	}
	rowsAffected, _ := res.RowsAffected()
	_, err = db.Exec("UPDATE task SET modified=datetime('now') WHERE id=?;", id)
	if err != nil {
		sess.showOrgMessage("Error updating entry modified column in deleteKeywords: %v", err)
		return -1
	}

	//_, err = fts_db.Exec("UPDATE fts SET tag='' WHERE lm_id=?", id)
	_, err = fts_db.Exec("UPDATE fts SET tag='' WHERE tid=?", entry_tid)
	if err != nil {
		sess.showOrgMessage("Error updating fts in deleteKeywords: %v", err)
		return -1
	}
	return int(rowsAffected)
}

// need to revisit
/*
func copyEntry() {
	// ? needs temp tid
	id := getId()
	row := db.QueryRow("SELECT title, star, note, context_tid, folder_tid FROM task WHERE id=?;", id)
	var e Entry
	err := row.Scan(
		&e.title,
		&e.star,
		&e.note,
		&e.context_tid,
		&e.folder_tid,
	)
	if err != nil {
		sess.showOrgMessage("Error in copyEntry trying to copy id %d: %v", id, err)
	}

	res, err := db.Exec("INSERT INTO task (title, star, note, context_tid, folder_tid, created, added, modified, deleted) "+
		"VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'), datetime('now'), false);",
		"copy of "+e.title, e.star, e.note, e.context_tid, e.folder_tid)
	if err != nil {
		sess.showOrgMessage("Error inserting copy of entry %q into sqlite: %v:", truncate(e.title, 15), err)
		return
	}
	lastId, _ := res.LastInsertId()
	newId := int(lastId)
	kwtids := getTaskKeywordTids(id)
	for _, keywordTid := range kwtids {
		//kwn := keywordName(keywordId)
		addTaskKeywordByTid(keywordTid, entry_tid, false) // means don't update fts
	}
	tag := getTaskKeywords(newId) // returns string
	_, err = fts_db.Exec("INSERT INTO fts (title, note, tag, tid) VALUES (?, ?, ?, ?);", e.title, e.note, tag, entry_tid)
	if err != nil {
		sess.showOrgMessage("Error inserting into fts_db for entry %q with id %d: %v", truncate(e.title, 15), newId, err)
		return
	}
}
*/

// not in use but worked
func highlightTerms__(text string, word_positions [][]int) string {

	delimiters := " |,.;?:()[]{}&#/`-'\"—_<>$~@=&*^%+!\t\n\\" //must have \f if using it as placeholder

	for _, v := range word_positions {
		sess.showEdMessage("%v", word_positions)

		// start and end are positions in the text
		// word_num is what word number we are at in the text
		//wp is the position that we are currently looking for to highlight

		word_num := -1 //word position in text
		end := -1
		var start int

		// need to be non-punctuation because syntax highlighting
		// appears to strip some punctuation
		pre := "uuu"
		post := "yyy"
		add := len(pre) + len(post)
		for _, wp := range v {

			for {
				// I don't think the check below is necessary but we'll see
				if end >= len(text)-1 {
					break
				}

				start = start + end + 1
				end = strings.IndexAny(text[start:], delimiters)
				if end == -1 {
					end = len(text) - 1
				}

				if end != 0 { //if end = 0 we were sitting on a delimiter like a space
					word_num++
				}

				if wp == word_num {
					//text = text[:start+end] + "\x1b[48;5;235m" + text[start+end:]
					text = fmt.Sprintf("%s%s%s", text[:start+end], post, text[start+end:])
					//text = text[:start] + "\x1b[48;5;31m" + text[start:]
					text = fmt.Sprintf("%s%s%s", text[:start], pre, text[start:])
					end += add
					break // this breaks out of loop that was looking for the current highlighted word position
				}
			}
		}
	}
	return text
}

// current method in use
func highlightTerms2(id int) string {
	if id == -1 {
		return "" // id given to new and unsaved entries
	}
	entry_tid := entryTidFromId(id) /////////////////////////////////////////////////////

	//row := fts_db.QueryRow("SELECT highlight(fts, 1, 'qx', 'qy') "+
	//	"FROM fts WHERE lm_id=$1 AND fts MATCH $2;", id, sess.fts_search_terms)
	row := fts_db.QueryRow("SELECT highlight(fts, 1, 'qx', 'qy') "+
		"FROM fts WHERE tid=$1 AND fts MATCH $2;", entry_tid, sess.fts_search_terms)

	var note sql.NullString
	err := row.Scan(&note)
	if err != nil {
		sess.showOrgMessage("Error in SELECT highlight(fts ...:%v", err)
		return ""
	}
	return note.String
}

// not currently in use but more general than generateWWString
// has a length parameter and takes a ret param
func generateWWString_(text string, width int, length int, ret string) string {

	if text == "" {
		return ""
	}

	if length <= 0 {
		length = maxInt
	}

	ss := strings.Split(text, "\n")
	var ab strings.Builder

	y := 0
	filerow := 0

	for _, s := range ss {
		if filerow == len(ss) {
			return ab.String()
		}

		if s == "" {
			if y == length-1 {
				return ab.String()
			}
			ab.WriteString(ret)
			filerow++
			y++
			continue
		}

		pos := 0
		prev_pos := 0

		for {
			if prev_pos+width > len(s)-1 {
				ab.WriteString(s[prev_pos:])
				if y == length-1 {
					return ab.String()
				}
				ab.WriteString(ret)
				y++
				filerow++
				break
			}
			pos = strings.LastIndex(s[:prev_pos+width], " ")
			if pos == -1 || pos == prev_pos-1 {
				pos = prev_pos + width - 1
			}

			ab.WriteString(s[prev_pos : pos+1])

			if y == length-1 {
				return ab.String()
			}
			ab.WriteString(ret)
			y++
			prev_pos = pos + 1
		}
	}
	return ab.String()
}

func generateWWString(text string, width int) string {
	if text == "" {
		return ""
	}
	ss := strings.Split(text, "\n")
	var ab strings.Builder
	y := 0
	filerow := 0

	for _, s := range ss {
		if filerow == len(ss) {
			return ab.String()
		}

		s = strings.ReplaceAll(s, "\t", "    ")

		if s == "" {
			ab.WriteString("\n")
			filerow++
			y++
			continue
		}

		// do not word wrap http[s] links
		if strings.Index(s, "](http") != -1 {
			ab.WriteString(s)
			ab.WriteString("\n")
			filerow++
			y++
			continue
		}

		start := 0
		end := 0

		for {
			if start+width > len(s)-1 {
				ab.WriteString(s[start:])
				ab.WriteString("\n")
				y++
				filerow++
				break
			}

			pos := strings.LastIndex(s[start:start+width], " ")
			if pos == -1 {
				end = start + width - 1
			} else {
				end = start + pos
			}
			ab.WriteString(s[start : end+1])
			// generating placeholder so markdown handles word wrap \n correctly
			// things like ** ....\n .....** correctly
			ab.WriteString("^^^")
			//ab.WriteString("\n")
			y++
			start = end + 1
		}
	}
	return ab.String()
}

func updateCodeFile(id int, text string) {
	var filePath string
	lang := Languages[taskContext(id)]
	if lang == "cpp" {
		filePath = "/home/slzatz/clangd_examples/test.cpp"
	} else if lang == "go" {
		filePath = "/home/slzatz/go_fragments/main.go"
	} else if lang == "python" {
		filePath = "/home/slzatz/python_fragments/main.py"
	} else {
		sess.showEdMessage("I don't recognize %q", taskContext(id))
		return
	}

	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		sess.showEdMessage("error opening file %s: %w", filePath, err)
		return
	}
	defer f.Close()

	f.Truncate(0)
	f.WriteString(text)
	f.Sync()
}

func moveDividerPct(pct int) {
	// note below only necessary if window resized or font size changed
	sess.textLines = sess.screenLines - 2 - TOP_MARGIN

	if pct == 100 {
		sess.divider = 1
	} else {
		sess.divider = sess.screenCols - pct*sess.screenCols/100
	}
	sess.totaleditorcols = sess.screenCols - sess.divider - 2
	sess.eraseScreenRedrawLines()

	if sess.divider > 10 {
		org.refreshScreen()
		org.drawStatusBar()
	}

	if sess.editorMode {
		sess.positionWindows()
		sess.eraseRightScreen() //erases editor area + statusbar + msg
		sess.drawRightScreen()
	} else if org.view == TASK {
		org.drawPreview()
	}
	sess.showOrgMessage("rows: %d  cols: %d  divider: %d", sess.screenLines, sess.screenCols, sess.divider)

	sess.returnCursor()
}

func moveDividerAbs(num int) {
	if num >= sess.screenCols {
		sess.divider = 1
	} else if num < 20 {
		sess.divider = sess.screenCols - 20
	} else {
		sess.divider = sess.screenCols - num
	}

	sess.edPct = 100 - 100*sess.divider/sess.screenCols
	sess.totaleditorcols = sess.screenCols - sess.divider - 2
	sess.eraseScreenRedrawLines()

	if sess.divider > 10 {
		org.refreshScreen()
		org.drawStatusBar()
	}

	if sess.editorMode {
		sess.positionWindows()
		sess.eraseRightScreen() //erases editor area + statusbar + msg
		sess.drawRightScreen()
	} else if org.view == TASK {
		org.drawPreview()
	}
	sess.showOrgMessage("rows: %d  cols: %d  divider: %d edPct: %d", sess.screenLines, sess.screenCols, sess.divider, sess.edPct)

	sess.returnCursor()
}

func tempTid(table string) int {
	var tid int
	err := db.QueryRow(fmt.Sprintf("SELECT MIN(tid) FROM %s;", table)).Scan(&tid)
	// if there are no keywords etc this will err; could make the variable sql.NullInt64
	if err != nil {
		sess.showEdMessage("error in tid from %s: %v", table, err)
		return 0
	}
	//sess.showEdMessage("The minimum tid is: %d", tid)
	return tid - 1
}
