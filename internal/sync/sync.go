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
	// The server's modified as this client last saw it (client rows only).
	serverModified sql.NullString
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
	// The server's modified as this client last saw it (client rows only).
	serverModified sql.NullString
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

func insertTaskKeywordTids(dbase *sql.DB, plg io.Writer, keywordTid, entryTid int, keywordUUID string) error {
	_, err := dbase.Exec("INSERT INTO task_keyword (task_tid, keyword_tid, keyword_uuid) VALUES ($1, $2, $3);",
		entryTid, keywordTid, keywordUUID)
	if err != nil {
		return fmt.Errorf("inserting server task_keyword (task %d, keyword %d): %v", entryTid, keywordTid, err)
	}
	fmt.Fprintf(plg, "Inserted into task_keyword entry tid **%d**, keyword_tid **%d**, keyword_uuid %s\n", entryTid, keywordTid, keywordUUID)
	return nil
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
	// modified::text is the server stamp kept in server_modified; it has to
	// be the same text every time, which time.Time's formatting isn't.
	query := fmt.Sprintf("SELECT tid, uuid, title, star, modified::text FROM %s WHERE modified > $1 AND deleted = $2;", ct)
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
			err = rows.Scan(&c.tid, &uuid, &c.title)
		} else {
			err = rows.Scan(&c.tid, &uuid, &c.title, &c.star, &c.modified)
		}
		if err != nil {
			return nil, fmt.Errorf("Error scanning server_%s: %v", ct, err)
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
			err = rows.Scan(&c.id, &tid, &c.uuid, &c.title)
		} else {
			err = rows.Scan(&c.id, &tid, &c.uuid, &c.title, &c.star, &c.modified)
		}
		if err != nil {
			return nil, fmt.Errorf("Error scanning client_%s: %v", ct, err)
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
	rows, err := s.PG.Query("SELECT tid, title, star, note, modified::text, added, archived, context_tid, folder_tid, context_uuid, folder_uuid FROM task WHERE modified > $1 AND deleted = $2 ORDER BY tid;", serverTime, false)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for server_updated_entries: %v", err)
	}
	for rows.Next() {
		var e EntryPlusTag
		var contextUUID, folderUUID sql.NullString
		if err = rows.Scan(&e.tid, &e.title, &e.star, &e.note, &e.modified, &e.added, &e.archived, &e.context_tid, &e.folder_tid, &contextUUID, &folderUUID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("Error scanning server_updated_entries: %v", err)
		}
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
		if err = rows.Scan(&e.tid, &e.title); err != nil {
			rows.Close()
			return nil, fmt.Errorf("Error scanning server_deleted_entries: %v", err)
		}
		changes.serverDeletedEntries = append(changes.serverDeletedEntries, e)
	}
	rows.Close()

	// Drop server rows this client already holds: with the watermark taken
	// two minutes early these are mostly our own pushes, and counting them
	// would make the reported change count (shown by hybrid) wrong.
	if err = s.dropSeen(changes); err != nil {
		return nil, err
	}

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
	rows, err = s.MainDB.Query("SELECT id, tid, title, star, note, modified, added, archived, context_tid, folder_tid, context_uuid, folder_uuid, server_modified FROM task WHERE substr(modified, 1, 19)  > ? AND deleted = ?;", clientTime, false)
	if err != nil {
		return nil, fmt.Errorf("Error in SELECT for client_updated_entries: %v", err)
	}
	for rows.Next() {
		var e newEntry
		// context_tid/folder_tid are deprecated and NULL on databases whose
		// schema has no DEFAULT for them; the uuids are authoritative and
		// syncEntriesToServer resolves a tid < 1 from them.
		var tid, contextTid, folderTid sql.NullInt64
		if err = rows.Scan(&e.id, &tid, &e.title, &e.star, &e.note, &e.modified, &e.added, &e.archived, &contextTid, &folderTid, &e.context_uuid, &e.folder_uuid, &e.serverModified); err != nil {
			rows.Close()
			return nil, fmt.Errorf("Error scanning client_updated_entries: %v", err)
		}
		e.tid = int(tid.Int64)
		e.context_tid = int(contextTid.Int64)
		e.folder_tid = int(folderTid.Int64)
		changes.clientUpdatedEntries = append(changes.clientUpdatedEntries, e)
	}
	rows.Close()

	rows, err = s.MainDB.Query("SELECT id, tid, title, modified, server_modified FROM task WHERE substr(modified, 1, 19) > $1 AND deleted = $2;", clientTime, true)
	if err != nil {
		return nil, fmt.Errorf("Error with retrieving client deleted entries: %v", err)
	}
	for rows.Next() {
		var e entry
		var tid sql.NullInt64
		if err = rows.Scan(&e.id, &tid, &e.title, &e.modified, &e.serverModified); err != nil {
			rows.Close()
			return nil, fmt.Errorf("Error scanning client_deleted_entries: %v", err)
		}
		e.tid = int(tid.Int64)
		changes.clientDeletedEntries = append(changes.clientDeletedEntries, e)
	}
	rows.Close()

	return changes, nil
}

// localServerModified returns the server stamp a client row holds, and
// whether the row exists at all.
func (s *Syncer) localServerModified(table string, tid int) (sql.NullString, bool, error) {
	var sm sql.NullString
	err := s.MainDB.QueryRow(fmt.Sprintf("SELECT server_modified FROM %s WHERE tid=?;", table), tid).Scan(&sm)
	if err == sql.ErrNoRows {
		return sm, false, nil
	}
	if err != nil {
		return sm, false, fmt.Errorf("looking up local %s tid %d: %v", table, tid, err)
	}
	return sm, true, nil
}

// dropSeen removes server changes applying would not alter: updates whose
// stamp the client already holds, and tombstones for rows it doesn't have.
func (s *Syncer) dropSeen(changes *syncChanges) error {
	keepUpdated := func(table string, tid int, modified string) (bool, error) {
		sm, _, err := s.localServerModified(table, tid)
		return !(sm.Valid && sm.String == modified), err
	}
	keepDeleted := func(table string, tid int) (bool, error) {
		_, exists, err := s.localServerModified(table, tid)
		return exists, err
	}

	for _, set := range []struct {
		ct      containerType
		updated *[]container
		deleted *[]container
	}{
		{containerTypeContext, &changes.serverUpdatedContexts, &changes.serverDeletedContexts},
		{containerTypeFolder, &changes.serverUpdatedFolders, &changes.serverDeletedFolders},
		{containerTypeKeyword, &changes.serverUpdatedKeywords, &changes.serverDeletedKeywords},
	} {
		var updated, deleted []container
		for _, c := range *set.updated {
			keep, err := keepUpdated(string(set.ct), c.tid, c.modified)
			if err != nil {
				return err
			}
			if keep {
				updated = append(updated, c)
			}
		}
		for _, c := range *set.deleted {
			keep, err := keepDeleted(string(set.ct), c.tid)
			if err != nil {
				return err
			}
			if keep {
				deleted = append(deleted, c)
			}
		}
		*set.updated, *set.deleted = updated, deleted
	}

	var entries []EntryPlusTag
	for _, e := range changes.serverUpdatedEntries {
		keep, err := keepUpdated("task", e.tid, e.modified)
		if err != nil {
			return err
		}
		if keep {
			entries = append(entries, e)
		}
	}
	changes.serverUpdatedEntries = entries

	var deleted []entry
	for _, e := range changes.serverDeletedEntries {
		keep, err := keepDeleted("task", e.tid)
		if err != nil {
			return err
		}
		if keep {
			deleted = append(deleted, e)
		}
	}
	changes.serverDeletedEntries = deleted
	return nil
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

// ---------------------------------------------------------------------------
// Applying changes.
//
// How a run decides what to send and what to keep (SYNC_ISSUES.md 1-3):
//
//   - Both watermarks are captured *before* any work and saved only at the
//     end. A row that changes while the run is in flight, on either side, is
//     therefore newer than the saved watermark and is picked up next time.
//     The server one is taken two minutes early, as the iOS client does,
//     to cover transactions that began before the pull and commit after it.
//   - The overlap means our own pushes come back on the next pull. Every
//     client row records the server's modified as it last saw it
//     (server_modified); a pulled row whose modified equals it is an echo
//     and is skipped.
//   - A local row is dirty when its modified is past the previous client
//     watermark. Rows written by the pull are stamped with the *new* client
//     watermark, so they are not dirty next time.
//   - A dirty row the server really changed is a conflict: the server's
//     version wins and the local one is kept as a new note,
//     "<title> (conflict YYYY-MM-DD)".
//   - Pushes are guarded with the row's server_modified. A refused push, or
//     any row that fails, is "touched" (modified = now) so it stays dirty
//     and is retried; a refused push turns into a conflict copy on the next
//     pull. The server watermark only advances when every pulled row
//     applied, since a pull cannot be retried any other way.

// run carries one Synchronize's watermarks and outcome.
type run struct {
	prevServer, prevClient string       // pull and push everything newer than these
	nextServer, nextClient string       // saved at the end of the run
	resolved               map[int]bool // local task ids the pull settled; not pushed
	resolvedContainers     map[containerType]map[int]bool
	copies                 []newEntry // conflict copies, pushed in this run
	conflicts              int
	deferred               int // pushes the server refused because it had moved on
	pullFailures           int
	pushFailures           int
	touchFailures          int
}

func (r *run) isDirty(modified string) bool {
	return tc(modified, 19, false) > r.prevClient
}

// execer is satisfied by *sql.DB and *sql.Tx.
type execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// ensureSyncColumns adds server_modified to the client tables that predate
// it. Idempotent; run by Synchronize because vimango-sync (hybrid's path)
// never goes through the TUI's startup migrations.
func (s *Syncer) ensureSyncColumns() error {
	for _, t := range []string{"task", "context", "folder", "keyword"} {
		has, err := s.hasServerModified(t)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		if _, err := s.MainDB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN server_modified TEXT;", t)); err != nil {
			return fmt.Errorf("adding server_modified to %s: %v", t, err)
		}
	}
	return nil
}

func (s *Syncer) hasServerModified(table string) (bool, error) {
	var n int
	err := s.MainDB.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name='server_modified';", table).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("checking %s for server_modified: %v", table, err)
	}
	return n > 0, nil
}

// touch marks a local row changed so the next sync retries it.
func (s *Syncer) touch(table string, id int, r *run, lg io.Writer) {
	_, err := s.MainDB.Exec(fmt.Sprintf("UPDATE %s SET modified=datetime('now') WHERE id=?;", table), id)
	if err != nil {
		r.touchFailures++
		fmt.Fprintf(lg, "**Error** marking local %s id %d for retry: %v\n", table, id, err)
	}
}

// markPushed records the server stamp a push returned. It also moves a
// modified inside the watermark overlap back to the new client watermark,
// so the row isn't pushed again next run (and echoed to every client) —
// but only if nothing edited the row while it was being pushed.
func (s *Syncer) markPushed(table string, id int, fetchedModified, serverModified string, r *run) error {
	_, err := s.MainDB.Exec(fmt.Sprintf("UPDATE %s SET server_modified=?, "+
		"modified=CASE WHEN modified IS ? AND substr(modified, 1, 19) > ? THEN ? ELSE modified END WHERE id=?;", table),
		serverModified, fetchedModified, r.nextClient, r.nextClient, id)
	return err
}

// syncContainersToClient applies containers the server changed.
func (s *Syncer) syncContainersToClient(ct containerType, containers []container, r *run, lg io.Writer) {
	for _, c := range containers {
		var id int
		var modified, serverModified sql.NullString
		query := fmt.Sprintf("SELECT id, modified, server_modified FROM %s WHERE tid=?;", ct)
		err := s.MainDB.QueryRow(query, c.tid).Scan(&id, &modified, &serverModified)
		switch {
		case err == sql.ErrNoRows:
			query = fmt.Sprintf("INSERT INTO %s (tid, uuid, title, star, modified, server_modified, deleted) VALUES (?, ?, ?, ?, ?, ?, false);", ct)
			if _, err = s.MainDB.Exec(query, c.tid, c.uuid, c.title, c.star, r.nextClient, c.modified); err != nil {
				r.pullFailures++
				fmt.Fprintf(lg, "**Error** inserting new %s %q into sqlite: %v\n", ct, c.title, err)
				continue
			}
			fmt.Fprintf(lg, "Inserted local %s: %q with tid: %v uuid: %s\n", ct, c.title, c.tid, c.uuid)
		case err != nil:
			r.pullFailures++
			fmt.Fprintf(lg, "**Error** looking up local %s with tid %d: %v\n", ct, c.tid, err)
		case serverModified.Valid && serverModified.String == c.modified:
			// Our own push coming back.
		default:
			query = fmt.Sprintf("UPDATE %s SET title=?, star=?, uuid=?, modified=?, server_modified=? WHERE id=?;", ct)
			if _, err = s.MainDB.Exec(query, c.title, c.star, c.uuid, r.nextClient, c.modified, id); err != nil {
				r.pullFailures++
				fmt.Fprintf(lg, "**Error** updating sqlite for %s with tid: %v: %v\n", ct, c.tid, err)
				continue
			}
			// The server wins a container both sides changed; pushing the
			// local version afterwards would leave the two sides disagreeing.
			if r.isDirty(modified.String) {
				r.resolvedContainers[ct][id] = true
				fmt.Fprintf(lg, "Server won: local %s %q (tid %d) was also changed on the server\n", ct, c.title, c.tid)
			}
			fmt.Fprintf(lg, "Updated local %s: %q with tid: %v uuid: %s\n", ct, c.title, c.tid, c.uuid)
		}
	}
}

// syncContainersToServer pushes containers the client changed.
func (s *Syncer) syncContainersToServer(ct containerType, containers []container, r *run, lg io.Writer) {
	for _, c := range containers {
		if r.resolvedContainers[ct][c.id] {
			continue
		}
		if err := s.pushContainer(ct, c, r, lg); err != nil {
			r.pushFailures++
			fmt.Fprintf(lg, "**Error** pushing %s %q (id %d), will retry next sync: %v\n", ct, c.title, c.id, err)
			s.touch(string(ct), c.id, r, lg)
		}
	}
}

func (s *Syncer) pushContainer(ct containerType, c container, r *run, lg io.Writer) error {
	var exists bool
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE tid=$1);", ct)
	if err := s.PG.QueryRow(query, c.tid).Scan(&exists); err != nil {
		return fmt.Errorf("SELECT EXISTS: %v", err)
	}

	var serverModified string
	if exists {
		query = fmt.Sprintf("UPDATE %s SET title=$1, star=$2, uuid=$3, modified=now() WHERE tid=$4 RETURNING modified::text;", ct)
		if err := s.PG.QueryRow(query, c.title, c.star, c.uuid, c.tid).Scan(&serverModified); err != nil {
			return fmt.Errorf("updating postgres: %v", err)
		}
		if err := s.markPushed(string(ct), c.id, c.modified, serverModified, r); err != nil {
			fmt.Fprintf(lg, "Error recording server stamp for local %s id %d: %v\n", ct, c.id, err)
		}
		fmt.Fprintf(lg, "Updated server %s: %q with tid: %v uuid: %s\n", ct, c.title, c.tid, c.uuid)
		return nil
	}

	var tid int
	query = fmt.Sprintf("INSERT INTO %s (title, star, uuid, modified, deleted) VALUES ($1, $2, $3, now(), false) RETURNING tid, modified::text;", ct)
	if err := s.PG.QueryRow(query, c.title, c.star, c.uuid).Scan(&tid, &serverModified); err != nil {
		return fmt.Errorf("inserting into postgres: %v", err)
	}
	query = fmt.Sprintf("UPDATE %s SET tid=? WHERE id=?;", ct)
	if _, err := s.MainDB.Exec(query, tid, c.id); err != nil {
		return fmt.Errorf("server row tid %d was created but setting the local tid failed: %v", tid, err)
	}
	if err := s.markPushed(string(ct), c.id, c.modified, serverModified, r); err != nil {
		fmt.Fprintf(lg, "Error recording server stamp for local %s id %d: %v\n", ct, c.id, err)
	}
	fmt.Fprintf(lg, "Inserted server %s %q with uuid: %s and updated local tid to %d\n", ct, c.title, c.uuid, tid)
	return nil
}

// localTask is the client row a pulled or server-deleted entry lands on.
type localTask struct {
	id             int
	modified       sql.NullString
	serverModified sql.NullString
	deleted        bool
	title          string
	note           sql.NullString
	star           bool
	archived       bool
	contextUUID    string
	folderUUID     string
}

func (s *Syncer) localTaskByTid(tid int) (*localTask, error) {
	var t localTask
	err := s.MainDB.QueryRow("SELECT id, modified, server_modified, deleted, title, note, star, archived, "+
		"context_uuid, folder_uuid FROM task WHERE tid=?;", tid).
		Scan(&t.id, &t.modified, &t.serverModified, &t.deleted, &t.title, &t.note, &t.star, &t.archived,
			&t.contextUUID, &t.folderUUID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// sameContent reports whether a local row already says what the server
// says, in which case both sides changing it is not a conflict.
func sameContent(t *localTask, e EntryPlusTag) bool {
	return t.title == e.title && t.note.String == e.note.String && t.star == e.star &&
		t.archived == e.archived && t.contextUUID == e.context_uuid && t.folderUUID == e.folder_uuid
}

// saveConflictCopy keeps a losing local edit as a new, unsynced note.
func saveConflictCopy(x execer, t *localTask) (newEntry, error) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	e := newEntry{
		title:        fmt.Sprintf("%s (conflict %s)", t.title, time.Now().Format("2006-01-02")),
		note:         t.note,
		star:         t.star,
		archived:     t.archived,
		context_uuid: t.contextUUID,
		folder_uuid:  t.folderUUID,
		added:        now,
		modified:     now,
	}
	res, err := x.Exec("INSERT INTO task (title, note, star, archived, context_uuid, folder_uuid, added, modified, deleted) "+
		"VALUES (?, ?, ?, ?, ?, ?, ?, ?, false);",
		e.title, e.note, e.star, e.archived, e.context_uuid, e.folder_uuid, e.added, e.modified)
	if err != nil {
		return newEntry{}, fmt.Errorf("saving conflict copy of %q: %v", t.title, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return newEntry{}, err
	}
	e.id = int(id)
	return e, nil
}

// withTx runs fn in a transaction. fn returns false to roll back and have
// the caller look at the row again (it changed underneath us).
func (s *Syncer) withTx(fn func(tx *sql.Tx) (bool, error)) (bool, error) {
	tx, err := s.MainDB.Begin()
	if err != nil {
		return false, err
	}
	ok, err := fn(tx)
	if err != nil || !ok {
		tx.Rollback()
		return false, err
	}
	return true, tx.Commit()
}

// syncEntriesToClient applies entries the server changed, then refreshes
// their keywords and FTS rows.
func (s *Syncer) syncEntriesToClient(entries []EntryPlusTag, r *run, lg io.Writer) {
	var applied []EntryPlusTag
	for _, e := range entries {
		ok, err := s.applyServerEntry(e, r, lg)
		if err != nil {
			r.pullFailures++
			fmt.Fprintf(lg, "**Error** applying server entry %q with tid %d: %v\n", truncate(e.title, 15), e.tid, err)
			continue
		}
		if ok {
			applied = append(applied, e)
		}
	}

	if len(applied) == 0 {
		return
	}

	tids := make([]int, len(applied))
	tidsIf := make([]interface{}, len(applied))
	for i := range applied {
		tids[i] = applied[i].tid
		tidsIf[i] = applied[i].tid
	}

	// Delete existing keywords and FTS entries for updated tasks
	in := "?" + strings.Repeat(",?", len(tids)-1)
	stmt := fmt.Sprintf("DELETE FROM task_keyword WHERE task_tid IN (%s);", in)
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
		for i := range applied {
			if tag, ok := tagMap[applied[i].tid]; ok {
				applied[i].tag = tag
				fmt.Fprintf(lg, "FTS tag will be updated for tid: %d, tag: %s\n", applied[i].tid, tag.String)
			}
		}
	}

	query, args := createBulkInsertQueryFTS3(len(applied), applied)
	err = bulkInsert(s.FtsDB, query, args)
	if err != nil {
		fmt.Fprintf(lg, "%v", err)
	} else {
		fmt.Fprintf(lg, "FTS entries updated for task tids: %v\n", tids)
	}
}

// applyServerEntry writes one pulled entry to the client. It returns false,
// without error, for an echo of our own push.
func (s *Syncer) applyServerEntry(e EntryPlusTag, r *run, lg io.Writer) (bool, error) {
	// A server task can reference a container uuid with no row anywhere
	// (SYNC_ISSUES.md, "Data finding"); the local foreign key would reject
	// it on every run. Show it under "none" instead.
	if e.context_uuid == "" || !s.containerExists(containerTypeContext, e.context_uuid) {
		fmt.Fprintf(lg, "Server entry tid %d references missing context %q; filing it under 'none' locally\n", e.tid, e.context_uuid)
		e.context_uuid, e.context_tid = DefaultContextUUID, DefaultContainerID
	}
	if e.folder_uuid == "" || !s.containerExists(containerTypeFolder, e.folder_uuid) {
		fmt.Fprintf(lg, "Server entry tid %d references missing folder %q; filing it under 'none' locally\n", e.tid, e.folder_uuid)
		e.folder_uuid, e.folder_tid = DefaultFolderUUID, DefaultContainerID
	}

	// Retried when the local row changes between reading and writing it —
	// a write from the editor or from hybrid landing mid-sync.
	for attempt := 0; attempt < 3; attempt++ {
		t, err := s.localTaskByTid(e.tid)
		if err != nil {
			return false, err
		}

		if t == nil {
			_, err = s.MainDB.Exec("INSERT INTO task (tid, title, star, added, archived, context_tid, folder_tid, context_uuid, folder_uuid, "+
				"note, modified, server_modified, deleted) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, false);",
				e.tid, e.title, e.star, e.added, e.archived, e.context_tid, e.folder_tid, e.context_uuid, e.folder_uuid,
				e.note, r.nextClient, e.modified)
			if err != nil {
				return false, err
			}
			fmt.Fprintf(lg, "Inserted client entry %q with tid **%d** context_uuid: %s folder_uuid: %s\n", e.title, e.tid, e.context_uuid, e.folder_uuid)
			return true, nil
		}

		if t.serverModified.Valid && t.serverModified.String == e.modified {
			return false, nil // our own push coming back
		}

		dirty := r.isDirty(t.modified.String)
		conflict := dirty && !t.deleted && !sameContent(t, e)
		var cp newEntry
		ok, err := s.withTx(func(tx *sql.Tx) (bool, error) {
			if conflict {
				var err error
				if cp, err = saveConflictCopy(tx, t); err != nil {
					return false, err
				}
			}
			res, err := tx.Exec("UPDATE task SET title=?, star=?, archived=?, context_tid=?, folder_tid=?, context_uuid=?, folder_uuid=?, "+
				"note=?, deleted=false, modified=?, server_modified=? WHERE id=? AND modified IS ?;",
				e.title, e.star, e.archived, e.context_tid, e.folder_tid, e.context_uuid, e.folder_uuid,
				e.note, r.nextClient, e.modified, t.id, t.modified)
			if err != nil {
				return false, err
			}
			n, err := res.RowsAffected()
			return n == 1, err
		})
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}

		if dirty {
			r.resolved[t.id] = true
		}
		if conflict {
			r.conflicts++
			r.copies = append(r.copies, cp)
			fmt.Fprintf(lg, "Server won: tid %d was changed here and on the server; the local version was saved as %q\n", e.tid, cp.title)
		} else if dirty && t.deleted {
			fmt.Fprintf(lg, "Server won: entry %q (tid %d) deleted here was changed on the server and has been restored\n", truncate(e.title, 15), e.tid)
		}
		fmt.Fprintf(lg, "Updated client entry %q with tid **%d** context_uuid: %s folder_uuid: %s\n", e.title, e.tid, e.context_uuid, e.folder_uuid)
		return true, nil
	}
	return false, fmt.Errorf("the local row kept changing during the sync")
}

func (s *Syncer) containerExists(ct containerType, uuid string) bool {
	var n int
	if err := s.MainDB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE uuid=?;", ct), uuid).Scan(&n); err != nil {
		return true // can't tell; leave the reference alone
	}
	return n > 0
}

// syncEntriesToServer pushes entries the client changed. Rows the pull
// already settled are skipped; conflict copies the pull made go up too.
func (s *Syncer) syncEntriesToServer(entries []newEntry, r *run, lg io.Writer) {
	for _, e := range append(entries, r.copies...) {
		if r.resolved[e.id] {
			continue
		}
		deferred, err := s.pushEntry(e, r, lg)
		switch {
		case err != nil:
			r.pushFailures++
			fmt.Fprintf(lg, "**Error** pushing entry %q (id %d), will retry next sync: %v\n", truncate(e.title, 15), e.id, err)
			s.touch("task", e.id, r, lg)
		case deferred:
			r.deferred++
			fmt.Fprintf(lg, "Server has a newer version of %q (tid %d); the local edit becomes a conflict copy on the next sync\n", truncate(e.title, 15), e.tid)
			s.touch("task", e.id, r, lg)
		}
	}
}

// errNoContainer means a task references a container uuid with no row at
// all — it can never resolve, unlike a container that hasn't synced yet.
var errNoContainer = fmt.Errorf("no such container")

// containerTid resolves a container uuid to its server tid. Pushing a task
// whose container never reached the server would leave the server task
// pointing at a uuid it has no row for.
func (s *Syncer) containerTid(ct containerType, uuid string) (int, error) {
	var tid sql.NullInt64
	err := s.MainDB.QueryRow(fmt.Sprintf("SELECT tid FROM %s WHERE uuid=?;", ct), uuid).Scan(&tid)
	if err == sql.ErrNoRows {
		return 0, errNoContainer
	}
	if err != nil {
		return 0, err
	}
	if !tid.Valid || tid.Int64 < 1 {
		return 0, fmt.Errorf("%s %s has not reached the server yet", ct, uuid)
	}
	return int(tid.Int64), nil
}

// repointToNone moves a local task off a container uuid that has no row
// (SYNC_ISSUES.md, "Data finding") onto the "none" container.
func (s *Syncer) repointToNone(ct containerType, e newEntry, lg io.Writer) (string, int, error) {
	field, uuid, none := "context_uuid", e.context_uuid, DefaultContextUUID
	if ct == containerTypeFolder {
		field, uuid, none = "folder_uuid", e.folder_uuid, DefaultFolderUUID
	}
	if _, err := s.MainDB.Exec(fmt.Sprintf("UPDATE task SET %s=? WHERE id=?;", field), none, e.id); err != nil {
		return "", 0, fmt.Errorf("moving local entry off missing %s %s: %v", ct, uuid, err)
	}
	fmt.Fprintf(lg, "Entry %q (id %d) referenced %s %s, which does not exist; moved it to 'none'\n", truncate(e.title, 15), e.id, ct, uuid)
	return none, DefaultContainerID, nil
}

// pushEntry sends one entry to the server. deferred means the server had
// moved on since this client last saw the row, so nothing was written.
func (s *Syncer) pushEntry(e newEntry, r *run, lg io.Writer) (deferred bool, err error) {
	if e.context_uuid == "" {
		e.context_uuid = DefaultContextUUID
	}
	if e.folder_uuid == "" {
		e.folder_uuid = DefaultFolderUUID
	}
	contextTid, err := s.containerTid(containerTypeContext, e.context_uuid)
	if err == errNoContainer {
		e.context_uuid, contextTid, err = s.repointToNone(containerTypeContext, e, lg)
	}
	if err != nil {
		return false, err
	}
	folderTid, err := s.containerTid(containerTypeFolder, e.folder_uuid)
	if err == errNoContainer {
		e.folder_uuid, folderTid, err = s.repointToNone(containerTypeFolder, e, lg)
	}
	if err != nil {
		return false, err
	}

	var tid int
	var serverModified string
	if e.tid < 1 {
		err := s.PG.QueryRow("INSERT INTO task (title, star, added, archived, context_tid, folder_tid, context_uuid, folder_uuid, note, modified, deleted) "+
			"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), false) RETURNING tid, modified::text",
			e.title, e.star, e.added, e.archived, contextTid, folderTid, e.context_uuid, e.folder_uuid, e.note).Scan(&tid, &serverModified)
		if err != nil {
			return false, fmt.Errorf("inserting server entry: %v", err)
		}
		if _, err = s.MainDB.Exec("UPDATE task SET tid=? WHERE id=?;", tid, e.id); err != nil {
			return false, fmt.Errorf("server entry tid %d was created but setting the local tid failed: %v", tid, err)
		}
		if err := s.markPushed("task", e.id, e.modified, serverModified, r); err != nil {
			fmt.Fprintf(lg, "Error recording server stamp for local task id %d: %v\n", e.id, err)
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
		// Only overwrite the version this client last saw. Rows synced
		// before server_modified existed have none; for those the server
		// must not have changed since the previous run's pull.
		err := s.PG.QueryRow("UPDATE task SET title=$1, star=$2, context_tid=$3, folder_tid=$4, context_uuid=$5, folder_uuid=$6, "+
			"note=$7, archived=$8, modified=now() WHERE tid=$9 AND deleted=false AND "+
			"(COALESCE(modified, '1970-01-01') = $10::timestamp OR "+
			"($10::timestamp IS NULL AND COALESCE(modified, '1970-01-01') <= $11::timestamp)) "+
			"RETURNING modified::text;",
			e.title, e.star, contextTid, folderTid, e.context_uuid, e.folder_uuid, e.note, e.archived, e.tid,
			e.serverModified, r.prevServer).Scan(&serverModified)
		if err == sql.ErrNoRows {
			return true, nil
		}
		if err != nil {
			return false, fmt.Errorf("updating server entry: %v", err)
		}
		tid = e.tid
		if err := s.markPushed("task", e.id, e.modified, serverModified, r); err != nil {
			fmt.Fprintf(lg, "Error recording server stamp for local task id %d: %v\n", e.id, err)
		}
		fmt.Fprintf(lg, "Updated server entry *%q* with tid **%d** context_uuid: %s folder_uuid: %s\n", truncate(e.title, 15), tid, e.context_uuid, e.folder_uuid)
	}

	// Update the server entry's keywords
	if _, err := s.PG.Exec("DELETE FROM task_keyword WHERE task_tid=$1;", tid); err != nil {
		return false, fmt.Errorf("deleting server task_keyword rows for tid %d: %v", tid, err)
	}
	for _, kw := range taskKeywordTidsAndUUIDs(s.MainDB, lg, tid) {
		if err := insertTaskKeywordTids(s.PG, lg, kw.tid, tid, kw.uuid); err != nil {
			return false, err
		}
	}
	return false, nil
}

// deleteServerEntriesFromClient removes entries deleted on the server. A
// local edit to one is kept as a conflict copy.
func (s *Syncer) deleteServerEntriesFromClient(entries []entry, r *run, lg io.Writer) {
	for _, e := range entries {
		if err := s.removeServerDeleted(e, r, lg); err != nil {
			r.pullFailures++
			fmt.Fprintf(lg, "**Error** deleting client entry %q with tid %d: %v\n", tc(e.title, 15, true), e.tid, err)
		}
	}
}

func (s *Syncer) removeServerDeleted(e entry, r *run, lg io.Writer) error {
	for attempt := 0; attempt < 3; attempt++ {
		t, err := s.localTaskByTid(e.tid)
		if err != nil {
			return err
		}
		if t == nil {
			return nil
		}
		dirty := r.isDirty(t.modified.String)
		conflict := dirty && !t.deleted
		var cp newEntry
		ok, err := s.withTx(func(tx *sql.Tx) (bool, error) {
			if conflict {
				var err error
				if cp, err = saveConflictCopy(tx, t); err != nil {
					return false, err
				}
			}
			if _, err := tx.Exec("DELETE FROM task_keyword WHERE task_tid=?;", e.tid); err != nil {
				return false, err
			}
			res, err := tx.Exec("DELETE FROM task WHERE id=? AND modified IS ?;", t.id, t.modified)
			if err != nil {
				return false, err
			}
			n, err := res.RowsAffected()
			return n == 1, err
		})
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if dirty {
			r.resolved[t.id] = true
		}
		if conflict {
			r.conflicts++
			r.copies = append(r.copies, cp)
			fmt.Fprintf(lg, "Server won: tid %d was changed here but deleted on the server; the local version was saved as %q\n", e.tid, cp.title)
		}
		if _, err = s.FtsDB.Exec("DELETE FROM fts WHERE tid=?;", e.tid); err != nil {
			fmt.Fprintf(lg, "Error deleting fts row for tid %d: %v\n", e.tid, err)
		}
		fmt.Fprintf(lg, "Deleted client entry %q with tid %d and its task_keyword rows\n", truncate(e.title, 15), e.tid)
		return nil
	}
	return fmt.Errorf("the local row kept changing during the sync")
}

// deleteClientEntriesFromServer pushes entries deleted on the client. The
// server is marked first; the local row goes only once the server has it.
func (s *Syncer) deleteClientEntriesFromServer(entries []entry, r *run, lg io.Writer) {
	for _, e := range entries {
		if r.resolved[e.id] {
			continue
		}
		deferred, err := s.pushDelete(e, r, lg)
		switch {
		case err != nil:
			r.pushFailures++
			fmt.Fprintf(lg, "**Error** deleting entry %q (id %d), will retry next sync: %v\n", tc(e.title, 15, true), e.id, err)
			s.touch("task", e.id, r, lg)
		case deferred:
			r.deferred++
			fmt.Fprintf(lg, "Server has a newer version of deleted entry %q (tid %d); it will be restored on the next sync\n", tc(e.title, 15, true), e.tid)
			s.touch("task", e.id, r, lg)
		}
	}
}

func (s *Syncer) pushDelete(e entry, r *run, lg io.Writer) (deferred bool, err error) {
	if e.tid >= 1 {
		var tid int
		err := s.PG.QueryRow("UPDATE task SET deleted=true, modified=now() WHERE tid=$1 AND deleted=false AND "+
			"(COALESCE(modified, '1970-01-01') = $2::timestamp OR "+
			"($2::timestamp IS NULL AND COALESCE(modified, '1970-01-01') <= $3::timestamp)) RETURNING tid;",
			e.tid, e.serverModified, r.prevServer).Scan(&tid)
		if err == sql.ErrNoRows {
			// Refused, or there is nothing live to delete.
			var deleted bool
			err = s.PG.QueryRow("SELECT deleted FROM task WHERE tid=$1;", e.tid).Scan(&deleted)
			if err == nil && !deleted {
				return true, nil
			}
			if err != nil && err != sql.ErrNoRows {
				return false, err
			}
		} else if err != nil {
			return false, fmt.Errorf("setting server entry to deleted: %v", err)
		} else {
			fmt.Fprintf(lg, "Updated server entry %q with tid %d to **deleted = true**\n", truncate(e.title, 15), e.tid)
		}
		if _, err = s.PG.Exec("DELETE FROM task_keyword WHERE task_tid=$1;", e.tid); err != nil {
			fmt.Fprintf(lg, "Error deleting task_keyword server rows where entry tid = %d: %v\n", e.tid, err)
		}
		if _, err = s.MainDB.Exec("DELETE FROM task_keyword WHERE task_tid=?;", e.tid); err != nil {
			return false, fmt.Errorf("deleting client task_keyword rows: %v", err)
		}
		if _, err = s.FtsDB.Exec("DELETE FROM fts WHERE tid=?;", e.tid); err != nil {
			fmt.Fprintf(lg, "Error deleting fts row for tid %d: %v\n", e.tid, err)
		}
	} else {
		fmt.Fprintf(lg, "There is no server entry to delete for client id %d\n", e.id)
	}
	if _, err = s.MainDB.Exec("DELETE FROM task WHERE id=?;", e.id); err != nil {
		return false, fmt.Errorf("deleting client entry: %v", err)
	}
	fmt.Fprintf(lg, "Deleted client entry %q with id %d\n", tc(e.title, 15, true), e.id)
	return false, nil
}

// deleteContainerFromBoth deletes a container from both server and client, updating task references
func (s *Syncer) deleteContainerFromBoth(ct containerType, c container, isServerDeleted bool, taskField string, lg io.Writer) error {
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
		return nil
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
		return fmt.Errorf("changing server entry %s for a deleted %s: %v", taskField, ct, err)
	} else {
		rowsAffected, _ := res.RowsAffected()
		fmt.Fprintf(lg, "The number of server entries that were changed to 'none': **%d**\n", rowsAffected)
	}

	query = fmt.Sprintf("UPDATE task SET %s=%d, %s='%s', modified=datetime('now') WHERE %s=?;",
		taskField, DefaultContainerID, uuidField, defaultUUID, match)
	res, err = s.MainDB.Exec(query, matchArg)
	if err != nil {
		return fmt.Errorf("changing client entry %s for a deleted %s: %v", taskField, ct, err)
	} else {
		rowsAffected, _ := res.RowsAffected()
		fmt.Fprintf(lg, "The number of client entries that were changed to 'none': **%d**\n", rowsAffected)
	}

	if isServerDeleted {
		// Delete from client only
		query = fmt.Sprintf("DELETE FROM %s WHERE tid=?", ct)
		_, err = s.MainDB.Exec(query, c.tid)
		if err != nil {
			return fmt.Errorf("deleting local %s %q with tid = %d: %v", ct, c.title, c.tid, err)
		}
		fmt.Fprintf(lg, "Deleted client %s %q with tid %d\n", ct, c.title, c.tid)
	} else {
		// Mark as deleted on server, delete from client
		query = fmt.Sprintf("UPDATE %s SET deleted=true, modified=now() WHERE tid=$1", ct)
		// The local row goes only once the server has the tombstone;
		// otherwise the delete would never reach the server.
		_, err = s.PG.Exec(query, c.tid)
		if err != nil {
			return fmt.Errorf("setting server %s %q with tid = %d to deleted: %v", ct, c.title, c.tid, err)
		}

		query = fmt.Sprintf("DELETE FROM %s WHERE id=?", ct)
		_, err = s.MainDB.Exec(query, c.id)
		if err != nil {
			return fmt.Errorf("deleting local %s %q with id %d: %v", ct, c.title, c.id, err)
		}
		fmt.Fprintf(lg, "Deleted client %s %q: id %d and updated server %s with tid %d to deleted = true\n", ct, c.title, c.id, ct, c.tid)
	}
	return nil
}

// deleteKeywordFromBoth deletes a keyword from both server and client, including task_keyword relationships
func (s *Syncer) deleteKeywordFromBoth(c container, isServerDeleted bool, lg io.Writer) error {
	if !isServerDeleted {
		_, err := s.PG.Exec("DELETE FROM task_keyword WHERE keyword_tid=$1;", c.tid)
		if err != nil {
			return fmt.Errorf("deleting from task_keyword server keyword_tid: %d: %v", c.tid, err)
		}
	}

	_, err := s.MainDB.Exec("DELETE FROM task_keyword WHERE keyword_tid=?;", c.tid)
	if err != nil {
		return fmt.Errorf("deleting from task_keyword client keyword_tid: %d: %v", c.tid, err)
	}

	if isServerDeleted {
		// Delete from client only
		_, err = s.MainDB.Exec("DELETE FROM keyword WHERE tid=?", c.tid)
		if err != nil {
			return fmt.Errorf("deleting client keyword with tid = %d: %v", c.tid, err)
		}
		fmt.Fprintf(lg, "Deleted client keyword %q with tid %d\n", truncate(c.title, 15), c.tid)
	} else {
		// Mark as deleted on server, delete from client
		_, err = s.PG.Exec("UPDATE keyword SET deleted=true, modified=now() WHERE tid=$1", c.tid)
		if err != nil {
			return fmt.Errorf("setting server keyword %q with tid %d to deleted: %v", c.title, c.tid, err)
		}

		_, err = s.MainDB.Exec("DELETE FROM keyword WHERE id=?", c.id)
		if err != nil {
			return fmt.Errorf("deleting client keyword %q with id %d: %v", c.title, c.id, err)
		}
		fmt.Fprintf(lg, "Deleted client keyword %q: id %d and updated server keyword with tid %d to deleted = true\n", c.title, c.id, c.tid)
	}
	return nil
}

// Probe reports how many server-side rows a sync would apply — "is the
// remote ahead of us?" — without applying anything.
//
// This is deliberately *not* Synchronize(reportOnly=true): that runs the
// whole engine (a dozen-plus round trips, dragging full note bodies over
// the wire just to count them). Probe is one SELECT of change stamps
// against Postgres plus point lookups in SQLite, so it is cheap enough to
// run unattended. It never writes to either database.
//
// The watermark is the same one Synchronize uses: the 'server' row of the
// client's sync table. That watermark is taken two minutes early, so a
// bare `modified > watermark` count would include our own recent pushes;
// each stamp is compared with the server_modified the local row holds
// instead, as the iOS client's probe does. A tombstone for a row we don't
// have counts for nothing, since applying it is a no-op.
//
// Returns the change count. Any error means "unknown", never zero.
func (s *Syncer) Probe() (int, error) {
	var serverTime string
	row := s.MainDB.QueryRow("SELECT timestamp FROM sync WHERE machine=$1;", "server")
	if err := row.Scan(&serverTime); err != nil {
		return 0, fmt.Errorf("retrieving last server sync: %v", err)
	}

	type stamp struct {
		table    string
		tid      int
		modified string
		deleted  bool
	}
	rows, err := s.PG.Query(`
		SELECT 'task', tid, modified::text, deleted FROM task WHERE modified > $1
		UNION ALL SELECT 'context', tid, modified::text, deleted FROM context WHERE modified > $1
		UNION ALL SELECT 'folder', tid, modified::text, deleted FROM folder WHERE modified > $1
		UNION ALL SELECT 'keyword', tid, modified::text, deleted FROM keyword WHERE modified > $1;`, serverTime)
	if err != nil {
		return 0, fmt.Errorf("fetching server changes since %s: %v", serverTime, err)
	}
	var stamps []stamp
	for rows.Next() {
		var st stamp
		if err := rows.Scan(&st.table, &st.tid, &st.modified, &st.deleted); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scanning server change stamps: %v", err)
		}
		stamps = append(stamps, st)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("reading server change stamps: %v", err)
	}

	count := 0
	hasColumn := map[string]bool{}
	for _, st := range stamps {
		has, seen := hasColumn[st.table]
		if !seen {
			if has, err = s.hasServerModified(st.table); err != nil {
				return 0, err
			}
			hasColumn[st.table] = has
		}
		query := fmt.Sprintf("SELECT NULL FROM %s WHERE tid=?;", st.table)
		if has {
			query = fmt.Sprintf("SELECT server_modified FROM %s WHERE tid=?;", st.table)
		}
		var serverModified sql.NullString
		err := s.MainDB.QueryRow(query, st.tid).Scan(&serverModified)
		switch {
		case err == sql.ErrNoRows:
			if !st.deleted {
				count++
			}
		case err != nil:
			return 0, fmt.Errorf("looking up local %s tid %d: %v", st.table, st.tid, err)
		case serverModified.Valid && serverModified.String == st.modified:
			// Already have this version (usually our own push).
		default:
			count++
		}
	}
	return count, nil
}

// Synchronize runs a sync between client (SQLite) and server (Postgres).
// reportOnly: if true, only reports changes without applying them.
//
// Returns the formatted markdown log and a non-nil error on fatal failure.
// On error, the returned log still contains diagnostic detail. Rows that
// fail individually are not fatal: they are logged and retried next run.
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

	if err = s.ensureSyncColumns(); err != nil {
		fmt.Fprintf(&lg, "Error preparing the client database: %v", err)
		return
	}

	r := &run{
		prevServer: serverTime,
		prevClient: clientTime,
		resolved:   map[int]bool{},
		resolvedContainers: map[containerType]map[int]bool{
			containerTypeContext: {}, containerTypeFolder: {}, containerTypeKeyword: {},
		},
	}
	// The next run's watermarks are taken now, before any work (see run).
	// localtimestamp, not now(): modified is a timestamp without time zone.
	if err = s.PG.QueryRow("SELECT (localtimestamp - interval '2 minutes')::text;").Scan(&r.nextServer); err != nil {
		fmt.Fprintf(&lg, "Error getting current time from server: %v", err)
		return
	}
	if err = s.MainDB.QueryRow("SELECT datetime('now', '-2 seconds');").Scan(&r.nextClient); err != nil {
		fmt.Fprintf(&lg, "Error getting current time from client: %v", err)
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

	// Pull: server -> client. Conflicts are settled here, so the push
	// below only sends what the pull left dirty, plus conflict copies.
	s.syncContainersToClient(containerTypeContext, changes.serverUpdatedContexts, r, &lg)
	s.syncContainersToClient(containerTypeFolder, changes.serverUpdatedFolders, r, &lg)
	s.syncContainersToClient(containerTypeKeyword, changes.serverUpdatedKeywords, r, &lg)
	s.syncEntriesToClient(changes.serverUpdatedEntries, r, &lg)
	s.deleteServerEntriesFromClient(changes.serverDeletedEntries, r, &lg)

	// Push: client -> server. Containers first, so tasks can reference them.
	s.syncContainersToServer(containerTypeContext, changes.clientUpdatedContexts, r, &lg)
	s.syncContainersToServer(containerTypeFolder, changes.clientUpdatedFolders, r, &lg)
	s.syncContainersToServer(containerTypeKeyword, changes.clientUpdatedKeywords, r, &lg)
	s.syncEntriesToServer(changes.clientUpdatedEntries, r, &lg)
	s.deleteClientEntriesFromServer(changes.clientDeletedEntries, r, &lg)

	// Delete containers
	serverDeleted := func(ct containerType, cs []container, field string) {
		for _, c := range cs {
			var derr error
			if ct == containerTypeKeyword {
				derr = s.deleteKeywordFromBoth(c, true, &lg)
			} else {
				derr = s.deleteContainerFromBoth(ct, c, true, field, &lg)
			}
			if derr != nil {
				r.pullFailures++
				fmt.Fprintf(&lg, "**Error** applying server delete of %s %q: %v\n", ct, c.title, derr)
			}
		}
	}
	clientDeleted := func(ct containerType, cs []container, field string) {
		for _, c := range cs {
			var derr error
			if ct == containerTypeKeyword {
				derr = s.deleteKeywordFromBoth(c, false, &lg)
			} else {
				derr = s.deleteContainerFromBoth(ct, c, false, field, &lg)
			}
			if derr != nil {
				r.pushFailures++
				fmt.Fprintf(&lg, "**Error** pushing delete of %s %q, will retry next sync: %v\n", ct, c.title, derr)
				s.touch(string(ct), c.id, r, &lg)
			}
		}
	}
	serverDeleted(containerTypeContext, changes.serverDeletedContexts, "context_tid")
	clientDeleted(containerTypeContext, changes.clientDeletedContexts, "context_tid")
	serverDeleted(containerTypeFolder, changes.serverDeletedFolders, "folder_tid")
	clientDeleted(containerTypeFolder, changes.clientDeletedFolders, "folder_tid")
	serverDeleted(containerTypeKeyword, changes.serverDeletedKeywords, "")
	clientDeleted(containerTypeKeyword, changes.clientDeletedKeywords, "")

	fmt.Fprint(&lg, "\n## Outcome\n")
	fmt.Fprintf(&lg, "- Conflicts (local version saved as a \"(conflict date)\" note): **%d**\n", r.conflicts)
	fmt.Fprintf(&lg, "- Pushes refused because the server had moved on (resolved next sync): %d\n", r.deferred)
	fmt.Fprintf(&lg, "- Rows that failed to pull: %d\n", r.pullFailures)
	fmt.Fprintf(&lg, "- Rows that failed to push (retried next sync): %d\n", r.pushFailures)

	// Update sync timestamps. A failed pull can only be retried by pulling
	// the same window again; a failed push was touched and retries itself.
	if r.pullFailures == 0 {
		if _, err = s.MainDB.Exec("UPDATE sync SET timestamp=$1 WHERE machine='server';", r.nextServer); err != nil {
			fmt.Fprintf(&lg, "Error updating client with server timestamp: %v\n", err)
			return
		}
	} else {
		fmt.Fprintf(&lg, "Server watermark **not advanced** (%d rows failed to pull); the next sync pulls them again\n", r.pullFailures)
	}
	if r.touchFailures == 0 {
		if _, err = s.MainDB.Exec("UPDATE sync SET timestamp=$1 WHERE machine='client';", r.nextClient); err != nil {
			fmt.Fprintf(&lg, "Error updating client with client timestamp: %v\n", err)
			return
		}
	} else {
		fmt.Fprintf(&lg, "Client watermark **not advanced** (%d failed rows could not be marked for retry)\n", r.touchFailures)
	}
	fmt.Fprintf(&lg, "\nClient watermark: %s\n", r.nextClient)
	fmt.Fprintf(&lg, "Server watermark: %s", r.nextServer)
	return
}
