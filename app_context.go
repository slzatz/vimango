package main

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"time"
  "fmt"
//  "os"
	
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// App encapsulates the global application state
type App struct {
	// Core components
	Session   *Session // Session handles terminal and screen management
	Organizer *Organizer // Organizer manages tasks and their display
	Editor    *Editor // Editor manages text editing
	Windows   []Window // Windows manages multiple windows in the application
  Database  *Database // Database handles database connections and queries
	
	// Database connections and other config
	Config  *dbConfig
	
	// Application state
	LastSync      time.Time
	SyncInProcess bool
	Run           bool
}

// CreateApp creates and initializes the application struct
func CreateApp() *App {
  db := &Database{}
  sess := &Session{}
	return &App{
		Session:   sess,
    Database: db,
    Editor:    &Editor{}, // May not need this here
    Organizer: &Organizer{Session: sess, Database: db},
		Windows:   make([]Window, 0),
		Run:       true,
	}
}

// FromFile returns a dbConfig struct parsed from a file.
func (a *App) FromFile(path string) (*dbConfig, error) {
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

// InitDatabases initializes database connections
func (a *App) InitDatabases(configPath string) error {
	//var err error
	
	// Read config file
	config, err := a.FromFile(configPath)
	if err != nil {
		return err
	}
	
	// Initialize main database
	a.Database.MainDB, err = sql.Open("sqlite3", config.Sqlite3.DB)
	if err != nil {
		return err
	}
	
	// Enable foreign keys
	_, err = a.Database.MainDB.Exec("PRAGMA foreign_keys=ON;")
	if err != nil {
		return err
	}
	
	// Initialize FTS database
	a.Database.FtsDB, err = sql.Open("sqlite3", config.Sqlite3.FTS_DB)
	if err != nil {
		return err
	}

	connect := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Postgres.Host,
		config.Postgres.Port,
		config.Postgres.User,
		config.Postgres.Password,
		config.Postgres.DB,
	)

	a.Database.PG, err = sql.Open("postgres", connect)
	if err != nil {
		//fmt.Fprintf("Error opening postgres db: %v", err)
		return err
	}
	//defer pdb.Close() //need to look at this

	// Ping to connection
	err = a.Database.PG.Ping()
	if err != nil {
		//fmt.Fprintf("postgres ping failure!: %v", err)
		return err
	}
  a.Config = config
	return nil
}

// InitApp initializes the application components
func (a *App) InitApp() {
	
	a.Organizer.cx = 0
	a.Organizer.cy = 0
	a.Organizer.fc = 0
	a.Organizer.fr = 0
	a.Organizer.rowoff = 0
	a.Organizer.coloff = 0
	a.Organizer.sort = "modified"
	a.Organizer.show_deleted = false
	a.Organizer.show_completed = false
	a.Organizer.message = ""
	a.Organizer.highlight[0], a.Organizer.highlight[1] = -1, -1
	a.Organizer.mode = NORMAL
	a.Organizer.last_mode = NORMAL
	a.Organizer.view = TASK
	
	if a.Config.Options.Type == "folder" {
		a.Organizer.taskview = BY_FOLDER
	} else {
		a.Organizer.taskview = BY_CONTEXT
	}
	
	a.Organizer.filter = a.Config.Options.Title
	a.Organizer.marked_entries = make(map[int]struct{})
	a.Organizer.vbuf = vim.BufferNew(0)
	vim.BufferSetCurrent(a.Organizer.vbuf)
}

// LoadInitialData loads the initial data for the organizer
func (a *App) LoadInitialData() {
	// Calculate layout dimensions
	a.Session.textLines = a.Session.screenLines - 2 - TOP_MARGIN
	a.Session.edPct = 60
	
	// Set divider based on percentage
	if a.Session.edPct == 100 {
		a.Session.divider = 1
	} else {
		a.Session.divider = a.Session.screenCols - a.Session.edPct*a.Session.screenCols/100
	}
	
	a.Session.totaleditorcols = a.Session.screenCols - a.Session.divider - 1
	a.Session.eraseScreenRedrawLines()
	
	a.Organizer.FilterEntries(MAX)
	if len(a.Organizer.rows) == 0 {
		a.Organizer.insertRow(0, "", true, false, false, BASE_DATE)
		a.Organizer.rows[0].dirty = false
		a.Session.showOrgMessage("No results were returned")
	}
	
	a.Organizer.readRowsIntoBuffer()
	a.Organizer.bufferTick = vim.BufferGetLastChangedTick(a.Organizer.vbuf)
	a.Organizer.drawPreview() 
	a.Organizer.refreshScreen()
	a.Organizer.drawStatusBar()

	a.Session.showOrgMessage("rows: %d  columns: %d", a.Session.screenLines, a.Session.screenCols)
	a.Session.returnCursor()
}

// Cleanup handles proper shutdown of resources
func (a *App) Cleanup() {
	//if a.DB != nil {
	if a.Database.MainDB != nil {
		a.Database.MainDB.Close()
	}
	
	if a.Database.FtsDB != nil {
		a.Database.FtsDB.Close()
	}
	if a.Database.PG != nil {
	  a.Database.PG.Close()
	}
	if a.Session != nil {
		a.Session.quitApp()
	}
}

// MainLoop is the main application loop
func (a *App) MainLoop() {
	
	// Set global reference for backward compatibility
	p = a.Editor
	
	// No need to sync windows as it's handled in main.go initialization
	
	for a.Run && sess.run {
		key, err := terminal.ReadKey()
		if err != nil {
			sess.showOrgMessage("Readkey problem %w", err)
		}

		var k int
		if key.Regular != 0 {
			k = int(key.Regular)
		} else {
			k = key.Special
		}

		if sess.editorMode {
			// Use our new context-based method
			//textChange := app.Editor.ProcessKey(app, k) // This is where the main loop will call the new method in editor_context.go
			textChange := editorProcessKey(k)

			if !sess.editorMode {
				continue
			}

			if textChange {
				p.scroll()
				p.drawText()
				p.drawStatusBar()
			}
		} else {
			a.Organizer.organizerProcessKey(k)
      //app.Organizer.ProcessKey(app, k) // This is where the main loop will call the new method
			a.Organizer.scroll()
			a.Organizer.refreshScreen()
			if sess.divider > 10 {
				a.Organizer.drawStatusBar()
			}
		}
		sess.returnCursor()
	}
	
	// Clean up when the main loop exits
	a.Cleanup()
}
