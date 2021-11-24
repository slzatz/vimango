package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
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

func timeDelta(t string) string {
	t0 := time.Now()
	t1, _ := time.Parse("2006-01-02T15:04:05Z", t)
	diff := t0.Sub(t1)

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

func keywordExists(name string) int {
	row := db.QueryRow("SELECT keyword.id FROM keyword WHERE keyword.name=?;", name)
	var id int
	err := row.Scan(&id)
	if err != nil {
		return -1
	}
	return id
}

func generateContextMap() {
	// if new client context hasn't been synched - tid =0
	rows, err := db.Query("SELECT tid, title FROM context;")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var tid int
		var title string

		err = rows.Scan(&tid, &title)
		org.context_map[title] = tid
	}
}

func generateFolderMap() {
	// if new client folder hasn't been synched - tid =0
	rows, err := db.Query("SELECT tid, title FROM folder;")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var tid int
		var title string

		err = rows.Scan(&tid, &title)
		org.folder_map[title] = tid
	}
}

func generateKeywordMap() {
	rows, err := db.Query("SELECT name FROM keyword;")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string

		err = rows.Scan(&name)
		org.keywordMap[name] = 0
	}
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

func toggleCompleted() {
	//orow& row = org.rows.at(org.fr);
	id := getId()

	var completed sql.NullTime
	if org.rows[org.fr].completed {
		completed = sql.NullTime{}
	} else {
		completed = sql.NullTime{Time: time.Now(), Valid: true}
	}

	_, err := db.Exec("UPDATE task SET completed=?, modified=datetime('now') WHERE id=?;",
		completed, id)

	if err != nil {
		sess.showOrgMessage("Error toggling entry id %d to completed: %v", id, err)
		return
	}

	org.rows[org.fr].completed = !org.rows[org.fr].completed
	sess.showOrgMessage("Toggle completed for entry %d succeeded", id)
}

func updateTaskContext(new_context string, id int) {
	context_tid := org.context_map[new_context]
	if context_tid == 0 {
		sess.showOrgMessage("%q has not been synched yet - must do that before adding tasks", new_context)
		return
	}

	_, err := db.Exec("UPDATE task SET context_tid=?, modified=datetime('now') WHERE id=?;",
		context_tid, id)

	if err != nil {
		sess.showOrgMessage("Error updating context for entry %d to %s: %v", id, new_context, err)
		return
	}
}

func updateTaskFolder(new_folder string, id int) {
	folder_tid := org.folder_map[new_folder]
	if folder_tid == 0 {
		sess.showOrgMessage("%q has not been synched yet - must do that before adding tasks", new_folder)
		return
	}

	_, err := db.Exec("UPDATE task SET folder_tid=?, modified=datetime('now') WHERE id=?;",
		folder_tid, id)

	if err != nil {
		sess.showOrgMessage("Error updating folder for entry %d to %s: %v", id, new_folder, err)
		return
	}
}

func updateNote(id int, text string) {

	//text := e.bufferToString()

	_, err := db.Exec("UPDATE task SET note=?, modified=datetime('now') WHERE id=?;",
		text, id)
	if err != nil {
		sess.showOrgMessage("Error in updateNote for entry with id %d: %v", id, err)
		return
	}

	/***************fts virtual table update*********************/

	_, err = fts_db.Exec("UPDATE fts SET note=? WHERE lm_id=?;", text, id)
	if err != nil {
		sess.showOrgMessage("Error in updateNote updating fts for entry with id %d: %v", id, err)
	}

	sess.showOrgMessage("Updated note and fts entry for item %d", id)
}

func getSyncItems(max int) {
	rows, err := db.Query(fmt.Sprintf("SELECT id, title, modified FROM sync_log ORDER BY modified DESC LIMIT %d", max))
	if err != nil {
		sess.showOrgMessage("Error in getSyncItems: %v", err)
		return
	}

	defer rows.Close()

	org.rows = nil
	for rows.Next() {
		var row Row
		var modified string

		err = rows.Scan(&row.id,
			&row.title,
			&modified,
		)

		if err != nil {
			sess.showOrgMessage("Error in getSyncItems: %v", err)
			return
		}

		row.modified = timeDelta(modified)
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

func filterEntries(taskView int, filter string, showDeleted bool, sort string, max int) []Row {

	s := "SELECT task.id, task.title, task.star, task.deleted, task.completed, task.modified FROM task "

	switch taskView {
	case BY_CONTEXT:
		s += "JOIN context ON context.tid=task.context_tid WHERE context.title=?"
	case BY_FOLDER:
		s += "JOIN folder ON folder.tid = task.folder_tid WHERE folder.title=?"
	case BY_KEYWORD:
		s += "JOIN task_keyword ON task.id=task_keyword.task_id " +
			"JOIN keyword ON keyword.id=task_keyword.keyword_id " +
			"WHERE task.id = task_keyword.task_id AND " +
			"task_keyword.keyword_id = keyword.id AND keyword.name=?"
	case BY_RECENT:
		s += "WHERE 1=1"
	default:
		sess.showOrgMessage("You asked for an unsupported db query")
		return []Row{}
	}

	if !showDeleted {
		s += " AND task.completed IS NULL AND task.deleted=false"
	}
	s += fmt.Sprintf(" ORDER BY task.star DESC, task.%s DESC LIMIT %d;", sort, max)
	//int sortcolnum = org.sort_map[org.sort] //cpp
	var rows *sql.Rows
	var err error
	if filter == "" { //Recent
		rows, err = db.Query(s)
	} else {
		rows, err = db.Query(s, filter)
	}
	if err != nil {
		sess.showOrgMessage("Error in getItems: %v", err)
		return []Row{}
	}

	defer rows.Close()

	var orgRows []Row
	for rows.Next() {
		var row Row
		var completed sql.NullTime
		var modified string

		err = rows.Scan(&row.id,
			&row.title,
			&row.star,
			&row.deleted,
			&completed,
			&modified,
		)

		if err != nil {
			sess.showOrgMessage("Error in filterEntries: %v", err)
			return orgRows
		}

		if completed.Valid {
			row.completed = true
		} else {
			row.completed = false
		}

		row.modified = timeDelta(modified)

		orgRows = append(orgRows, row)

	}
	return orgRows
}

func updateTitle() {

	// needs to be a pointer because may send to insertRow
	row := &org.rows[org.fr]

	if row.id == -1 {
		// want to send pointer to insertRow
		insertRow(row)
		return
	}

	_, err := db.Exec("UPDATE task SET title=?, modified=datetime('now') WHERE id=?", row.title, row.id)
	if err != nil {
		sess.showOrgMessage("Error in updateTitle for id %d: %v", row.id, err)
		return
	}

	/***************fts virtual table update*********************/

	_, err = fts_db.Exec("UPDATE fts SET title=? WHERE lm_id=?;", row.title, row.id)
	if err != nil {
		sess.showOrgMessage("Error in updateTitle update fts for id %d: %v", row.id, err)
		return
	}
}

func updateRows() {
	var updated_rows []int

	for _, row := range org.rows {
		if !row.dirty {
			continue
		}

		if row.id == -1 {
			id := insertRow(&row)
			updated_rows = append(updated_rows, id)
			row.dirty = false
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

func insertRow(row *Row) int {

	folder_tid := 1
	context_tid := 1

	switch org.taskview {
	case BY_CONTEXT:
		context_tid = org.context_map[org.filter]
	case BY_FOLDER:
		folder_tid = org.folder_map[org.filter]
		//case BY_KEYWORD:
		//case BY_RECENT:
	}

	res, err := db.Exec("INSERT INTO task (tid, title, folder_tid, context_tid, "+
		"star, added, note, deleted, created, modified) "+
		"VALUES (0, ?, ?, ?, True, date(), '', False, "+
		"date(), datetime('now'));",
		row.title, folder_tid, context_tid)

	/*
	   not used:
	   tid,
	   tag,
	   duetime,
	   completed,
	   duedate,
	   repeat,
	   remind
	*/
	if err != nil {
		return -1
	}

	row_id, err := res.LastInsertId()
	if err != nil {
		sess.showOrgMessage("Error in insertRow for %s: %v", row.title, err)
		return -1
	}
	row.id = int(row_id)
	row.dirty = false

	/***************fts virtual table update*********************/

	//_, err = fts_db.Exec("INSERT INTO fts (title, lm_id) VALUES (?, ?);", row.title, row.id)
	_, err = fts_db.Exec("INSERT INTO fts (title, note, tag, lm_id) VALUES (?, ?, ?, ?);", row.title, "", "", row.id)
	if err != nil {
		sess.showOrgMessage("Error in insertRow inserting into fts for %s: %v", row.title, err)
		return row.id
	}

	sess.showOrgMessage("Successfully inserted new row with id {} and indexed it (new vesrsion)", row.id)

	return row.id
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
	var note string
	err := row.Scan(&note)
	if err != nil {
		return ""
	}
	return note
}

func readNoteIntoBuffer(e *Editor, id int) {
	if id == -1 {
		return // id given to new and unsaved entries
	}

	row := db.QueryRow("SELECT note FROM task WHERE id=?;", id)
	var note string
	err := row.Scan(&note)
	if err != nil {
		return
	}

	e.bb = bytes.Split([]byte(note), []byte("\n")) // yes, you need to do it this way

	/*
		e.vbuf, err = v.CreateBuffer(true, false)
		if err != nil {
			sess.showOrgMessage("%v", err)
		}
	*/
	e.vbuf = vim.BufferNew(0)

	/*
		err = v.SetCurrentBuffer(e.vbuf)
		if err != nil {
			sess.showOrgMessage("%v", err)
		} else {
			sess.showOrgMessage("%v", e.vbuf)
		}
	*/
	vim.BufferSetCurrent(e.vbuf)

	/*
		err = v.SetBufferLines(e.vbuf, 0, -1, true, e.bb)
		if err != nil {
			sess.showEdMessage("Error in SetBufferLines in dbfuc: %v", err)
		}
	*/
	vim.BufferSetLinesBBB(e.vbuf, e.bb)

	/*
		err = v.Command(fmt.Sprintf("w temp/buf%d", e.vbuf))
		if err != nil {
			sess.showEdMessage("Error in writing file in dbfunc: %v", err)
		}
	*/
	vim.Execute(fmt.Sprintf("w temp/buf%d", vim.BufferGetId(e.vbuf)))
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

func getEntryInfo(id int) Entry {
	if id == -1 {
		return Entry{}
	}
	row := db.QueryRow("SELECT id, tid, title, created, folder_tid, context_tid, star, added, completed, deleted, modified FROM task WHERE id=?;", id)

	var e Entry
	var tid sql.NullInt64
	err := row.Scan(
		&e.id,
		&tid,
		&e.title,
		&e.created,
		&e.folder_tid,
		&e.context_tid,
		&e.star,
		&e.added,
		&e.completed,
		&e.deleted,
		&e.modified,
	)
	if err != nil {
		sess.showOrgMessage("Error in getEntryInfo for id %d: %v", id, err)
		return Entry{}
	}
	if tid.Valid {
		e.tid = int(tid.Int64)
	} else {
		e.tid = 0
	}
	return e
}

/*
func getFolderTid(id int) int {
	row := db.QueryRow("SELECT folder_tid FROM task WHERE id=?;", id)
	var tid int
	err := row.Scan(&tid)
	if err != nil {
		return -1
	}
	return tid
}
*/

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
func getContextTid(id int) int {
	row := db.QueryRow("SELECT context_tid FROM task WHERE id=?;", id)
	var tid int
	err := row.Scan(&tid)
	if err != nil {
		return -1
	}
	return tid
}

// used in Editor.cpp
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

	rows, err := db.Query("SELECT keyword.name FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.id=task_keyword.keyword_id WHERE task_keyword.task_id=?;",
		id)
	if err != nil {
		sess.showOrgMessage("Error in getTaskKeywords for entry id %d: %v", id, err)
		return ""
	}
	defer rows.Close()

	kk := []string{}
	for rows.Next() {
		var name string

		err = rows.Scan(&name)
		kk = append(kk, name)
	}
	if len(kk) == 0 {
		return ""
	}
	return strings.Join(kk, ",")
}

func getTaskKeywordIds(id int) []int {

	kk := []int{}
	rows, err := db.Query("SELECT keyword_id FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.id=task_keyword.keyword_id WHERE task_keyword.task_id=?;", id)
	if err != nil {
		sess.showOrgMessage("Error in getTaskKeywordIds for entry id %d: %v", id, err)
		return kk
	}
	defer rows.Close()

	for rows.Next() {
		var k int
		err = rows.Scan(&k)
		kk = append(kk, k)
	}
	return kk
}

func searchEntries(st string, showDeleted, help bool) []Row {

	rows, err := fts_db.Query("SELECT lm_id, highlight(fts, 0, '\x1b[48;5;31m', '\x1b[49m') "+
		"FROM fts WHERE fts MATCH ? ORDER BY bm25(fts, 2.0, 1.0, 5.0);",
		st)

	defer rows.Close()

	var ftsIds []int
	var ftsTitles = make(map[int]string)

	for rows.Next() {
		var ftsId int
		var ftsTitle string

		err = rows.Scan(
			&ftsId,
			&ftsTitle,
		)

		if err != nil {
			sess.showOrgMessage("Error trying to retrieve search info from fts_db - term: %s; %v", st, err)
			return []Row{}
		}
		ftsIds = append(ftsIds, ftsId)
		ftsTitles[ftsId] = ftsTitle
	}

	if len(ftsIds) == 0 {
		return []Row{}
	}

	var stmt string

	// As noted above, if the item is deleted (gone) from the db it's id will not be found if it's still in fts
	if help {
		stmt = "SELECT task.id, task.title, task.star, task.deleted, task.completed, task.modified FROM task WHERE task.context_tid = 16 and task.id IN ("
	} else {
		stmt = "SELECT task.id, task.title, task.star, task.deleted, task.completed, task.modified FROM task WHERE task.id IN ("
	}

	max := len(ftsIds) - 1
	for i := 0; i < max; i++ {
		stmt += strconv.Itoa(ftsIds[i]) + ", "
	}

	stmt += strconv.Itoa(ftsIds[max]) + ")"
	if showDeleted {
		stmt += " ORDER BY "
	} else {
		stmt += " AND task.completed IS NULL AND task.deleted = False ORDER BY "
	}

	for i := 0; i < max; i++ {
		stmt += "task.id = " + strconv.Itoa(ftsIds[i]) + " DESC, "
	}
	stmt += "task.id = " + strconv.Itoa(ftsIds[max]) + " DESC"

	rows, err = db.Query(stmt)
	var orgRows []Row
	for rows.Next() {
		var row Row
		var completed sql.NullString
		var modified string

		err = rows.Scan(
			&row.id,
			&row.title,
			&row.star,
			&row.deleted,
			&completed,
			&modified,
		)

		if err != nil {
			sess.showOrgMessage("Error in searchEntries reading rows")
			return []Row{}
		}

		if completed.Valid {
			row.completed = true
		} else {
			row.completed = false
		}

		row.modified = timeDelta(modified)
		row.ftsTitle = ftsTitles[row.id]

		orgRows = append(orgRows, row)
	}
	return orgRows
}

func getContainers() {
	org.rows = nil

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
		columns = "id, name, star, deleted, modified"
		orderBy = "name"
	default:
		sess.showOrgMessage("Somehow you are in a view I can't handle")
		return
	}

	stmt := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s COLLATE NOCASE ASC;", columns, table, orderBy)
	rows, err := db.Query(stmt)
	if err != nil {
		sess.showOrgMessage("Error SELECTING %s FROM %s", columns, table)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var r Row
		var modified string
		rows.Scan(
			&r.id,
			&r.title,
			&r.star,
			&r.deleted,
			&modified,
		)

		r.modified = timeDelta(modified)
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
		columns = "id, name, star"
		orderBy = "name"
	default:
		sess.showOrgMessage("Somehow you are in a view I can't handle")
		return
	}

	stmt := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s COLLATE NOCASE ASC;", columns, table, orderBy)
	rows, err := db.Query(stmt)
	if err != nil {
		sess.showOrgMessage("Error SELECTING %s FROM %s", columns, table)
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

	var table string
	var countQuery string
	var columns string
	switch org.view {
	case CONTEXT:
		table = "context"
		// Note: the join for context and folder is on the context/folder *tid*
		countQuery = "SELECT COUNT(*) FROM task JOIN context ON context.tid = task.context_tid WHERE context.id=?;"
		//columns = "id, tid, title, \"default\", created, deleted, modified"
		columns = "id, tid, title, star, created, deleted, modified"
	case FOLDER:
		table = "folder"
		countQuery = "SELECT COUNT(*) FROM task JOIN folder ON folder.tid = task.folder_tid WHERE folder.id=?;"
		//columns = "id, tid, title, private, created, deleted, modified"
		columns = "id, tid, title, star, created, deleted, modified"
	case KEYWORD:
		table = "keyword"
		countQuery = "SELECT COUNT(*) FROM task_keyword WHERE keyword_id=?;"
		columns = "id, tid, name, star, deleted, modified"
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

	stmt := fmt.Sprintf("SELECT %s FROM %s WHERE id=?;", columns, table)
	row = db.QueryRow(stmt, id)
	var tid sql.NullInt64
	if org.view == KEYWORD {
		err = row.Scan(
			&c.id,
			&tid,
			&c.title,
			&c.star,
			&c.deleted,
			&c.modified,
		)
	} else {
		err = row.Scan(
			&c.id,
			&tid,
			&c.title,
			&c.star,
			&c.created,
			&c.deleted,
			&c.modified,
		)
	}
	if err != nil {
		sess.showOrgMessage("Error in getContainerInfo: %v", err)
		return Container{}
	}

	if tid.Valid {
		c.tid = int(tid.Int64)
	} else {
		c.tid = 0
	}

	return c
}

func addTaskKeyword(keyword_id, entry_id int, update_fts bool) {

	_, err := db.Exec("INSERT OR IGNORE INTO task_keyword (task_id, keyword_id) VALUES (?, ?);",
		entry_id, keyword_id)

	if err != nil {
		sess.showOrgMessage("Error in addTaskKeyword = INSERT or IGNORE INTO task_keyword: %v", err)
		return
	}

	_, err = db.Exec("UPDATE task SET modified = datetime('now') WHERE id=?;", entry_id)
	if err != nil {
		sess.showOrgMessage("Error in addTaskKeyword - Update task modified: %v", err)
		return
	}

	// *************fts virtual table update**********************
	if !update_fts {
		return
	}
	s := getTaskKeywords(entry_id)
	_, err = fts_db.Exec("UPDATE fts SET tag=? WHERE lm_id=?;", s, entry_id)
	if err != nil {
		sess.showOrgMessage("Error in addTaskKeyword - fts Update: %v", err)
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
	var table string
	var column string
	switch org.view {
	case CONTEXT:
		table = "context"
		column = "title"
	case FOLDER:
		table = "folder"
		column = "title"
	case KEYWORD:
		table = "keyword"
		column = "name"
	default:
		sess.showOrgMessage("Somehow that's a container I don't recognize")
		return
	}

	stmt := fmt.Sprintf("UPDATE %s SET %s=?, modified=datetime('now') WHERE id=?",
		table, column)
	_, err := db.Exec(stmt, row.title, row.id)
	if err != nil {
		sess.showOrgMessage("Error updating %s title for %d", table, row.id)
	}
	switch org.view {
	case CONTEXT:
		generateContextMap()
	case FOLDER:
		generateFolderMap()
	case KEYWORD:
		generateKeywordMap()
	}
}

func insertContainer(row *Row) int {
	var stmt string
	if org.view != KEYWORD {
		var table string
		switch org.view {
		case CONTEXT:
			table = "context"
		case FOLDER:
			table = "folder"
		default:
			sess.showOrgMessage("Somehow that's a container I don't recognize")
			return -1
		}

		stmt = fmt.Sprintf("INSERT INTO %s (title, star, deleted, created, modified, tid) ",
			table)

		stmt += "VALUES (?, ?, False, datetime('now'), datetime('now'), 0);"
	} else {

		stmt = "INSERT INTO keyword (name, star, deleted, modified, tid) " +
			"VALUES (?, ?, False, datetime('now'), 0);"
	}

	res, err := db.Exec(stmt, row.title, row.star)
	if err != nil {
		sess.showOrgMessage("Error in insertContainer: %v", err)
		return -1
	}
	switch org.view {
	case CONTEXT:
		generateContextMap()
	case FOLDER:
		generateFolderMap()
	case KEYWORD:
		generateKeywordMap()
	}
	id, _ := res.LastInsertId()
	row.id = int(id)
	row.dirty = false

	return row.id
}

func deleteKeywords(id int) int {
	res, err := db.Exec("DELETE FROM task_keyword WHERE task_id=?;", id)
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

	_, err = fts_db.Exec("UPDATE fts SET tag='' WHERE lm_id=?", id)
	if err != nil {
		sess.showOrgMessage("Error updating fts in deleteKeywords: %v", err)
		return -1
	}
	return int(rowsAffected)
}

func copyEntry() {
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
	kwids := getTaskKeywordIds(id)
	for _, keywordId := range kwids {
		addTaskKeyword(keywordId, newId, false) // means don't update fts
	}
	tag := getTaskKeywords(newId) // returns string
	_, err = fts_db.Exec("INSERT INTO fts (title, note, tag, lm_id) VALUES (?, ?, ?, ?);", e.title, e.note, tag, newId)
	if err != nil {
		sess.showOrgMessage("Error inserting into fts_db for entry %q with id %d: %v", truncate(e.title, 15), newId, err)
		return
	}
}

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

	row := fts_db.QueryRow("SELECT highlight(fts, 1, 'qx', 'qy') "+
		"FROM fts WHERE lm_id=$1 AND fts MATCH $2;", id, sess.fts_search_terms)

	var note string
	err := row.Scan(&note)
	sess.showOrgMessage("%v", err)
	if err != nil {
		return ""
	}

	return note
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
			ab.WriteString("\n")
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
	} else if org.view == TASK && org.mode != NO_ROWS {
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
	} else if org.view == TASK && org.mode != NO_ROWS {
		org.drawPreview()
	}
	sess.showOrgMessage("rows: %d  cols: %d  divider: %d", sess.screenLines, sess.screenCols, sess.divider)

	sess.returnCursor()
}
