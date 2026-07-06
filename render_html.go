package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/slzatz/vimango/auth"
)

// DetermineRenderHTML returns the note id passed to --render-html, or -1 if
// the flag is absent. ValidateArgs has already guaranteed the value is
// present and numeric.
func DetermineRenderHTML(args []string) int {
	for i, arg := range args {
		if arg == "--render-html" && i+1 < len(args) {
			if id, err := strconv.Atoi(args[i+1]); err == nil {
				return id
			}
		}
	}
	return -1
}

// RunRenderHTML renders note <id> as a standalone HTML document on stdout
// and returns a process exit code. Headless: no terminal probes, no vim, no
// raw mode — safe to spawn from a host app with stdout piped (the hybrid
// app's preview pane). Google Drive images resolve to inline data URIs via
// the same auth token and image cache the TUI uses; if Drive is not
// configured the image markdown is passed through untouched.
func RunRenderHTML(id int) int {
	app = CreateApp()

	prefs := app.LoadPreferences("preferences.json")
	app.imageCacheMaxWidth = prefs.ImageCacheMaxWidth

	// Optional, same as the TUI boot path: without it gdrive: images stay
	// as-is and everything else still renders.
	if srv, err := auth.GetDriveService(); err == nil {
		app.Session.googleDrive = srv
	}
	initImageCache()

	if err := app.InitDatabases("config.json", DetermineSQLiteDriver(os.Args)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not open databases: %v\n", err)
		return 1
	}

	var title string
	var note sql.NullString
	row := app.Database.MainDB.QueryRow("SELECT title, note FROM task WHERE id=?;", id)
	if err := row.Scan(&title, &note); err != nil {
		fmt.Fprintf(os.Stderr, "Error: no note with id %d: %v\n", id, err)
		return 1
	}

	// Embedded-pane style (standalone=false): no <h1> — the host app
	// shows the title in its own native header — and left-aligned
	// content instead of a centered reading column.
	html, err := RenderNoteAsHTML(title, note.String, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not render note %d: %v\n", id, err)
		return 1
	}

	fmt.Print(html)
	return 0
}
