package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2"
	_ "github.com/lib/pq" // the "postgres" driver InitDatabases opens the server with

	"github.com/slzatz/vimango/internal/atomicfile"
	"github.com/slzatz/vimango/internal/editor"
	"github.com/slzatz/vimango/internal/localdb"
	"github.com/slzatz/vimango/internal/store"
	"github.com/slzatz/vimango/rawmode"
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// App encapsulates the global application state
type App struct {
	// Core components
	Session   *Session     // Session handles terminal and screen management
	Screen    *Screen      // Screen handles screen management
	Organizer *Organizer   // the list of notes, in the TUI; nil in --editor mode, which has none
	Store     *store.Store // vimango.db: the notes and their containers
	PG        *sql.DB      // the PostgreSQL server, for sync; nil when none is configured

	RenderManager *RenderManager // Renders the organizer's preview; loads images in the background

	// Events for MainLoop, each posted by another goroutine
	keyEvents chan terminal.Key // Key events from reader goroutine
	keyErrors chan error        // Key read errors
	resizeCh  chan struct{}     // Terminal resized (SIGWINCH)

	// Database connections and other config
	Config        *dbConfig
	markdownStyle *chroma.Style // the colours the editor draws markdown in (config.json's chroma.style)

	// Application state
	SyncInProcess      bool
	Run                bool
	kitty              bool   // true if running in kitty-graphics-compatible terminal (kitty, ghostty, etc.)
	kittyVersion       string // kitty version (only set for actual kitty terminal)
	kittyPlace         bool   // true if terminal supports Unicode placeholders
	kittyTextSizing    bool   // true if terminal supports OSC 66 text sizing (kitty 0.40.0+ only)
	showImages         bool   // true if inline images should be displayed
	showImageInfo      bool   // true if Google Drive folder/filename should be displayed above images
	imageScale         int    // image width in columns (default: 45)
	imageCacheMaxWidth int    // max pixel width for cached Google Drive images (default: 800)
	preferencesPath    string // path to preferences.json file
	origTermCfg        []byte // original terminal configuration
}

// CreateApp creates and initializes the application struct
func CreateApp() *App {
	sess := &Session{}
	screen := &Screen{Session: sess}
	kitty := IsTermKitty()
	return &App{
		Session:   sess,
		Screen:    screen,
		keyEvents: make(chan terminal.Key, 16),
		keyErrors: make(chan error, 1),
		resizeCh:  make(chan struct{}, 1),
		Run:       true,
		kitty:     kitty, // default to false
	}
}

// editorHost is what this app gives an editor it opens: the database, the
// screen's notice box, the colours for markdown, and what closing the note
// and typing :sync come to here.
func (a *App) editorHost() editor.Host {
	return editor.Host{
		Store:         a.Store,
		Notices:       a.Screen,
		MarkdownStyle: a.markdownStyle,
		OnClose:       a.noteClosed,
		SyncAnswer:    a.syncAnswer(),
	}
}

// syncAnswer is what an editor says to :sync, which is never its own to run
// (Editor.noSync). In --editor mode a host app owns sync: it runs
// vimango-sync itself and refreshes its list afterwards, and a sync here
// would run behind its back. In the TUI it is the organizer's, once the note
// is closed.
func (a *App) syncAnswer() string {
	if a.Session.editorOnly {
		return "sync is handled by the app — use the Sync menu"
	}
	return "sync is the organizer's: close the note (:x or :q), then :sync"
}

// loadConfig reads config.json.
// Sensitive credentials can be overridden via environment variables:
//   - VIMANGO_PG_PASSWORD: PostgreSQL password
func loadConfig(path string) (*dbConfig, error) {
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

	return &cfg, nil
}

// LoadPreferences loads user preferences from preferences.json, and
// remembers the path for SavePreferences.
func (a *App) LoadPreferences(path string) Preferences {
	a.preferencesPath = path
	return readPreferences(path)
}

// readPreferences reads preferences.json. It returns the defaults if the
// file doesn't exist or is invalid, and for any value out of range.
func readPreferences(path string) Preferences {
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

	if err := atomicfile.Write(a.preferencesPath, data, 0644); err != nil {
		return fmt.Errorf("write preferences: %w", err)
	}
	return nil
}

// handleResize refits the screen to the terminal's new size and redraws.
// Called from MainLoop when resizeCh fires -- never from the signal goroutine
// itself: the redraw reaches libvim (brace matching, spell errors) and the
// same Screen, Editor and Organizer state a key in progress is using.
func (a *App) handleResize() {
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

	org := a.Organizer // nil in --editor mode: there is only the editor to redraw
	if org != nil && a.Screen.divider > 10 {
		org.refreshScreen()
		org.drawStatusBar()
	}

	if a.Session.editor != nil {
		a.Screen.positionEditor()
		a.Screen.eraseRightScreen() //erases editor area + statusbar + msg
		a.Screen.drawRightScreen()
	} else if org != nil && org.view == TASK {
		org.displayNote()
	}
	// (not in the hybrid app: every host-window resize lands here via
	// SIGWINCH and would flash this over the editor's message row)
	if org != nil {
		org.ShowMessage(BL, "rows: %d  cols: %d  divider: %d", a.Screen.screenLines, a.Screen.screenCols, a.Screen.divider)
	}
	a.returnCursor()
}

// ErrDatabaseNotFound is returned when SQLite database files don't exist
var ErrDatabaseNotFound = fmt.Errorf("database files not found")

// openStore opens the vimango.db that config.json names, as every mode of
// vimango does at startup: the TUI and --editor through InitDatabases, and
// --render-html by itself.
func openStore(config *dbConfig) (*store.Store, error) {
	// Check that the database file exists before opening it
	// (sql.Open for SQLite creates an empty file, which we want to avoid)
	if _, err := os.Stat(config.Sqlite3.DB); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s not found - run './vimango --init' to create databases", ErrDatabaseNotFound, config.Sqlite3.DB)
	}
	db, err := localdb.Open(config.Sqlite3.DB)
	if err != nil {
		return nil, err
	}
	// Bring the database up to date with how this build stores it (WAL, the
	// search index). Quiet, and a failure is not fatal: it means another
	// process was mid-write, and the next start does it. Until then :find
	// may report that task_fts is missing. `vimango --migrate` reports.
	_, _ = localdb.Migrate(db)
	return store.New(db), nil
}

// InitDatabases initializes database connections
func (a *App) InitDatabases(configPath string) error {
	config, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	a.Store, err = openStore(config)
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

		// sql.Open only validates the connection string; no network I/O
		// happens until the first query, so booting offline works and sync
		// recovers on its own when connectivity returns. Reachability is
		// checked in Synchronize, where a failure can be reported usefully.
		a.PG, err = sql.Open("postgres", connect)
		if err != nil {
			return err
		}
	}
	// (with no Postgres configured a.PG stays nil, and sync says so)

	a.Config = config
	return nil
}

// InitApp initializes the application components
// InitApp finishes setting the application up once the configuration and the
// terminal are known. The organizer is made here, and only for the TUI: in
// --editor mode a host app has its own list of notes, vimango is the editor
// alone, and a.Organizer stays nil.
func (a *App) InitApp() {
	a.markdownStyle, _ = editor.LoadMarkdownStyle(a.Config.Chroma.Style)
	a.Screen.glamourStyle = a.Config.Glamour.Style
	a.Session.imgSizeY = 800

	if a.Session.editorOnly {
		return
	}
	a.Organizer = a.newOrganizer()
	// the organizer's preview of the current note, and its images
	a.RenderManager = NewRenderManager(a)
}

// newOrganizer is the TUI's list of notes, on the context or folder that
// config.json names. Its rows are loaded by LoadInitialData.
func (a *App) newOrganizer() *Organizer {
	o := &Organizer{Session: a.Session, Screen: a.Screen, Store: a.Store}
	o.sort = "modified"
	o.highlight[0], o.highlight[1] = -1, -1
	o.mode = NORMAL
	o.view = TASK
	o.normalCmds = a.setOrganizerNormalCmds(o)
	o.exCmds = a.setOrganizerExCmds(o)
	o.filterList = o.filterNames()

	if a.Config.Options.Type == "folder" {
		o.taskview = BY_FOLDER
	} else {
		o.taskview = BY_CONTEXT
	}
	o.filter = a.Config.Options.Title
	o.marked_entries = make(map[int]struct{})
	o.vbuf = vim.NewBuffer(0)
	vim.SetCurrentBuffer(o.vbuf)
	return o
}

// LoadInitialData lays the screen out and, in the TUI, loads and draws the
// organizer's first list.
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

	org := a.Organizer
	if org == nil {
		return // --editor: the note is opened next, and is all there is to draw
	}
	if org.setRows(org.entries()) {
		org.ShowMessage(BL, "No results were returned")
	}
	org.displayNote()
	org.refreshScreen()
	org.drawStatusBar()

	org.ShowMessage(BL, "rows: %d  columns: %d", a.Screen.screenLines, a.Screen.screenCols)
	a.returnCursor()
}

// openEditor opens a note in the editor and shows it. Making the editor on
// the note is the editor's own (editor.Open); what is the app's is that it is
// now the open note, where keys go, and where on the screen it is drawn.
// title is what the editor shows as the note's; "" is the note's title in
// the database. An error means the note could not be read, and no editor was
// opened.
//
// The organizer calls this for the row it is on; --editor calls it for the
// note the host asked for, with no organizer there.
func (a *App) openEditor(id int, title string) error {
	ae, err := editor.Open(a.editorHost(), id, title)
	if err != nil {
		return err
	}
	a.Session.editor = ae
	a.Screen.positionEditor()
	a.Screen.eraseRightScreen() //erases editor area + statusbar + msg
	a.Screen.drawRightScreen()
	return nil
}

// noteClosed is what follows the editor closing its note. In the TUI that is
// the organizer again, on the row it was on. In --editor mode there is
// nothing to go back to: the process ends, which is how the host learns the
// note was closed.
func (a *App) noteClosed() {
	if a.Organizer == nil {
		// (the buffer is left as it is; the main loop finishes this
		// iteration before a.Run is checked)
		a.Run = false
		return
	}
	vim.ExecuteCommand("bw") // wipe out the buffer
	vim.SetScreenLayout(nil)
	a.Session.editor = nil
	vim.SetCurrentBuffer(a.Organizer.vbuf)
	a.Screen.eraseRightScreen()
	a.Organizer.displayNote()
	a.returnCursor() //because main while loop if started in editor_mode -- need this 09302020
}

// mostRecentNote is the note --editor opens when it is not told which: the
// one modified last. ok is false when there are no notes.
func (a *App) mostRecentNote() (id int, ok bool) {
	entries, err := a.Store.Entries(store.Recent, "", store.Listing{Sort: "modified", Limit: 1})
	if err != nil || len(entries) == 0 {
		return 0, false
	}
	return entries[0].ID, true
}

// organizer and editor scroll methods set cx and cy
// returnCursor positions the terminal cursor in the right place
func (a *App) returnCursor() {
	var ab strings.Builder
	if ae := a.Session.editor; ae != nil {
		row, col, show := ae.Cursor()
		if !show {
			return
		}
		fmt.Fprintf(&ab, "\x1b[%d;%dH", row, col)
	} else if a.Organizer != nil {
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
	// Persist registers for the next run (see registers.go) while vim
	// state is still alive.
	editor.SaveRegisters()

	// Stop async render manager
	if a.RenderManager != nil {
		a.RenderManager.Stop()
	}

	if a.Store != nil {
		a.Store.Close()
	}
	if a.PG != nil {
		a.PG.Close()
	}

	// The kitty image path decodes HEIC, and that leaves a plugin
	// subprocess running which outlives this one unless it is killed.
	// Must come before quitApp, which calls os.Exit and so runs nothing
	// after it.
	ShutdownHEICDecoder()

	a.quitApp()
}

func (a *App) quitApp() {
	fmt.Print("\x1b[2J\x1b[H") //clears the screen and sends cursor home
	fmt.Print(terminal.BracketedPasteOff)

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

// MainLoop is the one goroutine that draws, changes editor and organizer
// state, and calls libvim (which is not thread-safe). Everything else -- the
// key reader, the resize signal, image loading -- runs elsewhere and reaches
// the UI only as an event on one of the channels selected here.
func (a *App) MainLoop() {
	a.startKeyReader()
	a.handleEvents()
	a.Cleanup()
}

// handleEvents is MainLoop between starting the key reader and cleaning up:
// it returns when a.Run goes false. Apart so that a test can feed it keys.
func (a *App) handleEvents() {
	org := a.Organizer

	// a nil channel is never ready, so no RenderManager means no such events
	var renderResults <-chan RenderResult
	if a.RenderManager != nil {
		renderResults = a.RenderManager.resultCh
	}

	// A key that begins a longer mapping ("j" when "jk" is mapped) waits in
	// vim for the next one. vim gives up waiting after 'timeoutlen'; libvim
	// has no clock, so the wait is kept here. nil -- never ready -- unless
	// the last key left vim holding something.
	var mappingTimeout <-chan time.Time

	for a.Run {
		select {
		case key := <-a.keyEvents:
			mappingTimeout = nil // the key settles it, one way or the other
			// text pasted from the terminal: not a key (paste.go)
			pasted := key.Special == terminal.KeyPaste
			var k int
			if pasted {
				k = editor.NoKey
			} else if key.Regular != 0 {
				k = int(key.Regular)
			} else {
				k = key.Special
			}
			// A key goes to the open note's editor, or else to the organizer.
			// In --editor there is no organizer, and nothing for a key to
			// go to once the note is closed.
			ae := a.Session.editor
			if ae == nil && org == nil {
				continue
			}
			if ae != nil {
				var redraw bool
				if pasted {
					redraw = ae.PasteText(key.Paste)
				} else {
					redraw = ae.ProcessKey(k)
				}

				// the key closed the note (the TUI): noteClosed has drawn
				// the organizer and placed the cursor
				if a.Session.editor == nil {
					continue
				}
				ae.DrawAfterKey(redraw, k)
				if wait, waiting := editor.MappingWait(); waiting {
					mappingTimeout = time.After(wait)
				}
			} else {
				var redraw RedrawScope
				if pasted {
					redraw = org.pasteText(key.Paste)
				} else {
					redraw = org.organizerProcessKey(k)
				}
				//org.ShowMessage(BR, "redraw: %d", redraw) ///DEBUG
				// the key opened a note: openEditor has drawn it
				if a.Session.editor != nil {
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
			switch {
			case err == nil:
			case org != nil:
				org.ShowMessage(BL, "Readkey problem %v", err)
			case a.Session.editor != nil:
				a.Session.editor.ShowAside("Readkey problem %v", err)
			}
		case <-mappingTimeout:
			mappingTimeout = nil
			if ae := a.Session.editor; ae != nil {
				ae.DrawAfterKey(ae.MappingTimeout(), editor.NoKey)
			}
		case <-a.resizeCh:
			a.handleResize()
		case res := <-renderResults:
			// a note's images finished loading in the background
			a.RenderManager.applyResult(res)
		}
		a.returnCursor()
	}
}
