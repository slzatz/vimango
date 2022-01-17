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

/* in common
type serverEntry struct {
	id         int
	title      string
	created    string
	folder_id  int
	context_id int
	star       bool
	note       sql.NullString
	added      sql.NullString
	completed  sql.NullString
	deleted    bool
	modified   string
}
*/

type EntryTag struct {
	serverEntry
	tag string
}

type TaskKeyword struct {
	task_id int
	keyword string
}

type TaskKeywordIds struct {
	task_id    int
	keyword_id int
}
type TaskTag struct {
	task_id int
	tag     string
}

// using EntryTag so we can later add the tag to the rest of the entry info
func getEntries(dbase *sql.DB, count int, plg io.Writer) []EntryTag {
	rows, err := dbase.Query("SELECT id, title, star, note, created, modified, context_id, folder_id, added, completed FROM task WHERE deleted=false ORDER BY id;")
	if err != nil {
		fmt.Fprintf(plg, "Error in getEntries: %v\n", err)
		return []EntryTag{}
	}

	entries := make([]EntryTag, 0, count)
	for rows.Next() {
		var e EntryTag
		rows.Scan(
			&e.id,
			&e.title,
			&e.star,
			&e.note,
			&e.created,
			&e.modified,
			&e.context_id,
			&e.folder_id,
			&e.added,
			&e.completed,
		)
		entries = append(entries, e)
	}
	return entries
}

// returns []struct{client_entry_tid, tag} - need to populate fts
func getTags(dbase *sql.DB, count int, plg io.Writer) []TaskTag {
	rows, err := dbase.Query("SELECT task_keyword.task_id, keyword.name FROM task_keyword LEFT OUTER JOIN keyword ON keyword.id=task_keyword.keyword_id ORDER BY task_keyword.task_id;")
	if err != nil {
		println(err)
		return []TaskTag{}
	}
	taskkeywords := make([]TaskKeyword, 0, count)
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

func getTaskKeywordIds(dbase *sql.DB, count int, plg io.Writer) []TaskKeywordIds {
	//rows, err := pdb.Query("Select task_id, keyword_id FROM task_keyword ORDER BY task_id;")
	rows, err := dbase.Query("SELECT task_id, keyword_id FROM task_keyword;")
	if err != nil {
		println(err)
		return []TaskKeywordIds{}
	}
	taskKeywordIds := make([]TaskKeywordIds, 0, count)
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

func createBulkInsertQuery(n int, entries []EntryTag) (query string, args []interface{}) {
	values := make([]string, n)
	args = make([]interface{}, n*8)
	pos := 0
	for i, e := range entries {
		values[i] = "(?, ?, ?, datetime('now'), ?, ?, ?, ?, ?, datetime('now'), false)"
		args[pos] = e.id
		args[pos+1] = e.title
		args[pos+2] = e.star
		args[pos+3] = e.added
		args[pos+4] = e.completed
		args[pos+5] = e.context_id
		args[pos+6] = e.folder_id
		args[pos+7] = e.note
		pos += 8
	}
	query = fmt.Sprintf("INSERT INTO task (tid, title, star, created, added, completed, context_tid, folder_tid, note, modified, deleted) VALUES %s", strings.Join(values, ", "))
	return
}

func createBulkInsertQueryFTS(n int, entries []EntryTag) (query string, args []interface{}) {
	values := make([]string, n)
	args = make([]interface{}, n*4)
	pos := 0
	for i, e := range entries {
		values[i] = "(?, ?, ?, ?)"
		args[pos] = e.title
		args[pos+1] = e.note
		args[pos+2] = e.tag
		args[pos+3] = e.id
		pos += 4
	}
	query = fmt.Sprintf("INSERT INTO fts (title, note, tag, tid) VALUES %s;", strings.Join(values, ", "))
	return
}

func createBulkInsertQueryTaskKeywordIds(n int, tk []TaskKeywordIds) (query string, args []interface{}) {
	values := make([]string, n)
	args = make([]interface{}, n*2)
	pos := 0
	for i, e := range tk {
		values[i] = "(?, ?)"
		args[pos] = e.task_id
		args[pos+1] = e.keyword_id
		pos += 2
	}
	query = fmt.Sprintf("INSERT INTO task_keyword (task_tid, keyword_tid) VALUES %s", strings.Join(values, ", "))
	return
}
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

func bulkLoad(reportOnly bool) (log string) {

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
	rows, err := pdb.Query("SELECT id, title, star, created, modified FROM context WHERE deleted=false ORDER BY id;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_contexts: %v", err)
		return
	}

	defer rows.Close()

	server_contexts := make([]Container, 0, contextCount)
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
			&c.title,
			&c.star,
			&c.created,
			&c.modified,
		)
		server_contexts = append(server_contexts, c)
	}
	c := server_contexts[0]
	_, err = db.Exec("UPDATE context SET title=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.id)
	if err != nil {
		fmt.Fprintf(&lg, "Error updating sqlite context with tid: %v: %v\n", c.id, err)
	}

	for _, c := range server_contexts[1:] {
		_, err := db.Exec("INSERT INTO context (tid, title, star, created, modified, deleted) VALUES (?,?,?,?, datetime('now'), false);",
			c.id, c.title, c.star, c.created)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting context into sqlite: %v\n", err)
			break
		}
	}

	//server folder -> client
	rows, err = pdb.Query("SELECT id, title, star, created, modified FROM folder WHERE deleted=false ORDER BY id;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_folders: %v", err)
		return
	}

	server_folders := make([]Container, 0, folderCount)
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
			&c.title,
			&c.star,
			&c.created,
			&c.modified,
		)
		server_folders = append(server_folders, c)
	}
	c = server_folders[0]
	_, err = db.Exec("UPDATE folder SET title=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.id)
	if err != nil {
		fmt.Fprintf(&lg, "Error updating sqlite folder with tid: %v: %v\n", c.id, err)
	}
	for _, c := range server_folders[1:] {
		_, err := db.Exec("INSERT INTO folder (tid, title, star, created, modified, deleted) VALUES (?,?,?,?, datetime('now'), false);",
			c.id, c.title, c.star, c.created)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting folder into sqlite: %v\n", err)
			break
		}
	}

	//server keyword -> client
	rows, err = pdb.Query("SELECT id, name, star, modified FROM keyword WHERE deleted=false ORDER BY id;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_keywords: %v", err)
		return
	}

	server_keywords := make([]Container, 0, keywordCount)
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.id,
			&c.title,
			&c.star,
			&c.modified,
		)
		server_keywords = append(server_keywords, c)
	}

	for _, c := range server_keywords {
		_, err := db.Exec("INSERT INTO keyword (tid, name, star, modified, deleted) VALUES (?,?,?, datetime('now'), false);",
			c.id, c.title, c.star)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting new keyword %q into sqlite: %v", truncate(c.title, 15), err)
			break
		}
	}

	/* for the code below need to add guard that entries and keywords might be zero */
	//server entries -> client
	entries := getEntries(pdb, taskCount, &lg)

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
		query, args := createBulkInsertQuery(len(e), e)
		err = bulkInsert(db, query, args)
		if err != nil {
			fmt.Fprintf(&lg, "%v\n", err)
		}
		if done {
			fmt.Fprintf(&lg, "\n- %d `entries` were added to the client db\n", m)
			break
		}
		i += 1
	}

	taskKeywordIds := getTaskKeywordIds(pdb, taskKeywordCount, &lg)
	i = 0
	n = 100
	done = false
	for {
		m := (i + 1) * n
		if m > len(taskKeywordIds) {
			m = len(taskKeywordIds)
			done = true
		}
		e := taskKeywordIds[i*n : m]
		query, args := createBulkInsertQueryTaskKeywordIds(len(e), e)
		err = bulkInsert(db, query, args)
		if err != nil {
			fmt.Fprintf(&lg, "%v\n", err)
		}
		if done {
			fmt.Fprintf(&lg, "- %d `taskKeywordIds` were added to the client db\n", m)
			break
		}
		i += 1
	}

	tags := getTags(pdb, taskKeywordCount, &lg)
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
		if entry.id == tags[i].task_id {
			entry.tag = tags[i].tag
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
		query, args := createBulkInsertQueryFTS(len(e), e)
		err = bulkInsert(db, query, args)
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
