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

	"golang.org/x/term"

	_ "github.com/lib/pq"
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

var config = &dbConfig{}

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Do you want to create a new remote (postgres) database? (y or N):")
	res, _ := reader.ReadString('\n')
	if strings.ToLower(res)[:1] != "y" {
		fmt.Println("exiting ...")
		return
	}

	filename := "config.json.test"
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
	pw := string(bpw)
	config.Postgres.Password = pw
	fmt.Print("\nWhat is the name of the database? ")
	res, _ = reader.ReadString('\n')
	dbName := strings.TrimSpace(res)
	config.Postgres.DB = dbName

	z, _ := json.Marshal(config)
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer f.Close()

	_, err = f.Write(z)
	if err != nil {
		log.Fatal(err)
	}
	//createPostgresDB() // need password
}

func createPostgresDB() {
	connect := fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=disable",
		config.Postgres.Host,
		config.Postgres.Port,
		config.Postgres.User,
		config.Postgres.Password,
	)

	pdb, err := sql.Open("postgres", connect)
	if err != nil {
		log.Fatal(err)
	}

	_, err = pdb.Exec("CREATE DATABASE " + config.Postgres.DB)
	if err != nil {
		log.Fatal(err)
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
	stmt := "INSERT INTO context (title, star, deleted, created, modified) "
	stmt += "VALUES (?, True, False, datetime('now'), datetime('now'));"
	_, err = pdb.Exec(stmt, "No Context")
	if err != nil {
		log.Fatal(err)
	}

	stmt = "INSERT INTO folder (title, star, deleted, created, modified) "
	stmt += "VALUES (?, True, False, datetime('now'), datetime('now'));"
	_, err = pdb.Exec(stmt, "No Folder")
	if err != nil {
		log.Fatal(err)
	}
	// Ping to connection
	/*
		err = pdb.Ping()
		if err != nil {
			fmt.Fprintf(&lg, "postgres ping failure!: %v", err)
			return
		}
	*/
}
