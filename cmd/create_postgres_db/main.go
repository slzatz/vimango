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

// FromFile returns a dbConfig struct parsed from a file.
func FromFile(path string) (*dbConfig, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg dbConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

var config = &dbConfig{}

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Do you want to create the tables for a new remote (postgres) database? (y or N):")
	res, _ := reader.ReadString('\n')
	if strings.ToLower(res)[:1] != "y" {
		fmt.Println("exiting ...")
		return
	}

	fmt.Println("Do you want to create the config file (vs using an existing one)? (y or N):")
	res, _ = reader.ReadString('\n')
	if strings.ToLower(res)[:1] == "y" {
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
	} else {
		var err error
		config, err = FromFile("config.json")
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("Do you want to create the remote database tables (because it is created but empty)? (y or N):")
	res, _ = reader.ReadString('\n')
	if strings.ToLower(res)[:1] != "y" {
		fmt.Println("exiting ...")
		return
	}
	fmt.Println("Just checking one more time -- do you want to create the necessary tables in the remote database? (y or N):")
	res, _ = reader.ReadString('\n')
	if strings.ToLower(res)[:1] != "y" {
		fmt.Println("exiting ...")
		return
	}
	createPostgresDB(config)
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

	// creating database separately as postgres user: [posgres@...]$ createdb <database>
	// the check below is to make sure the database is empty
	var exists bool
	err = pdb.QueryRow("SELECT EXISTS(SELECT FROM context);").Scan(&exists)
	if exists {
		fmt.Printf("The database %q does not appear to be empty", config.Postgres.DB)
		os.Exit(1)
	}

	path := "postgres_init3.sql"
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	_, err = pdb.Exec(string(b))
	if err != nil {
		log.Fatal(err)
	}
}
