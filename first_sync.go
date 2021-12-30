package main

/** note that sqlite datetime('now') returns utc **/

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

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

	// Ping to connection
	err = pdb.Ping()
	if err != nil {
		fmt.Fprintf(&lg, "postgres ping failure!: %v", err)
		return
	}

	//server contexts
	rows, err := pdb.Query("SELECT id, title, star, created, modified FROM context WHERE context.deleted = FALSE ORDER BY id;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_contexts: %v", err)
		return
	}

	defer rows.Close()

	fmt.Fprint(&lg, "## Server Changes\n")

	var server_contexts []Container
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
	fmt.Fprintf(&lg, "- `Contexts`: **%d**\n", len(server_contexts))

	//server folders
	rows, err = pdb.Query("SELECT id, title, star, created, modified FROM folder WHERE folder.deleted = FALSE ORDER BY id;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_folders: %v", err)
		return
	}

	var server_folders []Container
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
	fmt.Fprintf(&lg, "- `Folders`: %d\n", len(server_folders))

	//server keywords
	rows, err = pdb.Query("SELECT id, name, star, modified FROM keyword WHERE keyword.deleted = FALSE;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_keywords: %v", err)
		return
	}

	var server_keywords []Container
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
	fmt.Fprintf(&lg, "- `Keywords`: %d\n", len(server_keywords))

	//server entries
	rows, err = pdb.Query("SELECT id, title, star, note, created, modified, context_id, folder_id, added, completed FROM task WHERE deleted = False ORDER By id;")
	if err != nil {
		fmt.Fprintf(&lg, "Error in SELECT for server_updated_entries: %v", err)
		return
	}

	var server_entries []serverEntry
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
	fmt.Fprintf(&lg, "- `Entries`: %d\n", len(server_entries))

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

	/**********should come before container deletes to change tasks here*****************/
	//i := 0
	for _, e := range server_entries {
		res, err := db.Exec("INSERT INTO task (tid, title, star, created, added, completed, context_tid, folder_tid, note, modified, deleted) "+
			"VALUES (?, ?, ?, datetime('now'), ?, ?, ?, ?, ?, datetime('now'), false);",
			e.id, e.title, e.star, e.added, e.completed, e.context_id, e.folder_id, e.note)
		if err != nil {
			fmt.Fprintf(&lg, "%v %v %v %v %v %v %v\n", e.id, e.title, e.star, e.context_id, e.folder_id, e.added, e.completed)
			fmt.Fprintf(&lg, "Error inserting entry %q into sqlite: %v\n", truncate(e.title, 15), err)
			continue
		}
		id, _ := res.LastInsertId()
		client_id := int(id)
		_, err = fts_db.Exec("INSERT INTO fts (title, note, lm_id) VALUES (?, ?, ?);", e.title, e.note, client_id)
		if err != nil {
			fmt.Fprintf(&lg, "Error inserting into fts_db for entry with id %d: %v\n", client_id, err)
			//break
		}

		// Update the client entry's keywords
		kwns := getTaskKeywordsS0(pdb, &lg, e.id) // returns []string
		for _, kwn := range kwns {
			keyword_id := keywordExistsS0(db, &lg, kwn)
			if keyword_id != -1 { // ? should create the keyword if it doesn't exist
				addTaskKeywordS0(db, &lg, keyword_id, client_id)
			}
		}
		tag := strings.Join(kwns, ",")
		_, err = fts_db.Exec("UPDATE fts SET tag=$1 WHERE lm_id=$2;", tag, client_id)
		if err != nil {
			fmt.Fprintf(&lg, "Error in Update tag in fts: %v\n", err)
		}
	}

	/*********************end of sync changes*************************/

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
	fmt.Fprintf(&lg, "Server UTC timestamp: %s", strings.Replace(tc(server_ts, 19, false), "T", " ", 1))
	success = true

	return
}
