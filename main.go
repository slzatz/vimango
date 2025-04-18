package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/slzatz/vimango/rawmode"
	"github.com/slzatz/vimango/vim"
)

// Global app struct
var app *App

// For backward compatibility - these can be removed in a follow-up refactoring
var sess *Session
var org *Organizer
var p *Editor
var db *sql.DB //may only be used in bulk_load
var fts_db *sql.DB
var config *dbConfig //should be easy to eliminate this global variable
var DB *Database

func main() {
	// Create new app context
	app = CreateApp()

	// Set global references for backward compatibility
	sess = app.Session
  org = app.Organizer
  DB = app.Database
	config = app.Config
  //p = app.Editor // do we need this here?

	// Initialize Vim
	vim.Init(0)
	
	// Initialize database connections
	err := app.InitDatabases("config.json")
	if err != nil {
		log.Fatal(err)
	}

	// Create markdown syntax highlighting style
	markdown_style, _ := selectMDStyle("gruvbox.xml")
	sess.markdown_style = markdown_style
	
	// Set session styles
	sess.style = [8]string{"dracula", "fruity", "gruvbox", "monokai", "native", "paraiso-dark", "rrt", "solarized-dark256"}
	sess.styleIndex = 2
	sess.imagePreview = false
	sess.imgSizeY = 800
	
	// Set up signal handling for terminal resize
	signal_chan := make(chan os.Signal, 1)
	signal.Notify(signal_chan, syscall.SIGWINCH)
	
	go func() {
		for {
			_ = <-signal_chan
			app.signalHandler() // this should probably be in the App struct
		}
	}()
	
	// Configure Vim settings
	vim.Execute("set iskeyword+=*")
	vim.Execute("set iskeyword+=`")
	
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
	
	// Initialize application components
	app.InitApp()
	
	// Load initial data
	app.LoadInitialData()
	
	// Set run flag
	app.Run = true
	
	// Run the main loop
	app.MainLoop()
}
