package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
)

// validFlags lists every command-line flag the application accepts.
// Keep this in sync with ShowHelp and the Determine*/CheckFor* functions.
var validFlags = map[string]bool{
	"--help":        true,
	"-h":            true,
	"--init":        true,
	"--go-sqlite":   true,
	"--cgo-sqlite":  true,
	"--editor":      true,
	"--open":        true, // takes a value: --open <id>; requires --editor
	"--render-html": true, // takes a value: --render-html <id>; headless, prints HTML and exits
}

// ValidateArgs checks that every argument in args[1:] is a recognized flag.
// On the first unknown argument it prints an error and the help text to
// stderr, then exits with status 2.
func ValidateArgs(args []string) {
	rest := args[1:]
	skipNext := false
	for i, arg := range rest {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--open" || arg == "--render-html" {
			if i+1 >= len(rest) {
				fmt.Fprintf(os.Stderr, "Error: %s requires a note id\n\n", arg)
				ShowHelp()
				os.Exit(2)
			}
			if _, err := strconv.Atoi(rest[i+1]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s requires a numeric note id, got %q\n\n", arg, rest[i+1])
				ShowHelp()
				os.Exit(2)
			}
			skipNext = true
			continue
		}
		if !validFlags[arg] {
			fmt.Fprintf(os.Stderr, "Error: unrecognized command-line argument %q\n\n", arg)
			ShowHelp()
			os.Exit(2)
		}
	}
	if editorOnly, openId := DetermineEditorBoot(args); openId != -1 && !editorOnly {
		fmt.Fprintf(os.Stderr, "Error: --open requires --editor\n\n")
		ShowHelp()
		os.Exit(2)
	}
}

// DetermineEditorBoot reports whether --editor was given and the note id
// passed to --open (-1 if absent). ValidateArgs has already guaranteed the
// --open value is present and numeric.
func DetermineEditorBoot(args []string) (editorOnly bool, openId int) {
	openId = -1
	for i, arg := range args {
		switch arg {
		case "--editor":
			editorOnly = true
		case "--open":
			if i+1 < len(args) {
				if id, err := strconv.Atoi(args[i+1]); err == nil {
					openId = id
				}
			}
		}
	}
	return editorOnly, openId
}

const version = "0.1.0"

// ShowHelp displays the help message for vimango
func ShowHelp() {
	helpText := `vimango - A vim-based note-taking application with SQLite storage

USAGE:
    vimango [OPTIONS]

DESCRIPTION:
    vimango is a terminal-based note-taking application that combines vim editing
    capabilities with SQLite database storage. It supports full-text search, markdown
    formatting, and an organizer interface for managing notes, contexts, folders, and
    keywords.

OPTIONS:
    --help, -h
        Display this help message and exit

    --init
        Run first-time setup. Creates config.json with default settings and
        initializes the SQLite databases (vimango.db and fts5_vimango.db).
        Use this when running vimango for the first time after cloning.

    --go-sqlite
        Use the pure Go SQLite driver (modernc.org/sqlite).
        Default: This is already the default driver.
        Note: Works on all platforms including Windows.

    --cgo-sqlite
        Use the CGO-based SQLite driver (github.com/mattn/go-sqlite3).
        Requirements: Only available on Linux/Unix with CGO-enabled builds.
        Note: Silently ignored on Windows or in pure Go builds.

    --editor
        Boot directly into the editor, full-width; the organizer is never
        rendered. Intended for host applications embedding vimango as an
        editor pane, but works in any terminal. Use :open <id> to switch
        notes; :q quits the application.

    --open <id>
        With --editor, load the note with the given database id into the
        boot editor window. Without --open, the most recently modified
        note in the configured startup view is opened. Requires --editor.

    --render-html <id>
        Render the note with the given database id as a standalone HTML
        document on stdout and exit. Headless (no terminal UI); intended
        for host applications embedding a preview pane. Google Drive
        images are inlined as data URIs when Drive is configured.

EXAMPLES:
    # First-time setup (creates config.json and databases)
    ./vimango --init

    # Run with default settings (pure Go SQLite, libvim)
    ./vimango

    # Run with CGO SQLite driver
    ./vimango --cgo-sqlite

    # Editor-only mode (e.g. embedded in a host app), opening note 5
    ./vimango --editor --open 5

    # Print note 5 as HTML (e.g. for a host app's preview pane)
    ./vimango --render-html 5

BUILD INFORMATION:
    Platform: %s
    Architecture: %s
    Go Version: %s

    Build (CGO is required — vim editing is libvim; Windows is unsupported):
        CGO_ENABLED=1 go build --tags="fts5,cgo"

FEATURES:
    - Vim-based text editing with normal and insert modes
    - SQLite database for note storage with FTS5 full-text search
    - Organizer mode for browsing and managing notes
    - Markdown support with preview and PDF export
    - Context, folder, and keyword organization
    - AI-powered deep research with web search and fetch
    - WebView integration for in-app web browsing
    - Spell checking (CGO builds only)

For more information, see the readme file in the project directory.
`

	fmt.Printf(helpText, runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// CheckForHelp checks if --help or -h flag is present in arguments
// Returns true if help was requested
func CheckForHelp(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			ShowHelp()
			return true
		}
	}
	return false
}
