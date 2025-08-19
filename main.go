package main

import (
	"fmt"
	"log"
	"os"

	"github.com/slzatz/vimango/auth"
	"github.com/slzatz/vimango/rawmode"
	"github.com/slzatz/vimango/vim"
)

// Global app struct
var app *App

func main() {
	app = CreateApp()
	srv, err := auth.GetDriveService()
	if err != nil {
		log.Fatalf("Failed to get Google Drive service: %v", err)
	}
	app.Session.googleDrive = srv

	// Initialize image cache
	initImageCache()

	// Configure Vim implementation selection
	vimConfig := DetermineVimDriver(os.Args)
	//LogVimDriverChoice(vimConfig) //debugging
	useGoVim := vimConfig.ShouldUseGoVim()

	// Set up logging if Go implementation is used
	if useGoVim {
		// Set up logging to file instead of console to avoid flashing messages
		logFile, err := os.OpenFile("govim_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(logFile)
			log.Println("Go Vim implementation initialized")
		}
	}

	// Initialize Vim with the appropriate implementation
	vim.InitializeVim(useGoVim, 0)

	// Configure SQLite driver selection
	// currently defaulting to modernc sqlite driver unless --cgo-sqlite3 is specified
	sqliteConfig := DetermineSQLiteDriver(os.Args)
	//LogSQLiteDriverChoice(sqliteConfig) //debugging

	// Initialize database connections
	err = app.InitDatabases("config.json", sqliteConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Set up platform-specific signal handling
	setupSignalHandling(app)

	// Configure Vim settings
	vim.ExecuteCommand("set iskeyword+=*")
	vim.ExecuteCommand("set iskeyword+=`")

	// Enable raw mode
	origCfg, err := rawmode.Enable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error enabling raw mode: %v", err)
		os.Exit(1)
	}

	app.origTermCfg = origCfg
	app.Session.editorMode = false

	// Get window size
	err = app.Screen.GetWindowSize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting window size: %v", err)
		os.Exit(1)
	}
	//os.Exit(0) //debugging

	app.InitApp()
	app.LoadInitialData()
	app.Run = true
	app.MainLoop()
}
