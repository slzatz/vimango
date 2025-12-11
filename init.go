package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
)

// Default UUIDs for the "none" containers
const (
	DefaultContextUUID = "00000000-0000-0000-0000-000000000001"
	DefaultFolderUUID  = "00000000-0000-0000-0000-000000000002"
)

// Schema for SQLite main database
const sqliteSchema = `
CREATE TABLE context (
	id INTEGER NOT NULL,
	tid INTEGER,
	uuid TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	star BOOLEAN DEFAULT FALSE,
	deleted BOOLEAN DEFAULT FALSE,
	modified TEXT DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE (tid),
	UNIQUE (title),
	CHECK (star IN (0, 1)),
	CHECK (deleted IN (0, 1))
);
CREATE TABLE folder (
	id INTEGER NOT NULL,
	tid INTEGER,
	uuid TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	star BOOLEAN DEFAULT FALSE,
	deleted BOOLEAN DEFAULT FALSE,
	modified TEXT DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE (tid),
	UNIQUE (title),
	CHECK (star IN (0, 1)),
	CHECK (deleted IN (0, 1))
);
CREATE TABLE keyword (
	id INTEGER NOT NULL,
	tid INTEGER,
	uuid TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	star BOOLEAN DEFAULT FALSE,
	deleted BOOLEAN DEFAULT FALSE,
	modified TEXT DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE (tid),
	UNIQUE (title),
	CHECK (star IN (0, 1)),
	CHECK (deleted IN (0, 1))
);
CREATE TABLE task (
	id INTEGER NOT NULL,
	tid INTEGER,
	star BOOLEAN DEFAULT FALSE,
	title TEXT NOT NULL,
	folder_tid INTEGER,
	context_tid INTEGER,
	folder_uuid TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000002',
	context_uuid TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001',
	note TEXT,
	archived BOOLEAN DEFAULT FALSE,
	deleted BOOLEAN DEFAULT FALSE,
	added TEXT NOT NULL,
	modified TEXT DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	FOREIGN KEY(folder_uuid) REFERENCES folder (uuid),
	FOREIGN KEY(context_uuid) REFERENCES context (uuid),
	UNIQUE (tid),
	CHECK (star IN (0, 1)),
	CHECK (archived IN (0, 1)),
	CHECK (deleted IN (0, 1))
);
CREATE TABLE sync (
	id INTEGER NOT NULL,
	machine TEXT NOT NULL,
	timestamp TEXT DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE (machine)
);
CREATE TABLE task_keyword (
	task_tid INTEGER NOT NULL,
	keyword_tid INTEGER,
	keyword_uuid TEXT NOT NULL,
	PRIMARY KEY (task_tid, keyword_uuid),
	FOREIGN KEY(task_tid) REFERENCES task (tid),
	FOREIGN KEY(keyword_uuid) REFERENCES keyword (uuid)
);
CREATE TABLE sync_log (
	id INTEGER NOT NULL,
	title TEXT,
	modified TEXT,
	note TEXT,
	PRIMARY KEY (id)
);
`

// generateUUID generates a new UUID string
func generateUUID() string {
	return uuid.New().String()
}

// CheckForInit checks if --init flag is present and runs initialization if so.
// Returns true if --init was handled (caller should exit), false otherwise.
func CheckForInit(args []string) bool {
	for _, arg := range args[1:] {
		if arg == "--init" {
			runInit()
			return true
		}
	}
	return false
}

// runInit performs first-time setup: creates config.json and SQLite databases
func runInit() {
	fmt.Println("Vimango First-Time Setup")
	fmt.Println("=========================")
	fmt.Println()

	// Check if config.json already exists
	if _, err := os.Stat("config.json"); err == nil {
		fmt.Println("Error: config.json already exists.")
		fmt.Println("If you want to reinitialize, please remove or rename config.json first.")
		os.Exit(1)
	}

	// Check if databases already exist
	if _, err := os.Stat("vimango.db"); err == nil {
		fmt.Println("Error: vimango.db already exists.")
		fmt.Println("If you want to reinitialize, please remove or rename the database first.")
		os.Exit(1)
	}
	if _, err := os.Stat("fts5_vimango.db"); err == nil {
		fmt.Println("Error: fts5_vimango.db already exists.")
		fmt.Println("If you want to reinitialize, please remove or rename the database first.")
		os.Exit(1)
	}

	// Check for required glamour style file
	if _, err := os.Stat("default.json"); os.IsNotExist(err) {
		if _, err := os.Stat("darkslz.json"); os.IsNotExist(err) {
			fmt.Println("Error: No glamour style file found (default.json or darkslz.json).")
			fmt.Println("Please ensure you have cloned the complete repository.")
			os.Exit(1)
		}
	}

	// Create default config
	config := &dbConfig{
		Sqlite3: struct {
			DB     string `json:"db"`
			FTS_DB string `json:"fts_db"`
		}{
			DB:     "vimango.db",
			FTS_DB: "fts5_vimango.db",
		},
		Options: struct {
			Type  string `json:"type"`
			Title string `json:"title"`
		}{
			Type:  "folder",
			Title: "none",
		},
		Chroma: struct {
			Style string `json:"style"`
		}{
			Style: "gruvbox_mod.xml",
		},
		Glamour: struct {
			Style string `json:"style"`
		}{
			Style: "default.json",
		},
	}

	// Write config.json
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Printf("Error creating config: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("config.json", configData, 0644); err != nil {
		fmt.Printf("Error writing config.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created config.json")

	// Create SQLite databases using the configurable driver
	// Use pure Go driver for init since it works everywhere
	sqliteConfig := &SQLiteConfig{Driver: SQLiteDriverModernC}

	// Create main database
	mainDB, err := sqliteConfig.OpenSQLiteDB("vimango.db")
	if err != nil {
		fmt.Printf("Error creating main database: %v\n", err)
		os.Exit(1)
	}
	defer mainDB.Close()

	// Execute schema
	if _, err := mainDB.Exec(sqliteSchema); err != nil {
		fmt.Printf("Error creating schema: %v\n", err)
		os.Exit(1)
	}

	// Insert default context and folder with known UUIDs
	stmt := "INSERT INTO context (title, tid, uuid) VALUES (?, 1, ?);"
	if _, err := mainDB.Exec(stmt, "none", DefaultContextUUID); err != nil {
		fmt.Printf("Error inserting default context: %v\n", err)
		os.Exit(1)
	}

	stmt = "INSERT INTO folder (title, tid, uuid) VALUES (?, 1, ?);"
	if _, err := mainDB.Exec(stmt, "none", DefaultFolderUUID); err != nil {
		fmt.Printf("Error inserting default folder: %v\n", err)
		os.Exit(1)
	}

	// Insert sync records
	if _, err := mainDB.Exec("INSERT INTO sync (machine) VALUES ('server');"); err != nil {
		fmt.Printf("Error inserting sync record: %v\n", err)
		os.Exit(1)
	}
	if _, err := mainDB.Exec("INSERT INTO sync (machine) VALUES ('client');"); err != nil {
		fmt.Printf("Error inserting sync record: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Created vimango.db with schema and default data")

	// Create FTS database
	ftsDB, err := sqliteConfig.OpenSQLiteDB("fts5_vimango.db")
	if err != nil {
		fmt.Printf("Error creating FTS database: %v\n", err)
		os.Exit(1)
	}
	defer ftsDB.Close()

	if _, err := ftsDB.Exec("CREATE VIRTUAL TABLE fts USING fts5 (title, note, tag, tid UNINDEXED);"); err != nil {
		fmt.Printf("Error creating FTS table: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created fts5_vimango.db with FTS5 virtual table")

	fmt.Println()
	fmt.Println("Setup complete! You can now run vimango normally.")
	fmt.Println()
	fmt.Println("Optional next steps:")
	fmt.Println("  - Edit config.json to add Claude API key for deep research")
	fmt.Println("  - Edit config.json to add PostgreSQL settings for remote sync")
	fmt.Println("  - Set up Google Drive credentials (see README.md)")
}
