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

func keywordExistsS0(dbase *sql.DB, plg io.Writer, name string) int {
	row := dbase.QueryRow("SELECT keyword.id FROM keyword WHERE keyword.name=$1;", name)
	var id int
	err := row.Scan(&id)
	if err != nil {
		fmt.Fprintf(plg, "Error in keywordExistsS0: %v\n", err)
		return -1
	}
	return id
}

func addTaskKeywordS0(dbase *sql.DB, plg io.Writer, keyword_id, entry_id int) {
	_, err := dbase.Exec("INSERT INTO task_keyword (task_id, keyword_id) VALUES ($1, $2);",
		entry_id, keyword_id)
	if err != nil {
		fmt.Fprintf(plg, "Error in addTaskKeywordS0 = INSERT INTO task_keyword: %v\n", err)
		return
	}
}

func getTaskKeywordsS0(dbase *sql.DB, plg io.Writer, id int) []string {

	rows, err := dbase.Query("SELECT keyword.name FROM task_keyword LEFT OUTER JOIN keyword ON "+
		"keyword.id=task_keyword.keyword_id WHERE task_keyword.task_id=$1;", id)
	if err != nil {
		fmt.Fprintf(plg, "Error in getTaskKeywordsS0: %v\n", err)
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

func firstSync(reportOnly bool) (log string) {

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
	fmt.Fprintf(&lg, "Starting initial sync at %v\n", t0)

	var count int

	//server contexts
	err = pdb.QueryRow("SELECT COUNT(*) FROM context WHERE deleted=false;").Scan(&count)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for server_contexts: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Contexts`: %d\n", count)

	rows, err := pdb.Query("SELECT id, title, star, created, modified FROM context WHERE deleted=false ORDER BY id;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_contexts: %v", err)
		return
	}

	defer rows.Close()

	server_contexts := make([]Container, 0, count)
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

	//server folders
	err = pdb.QueryRow("SELECT COUNT(*) FROM folder WHERE deleted=false;").Scan(&count)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for server_folders: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Folders`: %d\n", count)
	rows, err = pdb.Query("SELECT id, title, star, created, modified FROM folder WHERE deleted=false ORDER BY id;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_folders: %v", err)
		return
	}

	server_folders := make([]Container, 0, count)
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

	//server keywords
	err = pdb.QueryRow("SELECT COUNT(*) FROM keyword WHERE deleted=false;").Scan(&count)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for server_keywords: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Keywords`: %d\n", count)
	rows, err = pdb.Query("SELECT id, name, star, modified FROM keyword WHERE deleted=false;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_keywords: %v", err)
		return
	}

	server_keywords := make([]Container, 0, count)
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

	//server entries
	err = pdb.QueryRow("SELECT COUNT(*) FROM task WHERE deleted=false;").Scan(&count)
	if err != nil {
		fmt.Fprintf(&lg, "Error in COUNT(*) for server_entries: %v", err)
		return
	}
	fmt.Fprintf(&lg, "- `Entries`: %d\n", count)

	rows, err = pdb.Query("SELECT id, title, star, note, created, modified, context_id, folder_id, added, completed FROM task WHERE deleted=false ORDER BY id;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_entries: %v", err)
		return
	}

	//var server_entries []serverEntry
	server_entries := make([]serverEntry, 0, count)
	for rows.Next() {
		var e serverEntry
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
		server_entries = append(server_entries, e)
	}

	if reportOnly {
		// note there is a defer log.String()
		return
	}

	/****************below is where changes start***********************************/

	//server contexts -> client
	c := server_contexts[0]
	_, err = db.Exec("UPDATE context SET title=?, star=?, modified=datetime('now') WHERE tid=?;", c.title, c.star, c.id)
	if err != nil {
		fmt.Fprintf(&lg, "Error updating sqlite context with tid: %v: %v\n", c.id, err)
	}

	for _, c := range server_contexts[1:] {
		_, err := db.Exec("INSERT INTO context (tid, title, star, created, modified, deleted) VALUES (?,?,?,?, datetime('now'), false);",
			c.id, c.title, c.star, c.created)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting context into sqlite: %v", err)
			break
		}
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
			fmt.Fprintf(&lg, "Error inserting folder into sqlite: %v", err)
			break
		}
	}

	for _, c := range server_keywords {
		_, err := db.Exec("INSERT INTO keyword (tid, name, star, modified, deleted) VALUES (?,?,?, datetime('now'), false);",
			c.id, c.title, c.star)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting new keyword %q into sqlite: %v", truncate(c.title, 15), err)
			break
		}
	}

	for i, e := range server_entries {
		if i%200 == 0 {
			sess.showEdMessage("%d entries processed", i)
		}
		var client_id int
		err := db.QueryRow("INSERT INTO task (tid, title, star, created, added, completed, context_tid, folder_tid, note, modified, deleted) "+
			"VALUES (?, ?, ?, datetime('now'), ?, ?, ?, ?, ?, datetime('now'), false) RETURNING id;",
			e.id, e.title, e.star, e.added, e.completed, e.context_id, e.folder_id, e.note).Scan(&client_id)

		if err != nil {
			fmt.Fprintf(&lg, "%v %v %v %v %v %v %v\n", e.id, e.title, e.star, e.context_id, e.folder_id, e.added, e.completed)
			fmt.Fprintf(&lg, "Error inserting entry %q into sqlite: %v\n", truncate(e.title, 15), err)
			continue
		}
		_, err = fts_db.Exec("INSERT INTO fts (title, note, lm_id) VALUES (?, ?, ?);", e.title, e.note, client_id)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting into fts_db for entry with id %d: %v\n", client_id, err)
		}

		// Update the client entry's keywords
		kwns := getTaskKeywordsS0(pdb, &lg, e.id)
		for _, kwn := range kwns {
			keyword_id := keywordExistsS0(db, &lg, kwn)
			if keyword_id != -1 {
				addTaskKeywordS0(db, &lg, keyword_id, client_id)
			}
		}
		tag := strings.Join(kwns, ",")
		_, err = fts_db.Exec("UPDATE fts SET tag=$1 WHERE lm_id=$2;", tag, client_id)
		if err != nil {
			fmt.Fprintf(&lg, "Error in Update tag in fts: %v\n", err)
		}
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
