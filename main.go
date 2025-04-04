package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/slzatz/vimango/rawmode"
	"github.com/slzatz/vimango/vim"
)

// Global app context
var appCtx *AppContext

// For backward compatibility - these can be removed in a follow-up refactoring
var sess *Session
var org *Organizer
var p *Editor
var db *sql.DB
var fts_db *sql.DB
var config *dbConfig
var windows []Window

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

func main() {
	// Create new app context
	appCtx = NewAppContext()
	
	// Initialize Vim
	vim.Init(0)
	
	// Initialize database connections
	err := appCtx.InitDatabases("config.json")
	if err != nil {
		log.Fatal(err)
	}
	
	// Set global references for backward compatibility
	sess = appCtx.Session
	config = appCtx.Config
	db = appCtx.DB
	fts_db = appCtx.FtsDB
	
	// Initialize windows array
	appCtx.Windows = make([]Window, 0)
	windows = appCtx.Windows // This is a slice, so need to make sure it's the same slice, not just a copy
	
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
			sess.signalHandler()
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
	
	sess.origTermCfg = origCfg
	sess.editorMode = false
	
	// Get window size
	err = sess.GetWindowSize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting window size: %v", err)
		os.Exit(1)
	}
	
	// Initialize application components
	appCtx.InitApp()
	org = appCtx.Organizer
	
	// Load initial data
	appCtx.LoadInitialData()
	
	// Set run flag
	sess.run = true
	appCtx.Run = true
	
	// Run the main loop
	appCtx.MainLoop()
}
