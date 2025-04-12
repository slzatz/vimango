//go:build exclude
package main

import (
	"database/sql"
	"fmt"
//  "os"
//	"strings"
)

/*
// DBContext handles database operations for the application
type DBContext struct {
	App    *AppContext
	MainDB *sql.DB
	FtsDB  *sql.DB
}

// NewDBContext creates a new database context
func NewDBContext(a *AppContext) *DBContext {
	return &DBContext{
		App:    a,
		MainDB: a.DB,
		FtsDB:  a.FtsDB,
	}
}
*/

// GetEntries retrieves entries based on filtering criteria
//func (dbc *DBContext) GetEntries(taskView int, filter interface{}, showDeleted bool, sort string, sortPriority bool, limit int) ([]Row, error) {
func (a *App) GetEntries(taskView int, filter interface{}, showDeleted bool, sort string, sortPriority bool, limit int) ([]Row, error) {
	// Get show_completed flag from the organizer
	showCompleted := a.Organizer.show_completed
	
	// Build the query based on task view
	query := fmt.Sprintf("SELECT task.id, task.title, task.star, task.deleted, task.archived, task.%s FROM task ", sort)
	
	// Add the appropriate join based on the task view
	switch taskView {
	case BY_CONTEXT:
		query += "JOIN context ON context.tid=task.context_tid WHERE context.title=?"
	case BY_FOLDER:
		query += "JOIN folder ON folder.tid = task.folder_tid WHERE folder.title=?"
	case BY_KEYWORD:
		query += "JOIN task_keyword ON task.tid=task_keyword.task_tid " +
			"JOIN keyword ON keyword.tid=task_keyword.keyword_tid " +
			"WHERE task.tid = task_keyword.task_tid AND " +
			"task_keyword.keyword_tid = keyword.tid AND keyword.title=?"
	case BY_RECENT:
		query += "WHERE 1=?"
		filter = 1
	case BY_JOIN:
		query += "WHERE task.id=?"
	case BY_FIND:
		// Handle search results
		query += "WHERE task.id IN (SELECT tid FROM fts WHERE fts MATCH ?)"
	default:
		return nil, fmt.Errorf("unknown task view type: %d", taskView)
	}
	
	// Add deleted filter
	if !showDeleted {
		query += " AND task.deleted=0"
	}
	
	// Add completed filter - in this database completed items are marked as archived
	if !showCompleted {
		query += " AND task.archived=0"
	}
	
	// Add ordering
	if sortPriority {
		query += " ORDER BY task.priority DESC, task." + sort + " DESC"
	} else {
		query += " ORDER BY task." + sort + " DESC"
	}
	
	// Add limit
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	
	// Execute the query
	//rows, err := a.DB.Query(query, filter)
  fmt.Printf("Query: %s\n", query)
	rows, err := DB.MainDB.Query(query, filter)
	if err != nil {
		return nil, fmt.Errorf("database query error: %v", err)
	}
  //os.Exit(0)
	defer rows.Close()
	
	// Process the results
	var result []Row
	for rows.Next() {
		var r Row
		var sortValue sql.NullString
		
		if err := rows.Scan(&r.id, &r.title, &r.star, &r.deleted, &r.archived, &sortValue); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}
		
		// Convert date to relative time format
		if sortValue.Valid {
			r.sort = timeDelta(sortValue.String)
		} else {
			r.sort = ""
		}
		
		result = append(result, r)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %v", err)
	}
	
	return result, nil
}

// FilterEntries filters entries based on criteria (compatibility wrapper)
func (a *App) FilterEntries(taskview int, filter string, showDeleted bool, sort string, sortPriority bool, limit int) []Row {
	// This wrapper maintains compatibility with the existing code
	result, err := a.GetEntries(taskview, filter, showDeleted, sort, sortPriority, limit)
  //os.Exit(0)
	if err != nil {
		a.Session.showOrgMessage("Error getting entries: %v", err)
		return []Row{}
	}
	return result
}

/*
// ReadNoteText reads a note from the database
func (dbc *DBContext) ReadNoteText(id int) (string, error) {
	if id == -1 {
		return "", nil // id given to new and unsaved entries
	}

	row := dbc.MainDB.QueryRow("SELECT note FROM task WHERE id=?;", id)
	var note sql.NullString
	err := row.Scan(&note)
	if err != nil {
		return "", fmt.Errorf("error retrieving note for id %d: %v", id, err)
	}

	if note.Valid {
		return note.String, nil
	}
	return "", nil
}

// ReadNoteIntoString reads a note from the database (compatibility wrapper)
func (dbc *DBContext) ReadNoteIntoString(id int) string {
	note, err := dbc.ReadNoteText(id)
	if err != nil {
		dbc.App.Session.showOrgMessage("Error: %v", err)
		return ""
	}
	return note
}

// UpdateNote updates a note in the database
func (dbc *DBContext) UpdateNote(id int, text string) error {
	var nullableText sql.NullString
	if len(text) != 0 {
		nullableText.String = text
		nullableText.Valid = true
	}

	// Update the main note in the task table
	_, err := dbc.MainDB.Exec("UPDATE task SET note=?, modified=datetime('now') WHERE id=?;",
		nullableText, id)
	if err != nil {
		return fmt.Errorf("error updating note for entry with id %d: %v", id, err)
	}

	// Update the full-text search table
	entry_tid := dbc.EntryTidFromId(id)
	_, err = dbc.FtsDB.Exec("UPDATE fts SET note=? WHERE tid=?;", text, entry_tid)
	if err != nil {
		return fmt.Errorf("error updating FTS for entry with id %d: %v", id, err)
	}

	return nil
}

// UpdateNoteWrapper updates a note and handles error display (compatibility wrapper)
func (dbc *DBContext) UpdateNoteWrapper(id int, text string) {
	err := dbc.UpdateNote(id, text)
	if err != nil {
		dbc.App.Session.showOrgMessage("Error: %v", err)
		return
	}
	dbc.App.Session.showOrgMessage("Updated note and FTS entry for entry %d", id)
}

// UpdateFTSNote updates a note in the full-text search database
func (dbc *DBContext) UpdateFTSNote(tid int, title string, note string) error {
	// In a follow-up refactoring, implementation would be moved here from dbfunc.go
	var err error
	
	// First delete any existing entry
	_, err = dbc.FtsDB.Exec("DELETE FROM fts WHERE tid=?;", tid)
	if err != nil {
		return fmt.Errorf("Error deleting from FTS: %v", err)
	}
	
	// Then insert the new entry
	_, err = dbc.FtsDB.Exec("INSERT INTO fts (title, note, tid) VALUES (?, ?, ?);", 
		title, note, tid)
	if err != nil {
		return fmt.Errorf("Error inserting into FTS: %v", err)
	}
	
	return nil
}

// UpdateTaskTitle updates the title of a task
func (dbc *DBContext) UpdateTaskTitle(id int, title string) error {
	// Update the title in the main task table
	_, err := dbc.MainDB.Exec("UPDATE task SET title=?, modified=datetime('now') WHERE id=?", 
		title, id)
	if err != nil {
		return fmt.Errorf("error updating title for id %d: %v", id, err)
	}

	// Update the title in the FTS table
	entry_tid := dbc.EntryTidFromId(id)
	_, err = dbc.FtsDB.Exec("UPDATE fts SET title=? WHERE tid=?;", title, entry_tid)
	if err != nil {
		return fmt.Errorf("error updating FTS title for id %d: %v", id, err)
	}

	return nil
}
// InsertNewTask inserts a new task into the database
func (dbc *DBContext) InsertNewTask(title string, star bool, deleted bool, archived bool, sortDate string) (int, error) {
	result, err := dbc.MainDB.Exec("INSERT INTO task (tid, title, star, deleted, archived, added, modified) VALUES (NULL, ?, ?, ?, ?, datetime('now'), datetime('now'));",
		title, star, deleted, archived)
	if err != nil {
		return -1, fmt.Errorf("error inserting new task: %v", err)
	}
	
	id, err := result.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("error getting last insert ID: %v", err)
	}
	
	return int(id), nil
}

// ToggleStar toggles the star status of an item
func (dbc *DBContext) ToggleStar(id int, currentState bool, tableName string) (bool, error) {
	// Toggle the star status
	newState := !currentState
	
	// Update the database
	query := fmt.Sprintf("UPDATE %s SET star=?, modified=datetime('now') WHERE id=?;", tableName)
	_, err := dbc.MainDB.Exec(query, newState, id)
	if err != nil {
		return currentState, fmt.Errorf("error toggling star for id %d in table %s: %v", id, tableName, err)
	}
	
	return newState, nil
}

// ContextList returns a list of contexts
func (dbc *DBContext) ContextList() []Container {
	// In a follow-up refactoring, implementation would be moved here from dbfunc.go
	// For now, implement a proper Container list instead of using map[string]struct{}
	rows, _ := dbc.MainDB.Query("SELECT id, tid, title, star, deleted, modified FROM context;")
	defer rows.Close()

	var containers []Container
	for rows.Next() {
		var c Container
		_ = rows.Scan(&c.id, &c.tid, &c.title, &c.star, &c.deleted, &c.modified)
		containers = append(containers, c)
	}
	return containers
}

// FolderList returns a list of folders
func (dbc *DBContext) FolderList() []Container {
	// In a follow-up refactoring, implementation would be moved here from dbfunc.go
	// For now, implement a proper Container list instead of using map[string]struct{}
	rows, _ := dbc.MainDB.Query("SELECT id, tid, title, star, deleted, modified FROM folder;")
	defer rows.Close()

	var containers []Container
	for rows.Next() {
		var c Container
		_ = rows.Scan(&c.id, &c.tid, &c.title, &c.star, &c.deleted, &c.modified)
		containers = append(containers, c)
	}
	return containers
}

// KeywordList returns a list of keywords
func (dbc *DBContext) KeywordList() []Container {
	// In a follow-up refactoring, implementation would be moved here from dbfunc.go
	// For now, implement a proper Container list instead of using map[string]struct{}
	rows, _ := dbc.MainDB.Query("SELECT id, tid, title, star, deleted, modified FROM keyword;")
	defer rows.Close()

	var containers []Container
	for rows.Next() {
		var c Container
		_ = rows.Scan(&c.id, &c.tid, &c.title, &c.star, &c.deleted, &c.modified)
		containers = append(containers, c)
	}
	return containers
}

// SearchEntries searches for entries
func (dbc *DBContext) SearchEntries(searchTerm string, showDeleted bool, help bool) ([]Row, error) {
	// Get show_completed flag from the organizer
	showCompleted := dbc.App.Organizer.show_completed
	
	// First query the FTS database for matching entries
	ftsQuery := "SELECT tid, highlight(fts, 0, '\x1b[48;5;31m', '\x1b[49m') FROM fts WHERE fts MATCH ? ORDER BY bm25(fts, 2.0, 1.0, 5.0);"
	ftsRows, err := dbc.FtsDB.Query(ftsQuery, searchTerm)
	if err != nil {
		return nil, fmt.Errorf("FTS query error: %v", err)
	}
	defer ftsRows.Close()
	
	// Process FTS results
	var ftsTids []int
	var ftsTitles = make(map[int]string)
	
	for ftsRows.Next() {
		var ftsTid int
		var ftsTitle string
		
		if err = ftsRows.Scan(&ftsTid, &ftsTitle); err != nil {
			return nil, fmt.Errorf("error scanning FTS results: %v", err)
		}
		
		ftsTids = append(ftsTids, ftsTid)
		ftsTitles[ftsTid] = ftsTitle
	}
	
	if err = ftsRows.Err(); err != nil {
		return nil, fmt.Errorf("FTS row iteration error: %v", err)
	}
	
	// If no results, return empty
	if len(ftsTids) == 0 {
		return []Row{}, nil
	}
	
	// Build query to get tasks by their transaction IDs
	sortField := "modified" // Default sort field
	
	// Start building the query
	stmt := fmt.Sprintf("SELECT task.id, task.tid, task.title, task.star, task.deleted, task.archived, task.%s FROM task WHERE ", sortField)
	
	// Add help filter if needed
	if help {
		stmt += "task.context_tid = 16 AND task.tid IN ("
	} else {
		stmt += "task.tid IN ("
	}
	
	// Add all transaction IDs
	placeholders := make([]string, len(ftsTids))
	for i := range ftsTids {
		placeholders[i] = "?"
	}
	stmt += strings.Join(placeholders, ", ") + ")"
	
	// Add filters for deleted/archived
	if !showDeleted {
		stmt += " AND task.deleted=0"
	}
	
	// Add completed filter
	if !showCompleted {
		stmt += " AND task.archived=0"
	}
	
	// Add order by clause to maintain FTS ranking
	stmt += " ORDER BY CASE task.tid"
	for i := range ftsTids {
		stmt += fmt.Sprintf(" WHEN ? THEN %d", i)
	}
	stmt += fmt.Sprintf(" ELSE %d END", len(ftsTids))
	
	// Prepare arguments for the query (tids for IN clause + tids for ORDER BY clause)
	args := make([]interface{}, len(ftsTids)*2)
	for i, tid := range ftsTids {
		args[i] = tid                  // For the IN clause
		args[i+len(ftsTids)] = tid     // For the ORDER BY clause
	}
	
	// Execute the query
	dbRows, err := dbc.MainDB.Query(stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("task query error: %v", err)
	}
	defer dbRows.Close()
	
	// Process results
	var result []Row
	for dbRows.Next() {
		var row Row
		var sortValue sql.NullString
		
		if err := dbRows.Scan(&row.id, &row.tid, &row.title, &row.star, &row.deleted, &row.archived, &sortValue); err != nil {
			return nil, fmt.Errorf("error scanning task results: %v", err)
		}
		
		// Replace title with highlighted FTS title if available
		if highlightedTitle, ok := ftsTitles[row.tid]; ok {
			row.ftsTitle = highlightedTitle
		} else {
			row.ftsTitle = row.title
		}
		
		// Convert date to relative time format
		if sortValue.Valid {
			row.sort = timeDelta(sortValue.String)
		} else {
			row.sort = ""
		}
		
		result = append(result, row)
	}
	
	if err := dbRows.Err(); err != nil {
		return nil, fmt.Errorf("task row iteration error: %v", err)
	}
	
	return result, nil
}

// SearchEntriesWrapper is a compatibility wrapper for the global searchEntries function
func (dbc *DBContext) SearchEntriesWrapper(term string, showDeleted bool, help bool) []Row {
	result, err := dbc.SearchEntries(term, showDeleted, help)
	if err != nil {
		dbc.App.Session.showOrgMessage("Search error: %v", err)
		return []Row{}
	}
	return result
}

// EntryTidFromId gets the transaction ID for an entry
func (dbc *DBContext) EntryTidFromId(id int) int {
	var tid int
	_ = dbc.MainDB.QueryRow("SELECT tid FROM task WHERE id=?;", id).Scan(&tid)
	return tid
}

// KeywordTidFromId gets the transaction ID for a keyword
func (dbc *DBContext) KeywordTidFromId(id int) int {
	var tid int
	_ = dbc.MainDB.QueryRow("SELECT tid FROM keyword WHERE id=?;", id).Scan(&tid)
	return tid
}

*/
