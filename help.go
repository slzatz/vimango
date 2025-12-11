package main

import (
	"fmt"
	"runtime"
)

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

    --go-vim
        Use the pure Go vim implementation (govim) instead of the CGO-based libvim.
        Default: Uses libvim on Linux/Unix when available, govim on Windows or when
        CGO is not available.

    --go-sqlite
        Use the pure Go SQLite driver (modernc.org/sqlite).
        Default: This is already the default driver.
        Note: Works on all platforms including Windows.

    --cgo-sqlite
        Use the CGO-based SQLite driver (github.com/mattn/go-sqlite3).
        Requirements: Only available on Linux/Unix with CGO-enabled builds.
        Note: Silently ignored on Windows or in pure Go builds.

EXAMPLES:
    # First-time setup (creates config.json and databases)
    ./vimango --init

    # Run with default settings (pure Go SQLite, libvim on Linux/CGO builds)
    ./vimango

    # Run with pure Go vim implementation
    ./vimango --go-vim

    # Run with CGO SQLite driver (requires CGO build)
    ./vimango --cgo-sqlite

    # Combine multiple options
    ./vimango --go-vim --cgo-sqlite

BUILD INFORMATION:
    Platform: %s
    Architecture: %s
    Go Version: %s

    Linux/Unix Pure Go Build:
        CGO_ENABLED=0 go build --tags=fts5

    Linux/Unix with CGO (libvim, hunspell, sqlite3):
        CGO_ENABLED=1 go build --tags="fts5,cgo"

    Windows Cross-Compilation:
        GOOS=windows GOARCH=amd64 go build --tags=fts5

PLATFORM-SPECIFIC BEHAVIOR:
    Windows:
        - Automatically uses pure Go implementations for vim and SQLite
        - --cgo-sqlite flag is ignored (not available on Windows)
        - Spell checking is not available (shows helpful message)

    Linux/Unix:
        - CGO builds support libvim, hunspell spell checking, and CGO SQLite
        - Pure Go builds use govim and modernc.org/sqlite
        - All features gracefully degrade when unavailable

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
