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

	_ "github.com/mattn/go-sqlite3"
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
	filename := "config.json.test"
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("There is no config file - do you want to create a new local (sqlite) database? (y or N):")
	res, _ := reader.ReadString('\n')
	if strings.ToLower(res)[:1] == "y" {
		fmt.Println("We're going to create a new local database")
		/*************************/
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("What do you want to name the database? ")
		res, _ := reader.ReadString('\n')
		res = strings.TrimSpace(res) + ".db"
		config := &dbConfig{}
		config.Sqlite3.DB = res
		config.Sqlite3.FTS_DB = "fts5_" + res
		config.Options.Type = "context"
		config.Options.Title = "none"
		z, _ := json.Marshal(config)
		f, err := os.Create(filename)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		_, err = f.Write(z)
		if err != nil {
			log.Fatal(err)
		}
		db, _ = sql.Open("sqlite3", config.Sqlite3.DB)
		fts_db, _ = sql.Open("sqlite3", config.Sqlite3.FTS_DB)
		createSqliteDB()
	}
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
	_, err = db.Exec("INSERT INTO sync (machine, timestamp) VALUES ('server', datetime('now'));")
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO sync (machine, timestamp) VALUES ('client', datetime('now'));")
	if err != nil {
		log.Fatal(err)
	}

}
