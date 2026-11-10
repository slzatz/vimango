package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/slzatz/vimango/internal/localdb"
)

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

// runInit performs first-time setup: creates config.json and the SQLite database
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
			DB string `json:"db"`
		}{
			DB: "vimango.db",
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

	// Create main database
	mainDB, err := localdb.Open("vimango.db")
	if err != nil {
		fmt.Printf("Error creating main database: %v\n", err)
		os.Exit(1)
	}
	defer mainDB.Close()

	// The schema, the two default containers, the sync watermarks, and the
	// search index.
	if err := localdb.Create(mainDB); err != nil {
		fmt.Printf("Error creating vimango.db: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Created vimango.db with schema, search index and default data")

	fmt.Println()
	fmt.Println("Setup complete! You can now run vimango normally.")
	fmt.Println()
	fmt.Println("Optional next steps:")
	fmt.Println("  - Edit config.json to add PostgreSQL settings for remote sync")
	fmt.Println("  - Set up Google Drive credentials (see README.md)")
}
