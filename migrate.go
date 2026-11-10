package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/slzatz/vimango/internal/localdb"
)

// CheckForMigrate reports whether --migrate was given.
func CheckForMigrate(args []string) bool {
	for _, arg := range args {
		if arg == "--migrate" {
			return true
		}
	}
	return false
}

// RunMigrate is `vimango --migrate`: bring this machine's vimango.db up to
// date (internal/localdb), say what changed, and return an exit code.
//
// Every start of vimango and vimango-sync runs the same migration quietly, so
// this is never required. It is for doing it deliberately -- after pulling a
// build that changes how the database is stored -- and seeing the result, or
// the reason it could not be done (another program in the middle of a write).
func RunMigrate() int {
	cfg, err := (&App{}).FromFile("config.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: reading config.json: %v\n", err)
		return 1
	}
	path := cfg.Sqlite3.DB
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s not found (run './vimango --init' for a new installation)\n", path)
		return 1
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: opening %s: %v\n", path, err)
		return 1
	}
	defer db.Close()

	changes, err := localdb.Migrate(db)
	for _, c := range changes {
		fmt.Printf("%s: %s\n", path, c)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s: %v\nNothing was left half-done; run --migrate again.\n", path, err)
		return 1
	}
	if len(changes) == 0 {
		fmt.Printf("%s: already up to date\n", path)
	}
	return 0
}
