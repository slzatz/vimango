package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/slzatz/vimango/rawmode"
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// App encapsulates the global application state
type App struct {
	// Core components
	Session   *Session   // Session handles terminal and screen management
	Screen    *Screen    // Screen handles screen management
	Organizer *Organizer // Organizer manages tasks and their display
	Editor    *Editor    // Editor manages text editing - there can be multiple editors
	Database  *Database  // Database handles database connections and queries

	// Research system
	ResearchManager *ResearchManager  // Manages background research tasks
	RenderManager   *RenderManager    // Manages async note rendering
	notifications   []string          // Queue of user notifications
	notificationMux sync.RWMutex      // Mutex for notifications
	notificationCh  chan struct{}     // Signals pending notifications
	keyEvents       chan terminal.Key // Key events from reader goroutine
	keyErrors       chan error        // Key read errors

	// Database connections and other config
	Config *dbConfig

	// Application state
	SyncInProcess      bool
	Run                bool
	kitty           bool   // true if running in kitty-graphics-compatible terminal (kitty, ghostty, etc.)
	kittyVersion    string // kitty version (only set for actual kitty terminal)
	kittyPlace      bool   // true if terminal supports Unicode placeholders
	kittyTextSizing bool   // true if terminal supports OSC 66 text sizing (kitty 0.40.0+ only)
	// kittyRelative bool // Reserved for future side-by-side image support (relative placements)
	showImages         bool   // true if inline images should be displayed
	showImageInfo      bool   // true if Google Drive folder/filename should be displayed above images
	imageScale         int    // image width in columns (default: 45)
	imageCacheMaxWidth int    // max pixel width for cached Google Drive images (default: 800)
	preferencesPath    string // path to preferences.json file
	origTermCfg        []byte // original terminal configuration
}

// CreateApp creates and initializes the application struct
func CreateApp() *App {
	db := &Database{}
	sess := &Session{}
	screen := &Screen{Session: sess}
	sess.Editors = make([]*Editor, 0)
	kitty := IsTermKitty()
	return &App{
		Session:  sess,
		Screen:   screen,
		Database: db,
		Organizer: &Organizer{Session: sess,
			Screen:   screen,
			Database: db,
		},
		notifications:  make([]string, 0),
		notificationCh: make(chan struct{}, 1),
		keyEvents:      make(chan terminal.Key, 16),
		keyErrors:      make(chan error, 1),
		Run:            true,
		kitty:          kitty, // default to false
	}
}

func (a *App) NewEditor() *Editor {
	editor := &Editor{
		cx:                 0, //actual cursor x position (takes into account any scroll/offset)
		cy:                 0, //actual cursor y position ""
		fc:                 0, //'file' x position as defined by reading sqlite text into rows vector
		fr:                 0, //'file' y position ""
		lineOffset:         0, //the number of lines of text at the top scrolled off the screen
		mode:               NORMAL,
		command:            "", // "normal mode" outside of editor commands - when editor is in normal mode
		command_line:       "",
		firstVisibleRow:    0,
		highlightSyntax:    true, // applies to golang, c++ etc. and markdown
		numberLines:        true,
		redraw:             false,
		left_margin_offset: LEFT_MARGIN_OFFSET, // 0 if not syntax highlighting b/o synt high =>line numbers
		modified:           false,
		tabCompletion: struct {
			list  []string
			index int
		}{list: nil, index: 0},
		Screen:   a.Screen,
		Session:  a.Session,
		Database: a.Database,
	}

	// Set up commands after editor creation so help can access registry
	editor.exCmds = a.setEditorExCmds(editor)
	editor.normalCmds = a.setEditorNormalCmds(editor)

	return editor
}

// FromFile returns a dbConfig struct parsed from a file.
// Sensitive credentials can be overridden via environment variables:
//   - VIMANGO_PG_PASSWORD: PostgreSQL password
//   - VIMANGO_CLAUDE_API_KEY: Claude API key
func (a *App) FromFile(path string) (*dbConfig, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg dbConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}

	// Override sensitive values from environment variables if set
	if pgPassword := os.Getenv("VIMANGO_PG_PASSWORD"); pgPassword != "" {
		cfg.Postgres.Password = pgPassword
	}
	if sslMode := os.Getenv("VIMANGO_PG_SSL_MODE"); sslMode != "" {
		cfg.Postgres.SSLMode = sslMode
	}
	if sslCACert := os.Getenv("VIMANGO_PG_SSL_CA_CERT"); sslCACert != "" {
		cfg.Postgres.SSLCACert = sslCACert
	}
	if claudeKey := os.Getenv("VIMANGO_CLAUDE_API_KEY"); claudeKey != "" {
		cfg.Claude.ApiKey = claudeKey
	}

	return &cfg, nil
}

// LoadPreferences loads user preferences from preferences.json
// Returns default preferences if file doesn't exist or is invalid
func (a *App) LoadPreferences(path string) Preferences {
	a.preferencesPath = path

	// Default preferences
	defaults := Preferences{
		ImageScale:         45,
		EdPct:              60,
		ImageCacheMaxWidth: 800,
	}

	// Try to read file
	b, err := ioutil.ReadFile(path)
	if err != nil {
		// File doesn't exist or can't be read - use defaults
		return defaults
	}

	// Try to parse JSON
	var prefs Preferences
	if err := json.Unmarshal(b, &prefs); err != nil {
		// Invalid JSON - use defaults
		return defaults
	}

	// Validate ranges
	if prefs.ImageScale < 10 || prefs.ImageScale > 100 {
		prefs.ImageScale = defaults.ImageScale
	}
	if prefs.EdPct < 1 || prefs.EdPct > 99 {
		prefs.EdPct = defaults.EdPct
	}
	// Validate ImageCacheMaxWidth (reasonable range: 400-2000, 0 means use default)
	if prefs.ImageCacheMaxWidth == 0 || prefs.ImageCacheMaxWidth < 400 || prefs.ImageCacheMaxWidth > 2000 {
		prefs.ImageCacheMaxWidth = defaults.ImageCacheMaxWidth
	}

	return prefs
}

// SavePreferences writes current preferences to preferences.json
// Uses atomic write pattern (write to temp, then rename)
func (a *App) SavePreferences() error {
	if a.preferencesPath == "" {
		return fmt.Errorf("preferences path not set")
	}

	prefs := Preferences{
		ImageScale:         a.imageScale,
		EdPct:              a.Screen.edPct,
		ImageCacheMaxWidth: a.imageCacheMaxWidth,
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(prefs, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal preferences: %w", err)
	}

	// Atomic write pattern
	tmpPath := a.preferencesPath + ".tmp"
	if err := ioutil.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, a.preferencesPath); err != nil {
		os.Remove(tmpPath) // Clean up temp file on error
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func (a *App) signalHandler() {
	err := a.Screen.GetWindowSize()
	if err != nil {
		//SafeExit(fmt.Errorf("couldn't get window size: %v", err))
		os.Exit(1)
	}
	a.moveDividerPct(a.Screen.edPct) // should change to Screen
}

func (a *App) moveDividerPct(pct int) {
	// note below only necessary if window resized or font size changed
	a.Screen.textLines = a.Screen.screenLines - 2 - TOP_MARGIN

	if pct == 100 {
		a.Screen.divider = 1
	} else {
		a.Screen.divider = a.Screen.screenCols - pct*a.Screen.screenCols/100
	}
	a.Screen.totaleditorcols = a.Screen.screenCols - a.Screen.divider - 2
	a.Screen.eraseScreenRedrawLines()

	if a.Screen.divider > 10 {
		a.Organizer.refreshScreen()
		a.Organizer.drawStatusBar()
	}

	if a.Session.editorMode {
		a.Screen.positionWindows()
		a.Screen.eraseRightScreen() //erases editor area + statusbar + msg
		a.Screen.drawRightScreen()
	} else if a.Organizer.view == TASK {
		a.Organizer.displayNote()
	}
	a.Organizer.ShowMessage(BL, "rows: %d  cols: %d  divider: %d", a.Screen.screenLines, a.Screen.screenCols, a.Screen.divider)
	a.returnCursor()
}

func (a *App) moveDividerAbs(num int) {
	if num >= a.Screen.screenCols {
		a.Screen.divider = 1
	} else if num < 20 {
		a.Screen.divider = a.Screen.screenCols - 20
	} else {
		a.Screen.divider = a.Screen.screenCols - num
	}
	a.Screen.edPct = 100 - 100*a.Screen.divider/a.Screen.screenCols
	a.Screen.totaleditorcols = a.Screen.screenCols - a.Screen.divider - 2
	a.Screen.eraseScreenRedrawLines()

	if a.Screen.divider > 10 {
		a.Organizer.refreshScreen()
		a.Organizer.drawStatusBar()
	}
	if a.Session.editorMode {
		a.Screen.positionWindows()
		a.Screen.eraseRightScreen() //erases editor area + statusbar + msg
		a.Screen.drawRightScreen()
	} else if a.Organizer.view == TASK {
		a.Organizer.displayNote()
	}

	// Save preferences after divider move
	if err := a.SavePreferences(); err != nil {
		// Silently ignore save errors (preferences not critical)
	}

	a.Organizer.ShowMessage(BL, "rows: %d  cols: %d  divider: %d edPct: %d", a.Screen.screenLines, a.Screen.screenCols, a.Screen.divider, a.Screen.edPct)
	a.returnCursor()
}

// ErrDatabaseNotFound is returned when SQLite database files don't exist
var ErrDatabaseNotFound = fmt.Errorf("database files not found")

// InitDatabases initializes database connections
func (a *App) InitDatabases(configPath string, sqliteConfig *SQLiteConfig) error {
	// Read config file
	config, err := a.FromFile(configPath)
	if err != nil {
		return err
	}

	// Check that database files exist before opening
	// (sql.Open for SQLite creates empty files, which we want to avoid)
	if _, err := os.Stat(config.Sqlite3.DB); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s not found - run './vimango --init' to create databases", ErrDatabaseNotFound, config.Sqlite3.DB)
	}
	if _, err := os.Stat(config.Sqlite3.FTS_DB); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s not found - run './vimango --init' to create databases", ErrDatabaseNotFound, config.Sqlite3.FTS_DB)
	}

	// Initialize main database
	a.Database.MainDB, err = sqliteConfig.OpenSQLiteDB(config.Sqlite3.DB)
	if err != nil {
		return err
	}
	// Enable foreign keys
	_, err = a.Database.MainDB.Exec("PRAGMA foreign_keys=ON;")
	if err != nil {
		return err
	}
	// Initialize FTS database
	a.Database.FtsDB, err = sqliteConfig.OpenSQLiteDB(config.Sqlite3.FTS_DB)
	if err != nil {
		return err
	}
	// Postgres is optional - only connect if host is configured
	if config.Postgres.Host != "" {
		// Default SSL mode to "disable" for backward compatibility
		sslMode := config.Postgres.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}

		// Build connection string
		connect := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			config.Postgres.Host,
			config.Postgres.Port,
			config.Postgres.User,
			config.Postgres.Password,
			config.Postgres.DB,
			sslMode,
		)

		// Add CA certificate path if provided (required for verify-ca and verify-full modes)
		if config.Postgres.SSLCACert != "" {
			connect += fmt.Sprintf(" sslrootcert=%s", config.Postgres.SSLCACert)
		}

		a.Database.PG, err = sql.Open("postgres", connect)
		if err != nil {
			return err
		}

		// Ping to verify connection
		err = a.Database.PG.Ping()
		if err != nil {
			return err
		}
	} else {
		// No Postgres configured - sync functionality will be disabled
		a.Database.PG = nil
	}

	a.Config = config
	return nil
}

// ValidateDatabaseSchema checks that the required tables exist in the databases.
// Returns an error if the schema is missing or incomplete.
func (a *App) ValidateDatabaseSchema() error {
	// Check main database tables
	requiredTables := []string{"task", "context", "folder", "keyword", "sync", "task_keyword"}
	for _, table := range requiredTables {
		var name string
		err := a.Database.MainDB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			return fmt.Errorf("required table '%s' not found in main database", table)
		}
	}

	// Check FTS database
	var ftsName string
	err := a.Database.FtsDB.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='fts'",
	).Scan(&ftsName)
	if err != nil {
		return fmt.Errorf("FTS table not found in FTS database")
	}

	return nil
}

// MigrateToUUID migrates existing databases to use UUID for containers.
// This is a one-time migration for databases created before UUID support.
func (a *App) MigrateToUUID() error {
	// Check if uuid column exists in context table
	var colName string
	err := a.Database.MainDB.QueryRow(
		"SELECT name FROM pragma_table_info('context') WHERE name='uuid'",
	).Scan(&colName)

	if err == nil {
		// uuid column already exists, check if migration is complete
		var nullCount int
		err = a.Database.MainDB.QueryRow(
			"SELECT COUNT(*) FROM context WHERE uuid IS NULL OR uuid = ''",
		).Scan(&nullCount)
		if err == nil && nullCount == 0 {
			// Migration already complete
			return nil
		}
	}

	// Need to run migration
	fmt.Println("Migrating database to UUID-based containers...")

	// Add uuid columns to container tables if they don't exist
	containerTables := []string{"context", "folder", "keyword"}
	for _, table := range containerTables {
		// Check if column exists
		var exists string
		err := a.Database.MainDB.QueryRow(
			fmt.Sprintf("SELECT name FROM pragma_table_info('%s') WHERE name='uuid'", table),
		).Scan(&exists)
		if err != nil {
			// Column doesn't exist, add it
			_, err = a.Database.MainDB.Exec(
				fmt.Sprintf("ALTER TABLE %s ADD COLUMN uuid TEXT", table),
			)
			if err != nil {
				return fmt.Errorf("failed to add uuid column to %s: %v", table, err)
			}
			fmt.Printf("  Added uuid column to %s\n", table)
		}

		// Generate UUIDs for existing rows that don't have one
		// Use tid to create predictable UUIDs for the default "none" entries
		rows, err := a.Database.MainDB.Query(
			fmt.Sprintf("SELECT id, tid, title FROM %s WHERE uuid IS NULL OR uuid = ''", table),
		)
		if err != nil {
			return fmt.Errorf("failed to query %s for migration: %v", table, err)
		}
		defer rows.Close()

		for rows.Next() {
			var id int
			var tid sql.NullInt64
			var title string
			if err := rows.Scan(&id, &tid, &title); err != nil {
				return fmt.Errorf("failed to scan %s row: %v", table, err)
			}

			var newUUID string
			// Use predefined UUIDs for the default containers
			if title == "none" && tid.Valid && tid.Int64 == 1 {
				if table == "context" {
					newUUID = DefaultContextUUID
				} else if table == "folder" {
					newUUID = DefaultFolderUUID
				} else {
					newUUID = generateUUID()
				}
			} else {
				newUUID = generateUUID()
			}

			_, err = a.Database.MainDB.Exec(
				fmt.Sprintf("UPDATE %s SET uuid = ? WHERE id = ?", table),
				newUUID, id,
			)
			if err != nil {
				return fmt.Errorf("failed to set uuid for %s id %d: %v", table, id, err)
			}
		}
		fmt.Printf("  Generated UUIDs for %s entries\n", table)
	}

	// Add uuid columns to task table if they don't exist
	taskUUIDCols := []struct {
		column       string
		defaultValue string
	}{
		{"context_uuid", DefaultContextUUID},
		{"folder_uuid", DefaultFolderUUID},
	}

	for _, col := range taskUUIDCols {
		var exists string
		err := a.Database.MainDB.QueryRow(
			fmt.Sprintf("SELECT name FROM pragma_table_info('task') WHERE name='%s'", col.column),
		).Scan(&exists)
		if err != nil {
			// Column doesn't exist, add it with default value
			_, err = a.Database.MainDB.Exec(
				fmt.Sprintf("ALTER TABLE task ADD COLUMN %s TEXT DEFAULT '%s'", col.column, col.defaultValue),
			)
			if err != nil {
				return fmt.Errorf("failed to add %s column to task: %v", col.column, err)
			}
			fmt.Printf("  Added %s column to task\n", col.column)
		}
	}

	// Migrate task references from tid to uuid
	// Update context_uuid based on context_tid
	_, err = a.Database.MainDB.Exec(`
		UPDATE task SET context_uuid = (
			SELECT uuid FROM context WHERE context.tid = task.context_tid
		) WHERE context_uuid IS NULL OR context_uuid = '' OR context_uuid = ?
	`, DefaultContextUUID)
	if err != nil {
		return fmt.Errorf("failed to migrate task context_uuid: %v", err)
	}
	fmt.Println("  Migrated task context references")

	// Update folder_uuid based on folder_tid
	_, err = a.Database.MainDB.Exec(`
		UPDATE task SET folder_uuid = (
			SELECT uuid FROM folder WHERE folder.tid = task.folder_tid
		) WHERE folder_uuid IS NULL OR folder_uuid = '' OR folder_uuid = ?
	`, DefaultFolderUUID)
	if err != nil {
		return fmt.Errorf("failed to migrate task folder_uuid: %v", err)
	}
	fmt.Println("  Migrated task folder references")

	// Add keyword_uuid column to task_keyword if it doesn't exist
	var kwUUIDExists string
	err = a.Database.MainDB.QueryRow(
		"SELECT name FROM pragma_table_info('task_keyword') WHERE name='keyword_uuid'",
	).Scan(&kwUUIDExists)
	if err != nil {
		_, err = a.Database.MainDB.Exec(
			"ALTER TABLE task_keyword ADD COLUMN keyword_uuid TEXT",
		)
		if err != nil {
			return fmt.Errorf("failed to add keyword_uuid column to task_keyword: %v", err)
		}
		fmt.Println("  Added keyword_uuid column to task_keyword")
	}

	// Migrate task_keyword references from keyword_tid to keyword_uuid
	_, err = a.Database.MainDB.Exec(`
		UPDATE task_keyword SET keyword_uuid = (
			SELECT uuid FROM keyword WHERE keyword.tid = task_keyword.keyword_tid
		) WHERE keyword_uuid IS NULL OR keyword_uuid = ''
	`)
	if err != nil {
		return fmt.Errorf("failed to migrate task_keyword keyword_uuid: %v", err)
	}
	fmt.Println("  Migrated task_keyword references")

	fmt.Println("Database migration complete!")
	return nil
}

// InitApp initializes the application components
func (a *App) InitApp() {

	markdown_style, _ := selectMDStyle(a.Config.Chroma.Style)
	a.Session.markdown_style = markdown_style
	a.Session.style = [8]string{"dracula", "fruity", "gruvbox", "monokai", "native", "paraiso-dark", "rrt", "solarized-dark256"}
	a.Session.styleIndex = 2
	a.Session.imagePreview = false
	a.Session.imgSizeY = 800

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
	//a.Organizer.last_mode = NORMAL
	a.Organizer.view = TASK
	a.Organizer.tabCompletion.list = nil
	a.Organizer.tabCompletion.index = 0
	a.Organizer.normalCmds = a.setOrganizerNormalCmds(a.Organizer)
	a.Organizer.exCmds = a.setOrganizerExCmds(a.Organizer)
	a.Organizer.filterList = a.setFilterList()

	if a.Config.Options.Type == "folder" {
		a.Organizer.taskview = BY_FOLDER
	} else {
		a.Organizer.taskview = BY_CONTEXT
	}

	a.Organizer.filter = a.Config.Options.Title
	a.Organizer.marked_entries = make(map[int]struct{})
	a.Organizer.vbuf = vim.NewBuffer(0)
	vim.SetCurrentBuffer(a.Organizer.vbuf)
}

func (a *App) LoadInitialData() {
	a.Screen.textLines = a.Screen.screenLines - 2 - TOP_MARGIN

	// Only set default edPct if not already set (e.g., from preferences)
	if a.Screen.edPct == 0 {
		a.Screen.edPct = 60
	}

	// Set divider based on percentage
	if a.Screen.edPct == 100 {
		a.Screen.divider = 1
	} else {
		a.Screen.divider = a.Screen.screenCols - a.Screen.edPct*a.Screen.screenCols/100
	}

	a.Screen.totaleditorcols = a.Screen.screenCols - a.Screen.divider - 1
	a.Screen.eraseScreenRedrawLines()

	a.Organizer.FilterEntries(MAX)
	if len(a.Organizer.rows) == 0 {
		a.Organizer.insertRow(0, "", true, false, false, BASE_DATE)
		a.Organizer.rows[0].dirty = false
		a.Organizer.ShowMessage(BL, "No results were returned")
	}

	a.Organizer.readRowsIntoBuffer()
	a.Organizer.bufferTick = a.Organizer.vbuf.GetLastChangedTick()
	a.Organizer.displayNote()
	a.Organizer.refreshScreen()
	a.Organizer.drawStatusBar()

	a.Organizer.ShowMessage(BL, "rows: %d  columns: %d", a.Screen.screenLines, a.Screen.screenCols)
	a.returnCursor()
}

// organizer and editor scroll methods set cx and cy
// returnCursor positions the terminal cursor in the right place
func (a *App) returnCursor() {
	var ab strings.Builder
	if a.Session.editorMode {
		ae := a.Session.activeEditor
		switch ae.mode {
		case PREVIEW, SPELLING, VIEW_LOG:
			// we don't need to position cursor and don't want cursor visible
			fmt.Print(ab.String())
			return
		case EX_COMMAND, SEARCH:
			fmt.Fprintf(&ab, "\x1b[%d;%dH", a.Screen.textLines+TOP_MARGIN+2, len(ae.command_line)+a.Screen.divider+2)
		default:
			fmt.Fprintf(&ab, "\x1b[%d;%dH", ae.cy+ae.top_margin, ae.cx+ae.left_margin+ae.left_margin_offset+1)
		}
	} else {
		switch a.Organizer.mode {
		case COMMAND_LINE:
			fmt.Fprintf(&ab, "\x1b[%d;%dH", a.Screen.textLines+2+TOP_MARGIN, len(a.Organizer.command_line)+LEFT_MARGIN+1)
		default:
			if a.Organizer.taskview == BY_FIND {
				fmt.Fprintf(&ab, "\x1b[%d;%dH\x1b[1;34m>", a.Organizer.cy+TOP_MARGIN+1, LEFT_MARGIN) //blue
			} else {
				fmt.Fprintf(&ab, "\x1b[%d;%dH\x1b[1;31m>", a.Organizer.cy+TOP_MARGIN+1, LEFT_MARGIN)
			}
			// below restores the cursor position based on org.cx and org.cy + margin
			fmt.Fprintf(&ab, "\x1b[%d;%dH", a.Organizer.cy+TOP_MARGIN+1, a.Organizer.cx+LEFT_MARGIN+1)
		}
	}
	ab.WriteString("\x1b[0m")   //return to default fg/bg
	ab.WriteString("\x1b[?25h") //shows the cursor
	fmt.Print(ab.String())
}

func (a *App) Cleanup() {
	// Stop async render manager
	if a.RenderManager != nil {
		a.RenderManager.Stop()
	}

	if a.Database.MainDB != nil {
		a.Database.MainDB.Close()
	}
	if a.Database.FtsDB != nil {
		a.Database.FtsDB.Close()
	}
	if a.Database.PG != nil {
		a.Database.PG.Close()
	}
	a.quitApp()
}

func (a *App) quitApp() {
	fmt.Print("\x1b[2J\x1b[H") //clears the screen and sends cursor home

	if err := rawmode.Restore(a.origTermCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: disabling raw mode: %s\r\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func (a *App) startKeyReader() {
	go func() {
		for {
			key, err := terminal.ReadKey()
			if err != nil {
				if err == terminal.ErrNoInput {
					continue
				}
				select {
				case a.keyErrors <- err:
				default:
				}
				continue
			}
			a.keyEvents <- key
		}
	}()
}

func (a *App) MainLoop() {
	org := a.Organizer
	a.startKeyReader()

	if a.HasNotifications() {
		a.processNotifications(org)
		a.returnCursor()
	}

	for a.Run {
		select {
		case key := <-a.keyEvents:
			var k int
			if key.Regular != 0 {
				k = int(key.Regular)
			} else {
				k = key.Special
			}
			if a.Session.editorMode {
				ae := a.Session.activeEditor
				redraw := ae.editorProcessKey(k)

				if !a.Session.editorMode {
					continue
				}
				offset_changed := ae.scroll()
				if redraw || offset_changed {
					ae.drawText()
					ae.drawStatusBar()
				}
			} else {
				redraw := org.organizerProcessKey(k)
				//org.ShowMessage(BR, "redraw: %d", redraw) ///DEBUG
				if a.Session.editorMode {
					a.returnCursor()
					continue
				}
				if org.scroll() { //|| org.taskview == BY_FIND {
					org.refreshScreen()
					a.returnCursor()
					continue
				}
				switch redraw {
				case RedrawNone:
					// do nothing
				case RedrawFull:
					org.refreshScreen()
				case RedrawPartial:
					if org.taskview == BY_FIND {
						org.refreshScreen() // not efficient since just need it to redraw previous row
					}
					org.drawActive()
				}

				if a.Screen.divider > 10 {
					org.drawStatusBar()
				}
			}
		case err := <-a.keyErrors:
			if err != nil {
				org.ShowMessage(BL, "Readkey problem %v", err)
			}
		case <-a.notificationCh:
			a.processNotificationsWithRedraw(org)
		}
		a.returnCursor()
	}
	a.Cleanup()
}

// addNotification adds a notification message to the queue
func (a *App) addNotification(message string) {
	a.notificationMux.Lock()
	a.notifications = append(a.notifications, message)
	a.notificationMux.Unlock()

	select {
	case a.notificationCh <- struct{}{}:
	default:
	}
}

// GetNotification retrieves and removes the next notification from the queue
func (a *App) GetNotification() string {
	a.notificationMux.Lock()
	defer a.notificationMux.Unlock()

	if len(a.notifications) == 0 {
		return ""
	}

	message := a.notifications[0]
	a.notifications = a.notifications[1:]
	return message
}

func (a *App) processNotifications(org *Organizer) {
	for {
		notification := a.GetNotification()
		if notification == "" {
			return
		}
		org.drawNotice(notification)
		org.altRowoff = 0
		org.mode = NAVIGATE_NOTICE
	}
}

// processNotificationsWithRedraw handles notifications including special redraw requests
func (a *App) processNotificationsWithRedraw(org *Organizer) {
	for {
		notification := a.GetNotification()
		if notification == "" {
			return
		}

		// Handle special preview redraw notification from async renderer
		if notification == "_REDRAW_PREVIEW_" {
			if !a.Session.editorMode && org.view == TASK {
				org.drawRenderedNote()
			}
			continue
		}

		// Handle regular notifications (research results, etc.)
		org.drawNotice(notification)
		org.altRowoff = 0
		org.mode = NAVIGATE_NOTICE
	}
}

// HasNotifications returns true if there are pending notifications
func (a *App) HasNotifications() bool {
	a.notificationMux.RLock()
	defer a.notificationMux.RUnlock()
	return len(a.notifications) > 0
}

// InitResearchManager initializes the research manager with API key
func (a *App) InitResearchManager(apiKey string) {
	if apiKey != "" {
		a.ResearchManager = NewResearchManager(a, apiKey)
	}
}

// InitRenderManager initializes the async render manager
func (a *App) InitRenderManager() {
	a.RenderManager = NewRenderManager(a)
}
