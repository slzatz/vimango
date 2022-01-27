package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func createBulkInsertQueryReverse(n int, entries []EntryPlusTag) (query string, args []interface{}) {
	values := make([]string, n)
	args = make([]interface{}, n*8)
	pos := 0
	for i, e := range entries {
		values[i] = fmt.Sprintf("($%d, $%d, $%d, now(), $%d, $%d, $%d, $%d, $%d, now(), false)", pos+1, pos+2, pos+3, pos+4, pos+5, pos+6, pos+7, pos+8)
		args[pos] = e.tid
		args[pos+1] = e.title
		args[pos+2] = e.star
		args[pos+3] = e.added
		args[pos+4] = e.completed
		args[pos+5] = e.context_tid
		args[pos+6] = e.folder_tid
		args[pos+7] = e.note
		pos += 8
	}
	query = fmt.Sprintf("INSERT INTO task (tid, title, star, created, added, completed, context_tid, folder_tid, note, modified, deleted) VALUES %s", strings.Join(values, ", "))
	return
}

func createBulkInsertQueryTaskKeywordPairsReverse(n int, tk []TaskKeywordPairs) (query string, args []interface{}) {
	values := make([]string, n)
	args = make([]interface{}, n*2)
	pos := 0
	for i, e := range tk {
		values[i] = fmt.Sprintf("($%d, $%d)", pos+1, pos+2)
		args[pos] = e.task_tid
		args[pos+1] = e.keyword_tid
		pos += 2
	}
	query = fmt.Sprintf("INSERT INTO task_keyword (task_tid, keyword_tid) VALUES %s", strings.Join(values, ", "))
	return
}

func reverseBulkLoad(reportOnly bool) (log string) {

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
		fmt.Fprintf(&lg, "Error opening postgres pdb: %v", err)
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
	err = db.QueryRow("SELECT COUNT(*) FROM context WHERE deleted=false;").Scan(&contextCount)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for client contexts: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Contexts`: %d\n", contextCount)

	var folderCount int
	err = db.QueryRow("SELECT COUNT(*) FROM folder WHERE deleted=false;").Scan(&folderCount)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for client folders: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Folders`: %d\n", folderCount)

	var keywordCount int
	err = db.QueryRow("SELECT COUNT(*) FROM keyword WHERE deleted=false;").Scan(&keywordCount)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for client keywords: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Keywords`: %d\n", keywordCount)

	var taskCount int
	err = db.QueryRow("SELECT COUNT(*) FROM task WHERE deleted=false;").Scan(&taskCount)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for client entries: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Entries`: %d\n", taskCount)

	var taskKeywordCount int
	err = db.QueryRow("SELECT COUNT(*) FROM task_keyword;").Scan(&taskKeywordCount)
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

	//client contexts -> server
	// shouldn't need deleted = false but doesn't hurt if something deleted never got synched
	rows, err := db.Query("SELECT tid, title, star FROM context WHERE deleted=false ORDER BY tid;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client contexts: %v", err)
		return
	}

	defer rows.Close()

	client_contexts := make([]Container, 0, contextCount)
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
			&c.star,
			//&c.created,
			//&c.modified,
		)
		client_contexts = append(client_contexts, c)
	}
	for _, c := range client_contexts {
		_, err := pdb.Exec("INSERT INTO context (tid, title, star, created, modified, deleted) VALUES ($1, $2, $3, now(), now(), false);",
			c.tid, c.title, c.star)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting context into sqlite: %v\n", err)
			break
		}
	}

	//client folder -> server
	rows, err = db.Query("SELECT tid, title, star FROM folder WHERE deleted=false ORDER BY tid;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_folders: %v", err)
		return
	}

	client_folders := make([]Container, 0, folderCount)
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
			&c.star,
			//&c.created,
			//&c.modified,
		)
		client_folders = append(client_folders, c)
	}
	for _, c := range client_folders {
		_, err := pdb.Exec("INSERT INTO folder (tid, title, star, created, modified, deleted) VALUES ($1, $2, $3, now(), now(), false);",
			c.tid, c.title, c.star)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting folder into sqlite: %v\n", err)
			break
		}
	}

	//client keyword -> server
	// note that the original database does not have a keyword created column
	//rows, err = db.Query("SELECT tid, title, star, created, modified FROM keyword WHERE deleted=false ORDER BY tid;")
	//more important note: current active db thinks its keyword name
	//rows, err = db.Query("SELECT tid, title, star FROM keyword WHERE deleted=false ORDER BY tid;")
	rows, err = db.Query("SELECT tid, name, star FROM keyword WHERE deleted=false ORDER BY tid;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for client_keywords: %v", err)
		return
	}

	client_keywords := make([]Container, 0, keywordCount)
	for rows.Next() {
		var c Container
		rows.Scan(
			&c.tid,
			&c.title,
			&c.star,
			//&c.created,
			//&c.modified,
		)
		client_keywords = append(client_keywords, c)
	}

	for _, c := range client_keywords {
		_, err := pdb.Exec("INSERT INTO keyword (tid, title, star, created, modified, deleted) VALUES ($1, $2, $3, now(), now(), false);",
			c.tid, c.title, c.star)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting new keyword %q into sqlite: %v", truncate(c.title, 15), err)
			break
		}
	}

	/* for the code below need to add guard that entries and keywords might be zero */
	//client entries -> server
	entries := getEntriesBulk(db, taskCount, &lg)

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
		query, args := createBulkInsertQueryReverse(len(e), e)
		err = bulkInsert2(pdb, query, args)
		if err != nil {
			fmt.Fprintf(&lg, "%v\n", err)
		}
		if done {
			fmt.Fprintf(&lg, "\n- %d `entries` were added to the client pdb\n", m)
			break
		}
		i += 1
	}

	taskKeywordPairs := getTaskKeywordPairsBulk(db, taskKeywordCount, &lg)
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
		query, args := createBulkInsertQueryTaskKeywordPairsReverse(len(e), e)
		//fmt.Fprintf(&lg, "query = %s\n, args = %v\n", query, args)
		err = bulkInsert2(pdb, query, args)
		if err != nil {
			fmt.Fprintf(&lg, "%v\n", err)
		}
		if done {
			fmt.Fprintf(&lg, "- %d `taskKeywordPairs` were added to the client pdb\n", m)
			break
		}
		i += 1
	}

	tables := []string{"task", "context", "folder", "keyword"}
	var nextTid int
	for _, table := range tables {
		stmt := fmt.Sprintf("SELECT SETVAL('%s_tid_seq', (SELECT (MAX(tid) + 1) FROM %s), FALSE);", table, table)
		err := pdb.QueryRow(stmt).Scan(&nextTid)
		if err != nil {
			fmt.Fprintf(&lg, "Error setting next tid for table %s: %v\n", table, err)
			continue
		}
		fmt.Fprintf(&lg, "Next tid for table %s: %d\n", table, nextTid)
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
