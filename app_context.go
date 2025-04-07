package main

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"time"
	
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// AppContext encapsulates the global application state
type AppContext struct {
	// Core components
	Session   *Session
	Organizer *Organizer
	Editor    *Editor
	Windows   []Window
	
	// Database connections and context
	Config  *dbConfig
	DB      *sql.DB
	FtsDB   *sql.DB
	DBCtx   *DBContext
	
	// Application state
	LastSync      time.Time
	SyncInProcess bool
	Run           bool
}

// NewAppContext creates and initializes a new application context
func NewAppContext() *AppContext {
	return &AppContext{
		Session:   &Session{},
		Windows:   make([]Window, 0),
		Run:       true,
	}
}

// FromFile returns a dbConfig struct parsed from a file.
func (a *AppContext) FromFile(path string) (*dbConfig, error) {
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
func (a *AppContext) InitDatabases(configPath string) error {
	var err error
	
	// Read config file
	a.Config, err = a.FromFile(configPath)
	if err != nil {
		return err
	}
	
	// Initialize main database
	a.DB, err = sql.Open("sqlite3", a.Config.Sqlite3.DB)
	if err != nil {
		return err
	}
	
	// Enable foreign keys
	_, err = a.DB.Exec("PRAGMA foreign_keys=ON;")
	if err != nil {
		return err
	}
	
	// Initialize FTS database
	a.FtsDB, err = sql.Open("sqlite3", a.Config.Sqlite3.FTS_DB)
	if err != nil {
		return err
	}
	
	// Initialize DB context
	a.DBCtx = NewDBContext(a)
	
	return nil
}

// InitApp initializes the application components
func (a *AppContext) InitApp() {
	// Initialize organizer
	a.Organizer = &Organizer{Session: a.Session}
  //app.Organizer = &Organizer // This is where we'd like to go
	
	// Initialize Organizer values that were previously in main.go
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
	
	// Initialize Editor (even though it's not used immediately)
	a.Editor = &Editor{}
	
	// Set global references for backward compatibility
	org = a.Organizer
}

// LoadInitialData loads the initial data for the organizer
func (a *AppContext) LoadInitialData() {
	org := a.Organizer
	sess := a.Session
	
	// Calculate layout dimensions
	sess.textLines = sess.screenLines - 2 - TOP_MARGIN
	sess.edPct = 60
	
	// Set divider based on percentage
	if sess.edPct == 100 {
		sess.divider = 1
	} else {
		sess.divider = sess.screenCols - sess.edPct*sess.screenCols/100
	}
	
	sess.totaleditorcols = sess.screenCols - sess.divider - 1
	sess.eraseScreenRedrawLines()
	
	// Load organizer data using DB context
	org.rows = a.FilterEntries(org.taskview, org.filter, org.show_deleted, org.sort, org.sortPriority, MAX)
	if len(org.rows) == 0 {
		org.insertRow(0, "", true, false, false, BASE_DATE)
		org.rows[0].dirty = false
		sess.showOrgMessage("No results were returned")
	}
	
	org.readRowsIntoBuffer()
	org.bufferTick = vim.BufferGetLastChangedTick(org.vbuf)
	org.drawPreview()
	org.refreshScreen()
	org.drawStatusBar()
	
	sess.showOrgMessage("rows: %d  columns: %d", sess.screenLines, sess.screenCols)
	sess.returnCursor()
}

// Cleanup handles proper shutdown of resources
func (a *AppContext) Cleanup() {
	if a.DB != nil {
		a.DB.Close()
	}
	
	if a.FtsDB != nil {
		a.FtsDB.Close()
	}
	
	if a.Session != nil {
		a.Session.quitApp()
	}
}

// SynchronizeWrapper provides a synchronization function that can be called from existing code
func (a *AppContext) SynchronizeWrapper(reportOnly bool) (string, error) {
	// This is a wrapper around the Synchronize method
	return a.Synchronize(reportOnly), nil
}

// MainLoop is the main application loop
func (a *AppContext) MainLoop() {
	sess := a.Session
	org := a.Organizer
	
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
			organizerProcessKey(k)
      //app.Organizer.ProcessKey(app, k) // This is where the main loop will call the new method
			org.scroll()
			org.refreshScreen()
			if sess.divider > 10 {
				org.drawStatusBar()
			}
		}
		sess.returnCursor()
	}
	
	// Clean up when the main loop exits
	a.Cleanup()
}
