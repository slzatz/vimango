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

func getTaskKeywordIds_x(dbase *sql.DB, in string, plg io.Writer) []TaskKeywordIds {
	//rows, err := pdb.Query("Select task_id, keyword_id FROM task_keyword ORDER BY task_id;")
	stmt := fmt.Sprintf("SELECT task_id, keyword_id FROM task_keyword WHERE task_id IN (%s);", in)
	rows, err := dbase.Query(stmt)
	if err != nil {
		println(err)
		return []TaskKeywordIds{}
	}
	taskKeywordIds := make([]TaskKeywordIds, 0)
	for rows.Next() {
		var tk TaskKeywordIds
		rows.Scan(
			&tk.task_id,
			&tk.keyword_id,
		)
		taskKeywordIds = append(taskKeywordIds, tk)
	}
	return taskKeywordIds
}

func getTags_x(dbase *sql.DB, in string, plg io.Writer) []TaskTag {
	stmt := fmt.Sprintf("SELECT task_keyword.task_id, keyword.name FROM task_keyword LEFT OUTER JOIN keyword ON keyword.id=task_keyword.keyword_id WHERE task_keyword.task_id in (%s) ORDER BY task_keyword.task_id;", in)
	rows, err := dbase.Query(stmt)
	if err != nil {
		fmt.Printf("Error in getTags_x: %v", err)
		return []TaskTag{}
	}
	taskkeywords := make([]TaskKeyword, 0)
	for rows.Next() {
		var tk TaskKeyword
		rows.Scan(
			&tk.task_id,
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
	var id int
	prev_id := taskkeywords[0].task_id
	for _, tk := range taskkeywords {
		id = tk.task_id
		if id == prev_id {
			keywords = append(keywords, tk.keyword)
		} else {
			tt.task_id = prev_id
			tt.tag = strings.Join(keywords, ",")
			tasktags = append(tasktags, tt)
			prev_id = id
			keywords = keywords[:0]
			keywords = append(keywords, tk.keyword)
		}
	}
	// need to get the last pair
	tt.task_id = id
	tt.tag = strings.Join(keywords, ",")
	tasktags = append(tasktags, tt)

	return tasktags
}

/*
func addServerTaskKeywordIds(dbase *sql.DB, plg io.Writer, keyword_id, entry_id int) {
	_, err := dbase.Exec("INSERT INTO task_keyword (task_id, keyword_id) VALUES ($1, $2);",
		entry_id, keyword_id)
	if err != nil {
		fmt.Fprintf(plg, "Error in addTaskKeywordS = INSERT INTO task_keyword: %v\n", err)
		return
	} else {
		fmt.Fprintf(plg, "Inserted into task_keyword entry id **%d** and keyword_id **%d**\n", entry_id, keyword_id)
	}
}

func addClientTaskKeywordTids(dbase *sql.DB, plg io.Writer, keyword_tid, entry_tid int) {
	_, err := dbase.Exec("INSERT INTO task_keyword (task_tid, keyword_tid) VALUES ($1, $2);",
		entry_tid, keyword_tid)
	if err != nil {
		fmt.Fprintf(plg, "Error in addTaskKeywordS = INSERT INTO task_keyword: %v\n", err)
		return
	} else {
		fmt.Fprintf(plg, "Inserted into task_keyword task_tid **%d** and keyword_tid **%d**\n", entry_tid, keyword_tid)
	}
}

func serverTaskKeywords(dbase *sql.DB, plg io.Writer, id int) []string {
	rows, err := dbase.Query("SELECT keyword.name FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.id=task_keyword.keyword_id WHERE task_keyword.task_id=$1;", id)
	if err != nil {
		fmt.Fprintf(plg, "Error in serverTaskKeywords: %v\n", err)
	}
	defer rows.Close()

	kk := []string{}
	for rows.Next() {
		var name string

		err = rows.Scan(&name)
		kk = append(kk, name)
	}
	return kk
}

// not in use
func clientTaskKeywords(dbase *sql.DB, plg io.Writer, tid int) []string {
	rows, err := dbase.Query("SELECT keyword.name FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.tid=task_keyword.keyword_tid WHERE task_keyword.task_tid=$1;", tid)
	if err != nil {
		fmt.Fprintf(plg, "Error in clientTaskKeywords: %v\n", err)
	}
	defer rows.Close()

	kk := []string{}
	for rows.Next() {
		var name string

		err = rows.Scan(&name)
		kk = append(kk, name)
	}
	return kk
}

func serverTaskKeywordIds_x(dbase *sql.DB, plg io.Writer, id int) []int { ////////////////////////////
	rows, err := dbase.Query("SELECT keyword.id FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.id=task_keyword.keyword_id WHERE task_keyword.task_id=$1;", id)
	if err != nil {
		fmt.Fprintf(plg, "Error in getTaskKeywordsS: %v\n", err)
	}
	defer rows.Close()

	keywordIds := []int{}
	for rows.Next() {
		var keywordId int

		err = rows.Scan(&keywordId)
		keywordIds = append(keywordIds, keywordId)
	}
	return keywordIds
}

func clientTaskKeywordTids(dbase *sql.DB, plg io.Writer, tid int) []int { ////////////////////////////
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
*/

func synchronize2(reportOnly bool) (log string) {

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
		log = lg.String()
		if reportOnly {
			log = "### Testing without Syncing\n\n" + log
			return
		}
		if success {
			log = "### Synchronization succeeded\n\n" + log
		} else {
			log = "### Synchronization failed\n\n" + log
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
	rows, err := pdb.Query("SELECT id, title, star, created, modified FROM context WHERE context.modified > $1 AND context.deleted = $2;", server_t, false)
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
			&c.id,
			&c.title,
			&c.star,
			&c.created,
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
	rows, err = pdb.Query("SELECT id, title FROM context WHERE context.modified > $1 AND context.deleted = $2;", server_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_deleted_contexts: %v\n", err)
		return
	}

	var server_deleted_contexts []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
			&c.title,
		//	&c.star,
		//	&c.created,
		//	&c.modified,
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
	rows, err = pdb.Query("SELECT id, title, star, created, modified FROM folder WHERE folder.modified > $1 AND folder.deleted = $2;", server_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_folders: %v", err)
		return
	}

	//defer rows.Close()

	var server_updated_folders []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
			&c.title,
			&c.star,
			&c.created,
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
	rows, err = pdb.Query("SELECT id, title FROM folder WHERE folder.modified > $1 AND folder.deleted = $2;", server_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_deleted_folders: %v", err)
		return
	}

	var server_deleted_folders []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
			&c.title,
			//&c.star,
			//&c.created,
			//&c.modified,
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
	rows, err = pdb.Query("SELECT id, name, star, modified FROM keyword WHERE keyword.modified > $1 AND keyword.deleted = $2;", server_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_keywords: %v", err)
		return
	}

	//defer rows.Close()

	var server_updated_keywords []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
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
	rows, err = pdb.Query("SELECT id, name FROM keyword WHERE keyword.modified > $1 AND keyword.deleted = $2;", server_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_deleted_keywords: %v", err)
		return
	}

	var server_deleted_keywords []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
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
	rows, err = pdb.Query("SELECT id, title, star, note, created, modified, added, completed, context_id, folder_id FROM task WHERE modified > $1 AND deleted = $2 ORDER BY id;", server_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_entries: %v", err)
		return
	}

	var server_updated_entries []EntryTag
	for rows.Next() {
		var e EntryTag
		rows.Scan(
			&e.id,
			&e.title,
			&e.star,
			&e.note,
			&e.created,
			&e.modified,
			&e.added,
			&e.completed,
			&e.context_id,
			&e.folder_id,
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
			fmt.Fprintf(&lg, "    - id: %d star: %t *%q* folder_id: %d context_id: %d  modified: %v\n", e.id, e.star, truncate(e.title, 15), e.context_id, e.folder_id, tc(e.modified, 19, false))
		}
	}

	//server deleted entries
	rows, err = pdb.Query("SELECT id, title FROM task WHERE modified > $1 AND deleted = $2;", server_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_deleted_entries: %v", err)
		return
	}

	var server_deleted_entries []Entry
	for rows.Next() {
		var e Entry
		rows.Scan(
			&e.id,
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
	rows, err = db.Query("SELECT id, tid, title, star, created, modified FROM context WHERE substr(context.modified, 1, 19) > $1 AND context.deleted = $2;", client_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_updated_contexts: %v", err)
		return
	}

	//defer rows.Close()
	fmt.Fprint(&lg, "## Client Changes\n")

	var client_updated_contexts []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
			&c.tid,
			&c.title,
			&c.star,
			&c.created,
			&c.modified,
		)
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
		rows.Scan(
			&c.id,
			&c.tid,
			&c.title,
		)
		client_deleted_contexts = append(client_deleted_contexts, c)
	}
	if len(client_deleted_contexts) > 0 {
		nn += len(client_deleted_contexts)
		fmt.Fprintf(&lg, "- Deleted client `Contexts`: %d\n", len(client_deleted_contexts))
	} else {
		lg.WriteString("- No `Contexts` deleted.\n")
	}

	//client updated folders
	rows, err = db.Query("SELECT id, tid, title, star, created, modified FROM folder WHERE substr(folder.modified, 1, 19) > $1 AND folder.deleted = $2;", client_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_updated_folders: %v", err)
		return
	}

	//defer rows.Close()

	var client_updated_folders []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
			&c.tid,
			&c.title,
			&c.star,
			&c.created,
			&c.modified,
		)
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
		rows.Scan(
			&c.id,
			&c.tid,
			&c.title,
		)
		client_deleted_folders = append(client_deleted_folders, c)
	}
	if len(client_deleted_folders) > 0 {
		nn += len(client_deleted_folders)
		fmt.Fprintf(&lg, "- Deleted client `Folders`: %d\n", len(client_updated_folders))
	} else {
		lg.WriteString("- No `Folders` deleted.\n")
	}

	//client updated keywords
	rows, err = db.Query("SELECT id, tid, name, star, modified FROM keyword WHERE substr(keyword.modified, 1, 19)  > $1 AND keyword.deleted = $2;", client_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_updated_keywords: %v", err)
		return
	}

	var client_updated_keywords []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
			&c.tid,
			&c.title,
			&c.star,
			&c.modified,
		)
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
	rows, err = db.Query("SELECT id, tid, name FROM keyword WHERE substr(keyword.modified, 1, 19) > $1 AND keyword.deleted = $2;", client_t, true)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_deleted_keywords: %v", err)
		return
	}

	var client_deleted_keywords []Container
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
			&c.tid,
			&c.title,
		)
		client_deleted_keywords = append(client_deleted_keywords, c)
	}
	if len(client_deleted_keywords) > 0 {
		nn += len(client_deleted_keywords)
		fmt.Fprintf(&lg, "- Deleted `Keywords`: %d\n", len(client_deleted_keywords))
	} else {
		lg.WriteString("- No `Keywords` deleted.\n")
	}

	//client updated entries
	rows, err = db.Query("SELECT id, tid, title, star, note, created, modified, added, completed, context_tid, folder_tid FROM task WHERE substr(modified, 1, 19)  > ? AND deleted = ?;", client_t, false)
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_updated_entries: %v", err)
		return
	}

	var client_updated_entries []Entry
	for rows.Next() {
		var e Entry
		rows.Scan(
			&e.id,
			&e.tid,
			&e.title,
			&e.star,
			&e.note,
			&e.created,
			&e.modified,
			&e.added,
			&e.completed,
			&e.context_tid,
			&e.folder_tid,
		)
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
		rows.Scan(
			&e.id,
			&e.tid,
			&e.title,
		)
		client_deleted_entries = append(client_deleted_entries, e)
	}
	if len(client_deleted_entries) > 0 {
		nn += len(client_deleted_entries)
		fmt.Fprintf(&lg, "- Deleted `Entries`: %d\n", len(client_deleted_entries))
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
		row := db.QueryRow("SELECT id from context WHERE tid=?", c.id)
		var id int
		err = row.Scan(&id)
		switch {
		case err == sql.ErrNoRows:
			res, err1 := db.Exec("INSERT INTO context (tid, title, star, created, modified, deleted) VALUES (?,?,?,?, datetime('now'), false);",
				c.id, c.title, c.star, c.created)
			if err1 != nil {
				fmt.Fprintf(&lg, "Problem inserting new context into sqlite: %w", err1)
				break
			}
			lastId, _ := res.LastInsertId()
			fmt.Fprintf(&lg, "Created new local context: %v with local id: %v and tid: %v\n", c.title, lastId, c.id)
		case err != nil:
			fmt.Fprintf(&lg, "Problem querying sqlite for a context with tid: %v: %w\n", c.id, err)
		default:
			_, err2 := db.Exec("UPDATE context SET title=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.id)
			if err2 != nil {
				fmt.Fprintf(&lg, "Problem updating sqlite for a context with tid: %v: %w\n", c.id, err2)
			} else {
				fmt.Fprintf(&lg, "Updated local context: %v with tid: %v\n", c.title, c.id)
			}
		}
	}

	for _, c := range client_updated_contexts {
		/*
			// server wins
			if server_id, found := server_updated_contexts_ids[c.tid]; found {
				fmt.Fprintf(&lg, "Server won updating server id/client tid: %v", server_id)
				continue
			}
		*/

		row := pdb.QueryRow("SELECT id from context WHERE id=$1", c.tid)
		var tid int
		err = row.Scan(&tid)
		switch {
		// server context doesn't exist
		case err == sql.ErrNoRows:
			/* this is where we could create a list of client entry ids where the context_tid = something less than 1
				rows, err = db.Query("SELECT id from task WHERE context_tid = c.tid;")
				defer rows.Close()
				var ids []int
				for rows.Next() {
				var id
				err = rows.Scan(&row.id)
				if err != nil ...
				ids = append(ids, id)
			}
			db.Execute("Update task SET context_tid = 1 WHERE context_tid = c.tid;")
			after the UPDATE context below.
			for id := range ids {
				text := strconv.Itoa(id)
				idsS = append(idsS, text)
			}
			in := strings.Join(idsS, ",")
			db.Execute(fmt.Sprintf("Update task SET context_tid=? WHERE id IN (%s);", in), tid)
			*/
			err = pdb.QueryRow("INSERT INTO context (title, star, created, modified, deleted) VALUES ($1, $2, $3, now(), false) RETURNING id;",
				c.title, c.star, c.created).Scan(&tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error inserting new context %q with id %d into postgres: %v", truncate(c.title, 15), c.id, err)
				break
			}
			_, err = db.Exec("UPDATE context SET tid=$1 WHERE id=$2;", tid, c.id)
			if err != nil {
				fmt.Fprintf(&lg, "Error setting tid for new client context %q with id %d to %d: %v\n", truncate(c.title, 15), c.id, tid, err)
				break
			}
			fmt.Fprintf(&lg, "Set value of tid for new client context %q with id: %d to tid = %d\n", c.title, c.id, tid)
		case err != nil:
			fmt.Fprintf(&lg, "Error querying postgres for a context with id: %v: %v\n", c.tid, err)
		default:
			_, err = pdb.Exec("UPDATE context SET title=$1, star=$2, modified=now() WHERE id=$3;", c.title, c.star, c.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error updating postgres for context %q with id %d: %v\n", truncate(c.title, 15), c.tid, err)
			} else {
				fmt.Fprintf(&lg, "Updated server/postgres context: *%q* with id: **%d**\n", c.title, c.tid)
			}
		}
	}

	for _, c := range server_updated_folders {
		row := db.QueryRow("SELECT id from folder WHERE tid=?", c.id)
		var id int
		err = row.Scan(&id)
		switch {
		case err == sql.ErrNoRows:
			res, err1 := db.Exec("INSERT INTO folder (tid, title, star, created, modified, deleted) VALUES (?,?,?,?, datetime('now'), false);",
				c.id, c.title, c.star, c.created)
			if err1 != nil {
				fmt.Fprintf(&lg, "Problem inserting new folder into sqlite: %w", err1)
				break
			}
			lastId, _ := res.LastInsertId()
			fmt.Fprintf(&lg, "Created new local folder: %v with local id: %v and tid: %v\n", c.title, lastId, c.id)
		case err != nil:
			fmt.Fprintf(&lg, "Problem querying sqlite for a folder with tid: %v: %w\n", c.id, err)
		default:
			_, err2 := db.Exec("UPDATE folder SET title=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.id)
			if err2 != nil {
				fmt.Fprintf(&lg, "Problem updating sqlite for a folder with tid: %v: %w\n", c.id, err2)
			} else {
				fmt.Fprintf(&lg, "Updated local folder: %v with tid: %v\n", c.title, c.id)
			}
		}
	}

	for _, c := range client_updated_folders {
		/*
			// server wins
			if server_id, found := server_updated_contexts_ids[c.tid]; found {
				fmt.Fprintf(&lg, "Server won updating server id/client tid: %v", server_id)
				continue
			}
		*/

		row := pdb.QueryRow("SELECT id from folder WHERE id=$1", c.tid)
		var tid int
		err = row.Scan(&tid)
		switch {
		// server folder doesn't exist
		case err == sql.ErrNoRows:
			err1 := pdb.QueryRow("INSERT INTO folder (title, star, created, modified, deleted) VALUES ($1, $2, $3, now(), false) RETURNING id;",
				c.title, c.star, c.created).Scan(&tid)
			if err1 != nil {
				fmt.Fprintf(&lg, "Problem inserting new folder %d: %s into postgres: %v", c.id, c.title, err1)
				break
			}
			_, err2 := db.Exec("UPDATE folder SET tid=$1 WHERE id=$2;", tid, c.id)
			if err2 != nil {
				fmt.Fprintf(&lg, "Error setting tid to %d for new client folder %q with id %d: %v\n", tid, truncate(c.title, 15), c.id, err2)
				break
			}
			fmt.Fprintf(&lg, "Set value of tid for new client folder *%q* with id: **%d** to tid = %d\n", c.title, c.id, tid)
		case err != nil:
			fmt.Fprintf(&lg, "Error querying postgres for a folder with id: %v: %v\n", c.tid, err)
		default:
			_, err3 := pdb.Exec("UPDATE folder SET title=$1, star=$2, modified=now() WHERE id=$3;", c.title, c.star, c.tid)
			if err3 != nil {
				fmt.Fprintf(&lg, "Error updating postgres for folder %q with id %d: %v\n", truncate(c.title, 15), c.tid, err3)
			} else {
				fmt.Fprintf(&lg, "Updated server/postgres folder *%q* with id **%d**\n", truncate(c.title, 15), c.tid)
			}
		}
	}

	for _, c := range server_updated_keywords {
		row := db.QueryRow("SELECT id from keyword WHERE tid=?", c.id)
		var id int
		err = row.Scan(&id)
		switch {
		case err == sql.ErrNoRows:
			res, err1 := db.Exec("INSERT INTO keyword (tid, name, star, modified, deleted) VALUES (?,?,?, datetime('now'), false);",
				c.id, c.title, c.star)
			if err1 != nil {
				fmt.Fprintf(&lg, "Error inserting new keyword %q into sqlite: %v", truncate(c.title, 15), err1)
				break
			}
			lastId, _ := res.LastInsertId()
			fmt.Fprintf(&lg, "Created new local keyword *%q* with local id **%d** and tid: **%d**\n", truncate(c.title, 15), lastId, c.id)
		case err != nil:
			fmt.Fprintf(&lg, "Error querying sqlite for a keyword with tid: %v: %w\n", c.id, err)
		default:
			_, err2 := db.Exec("UPDATE keyword SET name=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.id)
			if err2 != nil {
				fmt.Fprintf(&lg, "Error updating local keyword %q with tid %d: %v\n", truncate(c.title, 15), c.id, err2)
			} else {
				fmt.Fprintf(&lg, "Updated local keyword %q with tid %d\n", truncate(c.title, 15), c.id)
			}
		}
	}

	for _, c := range client_updated_keywords {
		/*
			// server wins
			if server_id, found := server_updated_contexts_ids[c.tid]; found {
				fmt.Fprintf(&lg, "Server won updating server id/client tid: %v", server_id)
				continue
			}
		*/

		row := pdb.QueryRow("SELECT id from keyword WHERE id=$1", c.tid)
		var tid int
		err = row.Scan(&tid)
		switch {
		// server keyword doesn't exist
		case err == sql.ErrNoRows:
			err1 := pdb.QueryRow("INSERT INTO keyword (name, star, modified, deleted) VALUES ($1, $2, now(), false) RETURNING id;",
				c.title, c.star).Scan(&tid)
			if err1 != nil {
				fmt.Fprintf(&lg, "Problem inserting new keyword: %d: %s into postgres: %v", c.id, c.title, err1)
				break
			}
			_, err2 := db.Exec("UPDATE keyword SET tid=$1 WHERE id=$2;", tid, c.id)
			if err2 != nil {
				fmt.Fprintf(&lg, "Error setting new client keyword's tid: %v; id: %v\n", tid, c.id, err2)
				break
			}
			fmt.Fprintf(&lg, "Set value of tid for new client keyword %q with id: %d to tid = %d\n", c.title, c.id, tid)
		case err != nil:
			fmt.Fprintf(&lg, "Error querying postgres for a keyword with id: %v: %v\n", c.tid, err)
		default:
			_, err3 := pdb.Exec("UPDATE keyword SET name=$1, star=$2, modified=now() WHERE id=$3;", c.title, c.star, c.tid)
			if err3 != nil {
				fmt.Fprintf(&lg, "Error updating postgres for a keyword with id: %v: %w\n", c.tid, err3)
			} else {
				fmt.Fprintf(&lg, "Updated server/postgres keyword: *%q* with id: **%d**\n", c.title, c.tid)
			}
		}
	}

	/**********should come before container deletes to change tasks here*****************/
	server_updated_entries_ids := make(map[int]struct{})
	var task_ids []string
	for _, e := range server_updated_entries {
		db.Exec("INSERT INTO task (tid, title, star, created, added, completed, context_tid, folder_tid, note, modified, deleted) VALUES"+
			"(?, ?, ?, datetime('now'), ?, ?, ?, ?, ?, datetime('now'), false) ON CONFLICT(tid) DO UPDATE SET "+
			"title=excluded.title, star=excluded.star, completed=excluded.completed, context_tid=excluded.context_tid, "+
			"folder_tid=excluded.folder_tid, note=excluded.note, modified=datetime('now');",
			e.id, e.title, e.star, e.added, e.completed, e.context_id, e.folder_id, e.note)
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Fprintf(&lg, "%sNon-error in INSERT ... ON CONFLICT (INSERT) for id/tid %d %q: %v%s\n", RED_BG, e.id, e.title, err, RESET)
			} else {
				fmt.Fprintf(&lg, "Error in INSERT ... ON CONFLICT for id/tid %d %q: %v\n", e.id, e.title, err)
				continue
			}
		} else {
			fmt.Fprintf(&lg, "Inserted or updated client entry %q with tid **%d**\n", e.title, e.id)
		}
		task_ids = append(task_ids, strconv.Itoa(e.id))
		server_updated_entries_ids[e.id] = struct{}{}
	}
	if len(server_updated_entries) != 0 {
		in := strings.Join(task_ids, ",")

		// need to delete all keywords for changed entries first
		stmt := fmt.Sprintf("DELETE FROM task_keyword WHERE task_tid in (%s);", in)
		_, err = db.Exec(stmt)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting from task_keyword from server ids %s: %v\n", in, err)
		}

		// need to delete all rows for changed entries in fts
		stmt = fmt.Sprintf("DELETE FROM fts WHERE tid in (%s);", in)
		_, err = fts_db.Exec(stmt)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting from fts from server ids %s: %v\n", in, err)
		}
		tks := getTaskKeywordIds_x(pdb, in, &lg)
		if len(tks) != 0 {
			query, args := createBulkInsertQueryTaskKeywordIds(len(tks), tks)
			err = bulkInsert(db, query, args)
			if err != nil {
				fmt.Fprintf(&lg, "%v\n", err)
			} else {
				fmt.Fprintf(&lg, "Keywords updated for task tids: %s\n", in)
			}
			tags := getTags_x(pdb, in, &lg)
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
				if entry.id == tags[i].task_id {
					entry.tag = tags[i].tag
					fmt.Fprintf(&lg, "FTS tag will be updated for tid: %d, tag: %s\n", entry.id, entry.tag)
					i += 1
					if i == len(tags) {
						break
					}
				}
			}
		}
		query, args := createBulkInsertQueryFTS(len(server_updated_entries), server_updated_entries)
		err = bulkInsert(fts_db, query, args)
		if err != nil {
			fmt.Fprintf(&lg, "%v", err)
		} else {
			fmt.Fprintf(&lg, "FTS entries updated for task tids: %s\n", in)
		}
	}

	for _, e := range client_updated_entries {
		// server wins if both client and server have updated an item
		if _, found := server_updated_entries_ids[e.tid]; found {
			fmt.Fprintf(&lg, "Server won: client entry %q with id %d and tid %d was updated by server\n", truncate(e.title, 15), e.id, e.tid)
			continue
		}

		var server_id int
		if e.tid < 1 {
			err := pdb.QueryRow("INSERT INTO task (title, star, created, added, completed, context_id, folder_id, note, modified, deleted) "+
				"VALUES ($1, $2, now(), $3, $4, $5, $6, $7, now(), false)  RETURNING id",
				e.title, e.star, e.added, e.completed, e.context_tid, e.folder_tid, e.note).Scan(&server_id)
			if err != nil {
				fmt.Fprintf(&lg, "Error inserting server entry: %v", err)
				continue
			}
			/*
				if we attach a keyword, this will fail because the task_tid of whatever(<1) will mean no task has the tid in task_keyword
				 the answer is a little messy
				SELECT keyword_tid FROM task_keyword WHERE task_tid=e.tid
				DELETE FROM task_keyword WHERE task_tid=e.tid
				for _, keyword_tid :=range keyword_tids INSERT INTO task_keyword(task_tid, keyword_tid)
				VALUES (server_id, keyword_tid) (goes after updating entry tid
			*/
			_, err = db.Exec("UPDATE task SET tid=? WHERE id=?;", server_id, e.id)
			if err != nil {
				fmt.Fprintf(&lg, "Error setting tid for client entry %q with id %d to tid %d: %v\n", truncate(e.title, 15), e.id, server_id, err)
				continue
			}
			_, err = fts_db.Exec("UPDATE fts SET tid=$1 WHERE tid=$2;", server_id, e.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error in Update tid in fts: %v\n", err)
			}
			fmt.Fprintf(&lg, "Created new server entry *%q* with id **%d**\n", truncate(e.title, 15), server_id)
			fmt.Fprintf(&lg, "and set tid for client entry (id %d) to tid %d\n", e.id, server_id)
		} else {
			_, err := pdb.Exec("UPDATE task SET title=$1, star=$2, context_id=$3, folder_id=$4, note=$5, completed=$6, modified=now() WHERE id=$7;",
				e.title, e.star, e.context_tid, e.folder_tid, e.note, e.completed, e.tid)
			if err != nil {
				fmt.Fprintf(&lg, "Error updating server entry: %v", err)
				continue
			}
			server_id = e.tid
			fmt.Fprintf(&lg, "Updated server entry *%q* with id **%d**\n", truncate(e.title, 15), server_id)
		}

		// Update the server entry's keywords
		_, err := pdb.Exec("DELETE FROM task_keyword WHERE task_id=$1;", server_id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting from task_keyword from server id %d: %v\n", server_id, err)
			continue
		}
		kwIds := clientTaskKeywordTids(db, &lg, server_id)
		for _, keywordId := range kwIds {
			addServerTaskKeywordIds(pdb, &lg, keywordId, server_id)
		}
	}

	// server deleted entries
	for _, e := range server_deleted_entries {
		_, err = db.Exec("DELETE FROM task_keyword WHERE task_tid=?;", e.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting task_keyword client rows where entry tid = %d: %v\n", e.id, err)
			continue
		}

		_, err := db.Exec("DELETE FROM task WHERE tid=?;", e.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting client entry %q with tid %d: %v\n", tc(e.title, 15, true), e.id, err)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client entry %q with tid %d\n", truncate(e.title, 15), e.id)
		fmt.Fprintf(&lg, "and on client deleted task_tid %d from task_keyword\n", e.id)
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

		_, err := pdb.Exec("UPDATE task SET deleted=true, modified=now() WHERE id=$1", e.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error setting server entry with id %d to deleted: %v\n", e.tid, err)
			continue
		}
		fmt.Fprintf(&lg, "Updated server entry %q with id %d to **deleted = true**\n", truncate(e.title, 15), e.tid)

		_, err = pdb.Exec("DELETE FROM task_keyword WHERE task_id=$1;", e.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting task_keyword server rows where entry id = %d: %v\n", e.tid, err)
			continue
		}
		fmt.Fprintf(&lg, "and on server deleted task_id %d from task_keyword\n", e.tid)

	}

	//server_deleted_contexts
	for _, c := range server_deleted_contexts {
		// I think the task changes may not be necessary because only a previous client sync can delete server context
		res, err := pdb.Exec("Update task SET context_id=1, modified=now() WHERE context_id=$1;", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change server/postgres entry context to 'none' for a deleted context: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of server entries that were changed to 'none' (might be zero): **%d**\n", rowsAffected)
		}
		res, err = db.Exec("Update task SET context_tid=1, modified=datetime('now') WHERE context_tid=?;", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change client/sqlite entry contexts for a deleted context: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of client entries that were changed to 'none' (might be zero): **%d**\n", rowsAffected)
		}

		// could use returning to get the id of the context that was deleted - would just be for log
		_, err = db.Exec("DELETE FROM context WHERE tid=?", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Problem deleting local context with tid = %v", c.id)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client context %s with tid %d", c.title, c.id)
	}

	// client deleted contexts
	for _, c := range client_deleted_contexts {
		res, err := pdb.Exec("Update task SET context_id=1, modified=now() WHERE context_id=$1;", c.tid) //?modified=now()
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change server/postgres entry contexts for a deleted context: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of server entries that were changed to 'none': **%d**\n", rowsAffected)
		}
		res, err = db.Exec("Update task SET context_tid=1, modified=now() WHERE context_tid=?;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change client/sqlite entry contexts for a deleted context: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of client entries that were changed to 'none': **%d**\n", rowsAffected)
		}
		// since on server, we just set deleted to true
		// since may have to sync with other clients
		_, err = pdb.Exec("UPDATE context SET deleted=true, modified=now() WHERE context.id=$1", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error (pdb.Exec) setting server context %s with id = %d to deleted: %v\n", c.title, c.tid, err)
			continue
		}
		_, err = db.Exec("DELETE FROM folder WHERE id=?", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting local context %s with id %d: %v", c.title, c.id, err)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client context %s: id %d and updated server context with id %d to deleted = true", c.title, c.id, c.tid)
	}

	//server_deleted_folders
	for _, c := range server_deleted_folders {
		// I think the task changes may not be necessary because only a previous client sync can delete server context
		// and that previous client sync should have changed the relevant tasks to 'none'
		res, err := pdb.Exec("Update task SET folder_id=1, modified=now() WHERE folder_id=$1;", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change server/postgres entry folder to 'none' for a deleted folder: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of server entries that were changed to 'none' (might be zero): **%d**\n", rowsAffected)
		}
		res, err = db.Exec("Update task SET folder_tid=1, modified=datetime('now') WHERE folder_tid=?;", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change client/sqlite entry folders for a deleted folder: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of client entries that were changed to 'none' (might be zero): **%d**\n", rowsAffected)
		}

		// could use returning to get the id of the folder that was deleted - would just be for log
		_, err = db.Exec("DELETE FROM folder WHERE tid=?", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Problem deleting local folder with tid = %v", c.id)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client folder %v with tid %v", c.title, c.id)
	}

	// client deleted folders
	for _, c := range client_deleted_folders {
		res, err := pdb.Exec("Update task SET folder_id=1, modified=now() WHERE folder_id=$1;", c.tid) //?modified=now()
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change server/postgres entry folders for a deleted folder: %v\n", err)
			continue
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of server entries that were changed to 'none': **%d**\n", rowsAffected)
		}
		res, err = db.Exec("Update task SET folder_tid=1, modified=now() WHERE folder_tid=?;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error trying to change client/sqlite entry folders for a deleted folder: %v\n", err)
		} else {
			rowsAffected, _ := res.RowsAffected()
			fmt.Fprintf(&lg, "The number of client entries that were changed to 'none': **%d**\n", rowsAffected)
		}
		// since on server, we just set deleted to true
		// since may have to sync with other clients
		_, err = pdb.Exec("UPDATE folder SET deleted=true, modified=now() WHERE folder.id=$1", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error (pdb.Exec) setting server folder %s with id = %v to deleted: %v\n", c.title, c.tid, err)
			continue
		}
		_, err = db.Exec("DELETE FROM folder WHERE id=?", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting local folder %s with id %d: %v", c.title, c.id, err)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client folder %s: id %d and updated server folder with id %d to deleted = true", c.title, c.id, c.tid)
	}

	//server_deleted_keywords
	for _, c := range server_deleted_keywords {
		_, err := db.Exec("DELETE FROM task_keyword WHERE keyword_tid=?;", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting from task_keyword client keyword_tid: %d", c.id)
		}
		_, err = db.Exec("DELETE FROM keyword WHERE tid=?", c.id)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting client keyword with tid = %v", c.id)
			continue
		}
		fmt.Fprintf(&lg, "Deleted client keyword %q with tid %d", truncate(c.title, 15), c.id)
	}

	// client deleted keywords
	for _, c := range client_deleted_keywords {
		_, err = pdb.Exec("DELETE FROM task_keyword WHERE keyword_id=$1;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting from task_keyword server keyword_id: %d", c.tid)
		}
		_, err = db.Exec("DELETE FROM task_keyword WHERE keyword_tid=?;", c.tid)
		if err != nil {
			fmt.Fprintf(&lg, "Error deleting client task_keyword with id %d", c.tid)
		}
		// since on server, we just set deleted to true
		// since may have to sync with other clients
		_, err := pdb.Exec("UPDATE keyword SET deleted=true WHERE keyword.id=$1", c.tid)
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
