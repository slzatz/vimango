package main

import (
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

/*
type NewEntryPlusTag struct {
	NewEntry
	tag string
}
*/

// using EntryPlusTag so we can later add the tag to the rest of the entry info
func getEntriesBulk(dbase *sql.DB, count int, plg io.Writer) []EntryPlusTag {
	rows, err := dbase.Query("SELECT tid, title, star, note, modified, context_tid, folder_tid, added, archived FROM task WHERE deleted=false ORDER BY tid;")
	if err != nil {
		fmt.Fprintf(plg, "Error in getEntriesBulk: %v\n", err)
		return []EntryPlusTag{}
	}

	entries := make([]EntryPlusTag, 0, count)
	for rows.Next() {
		var e EntryPlusTag
		rows.Scan(
			&e.tid,
			&e.title,
			&e.star,
			&e.note,
			&e.modified,
			&e.context_tid,
			&e.folder_tid,
			&e.added,
			&e.archived,
		)
		entries = append(entries, e)
	}
	return entries
}

// returns []struct{client_entry_tid, tag} - need to populate fts
func getTagsBulk(dbase *sql.DB, count int, plg io.Writer) []TaskTag3 {
	rows, err := dbase.Query("SELECT task_keyword.task_tid, keyword.title FROM task_keyword LEFT OUTER JOIN keyword ON keyword.tid=task_keyword.keyword_tid ORDER BY task_keyword.task_tid;")
	if err != nil {
		println(err)
		return []TaskTag3{}
	}
	taskkeywords := make([]TaskKeyword3, 0, count)
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
			tt.tag.String = strings.Join(keywords, ",")
			tt.tag.Valid = true
			tasktags = append(tasktags, tt)
			prev_tid = tid
			keywords = keywords[:0]
			keywords = append(keywords, tk.keyword)
		}
	}
	// need to get the last pair
	tt.task_tid = tid
	tt.tag.String = strings.Join(keywords, ",")
	tt.tag.Valid = true
	tasktags = append(tasktags, tt)

	return tasktags
}

func getTaskKeywordPairsBulk(dbase *sql.DB, count int, plg io.Writer) []TaskKeywordPairs {
	rows, err := dbase.Query("SELECT task_tid, keyword_tid FROM task_keyword;")
	if err != nil {
		println(err)
		return []TaskKeywordPairs{}
	}
	taskKeywordPairs := make([]TaskKeywordPairs, 0, count)
	for rows.Next() {
		var tk TaskKeywordPairs
		rows.Scan(
			&tk.task_tid,
			&tk.keyword_tid,
		)
		taskKeywordPairs = append(taskKeywordPairs, tk)
	}
	return taskKeywordPairs
}

func createBulkInsertQuery2(n int, entries []EntryPlusTag) (query string, args []interface{}) {
	values := make([]string, n)
	args = make([]interface{}, n*8)
	pos := 0
	for i, e := range entries {
		values[i] = "(?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), false)"
		args[pos] = e.tid
		args[pos+1] = e.title
		args[pos+2] = e.star
		args[pos+3] = e.added
		args[pos+4] = e.archived
		args[pos+5] = e.context_tid
		args[pos+6] = e.folder_tid
		args[pos+7] = e.note
		pos += 8
	}
	query = fmt.Sprintf("INSERT INTO task (tid, title, star, added, archived, context_tid, folder_tid, note, modified, deleted) VALUES %s", strings.Join(values, ", "))
	return
}

func bulkInsert2(dbase *sql.DB, query string, args []interface{}) (err error) {
	stmt, err := dbase.Prepare(query)
	if err != nil {
		return fmt.Errorf("Error in bulkInsert2 Prepare: %v", err)
	}

	_, err = stmt.Exec(args...)
	if err != nil {
		return fmt.Errorf("Error in bulkInsert2 Exec: %v", err)
	}

	return
}

func bulkLoad2(reportOnly bool) (log string) {

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

	err = pdb.Ping()
	if err != nil {
		fmt.Fprintf(&lg, "postgres ping failure!: %v", err)
		return
	}

	t0 := time.Now()
	fmt.Fprintf(&lg, "Starting initial sync at %v\n", t0.Format("2006-01-02 15:04:05"))

	var contextCount int
	err = pdb.QueryRow("SELECT COUNT(*) FROM context WHERE deleted=false;").Scan(&contextCount)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for server_contexts: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Contexts`: %d\n", contextCount)

	var folderCount int
	err = pdb.QueryRow("SELECT COUNT(*) FROM folder WHERE deleted=false;").Scan(&folderCount)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for server_folders: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Folders`: %d\n", folderCount)

	var keywordCount int
	err = pdb.QueryRow("SELECT COUNT(*) FROM keyword WHERE deleted=false;").Scan(&keywordCount)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for server_keywords: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Keywords`: %d\n", keywordCount)

	var taskCount int
	err = pdb.QueryRow("SELECT COUNT(*) FROM task WHERE deleted=false;").Scan(&taskCount)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for server_entries: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Entries`: %d\n", taskCount)

	var taskKeywordCount int
	err = pdb.QueryRow("SELECT COUNT(*) FROM task_keyword;").Scan(&taskKeywordCount)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for task_keywords: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Task Keyword Combos`: %d\n", taskKeywordCount)

	if reportOnly {
		// note there is a defer log.String()
		return
	}

	/****************below is where changes start***********************************/

	//server contexts -> client
	rows, err := pdb.Query("SELECT tid, title, star, modified FROM context WHERE deleted=false ORDER BY tid;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_contexts: %v", err)
		return
	}

	defer rows.Close()

	server_contexts := make([]Container, 0, contextCount)
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
			&c.star,
			&c.modified,
		)
		server_contexts = append(server_contexts, c)
	}
	c := server_contexts[0]
	_, err = db.Exec("UPDATE context SET title=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.tid)
	if err != nil {
		fmt.Fprintf(&lg, "Error updating sqlite context with tid: %v: %v\n", c.tid, err)
	}

	for _, c := range server_contexts[1:] {
		_, err := db.Exec("INSERT INTO context (tid, title, star, modified, deleted) VALUES (?,?,?, datetime('now'), false);",
			c.tid, c.title, c.star)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting context into sqlite: %v\n", err)
			break
		}
	}

	//server folder -> client
	rows, err = pdb.Query("SELECT tid, title, star, modified FROM folder WHERE deleted=false ORDER BY tid;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_folders: %v", err)
		return
	}

	server_folders := make([]Container, 0, folderCount)
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
			&c.star,
			&c.modified,
		)
		server_folders = append(server_folders, c)
	}
	c = server_folders[0]
	_, err = db.Exec("UPDATE folder SET title=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.tid)
	if err != nil {
		fmt.Fprintf(&lg, "Error updating sqlite folder with tid: %v: %v\n", c.tid, err)
	}
	for _, c := range server_folders[1:] {
		_, err := db.Exec("INSERT INTO folder (tid, title, star, modified, deleted) VALUES (?,?,?, datetime('now'), false);",
			c.tid, c.title, c.star)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting folder into sqlite: %v\n", err)
			break
		}
	}

	//server keyword -> client
	rows, err = pdb.Query("SELECT tid, title, star, modified FROM keyword WHERE deleted=false ORDER BY tid;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_keywords: %v", err)
		return
	}

	server_keywords := make([]Container, 0, keywordCount)
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
			&c.star,
			&c.modified,
		)
		server_keywords = append(server_keywords, c)
	}

	for _, c := range server_keywords {
		_, err := db.Exec("INSERT INTO keyword (tid, title, star, modified, deleted) VALUES (?,?,?, datetime('now'), false);",
			c.tid, c.title, c.star)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting new keyword %q into sqlite: %v", truncate(c.title, 15), err)
			break
		}
	}

	/* for the code below need to add guard that entries and keywords might be zero */
	//server entries -> client
	entries := getEntriesBulk(pdb, taskCount, &lg)

	i := 0
	n := 100
	done := false
	for {
		m := (i + 1) * n
		if m > len(entries) {
			m = len(entries)
			done = true
		}
		e := entries[i*n : m]
		query, args := createBulkInsertQuery2(len(e), e)
		err = bulkInsert2(db, query, args)
		if err != nil {
			fmt.Fprintf(&lg, "%v\n", err)
		}
		if done {
			fmt.Fprintf(&lg, "\n- %d `entries` were added to the client db\n", m)
			break
		}
		i += 1
	}

	taskKeywordPairs := getTaskKeywordPairsBulk(pdb, taskKeywordCount, &lg)
	i = 0
	n = 100
	done = false
	for {
		m := (i + 1) * n
		if m > len(taskKeywordPairs) {
			m = len(taskKeywordPairs)
			done = true
		}
		e := taskKeywordPairs[i*n : m]
		query, args := createBulkInsertQueryTaskKeywordPairs(len(e), e)
		err = bulkInsert2(db, query, args)
		if err != nil {
			fmt.Fprintf(&lg, "%v\n", err)
		}
		if done {
			fmt.Fprintf(&lg, "- %d `taskKeywordPairs` were added to the client db\n", m)
			break
		}
		i += 1
	}

	tags := getTagsBulk(pdb, taskKeywordCount, &lg)
	/*tags and entries must be sorted before updating entries with tag; done in queries
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].task_id < tags[j].task_id
	})
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].id < entries[j].id
	})
	*/
	i = 0
	for j := 0; ; j++ {
		// below check shouldn't be necessary
		if j == len(entries) {
			break
		}
		entry := &entries[j]
		if entry.tid == tags[i].task_tid {
			entry.tag = tags[i].tag //tags[i].tag type = sql.NullString
			i += 1
			if i == len(tags) {
				break
			}
		}
	}
	// this should be broken up like the other bulk inserts
	/*
		query, args := createBulkInsertQueryFTS(len(entries), entries)
		err = bulkInsert(fts_db, query, args)
		if err != nil {
			fmt.Fprintf(&lg, "%v", err)
		}
	*/

	i = 0
	n = 100
	done = false
	for {
		m := (i + 1) * n
		if m > len(entries) {
			m = len(entries)
			done = true
		}
		e := entries[i*n : m]
		query, args := createBulkInsertQueryFTS3(len(e), e)
		err = bulkInsert2(fts_db, query, args)
		if err != nil {
			fmt.Fprintf(&lg, "%v\n", err)
		}
		if done {
			fmt.Fprintf(&lg, "\n- %d `entries` were added to the client FTS5 db\n", m)
			break
		}
		i += 1
	}

	/*********************end of sync*************************/

	var server_ts string
	row := pdb.QueryRow("SELECT now();")
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
	fmt.Fprintf(&lg, "Server UTC timestamp: %s\n", strings.Replace(tc(server_ts, 19, false), "T", " ", 1))

	fmt.Fprintf(&lg, "Initial sync took %v seconds\n", int(time.Since(t0)/1000000000))

	success = true
	return
}
