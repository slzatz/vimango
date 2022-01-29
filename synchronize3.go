package main

/** note that sqlite datetime('now') returns utc **/

import (
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type EntryPlusTag struct {
	Entry
	tag string
}

type TaskKeywordPairs struct {
	task_tid    int
	keyword_tid int
}

type TaskTag3 struct {
	task_tid int
	tag      string
}

type TaskKeyword3 struct {
	task_tid int
	keyword  string
}

func createBulkInsertQueryFTS3(n int, entries []NewEntryPlusTag) (query string, args []interface{}) {
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
		args[pos] = e.task_tid
		args[pos+1] = e.keyword_tid
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
			&tk.task_tid,
			&tk.keyword_tid,
		)
		tkPairs = append(tkPairs, tk)
	}
	return tkPairs
}

func TaskKeywordTids(dbase *sql.DB, plg io.Writer, tid int) []int { ////////////////////////////
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

func insertTaskKeywordTids(dbase *sql.DB, plg io.Writer, keyword_tid, entry_tid int) {
	_, err := dbase.Exec("INSERT INTO task_keyword (task_tid, keyword_tid) VALUES ($1, $2);",
		entry_tid, keyword_tid)
	if err != nil {
		fmt.Fprintf(plg, "Error in insertTaskKeywordTids: %v\n", err)
		return
	} else {
		fmt.Fprintf(plg, "Inserted into task_keyword entry tid **%d** and keyword_tid **%d**\n", entry_tid, keyword_tid)
	}
}
func getTags3(dbase *sql.DB, in string, plg io.Writer) []TaskTag3 {
	stmt := fmt.Sprintf("SELECT task_keyword.task_tid, keyword.title FROM task_keyword LEFT OUTER JOIN keyword ON keyword.tid=task_keyword.keyword_tid WHERE task_keyword.task_tid in (%s) ORDER BY task_keyword.task_tid;", in)
	rows, err := dbase.Query(stmt)
	if err != nil {
		fmt.Printf("Error in getTags_x: %v", err)
		return []TaskTag3{}
	}
	taskkeywords := make([]TaskKeyword3, 0)
	for rows.Next() {
		var tk TaskKeyword3
		rows.Scan(
			&tk.task_tid,
			&tk.keyword,
		)
		taskkeywords = append(taskkeywords, tk)
	}
	if len(taskkeywords) == 0 {
		return []TaskTag3{}
	}
	tasktags := make([]TaskTag3, 0, 1000)
	keywords := make([]string, 0, 5)
	var tt TaskTag3
	var tid int
	prev_tid := taskkeywords[0].task_tid
	for _, tk := range taskkeywords {
		tid = tk.task_tid
		if tid == prev_tid {
			keywords = append(keywords, tk.keyword)
		} else {
			tt.task_tid = prev_tid
			tt.tag = strings.Join(keywords, ",")
			tasktags = append(tasktags, tt)
			prev_tid = tid
			keywords = keywords[:0]
			keywords = append(keywords, tk.keyword)
		}
	}
	// need to get the last pair
	tt.task_tid = tid
	tt.tag = strings.Join(keywords, ",")
	tasktags = append(tasktags, tt)

	return tasktags
}

func synchronize3(reportOnly bool) (log string) {

	connect := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Postgres.Host,
		config.Postgres.Port,
		config.Postgres.User,
		config.Postgres.Password,
		config.Postgres.DB,
	)

	var lg strings.Builder
	var success bool
	defer func() {
		partialHost := "..." + strings.SplitAfterN(config.Postgres.Host, ".", 3)[2]
		text := fmt.Sprintf("server %s (%s)\n\n%s", config.Postgres.DB, partialHost, lg.String())
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

	pdb, err := sql.Open("postgres", connect)
	if err != nil {
		fmt.Fprintf(&lg, "Error opening postgres db: %v", err)
		return
	}
	defer pdb.Close()

	// Ping to connection
	err = pdb.Ping()
	if err != nil {
		fmt.Fprintf(&lg, "postgres ping failure!: %v", err)
		return
	}

	nn := 0 //number of changes

	row := db.QueryRow("SELECT timestamp FROM sync WHERE machine=$1;", "client")
	var raw_client_t string
	err = row.Scan(&raw_client_t)
	if err != nil {
		fmt.Fprintf(&lg, "Error retrieving last client sync: %v", err)
		return
	}
	//last_client_sync, _ := time.Parse("2006-01-02T15:04:05Z", client_t)
	// note postgres doesn't seem to require the below and seems to be really doing a date comparison
	client_t := raw_client_t[0:10] + " " + raw_client_t[11:19]

	var server_t string
	row = db.QueryRow("SELECT timestamp FROM sync WHERE machine=$1;", "server")
	err = row.Scan(&server_t)
	if err != nil {
		fmt.Fprintf(&lg, "Error retrieving last server sync: %v", err)
		return
	}

	fmt.Fprintf(&lg, "Local time is %v\n", time.Now())
	fmt.Fprintf(&lg, "UTC time is %v\n", time.Now().UTC())
	fmt.Fprintf(&lg, "Server last sync: %v\n", server_t)
	fmt.Fprintf(&lg, "(raw) Client last sync: %v\n", raw_client_t)
	fmt.Fprintf(&lg, "Client last sync: %v\n", client_t)

	//server updated contexts
	rows, err := pdb.Query("SELECT tid, title, star, modified FROM context WHERE context.modified > $1 AND context.deleted = $2;", server_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_contexts: %v", err)
		return
	}

	defer rows.Close()

	fmt.Fprint(&lg, "## Server Changes\n")

	var server_updated_contexts []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
			&c.star,
			&c.modified,
		)
		server_updated_contexts = append(server_updated_contexts, c)
	}
	if len(server_updated_contexts) > 0 {
		nn += len(server_updated_contexts)
		fmt.Fprintf(&lg, "- Updated `Contexts`(new and modified): **%d**\n", len(server_updated_contexts))
	} else {
		lg.WriteString("- No `Contexts` updated (new and modified).\n")
	}

	//server deleted contexts
	rows, err = pdb.Query("SELECT tid, title FROM context WHERE context.modified > $1 AND context.deleted = $2;", server_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_deleted_contexts: %v\n", err)
		return
	}

	var server_deleted_contexts []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
		)
		server_deleted_contexts = append(server_deleted_contexts, c)
	}
	if len(server_deleted_contexts) > 0 {
		nn += len(server_deleted_contexts)
		fmt.Fprintf(&lg, "- Deleted `Contexts`: %d\n", len(server_deleted_contexts))
	} else {
		lg.WriteString("- No `Contexts` deleted.\n")
	}

	//server updated folders
	rows, err = pdb.Query("SELECT tid, title, star, modified FROM folder WHERE folder.modified > $1 AND folder.deleted = $2;", server_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_folders: %v", err)
		return
	}

	//defer rows.Close()

	var server_updated_folders []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
			&c.star,
			&c.modified,
		)
		server_updated_folders = append(server_updated_folders, c)
	}
	if len(server_updated_folders) > 0 {
		nn += len(server_updated_folders)
		fmt.Fprintf(&lg, "- `Folders` Updated: %d\n", len(server_updated_folders))
	} else {
		lg.WriteString("- No `Folders` updated.\n")
	}

	//server deleted folders
	rows, err = pdb.Query("SELECT tid, title FROM folder WHERE folder.modified > $1 AND folder.deleted = $2;", server_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_deleted_folders: %v", err)
		return
	}

	var server_deleted_folders []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
		)
		server_deleted_folders = append(server_deleted_folders, c)
	}
	if len(server_deleted_folders) > 0 {
		nn += len(server_deleted_folders)
		fmt.Fprintf(&lg, "- Deleted `Folders`: %d\n", len(server_deleted_folders))
	} else {
		lg.WriteString("- No `Folders` deleted.\n")
	}

	//server updated keywords
	rows, err = pdb.Query("SELECT tid, title, star, modified FROM keyword WHERE keyword.modified > $1 AND keyword.deleted = $2;", server_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_keywords: %v", err)
		return
	}

	//defer rows.Close()

	var server_updated_keywords []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
			&c.star,
			&c.modified,
		)
		server_updated_keywords = append(server_updated_keywords, c)
	}
	if len(server_updated_keywords) > 0 {
		nn += len(server_updated_keywords)
		fmt.Fprintf(&lg, "- Updated `Keywords`: %d\n", len(server_updated_keywords))
	} else {
		lg.WriteString("- No `Keywords` updated.\n")
	}

	//server deleted keywords
	rows, err = pdb.Query("SELECT tid, title FROM keyword WHERE keyword.modified > $1 AND keyword.deleted = $2;", server_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_deleted_keywords: %v", err)
		return
	}

	var server_deleted_keywords []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
			//&c.star,
			//&c.modified,
		)
		server_deleted_keywords = append(server_deleted_keywords, c)
	}
	if len(server_deleted_keywords) > 0 {
		nn += len(server_deleted_keywords)
		fmt.Fprintf(&lg, "- Deleted server `Keywords`: %d\n", len(server_deleted_keywords))
	} else {
		lg.WriteString("- No `Keywords` deleted.\n")
	}

	//server updated entries
	rows, err = pdb.Query("SELECT tid, title, star, note, modified, added, archived, context_tid, folder_tid FROM task WHERE modified > $1 AND deleted = $2 ORDER BY tid;", server_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_entries: %v", err)
		return
	}

	var server_updated_entries []NewEntryPlusTag
	for rows.Next() {
		var e NewEntryPlusTag
		rows.Scan(
			&e.tid,
			&e.title,
			&e.star,
			&e.note,
			&e.modified,
			&e.added,
			&e.archived,
			&e.context_tid,
			&e.folder_tid,
		)
		server_updated_entries = append(server_updated_entries, e)
	}
	if len(server_updated_entries) > 0 {
		nn += len(server_updated_entries)
		fmt.Fprintf(&lg, "- Updated `Entries`: %d\n", len(server_updated_entries))
	} else {
		lg.WriteString("- No `Entries` updated.\n")
	}
	if len(server_updated_entries) < 100 {
		for _, e := range server_updated_entries {
			fmt.Fprintf(&lg, "    - tid: %d star: %t *%q* folder_tid: %d context_tid: %d  modified: %v\n", e.tid, e.star, truncate(e.title, 15), e.context_tid, e.folder_tid, tc(e.modified, 19, false))
		}
	}

	//server deleted entries
	rows, err = pdb.Query("SELECT tid, title FROM task WHERE modified > $1 AND deleted = $2;", server_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_deleted_entries: %v", err)
		return
	}

	var server_deleted_entries []Entry
	for rows.Next() {
		var e Entry
		rows.Scan(
			&e.tid,
			&e.title,
		)
		server_deleted_entries = append(server_deleted_entries, e)
	}
	if len(server_deleted_entries) > 0 {
		nn += len(server_deleted_entries)
		fmt.Fprintf(&lg, "- Deleted `Entries`: %d\n", len(server_deleted_entries))
	} else {
		lg.WriteString("- No `Entries` deleted.\n")
	}

	//Client changes

	//client updated contexts
	rows, err = db.Query("SELECT id, tid, title, star, modified FROM context WHERE substr(context.modified, 1, 19) > $1 AND context.deleted = $2;", client_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_updated_contexts: %v", err)
		return
	}

	//defer rows.Close()
	fmt.Fprint(&lg, "## Client Changes\n")

	var client_updated_contexts []Container
	for rows.Next() {
		var c Container
		var tid sql.NullInt64
		rows.Scan(
			&c.id,
			&tid,
			&c.title,
			&c.star,
			&c.modified,
		)
		c.tid = int(tid.Int64)
		client_updated_contexts = append(client_updated_contexts, c)
	}
	if len(client_updated_contexts) > 0 {
		nn += len(client_updated_contexts)
		fmt.Fprintf(&lg, "- `Contexts` updated: %d\n", len(client_updated_contexts))
	} else {
		lg.WriteString("- No `Contexts` updated.\n")
	}
	for _, c := range client_updated_contexts {
		fmt.Fprintf(&lg, "    - id: %d; tid: %d %q; modified: %v\n", c.id, c.tid, tc(c.title, 15, true), tc(c.modified, 19, false))
	}

	//client deleted contexts
	rows, err = db.Query("SELECT id, tid, title FROM context WHERE substr(context.modified, 1, 19) > $1 AND context.deleted = $2;", client_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_deleted_contexts: %v", err)
		return
	}

	var client_deleted_contexts []Container
	for rows.Next() {
		var c Container
		var tid sql.NullInt64
		rows.Scan(
			&c.id,
			&tid,
			&c.title,
		)
		c.tid = int(tid.Int64)
		client_deleted_contexts = append(client_deleted_contexts, c)
	}
	if len(client_deleted_contexts) > 0 {
		nn += len(client_deleted_contexts)
		fmt.Fprintf(&lg, "- Deleted client `Contexts`: %d\n", len(client_deleted_contexts))
		for _, e := range client_deleted_contexts {
			fmt.Fprintf(&lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		lg.WriteString("- No `Contexts` deleted.\n")
	}

	//client updated folders
	rows, err = db.Query("SELECT id, tid, title, star, modified FROM folder WHERE substr(folder.modified, 1, 19) > $1 AND folder.deleted = $2;", client_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_updated_folders: %v", err)
		return
	}

	//defer rows.Close()

	var client_updated_folders []Container
	for rows.Next() {
		var c Container
		var tid sql.NullInt64
		rows.Scan(
			&c.id,
			&tid,
			&c.title,
			&c.star,
			&c.modified,
		)
		c.tid = int(tid.Int64)
		client_updated_folders = append(client_updated_folders, c)
	}
	if len(client_updated_folders) > 0 {
		nn += len(client_updated_folders)
		fmt.Fprintf(&lg, "- Updated `Folders`: %d\n", len(client_updated_folders))
	} else {
		lg.WriteString("- No `Folders` updated.\n")
	}
	for _, c := range client_updated_folders {
		fmt.Fprintf(&lg, "    - id: %d; tid: %d %q; modified: %v\n", c.id, c.tid, tc(c.title, 15, true), tc(c.modified, 19, false))
	}

	//client deleted folders
	rows, err = db.Query("SELECT id, tid, title FROM folder WHERE substr(folder.modified, 1, 19) > $1 AND folder.deleted = $2;", client_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_deleted_folders: %v", err)
		return
	}

	var client_deleted_folders []Container
	for rows.Next() {
		var c Container
		var tid sql.NullInt64
		rows.Scan(
			&c.id,
			&tid,
			&c.title,
		)
		c.tid = int(tid.Int64)
		client_deleted_folders = append(client_deleted_folders, c)
	}
	if len(client_deleted_folders) > 0 {
		nn += len(client_deleted_folders)
		fmt.Fprintf(&lg, "- Deleted client `Folders`: %d\n", len(client_updated_folders))
		for _, e := range client_deleted_folders {
			fmt.Fprintf(&lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		lg.WriteString("- No `Folders` deleted.\n")
	}

	//client updated keywords
	rows, err = db.Query("SELECT id, tid, title, star, modified FROM keyword WHERE substr(keyword.modified, 1, 19)  > $1 AND keyword.deleted = $2;", client_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_updated_keywords: %v", err)
		return
	}

	var client_updated_keywords []Container
	for rows.Next() {
		var c Container
		var tid sql.NullInt64
		rows.Scan(
			&c.id,
			&tid,
			&c.title,
			&c.star,
			&c.modified,
		)
		c.tid = int(tid.Int64)
		client_updated_keywords = append(client_updated_keywords, c)
	}
	if len(client_updated_keywords) > 0 {
		nn += len(client_updated_keywords)
		fmt.Fprintf(&lg, "- Updated `Keywords`: %d\n", len(client_updated_keywords))
	} else {
		lg.WriteString("- No `Keywords` updated.\n")
	}
	for _, c := range client_updated_keywords {
		fmt.Fprintf(&lg, "    - id: %d; tid: %d %q; modified: %v\n", c.id, c.tid, tc(c.title, 15, true), tc(c.modified, 19, false))
	}

	//client deleted keywords
	rows, err = db.Query("SELECT id, tid, title FROM keyword WHERE substr(keyword.modified, 1, 19) > $1 AND keyword.deleted = $2;", client_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_deleted_keywords: %v", err)
		return
	}

	var client_deleted_keywords []Container
	for rows.Next() {
		var c Container
		var tid sql.NullInt64
		rows.Scan(
			&c.id,
			&tid,
			&c.title,
		)
		c.tid = int(tid.Int64)
		client_deleted_keywords = append(client_deleted_keywords, c)
	}
	if len(client_deleted_keywords) > 0 {
		nn += len(client_deleted_keywords)
		fmt.Fprintf(&lg, "- Deleted `Keywords`: %d\n", len(client_deleted_keywords))
		for _, e := range client_deleted_keywords {
			fmt.Fprintf(&lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		lg.WriteString("- No `Keywords` deleted.\n")
	}

	//client updated entries
	rows, err = db.Query("SELECT id, tid, title, star, note, modified, added, archived, context_tid, folder_tid FROM task WHERE substr(modified, 1, 19)  > ? AND deleted = ?;", client_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_updated_entries: %v", err)
		return
	}

	var client_updated_entries []NewEntry
	for rows.Next() {
		var e NewEntry
		var tid sql.NullInt64
		rows.Scan(
			&e.id,
			&tid,
			&e.title,
			&e.star,
			&e.note,
			&e.modified,
			&e.added,
			&e.archived,
			&e.context_tid,
			&e.folder_tid,
		)
		e.tid = int(tid.Int64)
		client_updated_entries = append(client_updated_entries, e)
	}
	if len(client_updated_entries) > 0 {
		nn += len(client_updated_entries)
		fmt.Fprintf(&lg, "- Updated `Entries`: %d\n", len(client_updated_entries))
	} else {
		lg.WriteString("- No `Entries` updated.\n")
	}
	for _, e := range client_updated_entries {
		fmt.Fprintf(&lg, "    - id: %d tid: %d star: %t *%q* context_tid: %d folder_tid: %d  modified: %v\n", e.id, e.tid, e.star, truncate(e.title, 15), e.context_tid, e.folder_tid, tc(e.modified, 19, false))
	}

	//client deleted entries
	rows, err = db.Query("SELECT id, tid, title FROM task WHERE substr(modified, 1, 19) > $1 AND deleted = $2;", client_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error with retrieving client deleted entries: %v", err)
		return
	}
	var client_deleted_entries []Entry
	for rows.Next() {
		var e Entry
		var tid sql.NullInt64
		rows.Scan(
			&e.id,
			&tid,
			&e.title,
		)
		e.tid = int(tid.Int64)
		client_deleted_entries = append(client_deleted_entries, e)
	}
	if len(client_deleted_entries) > 0 {
		nn += len(client_deleted_entries)
		fmt.Fprintf(&lg, "- Deleted `Entries`: %d\n", len(client_deleted_entries))
		for _, e := range client_deleted_entries {
			fmt.Fprintf(&lg, "    - id: %d tid: %d *%q*\n", e.id, e.tid, truncate(e.title, 15))
		}
	} else {
		lg.WriteString("- No `Entries` deleted.\n")
	}

	fmt.Fprintf(&lg, "\nNumber of changes (before accounting for server/client conflicts) is: **%d**\n\n", nn)
	if reportOnly {
		// note there is a defer log.String()
		return
	}

	/****************below is where changes start***********************************/

	//updated server contexts -> client
	for _, c := range server_updated_contexts {
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM context WHERE tid=?)", c.tid).Scan(&exists)
		if err != nil {
			fmt.Fprintf(&lg, "Error SELECT EXISTS(SELECT 1 FROM context ...: %v\n", err)
			continue
		}

		if exists {
			_, err := db.Exec("UPDATE context SET title=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error updating sqlite for a context with tid: %v: %w\n", c.tid, err)
			} else {
				fmt.Fprintf(&lg, "Updated local context: %q with tid: %v\n", c.title, c.tid)
			}
		} else {
			_, err := db.Exec("INSERT INTO context (tid, title, star, modified, deleted) VALUES (?,?,?, datetime('now'), false);",
				c.tid, c.title, c.star)
			if err != nil {
				fmt.Fprintf(&lg, "Error inserting new context into sqlite: %v\n", err)
			}
		}
	}

	for _, c := range client_updated_contexts {
		var exists bool
		err := pdb.QueryRow("SELECT EXISTS(SELECT 1 FROM context WHERE tid=$1);", c.tid).Scan(&exists)
		if err != nil {
			fmt.Fprintf(&lg, "Error SELECT EXISTS(SELECT 1 FROM context ...: %v\n", err)
			continue
		}

		if exists {
			_, err := pdb.Exec("UPDATE context SET title=$1, star=$2, modified=now() WHERE tid=$3;", c.title, c.star, c.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error updating postgres for a context with tid: %d: %v\n", c.tid, err)
			} else {
				fmt.Fprintf(&lg, "Updated server context: %q with tid: %v\n", c.title, c.tid)
			}
		} else {
			var tid int
			err := pdb.QueryRow("INSERT INTO context (title, star, modified, deleted) VALUES ($1, $2, now(), false) RETURNING tid;",
				c.title, c.star).Scan(&tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error inserting new context into postgres and returning tid: %v\n", err)
				continue
			}
			_, err = db.Exec("UPDATE context SET tid=? WHERE id=?;", tid, c.id)
			if err != nil {
				fmt.Fprintf(&lg, "Error on UPDATE context SET tid ...: %v\n", err)
			} else {
				fmt.Fprintf(&lg, "Inserted server context %q and updated local tid to %d\n", c.title, tid)
			}
		}
	}

	for _, c := range server_updated_folders {
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM folder WHERE tid=?);", c.tid).Scan(&exists)
		if err != nil {
			fmt.Fprintf(&lg, "Error SELECT EXISTS(SELECT 1 FROM folder ...: %v\n", err)
			continue
		}

		if exists {
			_, err := db.Exec("UPDATE folder SET title=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error updating sqlite for a folder with tid: %d: %v\n", c.tid, err)
			} else {
				fmt.Fprintf(&lg, "Updated local folder: %q with tid: %v\n", c.title, c.tid)
			}
		} else {
			_, err := db.Exec("INSERT INTO folder (tid, title, star, modified, deleted) VALUES (?,?,?, datetime('now'), false);",
				c.tid, c.title, c.star)
			if err != nil {
				fmt.Fprintf(&lg, "Error inserting new folder into sqlite: %v\n", err)
			}
		}
	}

	for _, c := range client_updated_folders {
		var exists bool
		err := pdb.QueryRow("SELECT EXISTS(SELECT 1 FROM folder WHERE tid=$1);", c.tid).Scan(&exists)
		if err != nil {
			fmt.Fprintf(&lg, "Error SELECT EXISTS(SELECT 1 FROM folder ...: %v\n", err)
			continue
		}

		if exists {
			_, err := pdb.Exec("UPDATE folder SET title=$1, star=$2, modified=now() WHERE tid=$3;", c.title, c.star, c.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error updating postgres for a folder with tid: %v: %w\n", c.tid, err)
			} else {
				fmt.Fprintf(&lg, "Updated server folder: %q with tid: %v\n", c.title, c.tid)
			}
		} else {
			var tid int
			err := pdb.QueryRow("INSERT INTO folder (title, star, modified, deleted) VALUES ($1, $2, now(), false) RETURNING tid;",
				c.title, c.star).Scan(&tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error inserting new folder into postgres and returning tid: %v\n", err)
				continue
			}
			_, err = db.Exec("UPDATE folder SET tid=? WHERE id=?;", tid, c.id)
			if err != nil {
				fmt.Fprintf(&lg, "Error on UPDATE folder SET tid ...: %v\n", err)
			} else {
				fmt.Fprintf(&lg, "Inserted server folder %q and updated local tid to %d\n", c.title, tid)
			}
		}
	}

	for _, c := range server_updated_keywords {
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM keyword WHERE tid=?);", c.tid).Scan(&exists)
		if err != nil {
			fmt.Fprintf(&lg, "Error SELECT EXISTS(SELECT 1 FROM keyword ...: %v\n", err)
			continue
		}

		if exists {
			_, err := db.Exec("UPDATE keyword SET title=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error updating sqlite for a keyword with tid: %v: %w\n", c.tid, err)
			} else {
				fmt.Fprintf(&lg, "Updated local keyword: %q with tid: %v\n", c.title, c.tid)
			}
		} else {
			_, err := db.Exec("INSERT INTO keyword (tid, title, star, modified, deleted) VALUES (?,?,?, datetime('now'), false);",
				c.tid, c.title, c.star)
			if err != nil {
				fmt.Fprintf(&lg, "Error inserting new keyword into sqlite: %v\n", err)
			}
		}
	}

	for _, c := range client_updated_keywords {
		var exists bool
		err := pdb.QueryRow("SELECT EXISTS(SELECT 1 FROM keyword WHERE tid=$1)", c.tid).Scan(&exists)
		if err != nil {
			fmt.Fprintf(&lg, "Error SELECT EXISTS(SELECT 1 FROM keyword ...: %v\n", err)
			continue
		}

		if exists {
			_, err := pdb.Exec("UPDATE keyword SET title=$1, star=$2, modified=now() WHERE tid=$3;", c.title, c.star, c.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error updating sqlite for a keyword with tid: %d: %v\n", c.tid, err)
			} else {
				fmt.Fprintf(&lg, "Updated local keyword: %q with tid: %v\n", c.title, c.tid)
			}
		} else {
			var tid int
			err := pdb.QueryRow("INSERT INTO keyword (title, star, modified, deleted) VALUES ($1, $2, now(), false) RETURNING tid;",
				c.title, c.star).Scan(&tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error inserting new keyword into postgres and returning tid: %v\n", err)
				continue
			}
			_, err = db.Exec("UPDATE keyword SET tid=? WHERE id=?;", tid, c.id)
			if err != nil {
				fmt.Fprintf(&lg, "Error on UPDATE keyword SET tid ...: %v\n", err)
			} else {
				fmt.Fprintf(&lg, "Inserted server keyword %q and updated local tid to %d\n", c.title, tid)
			}
		}
	}

	/**********should come before container deletes to change tasks here*****************/
	server_updated_entries_tids := make(map[int]struct{})
	var task_tids []string
	for _, e := range server_updated_entries {
		_, err := db.Exec("INSERT INTO task (tid, title, star, added, archived, context_tid, folder_tid, note, modified, deleted) VALUES"+
			"(?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), false) ON CONFLICT(tid) DO UPDATE SET "+
			"title=excluded.title, star=excluded.star, archived=excluded.archived, context_tid=excluded.context_tid, "+
			"folder_tid=excluded.folder_tid, note=excluded.note, modified=datetime('now');",
			e.tid, e.title, e.star, e.added, e.archived, e.context_tid, e.folder_tid, e.note)
		if err != nil {
			fmt.Fprintf(&lg, "**Error** in INSERT ... ON CONFLICT for id/tid %d %q: %v\n", e.id, e.title, err)
			continue
		} else {
			fmt.Fprintf(&lg, "Inserted or updated client entry %q with tid **%d**\n", e.title, e.tid)
		}
		task_tids = append(task_tids, strconv.Itoa(e.tid))
		server_updated_entries_tids[e.tid] = struct{}{}
	}
	if len(server_updated_entries) != 0 {
		in := strings.Join(task_tids, ",")

		// need to delete all keywords for changed entries first
		stmt := fmt.Sprintf("DELETE FROM task_keyword WHERE task_tid in (%s);", in)
		_, err = db.Exec(stmt)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting from client task_keyword for server tids %s: %v\n", in, err)
		}

		// need to delete all rows for changed entries in fts
		// note only rows for existing client entries will be in fts
		stmt = fmt.Sprintf("DELETE FROM fts WHERE tid in (%s);", in)
		_, err = fts_db.Exec(stmt)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting from fts from server ids %s: %v\n", in, err)
		}
		tks := getTaskKeywordPairs(pdb, in, &lg)
		if len(tks) != 0 {
			query, args := createBulkInsertQueryTaskKeywordPairs(len(tks), tks)
			err = bulkInsert(db, query, args)
			if err != nil {
				fmt.Fprintf(&lg, "%v\n", err)
			} else {
				fmt.Fprintf(&lg, "Keywords updated for task tids: %s\n", in)
			}
			tags := getTags3(pdb, in, &lg)
			//entries and tags must be sorted before updating server_updated_entries with tag
			/* there is an ORDER BY so also shouldn't need to do the sorts
			sort.Slice(tags, func(i, j int) bool {
				return tags[i].task_id < tags[j].task_id
			})
				sort.Slice(server_updated_entries, func(i, j int) bool {
					return server_updated_entries[i].id < server_updated_entries[j].id
				})
			*/
			i := 0
			for j := 0; ; j++ {
				// below check shouldn't be necessary
				if j == len(server_updated_entries) {
					break
				}
				entry := &server_updated_entries[j]
				if entry.tid == tags[i].task_tid {
					entry.tag = tags[i].tag
					fmt.Fprintf(&lg, "FTS tag will be updated for tid: %d, tag: %s\n", entry.tid, entry.tag)
					i += 1
					if i == len(tags) {
						break
					}
				}
			}
		}
		query, args := createBulkInsertQueryFTS3(len(server_updated_entries), server_updated_entries)
		err = bulkInsert(fts_db, query, args)
		if err != nil {
			fmt.Fprintf(&lg, "%v", err)
		} else {
			fmt.Fprintf(&lg, "FTS entries updated for task tids: %s\n", in)
		}
	}

	for _, e := range client_updated_entries {
		// server wins if both client and server have updated an item
		if _, found := server_updated_entries_tids[e.tid]; found {
			fmt.Fprintf(&lg, "Server won: client entry %q with id %d and tid %d was updated by server\n", truncate(e.title, 15), e.id, e.tid)
			continue
		}

		var tid int
		if e.tid < 1 {
			err := pdb.QueryRow("INSERT INTO task (title, star, added, archived, context_tid, folder_tid, note, modified, deleted) "+
				"VALUES ($1, $2, $3, $4, $5, $6, $7, now(), false)  RETURNING tid",
				e.title, e.star, e.added, e.archived, e.context_tid, e.folder_tid, e.note).Scan(&tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error inserting server entry: %v", err)
				continue
			}
			_, err = db.Exec("UPDATE task SET tid=? WHERE id=?;", tid, e.id)
			if err != nil {
				fmt.Fprintf(&lg, "Error setting tid for client entry %q with id %d to tid %d: %v\n", truncate(e.title, 15), e.id, tid, err)
				continue
			}
			/*For new entries there are no records in FTS
			_, err = fts_db.Exec("UPDATE fts SET tid=$1 WHERE tid=$2;", tid, e.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error in Update tid in fts: %v\n", err)
			}
			*/
			fmt.Fprintf(&lg, "Created new server entry *%q* with tid **%d**\n", truncate(e.title, 15), tid)
			fmt.Fprintf(&lg, "and set tid for client entry with id **%d**\n", e.id)
		} else {
			_, err := pdb.Exec("UPDATE task SET title=$1, star=$2, context_tid=$3, folder_tid=$4, note=$5, archived=$6, modified=now() WHERE tid=$7;",
				e.title, e.star, e.context_tid, e.folder_tid, e.note, e.archived, e.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error updating server entry: %v", err)
				continue
			}
			tid = e.tid
			fmt.Fprintf(&lg, "Updated server entry *%q* with tid **%d**\n", truncate(e.title, 15), tid)
		}

		// Update the server entry's keywords
		_, err := pdb.Exec("DELETE FROM task_keyword WHERE task_tid=$1;", tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting from task_keyword from server tid %d: %v\n", tid, err)
			continue
		}
		kwTids := TaskKeywordTids(db, &lg, tid)
		for _, kwTid := range kwTids {
			insertTaskKeywordTids(pdb, &lg, kwTid, tid)
		}
	}

	// server deleted entries
	for _, e := range server_deleted_entries {
		_, err = db.Exec("DELETE FROM task_keyword WHERE task_tid=?;", e.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting task_keyword client rows where entry tid = %d: %v\n", e.tid, err)
			continue
		}

		_, err := db.Exec("DELETE FROM task WHERE tid=?;", e.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting client entry %q with tid %d: %v\n", tc(e.title, 15, true), e.tid, err)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client entry %q with tid %d\n", truncate(e.title, 15), e.tid)
		fmt.Fprintf(&lg, "and on client deleted task_tid %d from task_keyword\n", e.tid)
	}

	// client deleted entries
	for _, e := range client_deleted_entries {

		_, err = db.Exec("DELETE FROM task_keyword WHERE task_tid=?;", e.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting task_keyword client rows where entry tid = %d: %v\n", e.tid, err)
			continue
		}
		_, err = db.Exec("DELETE FROM task WHERE id=?", e.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting client entry %q with id %d: %v\n", tc(e.title, 15, true), e.id, err)
			continue
		}

		fmt.Fprintf(&lg, "Deleted client entry %q with id %d\n", tc(e.title, 15, true), e.id)
		fmt.Fprintf(&lg, "and on client deleted task_tid %d from task_keyword\n", e.tid)

		// since on server, we just set deleted to true
		// since may have to sync with other clients
		// also note client task may have been new (never synced) and deleted (tid=0)
		if e.tid < 1 {
			fmt.Fprintf(&lg, "There is no server entry to delete for client id %d\n", e.id)
			continue
		}

		_, err := pdb.Exec("UPDATE task SET deleted=true, modified=now() WHERE tid=$1", e.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error setting server entry with id %d to deleted: %v\n", e.tid, err)
			continue
		}
		fmt.Fprintf(&lg, "Updated server entry %q with id %d to **deleted = true**\n", truncate(e.title, 15), e.tid)

		_, err = pdb.Exec("DELETE FROM task_keyword WHERE task_tid=$1;", e.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting task_keyword server rows where entry tid = %d: %v\n", e.tid, err)
			continue
		}
		fmt.Fprintf(&lg, "and on server deleted task_tid %d from task_keyword\n", e.tid)

	}

	//server_deleted_contexts - start here
	for _, c := range server_deleted_contexts {
		// I think the task changes may not be necessary because only a previous client sync can delete server context
		res, err := pdb.Exec("Update task SET context_tid=1, modified=now() WHERE context_tid=$1;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change server/postgres entry context to 'none' for a deleted context: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of server entries that were changed to 'none' (might be zero): **%d**\n", rowsAffected)
		}
		res, err = db.Exec("Update task SET context_tid=1, modified=datetime('now') WHERE context_tid=?;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change client/sqlite entry contexts for a deleted context: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of client entries that were changed to 'none' (might be zero): **%d**\n", rowsAffected)
		}

		// could use returning to get the id of the context that was deleted - would just be for log
		_, err = db.Exec("DELETE FROM context WHERE tid=?", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting local context %q with tid = %d", c.title, c.tid)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client context %s with tid %d", c.title, c.tid)
	}

	// client deleted contexts
	for _, c := range client_deleted_contexts {
		res, err := pdb.Exec("Update task SET context_tid=1, modified=now() WHERE context_tid=$1;", c.tid) //?modified=now()
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change server/postgres entry contexts for a deleted context: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of server entries that were changed to 'none': **%d**\n", rowsAffected)
		}
		res, err = db.Exec("Update task SET context_tid=1, modified=datetime('now') WHERE context_tid=?;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change client/sqlite entry contexts for a deleted context: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of client entries that were changed to 'none': **%d**\n", rowsAffected)
		}
		// since on server, we just set deleted to true
		// since may have to sync with other clients
		_, err = pdb.Exec("UPDATE context SET deleted=true, modified=now() WHERE context.tid=$1", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error (pdb.Exec) setting server context %s with tid = %d to deleted: %v\n", c.title, c.tid, err)
			continue
		}
		_, err = db.Exec("DELETE FROM context WHERE id=?", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting local context %q with id %d: %v", c.title, c.id, err)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client context %s: id %d and updated server context with tid %d to deleted = true", c.title, c.id, c.tid)
	}

	//server_deleted_folders
	for _, c := range server_deleted_folders {
		// I think the task changes may not be necessary because only a previous client sync can delete server context
		// and that previous client sync should have changed the relevant tasks to 'none'
		res, err := pdb.Exec("Update task SET folder_tid=1, modified=now() WHERE folder_tid=$1;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change server/postgres entry folder to 'none' for a deleted folder: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of server entries that were changed to 'none' (might be zero): **%d**\n", rowsAffected)
		}
		res, err = db.Exec("Update task SET folder_tid=1, modified=datetime('now') WHERE folder_tid=?;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change client/sqlite entry folders for a deleted folder: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of client entries that were changed to 'none' (might be zero): **%d**\n", rowsAffected)
		}

		// could use returning to get the id of the folder that was deleted - would just be for log
		_, err = db.Exec("DELETE FROM folder WHERE tid=?", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting local folder %q with tid = %d", c.title, c.tid)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client folder %q with tid %d", c.title, c.tid)
	}

	// client deleted folders
	for _, c := range client_deleted_folders {
		res, err := pdb.Exec("Update task SET folder_tid=1, modified=now() WHERE folder_tid=$1;", c.tid) //?modified=now()
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change server/postgres entry folders for a deleted folder: %v\n", err)
			continue
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of server entries that were changed to 'none': **%d**\n", rowsAffected)
		}
		res, err = db.Exec("Update task SET folder_tid=1, modified=datetime('now') WHERE folder_tid=?;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change client/sqlite entry folders for a deleted folder: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of client entries that were changed to 'none': **%d**\n", rowsAffected)
		}
		// since on server, we just set deleted to true
		// since may have to sync with other clients
		_, err = pdb.Exec("UPDATE folder SET deleted=true, modified=now() WHERE folder.tid=$1", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error (pdb.Exec) setting server folder %s with id = %v to deleted: %v\n", c.title, c.tid, err)
			continue
		}
		_, err = db.Exec("DELETE FROM folder WHERE id=?", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting local folder %q with id %d: %v", c.title, c.id, err)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client folder %q: id %d and updated server folder with tid %d to deleted = true", c.title, c.id, c.tid)
	}

	//server_deleted_keywords - start here
	for _, c := range server_deleted_keywords {
		_, err := db.Exec("DELETE FROM task_keyword WHERE keyword_tid=?;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting from task_keyword client keyword_tid: %d", c.tid)
		}
		_, err = db.Exec("DELETE FROM keyword WHERE tid=?", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting client keyword with tid = %v", c.tid)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client keyword %q with tid %d", truncate(c.title, 15), c.tid)
	}

	// client deleted keywords
	for _, c := range client_deleted_keywords {
		_, err = pdb.Exec("DELETE FROM task_keyword WHERE keyword_tid=$1;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting from task_keyword server keyword_tid: %d", c.tid)
		}
		_, err = db.Exec("DELETE FROM task_keyword WHERE keyword_tid=?;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting client task_keyword with id %d", c.tid)
		}
		// since on server, we just set deleted to true
		// since may have to sync with other clients
		_, err := pdb.Exec("UPDATE keyword SET deleted=true WHERE keyword.tid=$1", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error (pdb.Exec) setting server keyword %s with id %d to deleted:%v\n", c.title, c.tid, err)
			continue
		}
		_, err = db.Exec("DELETE FROM keyword WHERE id=?", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting client keyword %s with id %d: %v\n", c.title, c.id, err)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client keyword %s: id %d and updated server keyword with id %d to deleted = true", c.title, c.id, c.tid)
	}
	/*********************end of sync changes*************************/

	var server_ts string
	row = pdb.QueryRow("SELECT now();")
	err = row.Scan(&server_ts)
	if err != nil {
		sess.showOrgMessage("Error with getting current time from server: %w", err)
		return
	}
	_, err = db.Exec("UPDATE sync SET timestamp=$1 WHERE machine='server';", server_ts)
	if err != nil {
		sess.showOrgMessage("Error updating client with server timestamp: %w", err)
		return
	}
	_, err = db.Exec("UPDATE sync SET timestamp=datetime('now') WHERE machine='client';")
	if err != nil {
		sess.showOrgMessage("Error updating client with client timestamp: %w", err)
		return
	}
	var client_ts string
	row = db.QueryRow("SELECT datetime('now');")
	err = row.Scan(&client_ts)
	if err != nil {
		sess.showOrgMessage("Error with getting current time from client: %w", err)
		return
	}
	fmt.Fprintf(&lg, "\nClient UTC timestamp: %s\n", client_ts)
	fmt.Fprintf(&lg, "Server UTC timestamp: %s", strings.Replace(tc(server_ts, 19, false), "T", " ", 1))
	//fmt.Fprintf(&lg, "\n### Synchronization succeeded")
	success = true

	return
}
