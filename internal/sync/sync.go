// Package sync implements bidirectional synchronization between a local
// SQLite database and a remote Postgres database.
//
// It is consumed by the terminal app (~/vimango) via a thin (*App) wrapper
// and by the standalone vimango-sync CLI under cmd/vimango-sync. Both
// callers construct a *Syncer with their DB handles and invoke
// Synchronize.
package sync

/** note that sqlite datetime('now') returns utc **/

import (
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Constants for default container IDs
const (
	DefaultContainerID = 1 // Represents "none" for context/folder
	// Sentinel uuids of the "none" context/folder rows (tid 1). Task rows
	// reference containers by uuid; the deprecated *_tid columns go stale.
	DefaultContextUUID = "00000000-0000-0000-0000-000000000001"
	DefaultFolderUUID  = "00000000-0000-0000-0000-000000000002"
)

// Syncer holds DB connections and Postgres metadata used to format the
// final log header. Construct via New.
type Syncer struct {
	MainDB *sql.DB
	FtsDB  *sql.DB
	PG     *sql.DB
	PGHost string
	PGDB   string
}

// New creates a Syncer.
func New(mainDB, ftsDB, pg *sql.DB, pgHost, pgDB string) *Syncer {
	return &Syncer{
		MainDB: mainDB,
		FtsDB:  ftsDB,
		PG:     pg,
		PGHost: pgHost,
		PGDB:   pgDB,
	}
}

// ---------------------------------------------------------------------------
// Internal types — these mirror the subset of common.go used by sync. They
// are kept package-private so the sync package is self-contained and does
// not require a shared types package.

// container is the in-flight representation of a context/folder/keyword.
type container struct {
	id       int
	tid      int
	uuid     string
	title    string
	star     bool
	deleted  bool
	modified string
	count    int
}

// entry is the in-flight representation of a task row used for deletes.
type entry struct {
	id           int
	tid          int
	title        string
	folder_tid   int
	context_tid  int
	folder_uuid  string
	context_uuid string
	star         bool
	note         sql.NullString
	added        sql.NullString
	completed    sql.NullString
	deleted      bool
	modified     string
}

// newEntry is the in-flight representation of a task row used for upserts.
type newEntry struct {
	id           int
	tid          int
	title        string
	folder_tid   int
	context_tid  int
	folder_uuid  string
	context_uuid string
	star         bool
	note         sql.NullString
	added        string
	archived     bool
	deleted      bool
	modified     string
}

// EntryPlusTag represents an entry with associated tag information
type EntryPlusTag struct {
	newEntry
	tag sql.NullString
}

// TaskKeywordPairs represents the relationship between tasks and keywords
type TaskKeywordPairs struct {
	taskTid     int
	keywordTid  int
	keywordUUID string
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

// KeywordTidUUID holds both tid and uuid for a keyword
type KeywordTidUUID struct {
	tid  int
	uuid string
}

// syncChanges holds all changes detected during sync
type syncChanges struct {
	serverUpdatedContexts []container
	serverDeletedContexts []container
	serverUpdatedFolders  []container
	serverDeletedFolders  []container
	serverUpdatedKeywords []container
	serverDeletedKeywords []container
	serverUpdatedEntries  []EntryPlusTag
	serverDeletedEntries  []entry
	clientUpdatedContexts []container
	clientDeletedContexts []container
	clientUpdatedFolders  []container
	clientDeletedFolders  []container
	clientUpdatedKeywords []container
	clientDeletedKeywords []container
	clientUpdatedEntries  []newEntry
	clientDeletedEntries  []entry
}

// containerType represents the type of container (context, folder, keyword)
type containerType string

const (
	containerTypeContext containerType = "context"
	containerTypeFolder  containerType = "folder"
	containerTypeKeyword containerType = "keyword"
)

// truncate / tc are the same string-shortening helpers as in common.go.
// Duplicated here so the sync package has no main-package import.
func truncate(s string, length int) string {
	if len(s) > length {
		return s[:length] + "..."
	}
	return s
}

func tc(s string, l int, b bool) string {
	if len(s) > l {
		e := ""
		if b {
			e = "..."
		}
		return s[:l] + e
	}
	return s
}

// ---------------------------------------------------------------------------

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
	args = make([]interface{}, n*3)
	pos := 0
	for i, e := range tk {
		values[i] = "(?, ?, ?)"
		args[pos] = e.taskTid
		args[pos+1] = e.keywordTid
		args[pos+2] = e.keywordUUID
		pos += 3
	}
	query = fmt.Sprintf("INSERT INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES %s", strings.Join(values, ", "))
	return
}

func getTaskKeywordPairsPQ(dbase *sql.DB, tids []int, plg io.Writer) []TaskKeywordPairs {
	rows, err := dbase.Query("SELECT task_tid, keyword_tid, keyword_uuid FROM task_keyword WHERE task_tid = ANY($1);", pq.Array(tids))
	if err != nil {
		fmt.Fprintf(plg, "Error in getTaskKeywordPairsPQ: %v\n", err)
		return []TaskKeywordPairs{}
	}
	tkPairs := make([]TaskKeywordPairs, 0)
	for rows.Next() {
		var tk TaskKeywordPairs
		var keywordUUID sql.NullString
		rows.Scan(
			&tk.taskTid,
			&tk.keywordTid,
			&keywordUUID,
		)
		tk.keywordUUID = keywordUUID.String
		tkPairs = append(tkPairs, tk)
	}
	return tkPairs
}

// taskKeywordTidsAndUUIDs returns both tid and uuid for keywords associated with a task.
func taskKeywordTidsAndUUIDs(dbase *sql.DB, plg io.Writer, taskTid int) []KeywordTidUUID {
	rows, err := dbase.Query("SELECT keyword.tid, keyword.uuid FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.tid=task_keyword.keyword_tid WHERE task_keyword.task_tid=?;", taskTid)
	if err != nil {
		fmt.Fprintf(plg, "Error in taskKeywordTidsAndUUIDs: %v\n", err)
		return []KeywordTidUUID{}
	}
	defer rows.Close()

	result := []KeywordTidUUID{}
	for rows.Next() {
		var kw KeywordTidUUID
		var tid sql.NullInt64
		var uuid sql.NullString
		err = rows.Scan(&tid, &uuid)
		if err == nil {
			kw.tid = int(tid.Int64)
			kw.uuid = uuid.String
			result = append(result, kw)
		}
	}
	return result
}

func insertTaskKeywordTids(dbase *sql.DB, plg io.Writer, keywordTid, entryTid int, keywordUUID string) {
	_, err := dbase.Exec("INSERT INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES ($1, $2, $3);",
		entryTid, keywordTid, keywordUUID)
	if err != nil {
		fmt.Fprintf(plg, "Error in insertTaskKeywordTids: %v\n", err)
		return
	}
	fmt.Fprintf(plg, "Inserted into task_keyword entry tid **%d**, keyword_tid **%d**, keyword_uuid %s\n", entryTid, keywordTid, keywordUUID)
}

func getTagsPQ(dbase *sql.DB, tids []int, plg io.Writer) []TaskTag {
	rows, err := dbase.Query("SELECT task_keyword.task_tid, keyword.title FROM task_keyword LEFT OUTER JOIN keyword ON keyword.tid=task_keyword.keyword_tid WHERE task_keyword.task_tid = ANY($1) ORDER BY task_keyword.task_tid;", pq.Array(tids))
	if err != nil {
		fmt.Fprintf(plg, "Error in getTagsPQ: %v\n", err)
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
	tt.taskTid = tid
	tt.tag.String = strings.Join(keywords, ",")
	tt.tag.Valid = true
	tasktags = append(tasktags, tt)

	return tasktags
}

func getTagSQ(dbase *sql.DB, tid int, plg io.Writer) string {
	rows, err := dbase.Query("SELECT keyword.title FROM task_keyword LEFT OUTER JOIN keyword ON keyword.tid=task_keyword.keyword_tid WHERE task_keyword.task_tid = ?;", tid)
	if err != nil {
		fmt.Fprintf(plg, "Error in getTagSQ: %v\n", err)
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

// ---------------------------------------------------------------------------
// Methods on *Syncer.

// fetchServerContainers fetches updated or deleted containers from server
func (s *Syncer) fetchServerContainers(ct containerType, serverTime string, deleted bool, lg io.Writer) ([]container, error) {
	query := fmt.Sprintf("SELECT tid, uuid, title, star, modified FROM %s WHERE modified > $1 AND deleted = $2;", ct)
	if deleted {
		query = fmt.Sprintf("SELECT tid, uuid, title FROM %s WHERE modified > $1 AND deleted = $2;", ct)
	}

	rows, err := s.PG.Query(query, serverTime, deleted)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for server_%s: %v", ct, err)
	}
	defer rows.Close()

	var containers []container
	for rows.Next() {
		var c container
		var uuid sql.NullString
		if deleted {
			rows.Scan(&c.tid, &uuid, &c.title)
		} else {
			rows.Scan(&c.tid, &uuid, &c.title, &c.star, &c.modified)
		}
		c.uuid = uuid.String
		containers = append(containers, c)
	}
	return containers, nil
}

// fetchClientContainers fetches updated or deleted containers from client
func (s *Syncer) fetchClientContainers(ct containerType, clientTime string, deleted bool, lg io.Writer) ([]container, error) {
	query := fmt.Sprintf("SELECT id, tid, uuid, title, star, modified FROM %s WHERE substr(modified, 1, 19) > $1 AND deleted = $2;", ct)
	if deleted {
		query = fmt.Sprintf("SELECT id, tid, uuid, title FROM %s WHERE substr(modified, 1, 19) > $1 AND deleted = $2;", ct)
	}

	rows, err := s.MainDB.Query(query, clientTime, deleted)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for client_%s: %v", ct, err)
	}
	defer rows.Close()

	var containers []container
	for rows.Next() {
		var c container
		var tid sql.NullInt64
		if deleted {
			rows.Scan(&c.id, &tid, &c.uuid, &c.title)
		} else {
			rows.Scan(&c.id, &tid, &c.uuid, &c.title, &c.star, &c.modified)
		}
		c.tid = int(tid.Int64)
		containers = append(containers, c)
	}
	return containers, nil
}

// fetchAllChanges retrieves all changes from both server and client
func (s *Syncer) fetchAllChanges(serverTime, clientTime string, lg io.Writer) (*syncChanges, error) {
	changes := &syncChanges{}
	var err error

	// Fetch server changes
	changes.serverUpdatedContexts, err = s.fetchServerContainers(containerTypeContext, serverTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.serverDeletedContexts, err = s.fetchServerContainers(containerTypeContext, serverTime, true, lg)
	if err != nil {
		return nil, err
	}

	changes.serverUpdatedFolders, err = s.fetchServerContainers(containerTypeFolder, serverTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.serverDeletedFolders, err = s.fetchServerContainers(containerTypeFolder, serverTime, true, lg)
	if err != nil {
		return nil, err
	}

	changes.serverUpdatedKeywords, err = s.fetchServerContainers(containerTypeKeyword, serverTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.serverDeletedKeywords, err = s.fetchServerContainers(containerTypeKeyword, serverTime, true, lg)
	if err != nil {
		return nil, err
	}

	// Fetch server entries
	rows, err := s.PG.Query("SELECT tid, title, star, note, modified, added, archived, context_tid, folder_tid, context_uuid, folder_uuid FROM task WHERE modified > $1 AND deleted = $2 ORDER BY tid;", serverTime, false)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for server_updated_entries: %v", err)
	}
	for rows.Next() {
		var e EntryPlusTag
		var contextUUID, folderUUID sql.NullString
		rows.Scan(&e.tid, &e.title, &e.star, &e.note, &e.modified, &e.added, &e.archived, &e.context_tid, &e.folder_tid, &contextUUID, &folderUUID)
		e.context_uuid = contextUUID.String
		e.folder_uuid = folderUUID.String
		changes.serverUpdatedEntries = append(changes.serverUpdatedEntries, e)
	}
	rows.Close()

	rows, err = s.PG.Query("SELECT tid, title FROM task WHERE modified > $1 AND deleted = $2;", serverTime, true)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for server_deleted_entries: %v", err)
	}
	for rows.Next() {
		var e entry
		rows.Scan(&e.tid, &e.title)
		changes.serverDeletedEntries = append(changes.serverDeletedEntries, e)
	}
	rows.Close()

	// Fetch client changes
	changes.clientUpdatedContexts, err = s.fetchClientContainers(containerTypeContext, clientTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.clientDeletedContexts, err = s.fetchClientContainers(containerTypeContext, clientTime, true, lg)
	if err != nil {
		return nil, err
	}

	changes.clientUpdatedFolders, err = s.fetchClientContainers(containerTypeFolder, clientTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.clientDeletedFolders, err = s.fetchClientContainers(containerTypeFolder, clientTime, true, lg)
	if err != nil {
		return nil, err
	}

	changes.clientUpdatedKeywords, err = s.fetchClientContainers(containerTypeKeyword, clientTime, false, lg)
	if err != nil {
		return nil, err
	}
	changes.clientDeletedKeywords, err = s.fetchClientContainers(containerTypeKeyword, clientTime, true, lg)
	if err != nil {
		return nil, err
	}

	// Fetch client entries
	rows, err = s.MainDB.Query("SELECT id, tid, title, star, note, modified, added, archived, context_tid, folder_tid, context_uuid, folder_uuid FROM task WHERE substr(modified, 1, 19)  > ? AND deleted = ?;", clientTime, false)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for client_updated_entries: %v", err)
	}
	for rows.Next() {
		var e newEntry
		var tid sql.NullInt64
		rows.Scan(&e.id, &tid, &e.title, &e.star, &e.note, &e.modified, &e.added, &e.archived, &e.context_tid, &e.folder_tid, &e.context_uuid, &e.folder_uuid)
		e.tid = int(tid.Int64)
		changes.clientUpdatedEntries = append(changes.clientUpdatedEntries, e)
	}
	rows.Close()

	rows, err = s.MainDB.Query("SELECT id, tid, title FROM task WHERE substr(modified, 1, 19) > $1 AND deleted = $2;", clientTime, true)
	if err != nil {
		return nil, fmt.Errorf("Error with retrieving client deleted entries: %v", err)
	}
	for rows.Next() {
		var e entry
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
		fmt.Fprint(lg, "- No `Contexts` updated (new and modified).\n")
	}

	if len(changes.serverDeletedContexts) > 0 {
		totalChanges += len(changes.serverDeletedContexts)
		fmt.Fprintf(lg, "- Deleted `Contexts`: %d\n", len(changes.serverDeletedContexts))
	} else {
		fmt.Fprint(lg, "- No `Contexts` deleted.\n")
	}

	if len(changes.serverUpdatedFolders) > 0 {
		totalChanges += len(changes.serverUpdatedFolders)
		fmt.Fprintf(lg, "- `Folders` Updated: %d\n", len(changes.serverUpdatedFolders))
	} else {
		fmt.Fprint(lg, "- No `Folders` updated.\n")
	}

	if len(changes.serverDeletedFolders) > 0 {
		totalChanges += len(changes.serverDeletedFolders)
		fmt.Fprintf(lg, "- Deleted `Folders`: %d\n", len(changes.serverDeletedFolders))
	} else {
		fmt.Fprint(lg, "- No `Folders` deleted.\n")
	}

	if len(changes.serverUpdatedKeywords) > 0 {
		totalChanges += len(changes.serverUpdatedKeywords)
		fmt.Fprintf(lg, "- Updated `Keywords`: %d\n", len(changes.serverUpdatedKeywords))
	} else {
		fmt.Fprint(lg, "- No `Keywords` updated.\n")
	}

	if len(changes.serverDeletedKeywords) > 0 {
		totalChanges += len(changes.serverDeletedKeywords)
		fmt.Fprintf(lg, "- Deleted server `Keywords`: %d\n", len(changes.serverDeletedKeywords))
	} else {
		fmt.Fprint(lg, "- No `Keywords` deleted.\n")
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
		fmt.Fprint(lg, "- No `Entries` updated.\n")
	}

	if len(changes.serverDeletedEntries) > 0 {
		totalChanges += len(changes.serverDeletedEntries)
		fmt.Fprintf(lg, "- Deleted `Entries`: %d\n", len(changes.serverDeletedEntries))
	} else {
		fmt.Fprint(lg, "- No `Entries` deleted.\n")
	}

	fmt.Fprint(lg, "## Client Changes\n")

	if len(changes.clientUpdatedContexts) > 0 {
		totalChanges += len(changes.clientUpdatedContexts)
		fmt.Fprintf(lg, "- `Contexts` updated: %d\n", len(changes.clientUpdatedContexts))
		for _, c := range changes.clientUpdatedContexts {
			fmt.Fprintf(lg, "    - id: %d; tid: %d %q; modified: %v\n", c.id, c.tid, tc(c.title, 15, true), tc(c.modified, 19, false))
		}
	} else {
		fmt.Fprint(lg, "- No `Contexts` updated.\n")
	}

	if len(changes.clientDeletedContexts) > 0 {
		totalChanges += len(changes.clientDeletedContexts)
		fmt.Fprintf(lg, "- Deleted client `Contexts`: %d\n", len(changes.clientDeletedContexts))
		for _, e := range changes.clientDeletedContexts {
			fmt.Fprintf(lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		fmt.Fprint(lg, "- No `Contexts` deleted.\n")
	}

	if len(changes.clientUpdatedFolders) > 0 {
		totalChanges += len(changes.clientUpdatedFolders)
		fmt.Fprintf(lg, "- Updated `Folders`: %d\n", len(changes.clientUpdatedFolders))
		for _, c := range changes.clientUpdatedFolders {
			fmt.Fprintf(lg, "    - id: %d; tid: %d %q; modified: %v\n", c.id, c.tid, tc(c.title, 15, true), tc(c.modified, 19, false))
		}
	} else {
		fmt.Fprint(lg, "- No `Folders` updated.\n")
	}

	if len(changes.clientDeletedFolders) > 0 {
		totalChanges += len(changes.clientDeletedFolders)
		fmt.Fprintf(lg, "- Deleted client `Folders`: %d\n", len(changes.clientDeletedFolders))
		for _, e := range changes.clientDeletedFolders {
			fmt.Fprintf(lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		fmt.Fprint(lg, "- No `Folders` deleted.\n")
	}

	if len(changes.clientUpdatedKeywords) > 0 {
		totalChanges += len(changes.clientUpdatedKeywords)
		fmt.Fprintf(lg, "- Updated `Keywords`: %d\n", len(changes.clientUpdatedKeywords))
		for _, c := range changes.clientUpdatedKeywords {
			fmt.Fprintf(lg, "    - id: %d; tid: %d %q; modified: %v\n", c.id, c.tid, tc(c.title, 15, true), tc(c.modified, 19, false))
		}
	} else {
		fmt.Fprint(lg, "- No `Keywords` updated.\n")
	}

	if len(changes.clientDeletedKeywords) > 0 {
		totalChanges += len(changes.clientDeletedKeywords)
		fmt.Fprintf(lg, "- Deleted `Keywords`: %d\n", len(changes.clientDeletedKeywords))
		for _, e := range changes.clientDeletedKeywords {
			fmt.Fprintf(lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		fmt.Fprint(lg, "- No `Keywords` deleted.\n")
	}

	if len(changes.clientUpdatedEntries) > 0 {
		totalChanges += len(changes.clientUpdatedEntries)
		fmt.Fprintf(lg, "- Updated `Entries`: %d\n", len(changes.clientUpdatedEntries))
		for _, e := range changes.clientUpdatedEntries {
			fmt.Fprintf(lg, "    - id: %d tid: %d star: %t *%q* context_tid: %d folder_tid: %d  modified: %v\n", e.id, e.tid, e.star, truncate(e.title, 15), e.context_tid, e.folder_tid, tc(e.modified, 19, false))
		}
	} else {
		fmt.Fprint(lg, "- No `Entries` updated.\n")
	}

	if len(changes.clientDeletedEntries) > 0 {
		totalChanges += len(changes.clientDeletedEntries)
		fmt.Fprintf(lg, "- Deleted `Entries`: %d\n", len(changes.clientDeletedEntries))
		for _, e := range changes.clientDeletedEntries {
			fmt.Fprintf(lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		fmt.Fprint(lg, "- No `Entries` deleted.\n")
	}

	return totalChanges
}

// syncContainersToClient syncs updated containers from server to client
func (s *Syncer) syncContainersToClient(ct containerType, containers []container, lg io.Writer) {
	for _, c := range containers {
		var exists bool
		query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE tid=?)", ct)
		err := s.MainDB.QueryRow(query, c.tid).Scan(&exists)
		if err != nil {
			fmt.Fprintf(lg, "Error SELECT EXISTS for %s: %v\n", ct, err)
			continue
		}

		if exists {
			query = fmt.Sprintf("UPDATE %s SET title=?, star=?, uuid=?, modified=datetime('now') WHERE tid=?;", ct)
			_, err := s.MainDB.Exec(query, c.title, c.star, c.uuid, c.tid)
			if err != nil {
				fmt.Fprintf(lg, "Error updating sqlite for %s with tid: %v: %v\n", ct, c.tid, err)
			} else {
				fmt.Fprintf(lg, "Updated local %s: %q with tid: %v uuid: %s\n", ct, c.title, c.tid, c.uuid)
			}
		} else {
			query = fmt.Sprintf("INSERT INTO %s (tid, uuid, title, star, modified, deleted) VALUES (?,?,?,?, datetime('now'), false);", ct)
			_, err := s.MainDB.Exec(query, c.tid, c.uuid, c.title, c.star)
			if err != nil {
				fmt.Fprintf(lg, "Error inserting new %s into sqlite: %v\n", ct, err)
			} else {
				fmt.Fprintf(lg, "Inserted local %s: %q with tid: %v uuid: %s\n", ct, c.title, c.tid, c.uuid)
			}
		}
	}
}

// syncContainersToServer syncs updated containers from client to server
func (s *Syncer) syncContainersToServer(ct containerType, containers []container, lg io.Writer) {
	for _, c := range containers {
		var exists bool
		query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE tid=$1);", ct)
		err := s.PG.QueryRow(query, c.tid).Scan(&exists)
		if err != nil {
			fmt.Fprintf(lg, "Error SELECT EXISTS for %s: %v\n", ct, err)
			continue
		}

		if exists {
			query = fmt.Sprintf("UPDATE %s SET title=$1, star=$2, uuid=$3, modified=now() WHERE tid=$4;", ct)
			_, err := s.PG.Exec(query, c.title, c.star, c.uuid, c.tid)
			if err != nil {
				fmt.Fprintf(lg, "Error updating postgres for %s with tid: %d: %v\n", ct, c.tid, err)
			} else {
				fmt.Fprintf(lg, "Updated server %s: %q with tid: %v uuid: %s\n", ct, c.title, c.tid, c.uuid)
			}
		} else {
			var tid int
			query = fmt.Sprintf("INSERT INTO %s (title, star, uuid, modified, deleted) VALUES ($1, $2, $3, now(), false) RETURNING tid;", ct)
			err := s.PG.QueryRow(query, c.title, c.star, c.uuid).Scan(&tid)
			if err != nil {
				fmt.Fprintf(lg, "Error inserting new %s into postgres and returning tid: %v\n", ct, err)
				continue
			}
			query = fmt.Sprintf("UPDATE %s SET tid=? WHERE id=?;", ct)
			_, err = s.MainDB.Exec(query, tid, c.id)
			if err != nil {
				fmt.Fprintf(lg, "Error on UPDATE %s SET tid ...: %v\n", ct, err)
			} else {
				fmt.Fprintf(lg, "Inserted server %s %q with uuid: %s and updated local tid to %d\n", ct, c.title, c.uuid, tid)
			}
		}
	}
}

// syncEntriesToClient syncs updated entries from server to client (including FTS and keywords)
func (s *Syncer) syncEntriesToClient(entries []EntryPlusTag, lg io.Writer) map[int]struct{} {
	updatedTids := make(map[int]struct{})
	var tids []int

	for _, e := range entries {
		_, err := s.MainDB.Exec("INSERT INTO task (tid, title, star, added, archived, context_tid, folder_tid, context_uuid, folder_uuid, note, modified, deleted) VALUES"+
			"(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), false) ON CONFLICT(tid) DO UPDATE SET "+
			"title=excluded.title, star=excluded.star, archived=excluded.archived, context_tid=excluded.context_tid, "+
			"folder_tid=excluded.folder_tid, context_uuid=excluded.context_uuid, folder_uuid=excluded.folder_uuid, "+
			"note=excluded.note, modified=datetime('now');",
			e.tid, e.title, e.star, e.added, e.archived, e.context_tid, e.folder_tid, e.context_uuid, e.folder_uuid, e.note)
		if err != nil {
			fmt.Fprintf(lg, "**Error** in INSERT ... ON CONFLICT for tid %d %q: %v\n", e.tid, e.title, err)
			continue
		}
		fmt.Fprintf(lg, "Inserted or updated client entry %q with tid **%d** context_uuid: %s folder_uuid: %s\n", e.title, e.tid, e.context_uuid, e.folder_uuid)
		tids = append(tids, e.tid)
		updatedTids[e.tid] = struct{}{}
	}

	if len(entries) == 0 {
		return updatedTids
	}

	// Delete existing keywords and FTS entries for updated tasks
	in := "?" + strings.Repeat(",?", len(tids)-1)
	stmt := fmt.Sprintf("DELETE FROM task_keyword WHERE task_tid IN (%s);", in)
	tidsIf := make([]interface{}, len(tids))
	for i := range tids {
		tidsIf[i] = tids[i]
	}
	_, err := s.MainDB.Exec(stmt, tidsIf...)
	if err != nil {
		fmt.Fprintf(lg, "Error deleting from client task_keyword for tids: %v: %v\n", tids, err)
	}

	stmt = fmt.Sprintf("DELETE FROM fts WHERE tid IN (%s);", in)
	_, err = s.FtsDB.Exec(stmt, tidsIf...)
	if err != nil {
		fmt.Fprintf(lg, "Error deleting from fts for tids: %v: %v\n", tids, err)
	}

	// Update keywords
	tks := getTaskKeywordPairsPQ(s.PG, tids, lg)
	if len(tks) != 0 {
		query, args := createBulkInsertQueryTaskKeywordPairs(len(tks), tks)
		err = bulkInsert(s.MainDB, query, args)
		if err != nil {
			fmt.Fprintf(lg, "%v\n", err)
		} else {
			fmt.Fprintf(lg, "Keywords updated for task tids: %v\n", tids)
		}

		tagMap := make(map[int]sql.NullString)
		for _, tag := range getTagsPQ(s.PG, tids, lg) {
			tagMap[tag.taskTid] = tag.tag
		}
		for i := range entries {
			if tag, ok := tagMap[entries[i].tid]; ok {
				entries[i].tag = tag
				fmt.Fprintf(lg, "FTS tag will be updated for tid: %d, tag: %s\n", entries[i].tid, tag.String)
			}
		}
	}

	query, args := createBulkInsertQueryFTS3(len(entries), entries)
	err = bulkInsert(s.FtsDB, query, args)
	if err != nil {
		fmt.Fprintf(lg, "%v", err)
	} else {
		fmt.Fprintf(lg, "FTS entries updated for task tids: %v\n", tids)
	}

	return updatedTids
}

// syncEntriesToServer syncs updated entries from client to server
func (s *Syncer) syncEntriesToServer(entries []newEntry, serverUpdatedTids map[int]struct{}, lg io.Writer) {
	for _, e := range entries {
		// Server wins if both client and server have updated an item
		if _, found := serverUpdatedTids[e.tid]; found {
			fmt.Fprintf(lg, "Server won: client entry %q with id %d and tid %d was updated by server\n", truncate(e.title, 15), e.id, e.tid)
			continue
		}

		// Resolve context_tid and folder_tid from uuid if needed
		// (In local-only mode, tid might be 0 but uuid is set)
		contextTid := e.context_tid
		folderTid := e.folder_tid

		if contextTid < 1 && e.context_uuid != "" {
			var tid sql.NullInt64
			err := s.MainDB.QueryRow("SELECT tid FROM context WHERE uuid = ?", e.context_uuid).Scan(&tid)
			if err == nil && tid.Valid {
				contextTid = int(tid.Int64)
			} else {
				contextTid = DefaultContainerID
			}
		}

		if folderTid < 1 && e.folder_uuid != "" {
			var tid sql.NullInt64
			err := s.MainDB.QueryRow("SELECT tid FROM folder WHERE uuid = ?", e.folder_uuid).Scan(&tid)
			if err == nil && tid.Valid {
				folderTid = int(tid.Int64)
			} else {
				folderTid = DefaultContainerID
			}
		}

		var tid int
		if e.tid < 1 {
			err := s.PG.QueryRow("INSERT INTO task (title, star, added, archived, context_tid, folder_tid, context_uuid, folder_uuid, note, modified, deleted) "+
				"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), false) RETURNING tid",
				e.title, e.star, e.added, e.archived, contextTid, folderTid, e.context_uuid, e.folder_uuid, e.note).Scan(&tid)
			if err != nil {
				fmt.Fprintf(lg, "Error inserting server entry: %v", err)
				continue
			}
			_, err = s.MainDB.Exec("UPDATE task SET tid=? WHERE id=?;", tid, e.id)
			if err != nil {
				fmt.Fprintf(lg, "Error setting tid for client entry %q with id %d to tid %d: %v\n", truncate(e.title, 15), e.id, tid, err)
				continue
			}

			// Create FTS entry for new entries
			taskTag := getTagSQ(s.MainDB, tid, lg)
			var tag sql.NullString
			if len(taskTag) > 0 {
				tag.String = taskTag
				tag.Valid = true
			}
			_, err = s.FtsDB.Exec("INSERT INTO fts (title, tag, note, tid) VALUES (?, ?, ?, ?);", e.title, tag, e.note, tid)
			if err != nil {
				fmt.Fprintf(lg, "Error in INSERT INTO fts: %v\n", err)
			}
			fmt.Fprintf(lg, "Created new server entry *%q* with tid **%d** context_uuid: %s folder_uuid: %s\n", truncate(e.title, 15), tid, e.context_uuid, e.folder_uuid)
			fmt.Fprintf(lg, "and set tid for client entry with id **%d** and created fts entry\n", e.id)
		} else {
			_, err := s.PG.Exec("UPDATE task SET title=$1, star=$2, context_tid=$3, folder_tid=$4, context_uuid=$5, folder_uuid=$6, note=$7, archived=$8, modified=now() WHERE tid=$9;",
				e.title, e.star, contextTid, folderTid, e.context_uuid, e.folder_uuid, e.note, e.archived, e.tid)
			if err != nil {
				fmt.Fprintf(lg, "Error updating server entry: %v", err)
				continue
			}
			tid = e.tid
			fmt.Fprintf(lg, "Updated server entry *%q* with tid **%d** context_uuid: %s folder_uuid: %s\n", truncate(e.title, 15), tid, e.context_uuid, e.folder_uuid)
		}

		// Update the server entry's keywords
		_, err := s.PG.Exec("DELETE FROM task_keyword WHERE task_tid=$1;", tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting from task_keyword from server tid %d: %v\n", tid, err)
			continue
		}
		kwPairs := taskKeywordTidsAndUUIDs(s.MainDB, lg, tid)
		for _, kw := range kwPairs {
			insertTaskKeywordTids(s.PG, lg, kw.tid, tid, kw.uuid)
		}
	}
}

// deleteServerEntriesFromClient removes entries deleted on server from client
func (s *Syncer) deleteServerEntriesFromClient(entries []entry, lg io.Writer) {
	for _, e := range entries {
		_, err := s.MainDB.Exec("DELETE FROM task_keyword WHERE task_tid=?;", e.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting task_keyword client rows where entry tid = %d: %v\n", e.tid, err)
			continue
		}

		_, err = s.MainDB.Exec("DELETE FROM task WHERE tid=?;", e.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting client entry %q with tid %d: %v\n", tc(e.title, 15, true), e.tid, err)
			continue
		}
		fmt.Fprintf(lg, "Deleted client entry %q with tid %d\n", truncate(e.title, 15), e.tid)
		fmt.Fprintf(lg, "and on client deleted task_tid %d from task_keyword\n", e.tid)
	}
}

// deleteClientEntriesFromServer marks entries deleted on client as deleted on server
func (s *Syncer) deleteClientEntriesFromServer(entries []entry, lg io.Writer) {
	for _, e := range entries {
		_, err := s.MainDB.Exec("DELETE FROM task_keyword WHERE task_tid=?;", e.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting task_keyword client rows where entry tid = %d: %v\n", e.tid, err)
			continue
		}
		_, err = s.MainDB.Exec("DELETE FROM task WHERE id=?", e.id)
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

		_, err = s.PG.Exec("UPDATE task SET deleted=true, modified=now() WHERE tid=$1", e.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error setting server entry with id %d to deleted: %v\n", e.tid, err)
			continue
		}
		fmt.Fprintf(lg, "Updated server entry %q with id %d to **deleted = true**\n", truncate(e.title, 15), e.tid)

		_, err = s.PG.Exec("DELETE FROM task_keyword WHERE task_tid=$1;", e.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting task_keyword server rows where entry tid = %d: %v\n", e.tid, err)
			continue
		}
		fmt.Fprintf(lg, "and on server deleted task_tid %d from task_keyword\n", e.tid)
	}
}

// deleteContainerFromBoth deletes a container from both server and client, updating task references
func (s *Syncer) deleteContainerFromBoth(ct containerType, c container, isServerDeleted bool, taskField string, lg io.Writer) {
	uuidField := "context_uuid"
	defaultUUID := DefaultContextUUID
	if ct == containerTypeFolder {
		uuidField = "folder_uuid"
		defaultUUID = DefaultFolderUUID
	}

	// The "none" container must never be deleted — every reassignment
	// below lands on it.
	if c.tid == DefaultContainerID || c.uuid == defaultUUID {
		fmt.Fprintf(lg, "Refusing to delete the 'none' %s (tid %d)\n", ct, c.tid)
		return
	}

	// Move the container's tasks to "none" (tid 1) on server and client.
	// Membership is matched by uuid — the authoritative reference; the
	// deprecated *_tid columns go stale (uuid-based reassignments never
	// update them) and would both miss and over-match. Both columns are
	// reset so the tid side can't dangle either. Containers predating the
	// uuid migration fall back to tid matching.
	match := uuidField
	var matchArg interface{} = c.uuid
	if c.uuid == "" {
		match = taskField
		matchArg = c.tid
	}
	query := fmt.Sprintf("UPDATE task SET %s=%d, %s='%s', modified=now() WHERE %s=$1;",
		taskField, DefaultContainerID, uuidField, defaultUUID, match)
	res, err := s.PG.Exec(query, matchArg)
	if err != nil {
		fmt.Fprintf(lg, "Error trying to change server entry %s for a deleted %s: %v\n", taskField, ct, err)
	} else {
		rowsAffected, _ := res.RowsAffected()
		fmt.Fprintf(lg, "The number of server entries that were changed to 'none': **%d**\n", rowsAffected)
	}

	query = fmt.Sprintf("UPDATE task SET %s=%d, %s='%s', modified=datetime('now') WHERE %s=?;",
		taskField, DefaultContainerID, uuidField, defaultUUID, match)
	res, err = s.MainDB.Exec(query, matchArg)
	if err != nil {
		fmt.Fprintf(lg, "Error trying to change client entry %s for a deleted %s: %v\n", taskField, ct, err)
	} else {
		rowsAffected, _ := res.RowsAffected()
		fmt.Fprintf(lg, "The number of client entries that were changed to 'none': **%d**\n", rowsAffected)
	}

	if isServerDeleted {
		// Delete from client only
		query = fmt.Sprintf("DELETE FROM %s WHERE tid=?", ct)
		_, err = s.MainDB.Exec(query, c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting local %s %q with tid = %d: %v\n", ct, c.title, c.tid, err)
		} else {
			fmt.Fprintf(lg, "Deleted client %s %q with tid %d\n", ct, c.title, c.tid)
		}
	} else {
		// Mark as deleted on server, delete from client
		query = fmt.Sprintf("UPDATE %s SET deleted=true, modified=now() WHERE tid=$1", ct)
		_, err = s.PG.Exec(query, c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error setting server %s %q with tid = %d to deleted: %v\n", ct, c.title, c.tid, err)
		}

		query = fmt.Sprintf("DELETE FROM %s WHERE id=?", ct)
		_, err = s.MainDB.Exec(query, c.id)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting local %s %q with id %d: %v\n", ct, c.title, c.id, err)
		} else {
			fmt.Fprintf(lg, "Deleted client %s %q: id %d and updated server %s with tid %d to deleted = true\n", ct, c.title, c.id, ct, c.tid)
		}
	}
}

// deleteKeywordFromBoth deletes a keyword from both server and client, including task_keyword relationships
func (s *Syncer) deleteKeywordFromBoth(c container, isServerDeleted bool, lg io.Writer) {
	if !isServerDeleted {
		_, err := s.PG.Exec("DELETE FROM task_keyword WHERE keyword_tid=$1;", c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting from task_keyword server keyword_tid: %d: %v\n", c.tid, err)
		}
	}

	_, err := s.MainDB.Exec("DELETE FROM task_keyword WHERE keyword_tid=?;", c.tid)
	if err != nil {
		fmt.Fprintf(lg, "Error deleting from task_keyword client keyword_tid: %d: %v\n", c.tid, err)
	}

	if isServerDeleted {
		// Delete from client only
		_, err = s.MainDB.Exec("DELETE FROM keyword WHERE tid=?", c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting client keyword with tid = %d: %v\n", c.tid, err)
		} else {
			fmt.Fprintf(lg, "Deleted client keyword %q with tid %d\n", truncate(c.title, 15), c.tid)
		}
	} else {
		// Mark as deleted on server, delete from client
		_, err = s.PG.Exec("UPDATE keyword SET deleted=true WHERE tid=$1", c.tid)
		if err != nil {
			fmt.Fprintf(lg, "Error setting server keyword %q with tid %d to deleted: %v\n", c.title, c.tid, err)
		}

		_, err = s.MainDB.Exec("DELETE FROM keyword WHERE id=?", c.id)
		if err != nil {
			fmt.Fprintf(lg, "Error deleting client keyword %q with id %d: %v\n", c.title, c.id, err)
		} else {
			fmt.Fprintf(lg, "Deleted client keyword %q: id %d and updated server keyword with tid %d to deleted = true\n", c.title, c.id, c.tid)
		}
	}
}

// Probe reports how many server-side rows have changed since the last
// sync — "is the remote ahead of us?" — without applying anything.
//
// This is deliberately *not* Synchronize(reportOnly=true): that runs the
// whole engine (a dozen-plus round trips, dragging full note bodies over
// the wire just to count them) and reads the client database, which is
// shared with the editor and --render-html. Probe is one SELECT against
// Postgres plus one watermark read from SQLite, so it is cheap enough to
// run unattended.
//
// The watermark is the same one Synchronize uses: the 'server' row of the
// client's sync table. No `deleted` filter — deletions bump `modified`
// too, and fetchAllChanges counts both, so an unfiltered count matches
// what a real sync would find.
//
// Returns the change count. Any error means "unknown", never zero.
func (s *Syncer) Probe() (int, error) {
	var serverTime string
	row := s.MainDB.QueryRow("SELECT timestamp FROM sync WHERE machine=$1;", "server")
	if err := row.Scan(&serverTime); err != nil {
		return 0, fmt.Errorf("retrieving last server sync: %v", err)
	}

	var count int
	row = s.PG.QueryRow(`
		SELECT (SELECT COUNT(*) FROM task    WHERE modified > $1)
		     + (SELECT COUNT(*) FROM context WHERE modified > $1)
		     + (SELECT COUNT(*) FROM folder  WHERE modified > $1)
		     + (SELECT COUNT(*) FROM keyword WHERE modified > $1);`, serverTime)
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("counting server changes since %s: %v", serverTime, err)
	}
	return count, nil
}

// Synchronize runs a sync between client (SQLite) and server (Postgres).
// reportOnly: if true, only reports changes without applying them.
//
// Returns the formatted markdown log and a non-nil error on fatal failure.
// On error, the returned log still contains diagnostic detail.
func (s *Syncer) Synchronize(reportOnly bool) (log string, err error) {
	var lg strings.Builder
	defer func() {
		// Format the final log header.
		var partialHost string
		parts := strings.SplitAfterN(s.PGHost, ".", 3)
		if len(parts) >= 3 {
			partialHost = "..." + parts[2]
		} else {
			partialHost = s.PGHost
		}
		text := fmt.Sprintf("server %s (%s)\n\n%s", s.PGDB, partialHost, lg.String())
		switch {
		case reportOnly:
			log = fmt.Sprintf("### (New) Testing without syncing: %s", text)
		case err == nil:
			log = fmt.Sprintf("### (New) Synchronization succeeded: %s", text)
		default:
			log = fmt.Sprintf("### (New) Synchronization failed: %s", text)
		}
	}()

	// Get sync timestamps
	row := s.MainDB.QueryRow("SELECT timestamp FROM sync WHERE machine=$1;", "client")
	var rawClientTime string
	if err = row.Scan(&rawClientTime); err != nil {
		fmt.Fprintf(&lg, "Error retrieving last client sync: %v", err)
		return
	}
	clientTime := rawClientTime[0:10] + " " + rawClientTime[11:19]

	var serverTime string
	row = s.MainDB.QueryRow("SELECT timestamp FROM sync WHERE machine=$1;", "server")
	if err = row.Scan(&serverTime); err != nil {
		fmt.Fprintf(&lg, "Error retrieving last server sync: %v", err)
		return
	}

	fmt.Fprintf(&lg, "Local time is %v\n", time.Now())
	fmt.Fprintf(&lg, "UTC time is %v\n", time.Now().UTC())
	fmt.Fprintf(&lg, "Server last sync: %v\n", serverTime)
	fmt.Fprintf(&lg, "(raw) Client last sync: %v\n", rawClientTime)
	fmt.Fprintf(&lg, "Client last sync: %v\n", clientTime)

	// Fetch all changes
	changes, ferr := s.fetchAllChanges(serverTime, clientTime, &lg)
	if ferr != nil {
		err = ferr
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
	s.syncContainersToClient(containerTypeContext, changes.serverUpdatedContexts, &lg)
	s.syncContainersToClient(containerTypeFolder, changes.serverUpdatedFolders, &lg)
	s.syncContainersToClient(containerTypeKeyword, changes.serverUpdatedKeywords, &lg)

	// Sync entries: server -> client
	serverUpdatedTids := s.syncEntriesToClient(changes.serverUpdatedEntries, &lg)

	// Sync containers: client -> server
	s.syncContainersToServer(containerTypeContext, changes.clientUpdatedContexts, &lg)
	s.syncContainersToServer(containerTypeFolder, changes.clientUpdatedFolders, &lg)
	s.syncContainersToServer(containerTypeKeyword, changes.clientUpdatedKeywords, &lg)

	// Sync entries: client -> server
	s.syncEntriesToServer(changes.clientUpdatedEntries, serverUpdatedTids, &lg)

	// Delete entries
	s.deleteServerEntriesFromClient(changes.serverDeletedEntries, &lg)
	s.deleteClientEntriesFromServer(changes.clientDeletedEntries, &lg)

	// Delete containers
	for _, c := range changes.serverDeletedContexts {
		s.deleteContainerFromBoth(containerTypeContext, c, true, "context_tid", &lg)
	}
	for _, c := range changes.clientDeletedContexts {
		s.deleteContainerFromBoth(containerTypeContext, c, false, "context_tid", &lg)
	}

	for _, c := range changes.serverDeletedFolders {
		s.deleteContainerFromBoth(containerTypeFolder, c, true, "folder_tid", &lg)
	}
	for _, c := range changes.clientDeletedFolders {
		s.deleteContainerFromBoth(containerTypeFolder, c, false, "folder_tid", &lg)
	}

	for _, c := range changes.serverDeletedKeywords {
		s.deleteKeywordFromBoth(c, true, &lg)
	}
	for _, c := range changes.clientDeletedKeywords {
		s.deleteKeywordFromBoth(c, false, &lg)
	}

	// Update sync timestamps
	var serverTS string
	row = s.PG.QueryRow("SELECT now();")
	if err = row.Scan(&serverTS); err != nil {
		fmt.Fprintf(&lg, "Error with getting current time from server: %v\n", err)
		return
	}
	if _, err = s.MainDB.Exec("UPDATE sync SET timestamp=$1 WHERE machine='server';", serverTS); err != nil {
		fmt.Fprintf(&lg, "Error updating client with server timestamp: %v\n", err)
		return
	}
	if _, err = s.MainDB.Exec("UPDATE sync SET timestamp=datetime('now') WHERE machine='client';"); err != nil {
		fmt.Fprintf(&lg, "Error updating client with client timestamp: %v\n", err)
		return
	}
	var clientTS string
	row = s.MainDB.QueryRow("SELECT datetime('now');")
	if err = row.Scan(&clientTS); err != nil {
		fmt.Fprintf(&lg, "Error with getting current time from client: %v\n", err)
		return
	}
	fmt.Fprintf(&lg, "\nClient UTC timestamp: %s\n", clientTS)
	fmt.Fprintf(&lg, "Server UTC timestamp: %s", strings.Replace(tc(serverTS, 19, false), "T", " ", 1))
	return
}
