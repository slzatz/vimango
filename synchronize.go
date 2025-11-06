package main

/** note that sqlite datetime('now') returns utc **/

import (
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// Constants for default container IDs
const (
	DefaultContainerID = 1 // Represents "none" for context/folder
)

// EntryPlusTag represents an entry with associated tag information
type EntryPlusTag struct {
	NewEntry
	tag sql.NullString
}

// TaskKeywordPairs represents the relationship between tasks and keywords
type TaskKeywordPairs struct {
	taskTid    int
	keywordTid int
}

// TaskTag represents a task with its associated tags
type TaskTag struct {
	taskTid int
	tag     sql.NullString
}

// TaskKeyword3 represents a task-keyword relationship with keyword title
type TaskKeyword3 struct {
	taskTid int
	keyword string
}

// syncChanges holds all changes detected during sync
type syncChanges struct {
	serverUpdatedContexts []Container
	serverDeletedContexts []Container
	serverUpdatedFolders  []Container
	serverDeletedFolders  []Container
	serverUpdatedKeywords []Container
	serverDeletedKeywords []Container
	serverUpdatedEntries  []EntryPlusTag
	serverDeletedEntries  []Entry
	clientUpdatedContexts []Container
	clientDeletedContexts []Container
	clientUpdatedFolders  []Container
	clientDeletedFolders  []Container
	clientUpdatedKeywords []Container
	clientDeletedKeywords []Container
	clientUpdatedEntries  []NewEntry
	clientDeletedEntries  []Entry
}

// containerType represents the type of container (context, folder, keyword)
type containerType string

const (
	containerTypeContext containerType = "context"
	containerTypeFolder  containerType = "folder"
	containerTypeKeyword containerType = "keyword"
)

func bulkInsert(dbase *sql.DB, query string, args []interface{}) (err error) {
	stmt, err := dbase.Prepare(query)
	if err != nil {
		return fmt.Errorf("Error in bulkInsert Prepare: %v", err)
	}

	_, err = stmt.Exec(args...)
	if err != nil {
		return fmt.Errorf("Error in bulkInsert Exec: %v", err)
	}

	return
}

func createBulkInsertQueryFTS3(n int, entries []EntryPlusTag) (query string, args []interface{}) {
	values := make([]string, n)
	args = make([]interface{}, n*4)
	pos := 0
	for i, e := range entries {
		values[i] = "(?, ?, ?, ?)"
		args[pos] = e.title
		args[pos+1] = e.note
		args[pos+2] = e.tag
		args[pos+3] = e.tid
		pos += 4
	}
	query = fmt.Sprintf("INSERT INTO fts (title, note, tag, tid) VALUES %s;", strings.Join(values, ", "))
	return
}

func createBulkInsertQueryTaskKeywordPairs(n int, tk []TaskKeywordPairs) (query string, args []interface{}) {
	values := make([]string, n)
	args = make([]interface{}, n*2)
	pos := 0
	for i, e := range tk {
		values[i] = "(?, ?)"
		args[pos] = e.taskTid
		args[pos+1] = e.keywordTid
		pos += 2
	}
	query = fmt.Sprintf("INSERT INTO task_keyword (task_tid, keyword_tid) VALUES %s", strings.Join(values, ", "))
	return
}

func getTaskKeywordPairs(dbase *sql.DB, in string, plg io.Writer) []TaskKeywordPairs {
	stmt := fmt.Sprintf("SELECT task_tid, keyword_tid FROM task_keyword WHERE task_tid IN (%s);", in)
	rows, err := dbase.Query(stmt)
	if err != nil {
		println(err)
		return []TaskKeywordPairs{}
	}
	tkPairs := make([]TaskKeywordPairs, 0)
	for rows.Next() {
		var tk TaskKeywordPairs
		rows.Scan(
			&tk.taskTid,
			&tk.keywordTid,
		)
		tkPairs = append(tkPairs, tk)
	}
	return tkPairs
}

func getTaskKeywordPairsPQ(dbase *sql.DB, tids []int, plg io.Writer) []TaskKeywordPairs {
	rows, err := dbase.Query("SELECT task_tid, keyword_tid FROM task_keyword WHERE task_tid = ANY($1);", pq.Array(tids))
	if err != nil {
		println(err)
		return []TaskKeywordPairs{}
	}
	tkPairs := make([]TaskKeywordPairs, 0)
	for rows.Next() {
		var tk TaskKeywordPairs
		rows.Scan(
			&tk.taskTid,
			&tk.keywordTid,
		)
		tkPairs = append(tkPairs, tk)
	}
	return tkPairs
}

func TaskKeywordTids(dbase *sql.DB, plg io.Writer, tid int) []int {
	rows, err := dbase.Query("SELECT keyword.tid FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.tid=task_keyword.keyword_tid WHERE task_keyword.task_tid=$1;", tid)
	if err != nil {
		fmt.Fprintf(plg, "Error in getTaskKeywordsS: %v\n", err)
	}
	defer rows.Close()

	keywordTids := []int{}
	for rows.Next() {
		var keywordTid int

		err = rows.Scan(&keywordTid)
		keywordTids = append(keywordTids, keywordTid)
	}
	return keywordTids
}

func insertTaskKeywordTids(dbase *sql.DB, plg io.Writer, keywordTid, entryTid int) {
	_, err := dbase.Exec("INSERT INTO task_keyword (task_tid, keyword_tid) VALUES ($1, $2);",
		entryTid, keywordTid)
	if err != nil {
		fmt.Fprintf(plg, "Error in insertTaskKeywordTids: %v\n", err)
		return
	} else {
		fmt.Fprintf(plg, "Inserted into task_keyword entry tid **%d** and keyword_tid **%d**\n", entryTid, keywordTid)
	}
}

func getTagsPQ(dbase *sql.DB, tids []int, plg io.Writer) []TaskTag {
	rows, err := dbase.Query("SELECT task_keyword.task_tid, keyword.title FROM task_keyword LEFT OUTER JOIN keyword ON keyword.tid=task_keyword.keyword_tid WHERE task_keyword.task_tid = ANY($1) ORDER BY task_keyword.task_tid;", pq.Array(tids))
	if err != nil {
		fmt.Printf("Error in getTags_x: %v", err)
		return []TaskTag{}
	}
	taskkeywords := make([]TaskKeyword3, 0)
	for rows.Next() {
		var tk TaskKeyword3
		rows.Scan(
			&tk.taskTid,
			&tk.keyword,
		)
		taskkeywords = append(taskkeywords, tk)
	}
	if len(taskkeywords) == 0 {
		return []TaskTag{}
	}
	tasktags := make([]TaskTag, 0, 1000)
	keywords := make([]string, 0, 5)
	var tt TaskTag
	var tid int
	prevTid := taskkeywords[0].taskTid
	for _, tk := range taskkeywords {
		tid = tk.taskTid
		if tid == prevTid {
			keywords = append(keywords, tk.keyword)
		} else {
			tt.taskTid = prevTid
			tt.tag.String = strings.Join(keywords, ",")
			tt.tag.Valid = true
			tasktags = append(tasktags, tt)
			prevTid = tid
			keywords = keywords[:0]
			keywords = append(keywords, tk.keyword)
		}
	}
	// need to get the last pair
	tt.taskTid = tid
	tt.tag.String = strings.Join(keywords, ",")
	tt.tag.Valid = true
	tasktags = append(tasktags, tt)

	return tasktags
}

func getTagSQ(dbase *sql.DB, tid int, plg io.Writer) string {
	rows, err := dbase.Query("SELECT keyword.title FROM task_keyword LEFT OUTER JOIN keyword ON keyword.tid=task_keyword.keyword_tid WHERE task_keyword.task_tid = ?;", tid)
	if err != nil {
		fmt.Printf("Error in getTagSQ: %v", err)
		return ""
	}
	tag := []string{}
	for rows.Next() {
		var kn string
		rows.Scan(&kn)
		tag = append(tag, kn)
	}
	return strings.Join(tag, ",")
}

// fetchServerContainers fetches updated or deleted containers from server
func (a *App) fetchServerContainers(containerType containerType, serverTime string, deleted bool, lg io.Writer) ([]Container, error) {
	query := fmt.Sprintf("SELECT tid, title, star, modified FROM %s WHERE modified > $1 AND deleted = $2;", containerType)
	if deleted {
		query = fmt.Sprintf("SELECT tid, title FROM %s WHERE modified > $1 AND deleted = $2;", containerType)
	}

	rows, err := a.Database.PG.Query(query, serverTime, deleted)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for server_%s: %v", containerType, err)
	}
	defer rows.Close()

	var containers []Container
	for rows.Next() {
		var c Container
		if deleted {
			rows.Scan(&c.tid, &c.title)
		} else {
			rows.Scan(&c.tid, &c.title, &c.star, &c.modified)
		}
		containers = append(containers, c)
	}
	return containers, nil
}

// fetchClientContainers fetches updated or deleted containers from client
func (a *App) fetchClientContainers(containerType containerType, clientTime string, deleted bool, lg io.Writer) ([]Container, error) {
	query := fmt.Sprintf("SELECT id, tid, title, star, modified FROM %s WHERE substr(modified, 1, 19) > $1 AND deleted = $2;", containerType)
	if deleted {
		query = fmt.Sprintf("SELECT id, tid, title FROM %s WHERE substr(modified, 1, 19) > $1 AND deleted = $2;", containerType)
	}

	rows, err := a.Database.MainDB.Query(query, clientTime, deleted)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for client_%s: %v", containerType, err)
	}
	defer rows.Close()

	var containers []Container
	for rows.Next() {
		var c Container
		var tid sql.NullInt64
		if deleted {
			rows.Scan(&c.id, &tid, &c.title)
		} else {
			rows.Scan(&c.id, &tid, &c.title, &c.star, &c.modified)
		}
		c.tid = int(tid.Int64)
		containers = append(containers, c)
	}
	return containers, nil
}

// fetchAllChanges retrieves all changes from both server and client
func (a *App) fetchAllChanges(serverTime, clientTime string, lg io.Writer) (*syncChanges, error) {
	changes := &syncChanges{}
	var err error

	// Fetch server changes
	changes.serverUpdatedContexts, err = a.fetchServerContainers(containerTypeContext, serverTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.serverDeletedContexts, err = a.fetchServerContainers(containerTypeContext, serverTime, true, lg)
	if err != nil {
		return nil, err
	}

	changes.serverUpdatedFolders, err = a.fetchServerContainers(containerTypeFolder, serverTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.serverDeletedFolders, err = a.fetchServerContainers(containerTypeFolder, serverTime, true, lg)
	if err != nil {
		return nil, err
	}

	changes.serverUpdatedKeywords, err = a.fetchServerContainers(containerTypeKeyword, serverTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.serverDeletedKeywords, err = a.fetchServerContainers(containerTypeKeyword, serverTime, true, lg)
	if err != nil {
		return nil, err
	}

	// Fetch server entries
	rows, err := a.Database.PG.Query("SELECT tid, title, star, note, modified, added, archived, context_tid, folder_tid FROM task WHERE modified > $1 AND deleted = $2 ORDER BY tid;", serverTime, false)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for server_updated_entries: %v", err)
	}
	for rows.Next() {
		var e EntryPlusTag
		rows.Scan(&e.tid, &e.title, &e.star, &e.note, &e.modified, &e.added, &e.archived, &e.context_tid, &e.folder_tid)
		changes.serverUpdatedEntries = append(changes.serverUpdatedEntries, e)
	}
	rows.Close()

	rows, err = a.Database.PG.Query("SELECT tid, title FROM task WHERE modified > $1 AND deleted = $2;", serverTime, true)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for server_deleted_entries: %v", err)
	}
	for rows.Next() {
		var e Entry
		rows.Scan(&e.tid, &e.title)
		changes.serverDeletedEntries = append(changes.serverDeletedEntries, e)
	}
	rows.Close()

	// Fetch client changes
	changes.clientUpdatedContexts, err = a.fetchClientContainers(containerTypeContext, clientTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.clientDeletedContexts, err = a.fetchClientContainers(containerTypeContext, clientTime, true, lg)
	if err != nil {
		return nil, err
	}

	changes.clientUpdatedFolders, err = a.fetchClientContainers(containerTypeFolder, clientTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.clientDeletedFolders, err = a.fetchClientContainers(containerTypeFolder, clientTime, true, lg)
	if err != nil {
		return nil, err
	}

	changes.clientUpdatedKeywords, err = a.fetchClientContainers(containerTypeKeyword, clientTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.clientDeletedKeywords, err = a.fetchClientContainers(containerTypeKeyword, clientTime, true, lg)
	if err != nil {
		return nil, err
	}

	// Fetch client entries
	rows, err = a.Database.MainDB.Query("SELECT id, tid, title, star, note, modified, added, archived, context_tid, folder_tid FROM task WHERE substr(modified, 1, 19)  > ? AND deleted = ?;", clientTime, false)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for client_updated_entries: %v", err)
	}
	for rows.Next() {
		var e NewEntry
		var tid sql.NullInt64
		rows.Scan(&e.id, &tid, &e.title, &e.star, &e.note, &e.modified, &e.added, &e.archived, &e.context_tid, &e.folder_tid)
		e.tid = int(tid.Int64)
		changes.clientUpdatedEntries = append(changes.clientUpdatedEntries, e)
	}
	rows.Close()

	rows, err = a.Database.MainDB.Query("SELECT id, tid, title FROM task WHERE substr(modified, 1, 19) > $1 AND deleted = $2;", clientTime, true)
	if err != nil {
		return nil, fmt.Errorf("Error with retrieving client deleted entries: %v", err)
	}
	for rows.Next() {
		var e Entry
		var tid sql.NullInt64
		rows.Scan(&e.id, &tid, &e.title)
		e.tid = int(tid.Int64)
		changes.clientDeletedEntries = append(changes.clientDeletedEntries, e)
	}
	rows.Close()

	return changes, nil
}

// reportChanges logs a summary of all detected changes
func (changes *syncChanges) reportChanges(lg io.Writer) int {
	totalChanges := 0

	fmt.Fprint(lg, "## Server Changes\n")

	if len(changes.serverUpdatedContexts) > 0 {
		totalChanges += len(changes.serverUpdatedContexts)
		fmt.Fprintf(lg, "- Updated `Contexts`(new and modified): **%d**\n", len(changes.serverUpdatedContexts))
	} else {
		lg.(*strings.Builder).WriteString("- No `Contexts` updated (new and modified).\n")
	}

	if len(changes.serverDeletedContexts) > 0 {
		totalChanges += len(changes.serverDeletedContexts)
		fmt.Fprintf(lg, "- Deleted `Contexts`: %d\n", len(changes.serverDeletedContexts))
	} else {
		lg.(*strings.Builder).WriteString("- No `Contexts` deleted.\n")
	}

	if len(changes.serverUpdatedFolders) > 0 {
		totalChanges += len(changes.serverUpdatedFolders)
		fmt.Fprintf(lg, "- `Folders` Updated: %d\n", len(changes.serverUpdatedFolders))
	} else {
		lg.(*strings.Builder).WriteString("- No `Folders` updated.\n")
	}

	if len(changes.serverDeletedFolders) > 0 {
		totalChanges += len(changes.serverDeletedFolders)
		fmt.Fprintf(lg, "- Deleted `Folders`: %d\n", len(changes.serverDeletedFolders))
	} else {
		lg.(*strings.Builder).WriteString("- No `Folders` deleted.\n")
	}

	if len(changes.serverUpdatedKeywords) > 0 {
		totalChanges += len(changes.serverUpdatedKeywords)
		fmt.Fprintf(lg, "- Updated `Keywords`: %d\n", len(changes.serverUpdatedKeywords))
	} else {
		lg.(*strings.Builder).WriteString("- No `Keywords` updated.\n")
	}

	if len(changes.serverDeletedKeywords) > 0 {
		totalChanges += len(changes.serverDeletedKeywords)
		fmt.Fprintf(lg, "- Deleted server `Keywords`: %d\n", len(changes.serverDeletedKeywords))
	} else {
		lg.(*strings.Builder).WriteString("- No `Keywords` deleted.\n")
	}

	if len(changes.serverUpdatedEntries) > 0 {
		totalChanges += len(changes.serverUpdatedEntries)
		fmt.Fprintf(lg, "- Updated `Entries`: %d\n", len(changes.serverUpdatedEntries))
		if len(changes.serverUpdatedEntries) < 100 {
			for _, e := range changes.serverUpdatedEntries {
				fmt.Fprintf(lg, "    - tid: %d star: %t *%q* folder_tid: %d context_tid: %d  modified: %v\n", e.tid, e.star, truncate(e.title, 15), e.context_tid, e.folder_tid, tc(e.modified, 19, false))
			}
		}
	} else {
		lg.(*strings.Builder).WriteString("- No `Entries` updated.\n")
	}

	if len(changes.serverDeletedEntries) > 0 {
		totalChanges += len(changes.serverDeletedEntries)
		fmt.Fprintf(lg, "- Deleted `Entries`: %d\n", len(changes.serverDeletedEntries))
	} else {
		lg.(*strings.Builder).WriteString("- No `Entries` deleted.\n")
	}

	fmt.Fprint(lg, "## Client Changes\n")

	if len(changes.clientUpdatedContexts) > 0 {
		totalChanges += len(changes.clientUpdatedContexts)
		fmt.Fprintf(lg, "- `Contexts` updated: %d\n", len(changes.clientUpdatedContexts))
		for _, c := range changes.clientUpdatedContexts {
			fmt.Fprintf(lg, "    - id: %d; tid: %d %q; modified: %v\n", c.id, c.tid, tc(c.title, 15, true), tc(c.modified, 19, false))
		}
	} else {
		lg.(*strings.Builder).WriteString("- No `Contexts` updated.\n")
	}

	if len(changes.clientDeletedContexts) > 0 {
		totalChanges += len(changes.clientDeletedContexts)
		fmt.Fprintf(lg, "- Deleted client `Contexts`: %d\n", len(changes.clientDeletedContexts))
		for _, e := range changes.clientDeletedContexts {
			fmt.Fprintf(lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		lg.(*strings.Builder).WriteString("- No `Contexts` deleted.\n")
	}

	if len(changes.clientUpdatedFolders) > 0 {
		totalChanges += len(changes.clientUpdatedFolders)
		fmt.Fprintf(lg, "- Updated `Folders`: %d\n", len(changes.clientUpdatedFolders))
		for _, c := range changes.clientUpdatedFolders {
			fmt.Fprintf(lg, "    - id: %d; tid: %d %q; modified: %v\n", c.id, c.tid, tc(c.title, 15, true), tc(c.modified, 19, false))
		}
	} else {
		lg.(*strings.Builder).WriteString("- No `Folders` updated.\n")
	}

	if len(changes.clientDeletedFolders) > 0 {
		totalChanges += len(changes.clientDeletedFolders)
		fmt.Fprintf(lg, "- Deleted client `Folders`: %d\n", len(changes.clientDeletedFolders))
		for _, e := range changes.clientDeletedFolders {
			fmt.Fprintf(lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		lg.(*strings.Builder).WriteString("- No `Folders` deleted.\n")
	}

	if len(changes.clientUpdatedKeywords) > 0 {
		totalChanges += len(changes.clientUpdatedKeywords)
		fmt.Fprintf(lg, "- Updated `Keywords`: %d\n", len(changes.clientUpdatedKeywords))
		for _, c := range changes.clientUpdatedKeywords {
			fmt.Fprintf(lg, "    - id: %d; tid: %d %q; modified: %v\n", c.id, c.tid, tc(c.title, 15, true), tc(c.modified, 19, false))
		}
	} else {
		lg.(*strings.Builder).WriteString("- No `Keywords` updated.\n")
	}

	if len(changes.clientDeletedKeywords) > 0 {
		totalChanges += len(changes.clientDeletedKeywords)
		fmt.Fprintf(lg, "- Deleted `Keywords`: %d\n", len(changes.clientDeletedKeywords))
		for _, e := range changes.clientDeletedKeywords {
			fmt.Fprintf(lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		lg.(*strings.Builder).WriteString("- No `Keywords` deleted.\n")
	}

	if len(changes.clientUpdatedEntries) > 0 {
		totalChanges += len(changes.clientUpdatedEntries)
		fmt.Fprintf(lg, "- Updated `Entries`: %d\n", len(changes.clientUpdatedEntries))
		for _, e := range changes.clientUpdatedEntries {
			fmt.Fprintf(lg, "    - id: %d tid: %d star: %t *%q* context_tid: %d folder_tid: %d  modified: %v\n", e.id, e.tid, e.star, truncate(e.title, 15), e.context_tid, e.folder_tid, tc(e.modified, 19, false))
		}
	} else {
		lg.(*strings.Builder).WriteString("- No `Entries` updated.\n")
	}

	if len(changes.clientDeletedEntries) > 0 {
		totalChanges += len(changes.clientDeletedEntries)
		fmt.Fprintf(lg, "- Deleted `Entries`: %d\n", len(changes.clientDeletedEntries))
		for _, e := range changes.clientDeletedEntries {
			fmt.Fprintf(lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		lg.(*strings.Builder).WriteString("- No `Entries` deleted.\n")
	}

	return totalChanges
}

// syncContainersToClient syncs updated containers from server to client
func (a *App) syncContainersToClient(containerType containerType, containers []Container, lg io.Writer) {
	for _, c := range containers {
		var exists bool
		query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE tid=?)", containerType)
		err := a.Database.MainDB.QueryRow(query, c.tid).Scan(&exists)
		if err != nil {
			fmt.Fprintf(lg, "Error SELECT EXISTS for %s: %v\n", containerType, err)
			continue
		}

		if exists {
			query = fmt.Sprintf("UPDATE %s SET title=?, star=?, modified=datetime('now') WHERE tid=?;", containerType)
			_, err := a.Database.MainDB.Exec(query, c.title, c.star, c.tid)
			if err != nil {
				fmt.Fprintf(lg, "Error updating sqlite for %s with tid: %v: %v\n", containerType, c.tid, err)
			} else {
				fmt.Fprintf(lg, "Updated local %s: %q with tid: %v\n", containerType, c.title, c.tid)
			}
		} else {
			query = fmt.Sprintf("INSERT INTO %s (tid, title, star, modified, deleted) VALUES (?,?,?, datetime('now'), false);", containerType)
			_, err := a.Database.MainDB.Exec(query, c.tid, c.title, c.star)
			if err != nil {
				fmt.Fprintf(lg, "Error inserting new %s into sqlite: %v\n", containerType, err)
			}
		}
	}
}

// syncContainersToServer syncs updated containers from client to server
func (a *App) syncContainersToServer(containerType containerType, containers []Container, lg io.Writer) {
	for _, c := range containers {
		var exists bool
		query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE tid=$1);", containerType)
		err := a.Database.PG.QueryRow(query, c.tid).Scan(&exists)
		if err != nil {
			fmt.Fprintf(lg, "Error SELECT EXISTS for %s: %v\n", containerType, err)
			continue
		}

		if exists {
			query = fmt.Sprintf("UPDATE %s SET title=$1, star=$2, modified=now() WHERE tid=$3;", containerType)
			_, err := a.Database.PG.Exec(query, c.title, c.star, c.tid)
			if err != nil {
				fmt.Fprintf(lg, "Error updating postgres for %s with tid: %d: %v\n", containerType, c.tid, err)
			} else {
				fmt.Fprintf(lg, "Updated server %s: %q with tid: %v\n", containerType, c.title, c.tid)
			}
		} else {
			var tid int
			query = fmt.Sprintf("INSERT INTO %s (title, star, modified, deleted) VALUES ($1, $2, now(), false) RETURNING tid;", containerType)
			err := a.Database.PG.QueryRow(query, c.title, c.star).Scan(&tid)
			if err != nil {
				fmt.Fprintf(lg, "Error inserting new %s into postgres and returning tid: %v\n", containerType, err)
				continue
			}
			query = fmt.Sprintf("UPDATE %s SET tid=? WHERE id=?;", containerType)
			_, err = a.Database.MainDB.Exec(query, tid, c.id)
			if err != nil {
				fmt.Fprintf(lg, "Error on UPDATE %s SET tid ...: %v\n", containerType, err)
			} else {
				fmt.Fprintf(lg, "Inserted server %s %q and updated local tid to %d\n", containerType, c.title, tid)
			}
		}
	}
}

// syncEntriesToClient syncs updated entries from server to client (including FTS and keywords)
func (a *App) syncEntriesToClient(entries []EntryPlusTag, lg io.Writer) map[int]struct{} {
	updatedTids := make(map[int]struct{})
	var tids []int

	for _, e := range entries {
		_, err := a.Database.MainDB.Exec("INSERT INTO task (tid, title, star, added, archived, context_tid, folder_tid, note, modified, deleted) VALUES"+
			"(?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), false) ON CONFLICT(tid) DO UPDATE SET "+
			"title=excluded.title, star=excluded.star, archived=excluded.archived, context_tid=excluded.context_tid, "+
			"folder_tid=excluded.folder_tid, note=excluded.note, modified=datetime('now');",
			e.tid, e.title, e.star, e.added, e.archived, e.context_tid, e.folder_tid, e.note)
		if err != nil {
			fmt.Fprintf(lg, "**Error** in INSERT ... ON CONFLICT for tid %d %q: %v\n", e.tid, e.title, err)
			continue
		} else {
			fmt.Fprintf(lg, "Inserted or updated client entry %q with tid **%d**\n", e.title, e.tid)
		}
		tids = append(tids, e.tid)
		updatedTids[e.tid] = struct{}{}
	}

	if len(entries) == 0 {
		return updatedTids
	}

	// Delete existing keywords and FTS entries for updated tasks
	s := "?" + strings.Repeat(",?", len(tids)-1)
	stmt := fmt.Sprintf("DELETE FROM task_keyword WHERE task_tid IN (%s);", s)
	tidsIf := make([]interface{}, len(tids))
	for i := range tids {
		tidsIf[i] = tids[i]
	}
	_, err := a.Database.MainDB.Exec(stmt, tidsIf...)
	if err != nil {
		fmt.Fprintf(lg, "Error deleting from client task_keyword for tids: %v: %v\n", tids, err)
	}

	stmt = fmt.Sprintf("DELETE FROM fts WHERE tid IN (%s);", s)
	_, err = a.Database.FtsDB.Exec(stmt, tidsIf...)
	if err != nil {
		fmt.Fprintf(lg, "Error deleting from fts for tids: %v: %v\n", tids, err)
	}

	// Update keywords
	tks := getTaskKeywordPairsPQ(a.Database.PG, tids, lg)
	if len(tks) != 0 {
		query, args := createBulkInsertQueryTaskKeywordPairs(len(tks), tks)
		err = bulkInsert(a.Database.MainDB, query, args)
		if err != nil {
			fmt.Fprintf(lg, "%v\n", err)
		} else {
			fmt.Fprintf(lg, "Keywords updated for task tids: %v\n", tids)
		}

		tags := getTagsPQ(a.Database.PG, tids, lg)
		i := 0
		for j := 0; ; j++ {
			if j == len(entries) {
				break
			}
			entry := &entries[j]
			if entry.tid == tags[i].taskTid {
				entry.tag = tags[i].tag
				fmt.Fprintf(lg, "FTS tag will be updated for tid: %d, tag: %s\n", entry.tid, entry.tag.String)
				i += 1
				if i == len(tags) {
					break
				}
			}
		}
	}

	query, args := createBulkInsertQueryFTS3(len(entries), entries)
	err = bulkInsert(a.Database.FtsDB, query, args)
	if err != nil {
		fmt.Fprintf(lg, "%v", err)
	} else {
		fmt.Fprintf(lg, "FTS entries updated for task tids: %v\n", tids)
	}

	return updatedTids
}

// syncEntriesToServer syncs updated entries from client to server
func (a *App) syncEntriesToServer(entries []NewEntry, serverUpdatedTids map[int]struct{}, lg io.Writer) {
	for _, e := range entries {
		// Server wins if both client and server have updated an item
		if _, found := serverUpdatedTids[e.tid]; found {
			fmt.Fprintf(lg, "Server won: client entry %q with id %d and tid %d was updated by server\n", truncate(e.title, 15), e.id, e.tid)
			continue
		}

		var tid int
		if e.tid < 1 {
			err := a.Database.PG.QueryRow("INSERT INTO task (title, star, added, archived, context_tid, folder_tid, note, modified, deleted) "+
				"VALUES ($1, $2, $3, $4, $5, $6, $7, now(), false)  RETURNING tid",
				e.title, e.star, e.added, e.archived, e.context_tid, e.folder_tid, e.note).Scan(&tid)
			if err != nil {
				fmt.Fprintf(lg, "Error inserting server entry: %v", err)
				continue
			}
			_, err = a.Database.MainDB.Exec("UPDATE task SET tid=? WHERE id=?;", tid, e.id)
			if err != nil {
				fmt.Fprintf(lg, "Error setting tid for client entry %q with id %d to tid %d: %v\n", truncate(e.title, 15), e.id, tid, err)
				continue
			}

			// Create FTS entry for new entries
			taskTag := getTagSQ(a.Database.MainDB, tid, lg)
			var tag sql.NullString
			if len(taskTag) > 0 {
				tag.String = taskTag
				tag.Valid = true
			}
			_, err = a.Database.FtsDB.Exec("INSERT INTO fts (title, tag, note, tid) VALUES (?, ?, ?, ?);", e.title, tag, e.note, tid)
			if err != nil {
				fmt.Fprintf(lg, "Error in INSERT INTO fts: %v\n", err)
			}
			fmt.Fprintf(lg, "Created new server entry *%q* with tid **%d**\n", truncate(e.title, 15), tid)
			fmt.Fprintf(lg, "and set tid for client entry with id **%d** and created fts entry\n", e.id)
		} else {
			_, err := a.Database.PG.Exec("UPDATE task SET title=$1, star=$2, context_tid=$3, folder_tid=$4, note=$5, archived=$6, modified=now() WHERE tid=$7;",
				e.title, e.star, e.context_tid, e.folder_tid, e.note, e.archived, e.tid)
			if err != nil {
				fmt.Fprintf(lg, "Error updating server entry: %v", err)
				continue
			}
			tid = e.tid
			fmt.Fprintf(lg, "Updated server entry *%q* with tid **%d**\n", truncate(e.title, 15), tid)
		}

		// Update the server entry's keywords
		_, err := a.Database.PG.Exec("DELETE FROM task_keyword WHERE task_tid=$1;", tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting from task_keyword from server tid %d: %v\n", tid, err)
			continue
		}
		kwTids := TaskKeywordTids(a.Database.MainDB, lg, tid)
		for _, kwTid := range kwTids {
			insertTaskKeywordTids(a.Database.PG, lg, kwTid, tid)
		}
	}
}

// deleteServerEntriesFromClient removes entries deleted on server from client
func (a *App) deleteServerEntriesFromClient(entries []Entry, lg io.Writer) {
	for _, e := range entries {
		_, err := a.Database.MainDB.Exec("DELETE FROM task_keyword WHERE task_tid=?;", e.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting task_keyword client rows where entry tid = %d: %v\n", e.tid, err)
			continue
		}

		_, err = a.Database.MainDB.Exec("DELETE FROM task WHERE tid=?;", e.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting client entry %q with tid %d: %v\n", tc(e.title, 15, true), e.tid, err)
			continue
		}
		fmt.Fprintf(lg, "Deleted client entry %q with tid %d\n", truncate(e.title, 15), e.tid)
		fmt.Fprintf(lg, "and on client deleted task_tid %d from task_keyword\n", e.tid)
	}
}

// deleteClientEntriesFromServer marks entries deleted on client as deleted on server
func (a *App) deleteClientEntriesFromServer(entries []Entry, lg io.Writer) {
	for _, e := range entries {
		_, err := a.Database.MainDB.Exec("DELETE FROM task_keyword WHERE task_tid=?;", e.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting task_keyword client rows where entry tid = %d: %v\n", e.tid, err)
			continue
		}
		_, err = a.Database.MainDB.Exec("DELETE FROM task WHERE id=?", e.id)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting client entry %q with id %d: %v\n", tc(e.title, 15, true), e.id, err)
			continue
		}

		fmt.Fprintf(lg, "Deleted client entry %q with id %d\n", tc(e.title, 15, true), e.id)
		fmt.Fprintf(lg, "and on client deleted task_tid %d from task_keyword\n", e.tid)

		// Mark as deleted on server (if it exists there)
		if e.tid < 1 {
			fmt.Fprintf(lg, "There is no server entry to delete for client id %d\n", e.id)
			continue
		}

		_, err = a.Database.PG.Exec("UPDATE task SET deleted=true, modified=now() WHERE tid=$1", e.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error setting server entry with id %d to deleted: %v\n", e.tid, err)
			continue
		}
		fmt.Fprintf(lg, "Updated server entry %q with id %d to **deleted = true**\n", truncate(e.title, 15), e.tid)

		_, err = a.Database.PG.Exec("DELETE FROM task_keyword WHERE task_tid=$1;", e.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting task_keyword server rows where entry tid = %d: %v\n", e.tid, err)
			continue
		}
		fmt.Fprintf(lg, "and on server deleted task_tid %d from task_keyword\n", e.tid)
	}
}

// deleteContainerFromBoth deletes a container from both server and client, updating task references
func (a *App) deleteContainerFromBoth(containerType containerType, c Container, isServerDeleted bool, taskField string, lg io.Writer) {
	// Update tasks to reference default container (ID 1 = "none")
	if !isServerDeleted {
		query := fmt.Sprintf("UPDATE task SET %s=1, modified=now() WHERE %s=$1;", taskField, taskField)
		res, err := a.Database.PG.Exec(query, c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error trying to change server entry %s for a deleted %s: %v\n", taskField, containerType, err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(lg, "The number of server entries that were changed to 'none': **%d**\n", rowsAffected)
		}
	}

	query := fmt.Sprintf("UPDATE task SET %s=1, modified=datetime('now') WHERE %s=?;", taskField, taskField)
	res, err := a.Database.MainDB.Exec(query, c.tid)
	if err != nil {
		fmt.Fprintf(lg, "Error trying to change client entry %s for a deleted %s: %v\n", taskField, containerType, err)
	} else {
		rowsAffected, _ := res.RowsAffected()
		fmt.Fprintf(lg, "The number of client entries that were changed to 'none': **%d**\n", rowsAffected)
	}

	if isServerDeleted {
		// Delete from client only
		query = fmt.Sprintf("DELETE FROM %s WHERE tid=?", containerType)
		_, err = a.Database.MainDB.Exec(query, c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting local %s %q with tid = %d: %v\n", containerType, c.title, c.tid, err)
		} else {
			fmt.Fprintf(lg, "Deleted client %s %q with tid %d\n", containerType, c.title, c.tid)
		}
	} else {
		// Mark as deleted on server, delete from client
		query = fmt.Sprintf("UPDATE %s SET deleted=true, modified=now() WHERE tid=$1", containerType)
		_, err = a.Database.PG.Exec(query, c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error setting server %s %q with tid = %d to deleted: %v\n", containerType, c.title, c.tid, err)
		}

		query = fmt.Sprintf("DELETE FROM %s WHERE id=?", containerType)
		_, err = a.Database.MainDB.Exec(query, c.id)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting local %s %q with id %d: %v\n", containerType, c.title, c.id, err)
		} else {
			fmt.Fprintf(lg, "Deleted client %s %q: id %d and updated server %s with tid %d to deleted = true\n", containerType, c.title, c.id, containerType, c.tid)
		}
	}
}

// deleteKeywordFromBoth deletes a keyword from both server and client, including task_keyword relationships
func (a *App) deleteKeywordFromBoth(c Container, isServerDeleted bool, lg io.Writer) {
	if !isServerDeleted {
		_, err := a.Database.PG.Exec("DELETE FROM task_keyword WHERE keyword_tid=$1;", c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting from task_keyword server keyword_tid: %d: %v\n", c.tid, err)
		}
	}

	_, err := a.Database.MainDB.Exec("DELETE FROM task_keyword WHERE keyword_tid=?;", c.tid)
	if err != nil {
		fmt.Fprintf(lg, "Error deleting from task_keyword client keyword_tid: %d: %v\n", c.tid, err)
	}

	if isServerDeleted {
		// Delete from client only
		_, err = a.Database.MainDB.Exec("DELETE FROM keyword WHERE tid=?", c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting client keyword with tid = %d: %v\n", c.tid, err)
		} else {
			fmt.Fprintf(lg, "Deleted client keyword %q with tid %d\n", truncate(c.title, 15), c.tid)
		}
	} else {
		// Mark as deleted on server, delete from client
		_, err = a.Database.PG.Exec("UPDATE keyword SET deleted=true WHERE tid=$1", c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error setting server keyword %q with tid %d to deleted: %v\n", c.title, c.tid, err)
		}

		_, err = a.Database.MainDB.Exec("DELETE FROM keyword WHERE id=?", c.id)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting client keyword %q with id %d: %v\n", c.title, c.id, err)
		} else {
			fmt.Fprintf(lg, "Deleted client keyword %q: id %d and updated server keyword with tid %d to deleted = true\n", c.title, c.id, c.tid)
		}
	}
}

// Synchronize synchronizes data between local and remote databases
// reportOnly: if true, only reports changes without applying them
func (a *App) Synchronize(reportOnly bool) (log string) {
	if a.SyncInProcess {
		return "Synchronization already in process"
	}

	a.SyncInProcess = true
	defer func() { a.SyncInProcess = false }()

	var lg strings.Builder
	var success bool
	defer func() {
		partialHost := "..." + strings.SplitAfterN(a.Config.Postgres.Host, ".", 3)[2]
		text := fmt.Sprintf("server %s (%s)\n\n%s", a.Config.Postgres.DB, partialHost, lg.String())
		if reportOnly {
			log = fmt.Sprintf("### Testing without syncing: %s", text)
			return
		}
		if success {
			log = fmt.Sprintf("### Synchronization succeeded: %s", text)
		} else {
			log = fmt.Sprintf("### Synchronization failed: %s", text)
		}
	}()

	// Get sync timestamps
	row := a.Database.MainDB.QueryRow("SELECT timestamp FROM sync WHERE machine=$1;", "client")
	var rawClientTime string
	err := row.Scan(&rawClientTime)
	if err != nil {
		fmt.Fprintf(&lg, "Error retrieving last client sync: %v", err)
		return
	}
	clientTime := rawClientTime[0:10] + " " + rawClientTime[11:19]

	var serverTime string
	row = a.Database.MainDB.QueryRow("SELECT timestamp FROM sync WHERE machine=$1;", "server")
	err = row.Scan(&serverTime)
	if err != nil {
		fmt.Fprintf(&lg, "Error retrieving last server sync: %v", err)
		return
	}

	fmt.Fprintf(&lg, "Local time is %v\n", time.Now())
	fmt.Fprintf(&lg, "UTC time is %v\n", time.Now().UTC())
	fmt.Fprintf(&lg, "Server last sync: %v\n", serverTime)
	fmt.Fprintf(&lg, "(raw) Client last sync: %v\n", rawClientTime)
	fmt.Fprintf(&lg, "Client last sync: %v\n", clientTime)

	// Fetch all changes
	changes, err := a.fetchAllChanges(serverTime, clientTime, &lg)
	if err != nil {
		fmt.Fprintf(&lg, "Error fetching changes: %v", err)
		return
	}

	// Report changes
	totalChanges := changes.reportChanges(&lg)
	fmt.Fprintf(&lg, "\nNumber of changes (before accounting for server/client conflicts) is: **%d**\n\n", totalChanges)

	if reportOnly {
		return
	}

	/**************** Apply changes *****************/

	// Sync containers: server -> client
	a.syncContainersToClient(containerTypeContext, changes.serverUpdatedContexts, &lg)
	a.syncContainersToClient(containerTypeFolder, changes.serverUpdatedFolders, &lg)
	a.syncContainersToClient(containerTypeKeyword, changes.serverUpdatedKeywords, &lg)

	// Sync entries: server -> client
	serverUpdatedTids := a.syncEntriesToClient(changes.serverUpdatedEntries, &lg)

	// Sync containers: client -> server
	a.syncContainersToServer(containerTypeContext, changes.clientUpdatedContexts, &lg)
	a.syncContainersToServer(containerTypeFolder, changes.clientUpdatedFolders, &lg)
	a.syncContainersToServer(containerTypeKeyword, changes.clientUpdatedKeywords, &lg)

	// Sync entries: client -> server
	a.syncEntriesToServer(changes.clientUpdatedEntries, serverUpdatedTids, &lg)

	// Delete entries
	a.deleteServerEntriesFromClient(changes.serverDeletedEntries, &lg)
	a.deleteClientEntriesFromServer(changes.clientDeletedEntries, &lg)

	// Delete containers
	for _, c := range changes.serverDeletedContexts {
		a.deleteContainerFromBoth(containerTypeContext, c, true, "context_tid", &lg)
	}
	for _, c := range changes.clientDeletedContexts {
		a.deleteContainerFromBoth(containerTypeContext, c, false, "context_tid", &lg)
	}

	for _, c := range changes.serverDeletedFolders {
		a.deleteContainerFromBoth(containerTypeFolder, c, true, "folder_tid", &lg)
	}
	for _, c := range changes.clientDeletedFolders {
		a.deleteContainerFromBoth(containerTypeFolder, c, false, "folder_tid", &lg)
	}

	for _, c := range changes.serverDeletedKeywords {
		a.deleteKeywordFromBoth(c, true, &lg)
	}
	for _, c := range changes.clientDeletedKeywords {
		a.deleteKeywordFromBoth(c, false, &lg)
	}

	// Update sync timestamps
	var serverTS string
	row = a.Database.PG.QueryRow("SELECT now();")
	err = row.Scan(&serverTS)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error with getting current time from server: %v", err)
		return
	}
	_, err = a.Database.MainDB.Exec("UPDATE sync SET timestamp=$1 WHERE machine='server';", serverTS)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error updating client with server timestamp: %v", err)
		return
	}
	_, err = a.Database.MainDB.Exec("UPDATE sync SET timestamp=datetime('now') WHERE machine='client';")
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error updating client with client timestamp: %v", err)
		return
	}
	var clientTS string
	row = a.Database.MainDB.QueryRow("SELECT datetime('now');")
	err = row.Scan(&clientTS)
	if err != nil {
		app.Organizer.ShowMessage(BL, "Error with getting current time from client: %v", err)
		return
	}
	fmt.Fprintf(&lg, "\nClient UTC timestamp: %s\n", clientTS)
	fmt.Fprintf(&lg, "Server UTC timestamp: %s", strings.Replace(tc(serverTS, 19, false), "T", " ", 1))
	success = true

	return
}
