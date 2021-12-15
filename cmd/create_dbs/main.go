package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"syscall"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/term"
)

type dbConfig struct {
	Postgres struct {
		Host     string `json:"host"`
		Port     string `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DB       string `json:"db"`
		Test     string `json:"test"`
	} `json:"postgres"`

	Sqlite3 struct {
		DB     string `json:"db"`
		FTS_DB string `json:"fts_db"`
	} `json:"sqlite3"`

	Options struct {
		Type  string `json:"type"`
		Title string `json:"title"`
	} `json:"options`
}

var db *sql.DB
var fts_db *sql.DB

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Do you want to create a new local (sqlite) database? (y or N):")
	res, _ := reader.ReadString('\n')
	if strings.ToLower(res)[:1] != "y" {
		fmt.Println("exiting ...")
		return
	}
	//fmt.Println("We're going to create a new local database")
	//reader := bufio.NewReader(os.Stdin)
	fmt.Print("What do you want to name the database? ")
	res, _ = reader.ReadString('\n')
	res = strings.TrimSpace(res) + ".db"
	if _, err := os.Stat(res); err == nil {
		fmt.Printf("The sqlite database %q already exists", res)
		os.Exit(1)
	}

	config := &dbConfig{}
	config.Sqlite3.DB = res
	config.Sqlite3.FTS_DB = "fts5_" + res
	config.Options.Type = "context"
	config.Options.Title = "none"
	db, _ = sql.Open("sqlite3", config.Sqlite3.DB)
	fts_db, _ = sql.Open("sqlite3", config.Sqlite3.FTS_DB)
	createSqliteDB()

	//reader := bufio.NewReader(os.Stdin)
	fmt.Println("Do you want to create a new remote (postgres) database? \n\x1b[1mNote you need to have created an empty postgres db already]\x1b[0m (y or N):")
	res, _ = reader.ReadString('\n')
	if strings.ToLower(res)[:1] != "y" {
		writeConfigFile(config)
		fmt.Println("exiting ...")
		return
	}

	fmt.Print("What is the host string for the database server? ")
	res, _ = reader.ReadString('\n')
	host := strings.TrimSpace(res)
	config.Postgres.Host = host
	fmt.Print("What is the port for the database server? ")
	res, _ = reader.ReadString('\n')
	port := strings.TrimSpace(res)
	config.Postgres.Port = port
	fmt.Print("Who is the database user? ")
	res, _ = reader.ReadString('\n')
	user := strings.TrimSpace(res)
	config.Postgres.User = user
	fmt.Print("What is the user password? ")
	bpw, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatal(err)
	}
	pw := string(bpw)
	config.Postgres.Password = pw
	fmt.Print("\nWhat is the name of the database? ")
	res, _ = reader.ReadString('\n')
	dbName := strings.TrimSpace(res)
	config.Postgres.DB = dbName

	writeConfigFile(config)
	createPostgresDB(config)
}

func createSqliteDB() {
	path := "sqlite_init.sql"
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(string(b))
	if err != nil {
		log.Fatal(err)
	}

	/*
		note we're creating the tid = 1 to match postgres id
		alternative is not to create the 'none' context or folder
		on the server and let the first sync create it
	*/
	_, err = db.Exec("INSERT INTO sync (machine, timestamp) VALUES ('server', datetime('now', '-5 seconds'));")
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO sync (machine, timestamp) VALUES ('client', datetime('now', '-5 seconds'));")
	if err != nil {
		log.Fatal(err)
	}

	stmt := "INSERT INTO context (title, star, deleted, created, modified, tid) "
	stmt += "VALUES (?, True, False, datetime('now'), datetime('now'), 1);"
	_, err = db.Exec(stmt, "none")
	if err != nil {
		log.Fatal(err)
	}

	stmt = "INSERT INTO folder (title, star, deleted, created, modified, tid) "
	stmt += "VALUES (?, True, False, datetime('now'), datetime('now'), 1);"
	_, err = db.Exec(stmt, "none")
	if err != nil {
		log.Fatal(err)
	}

	_, err = fts_db.Exec("CREATE VIRTUAL TABLE fts USING fts5 (title, note, tag, lm_id UNINDEXED);")
	if err != nil {
		log.Fatal(err)
	}

	/*
		var server_ts string
		row = pdb.QueryRow("SELECT now();")
		err = row.Scan(&server_ts)
		if err != nil {
		  sess.showOrgMessage("Error with getting current time from server: %w", err)
			return
		}
	*/

}

func createPostgresDB(config *dbConfig) {
	connect := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Postgres.Host,
		config.Postgres.Port,
		config.Postgres.User,
		config.Postgres.Password,
		config.Postgres.DB,
	)

	pdb, err := sql.Open("postgres", connect)
	if err != nil {
		log.Fatal(err)
	}
	defer pdb.Close()

	// Ping to connection
	/*
		err = pdb.Ping()
		if err != nil {
			log.Fatalf("postgres ping failure!: %v", err)
			return
		}
	*/

	// creating database separately as postgres user: [posgres@...]$ createdb <database>
	// the check below is to make sure the database is empty
	var exists bool
	err = pdb.QueryRow("SELECT EXISTS(SELECT FROM context);").Scan(&exists)
	if exists {
		fmt.Printf("The database %q does not appear to be empty", config.Postgres.DB)
		os.Exit(1)
	}

	path := "postgres_init.sql"
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	_, err = pdb.Exec(string(b))
	if err != nil {
		log.Fatal(err)
	}

	/*appears that you need to reconnect to the postgres db to
	start updating/querying the database after creating the tables above
	However, it appears that it's better not to try to create the none
	rows of context and folder and let them be created with the first sync
	*/

	/*
		pdb0, err := sql.Open("postgres", connect)
		if err != nil {
			log.Fatal(err)
		}
		defer pdb0.Close()

		stmt := "INSERT INTO context (title, star, deleted, created, modified) "
		stmt += "VALUES ($1, true, false, now(), now());"
		_, err = pdb0.Exec(stmt, "none")
		if err != nil {
			fmt.Println("INSERT INTO context failed")
			log.Fatal(err)
		}

		stmt = "INSERT INTO folder (title, star, deleted, created, modified) "
		stmt += "VALUES ($1, true, false, now(), now());"
		_, err = pdb0.Exec(stmt, "none")
		if err != nil {
			fmt.Println("INSERT INTO folder failed")
			log.Fatal(err)
		}
	*/

}

func writeConfigFile(config *dbConfig) {
	z, _ := json.MarshalIndent(config, "", "  ")
	f, err := os.Create("config.json")
	if err != nil {
		log.Fatal(err)
		return
	}
	defer f.Close()

	_, err = f.Write(z)
	if err != nil {
		log.Fatal(err)
	}
}
