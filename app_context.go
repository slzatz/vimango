package main

import (
	"database/sql"
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

// InitDatabases initializes database connections
func (app *AppContext) InitDatabases(configPath string) error {
	var err error
	
	// Read config file
	app.Config, err = FromFile(configPath)
	if err != nil {
		return err
	}
	
	// Initialize main database
	app.DB, err = sql.Open("sqlite3", app.Config.Sqlite3.DB)
	if err != nil {
		return err
	}
	
	// Enable foreign keys
	_, err = app.DB.Exec("PRAGMA foreign_keys=ON;")
	if err != nil {
		return err
	}
	
	// Initialize FTS database
	app.FtsDB, err = sql.Open("sqlite3", app.Config.Sqlite3.FTS_DB)
	if err != nil {
		return err
	}
	
	// Initialize DB context
	app.DBCtx = NewDBContext(app)
	
	return nil
}

// InitApp initializes the application components
func (app *AppContext) InitApp() {
	// Initialize organizer
	app.Organizer = &Organizer{Session: app.Session}
	
	// Initialize Organizer values that were previously in main.go
	app.Organizer.cx = 0
	app.Organizer.cy = 0
	app.Organizer.fc = 0
	app.Organizer.fr = 0
	app.Organizer.rowoff = 0
	app.Organizer.coloff = 0
	app.Organizer.sort = "modified"
	app.Organizer.show_deleted = false
	app.Organizer.show_completed = false
	app.Organizer.message = ""
	app.Organizer.highlight[0], app.Organizer.highlight[1] = -1, -1
	app.Organizer.mode = NORMAL
	app.Organizer.last_mode = NORMAL
	app.Organizer.view = TASK
	
	if app.Config.Options.Type == "folder" {
		app.Organizer.taskview = BY_FOLDER
	} else {
		app.Organizer.taskview = BY_CONTEXT
	}
	
	app.Organizer.filter = app.Config.Options.Title
	app.Organizer.marked_entries = make(map[int]struct{})
	app.Organizer.vbuf = vim.BufferNew(0)
	vim.BufferSetCurrent(app.Organizer.vbuf)
	
	// Initialize Editor (even though it's not used immediately)
	app.Editor = &Editor{}
	
	// Set global references for backward compatibility
	org = app.Organizer
}

// LoadInitialData loads the initial data for the organizer
func (app *AppContext) LoadInitialData() {
	org := app.Organizer
	sess := app.Session
	
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
	org.rows = app.DBCtx.FilterEntries(org.taskview, org.filter, org.show_deleted, org.sort, org.sortPriority, MAX)
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
func (app *AppContext) Cleanup() {
	if app.DB != nil {
		app.DB.Close()
	}
	
	if app.FtsDB != nil {
		app.FtsDB.Close()
	}
	
	if app.Session != nil {
		app.Session.quitApp()
	}
}

// SynchronizeWrapper provides a synchronization function that can be called from existing code
func (app *AppContext) SynchronizeWrapper(reportOnly bool) (string, error) {
	// This is a wrapper around the Synchronize method
	return app.Synchronize(reportOnly), nil
}

// MainLoop is the main application loop
func (app *AppContext) MainLoop() {
	sess := app.Session
	org := app.Organizer
	
	// Set global reference for backward compatibility
	p = app.Editor
	
	// No need to sync windows as it's handled in main.go initialization
	
	for app.Run && sess.run {
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
			org.scroll()
			org.refreshScreen()
			if sess.divider > 10 {
				org.drawStatusBar()
			}
		}
		sess.returnCursor()
	}
	
	// Clean up when the main loop exits
	app.Cleanup()
}