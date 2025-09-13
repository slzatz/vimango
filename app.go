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
	ResearchManager *ResearchManager // Manages background research tasks
	notifications   []string         // Queue of user notifications
	notificationMux sync.RWMutex     // Mutex for notifications

	// Database connections and other config
	Config *dbConfig

	// Application state
	//LastSync      time.Time // calculated when syncing but not saved
	SyncInProcess bool
	Run           bool
	kitty         bool   // true if running in kitty terminal
	origTermCfg   []byte // original terminal configuration
}

// CreateApp creates and initializes the application struct
func CreateApp() *App {
	db := &Database{}
	sess := &Session{}
	screen := &Screen{Session: sess}
	sess.Windows = make([]Window, 0)
	kitty := IsTermKitty()
	return &App{
		Session:  sess,
		Screen:   screen,
		Database: db,
		Organizer: &Organizer{Session: sess,
			Screen:   screen,
			Database: db,
		},
		notifications: make([]string, 0),
		Run:           true,
		kitty:         kitty, // default to false
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
		output:             nil,
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
		a.Organizer.drawPreview()
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
		a.Organizer.drawPreview()
	}
	a.Organizer.ShowMessage(BL, "rows: %d  cols: %d  divider: %d edPct: %d", a.Screen.screenLines, a.Screen.screenCols, a.Screen.divider, a.Screen.edPct)
	a.returnCursor()
}

// InitDatabases initializes database connections
func (a *App) InitDatabases(configPath string, sqliteConfig *SQLiteConfig) error {
	// Read config file
	config, err := a.FromFile(configPath)
	if err != nil {
		return err
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
	connect := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Postgres.Host,
		config.Postgres.Port,
		config.Postgres.User,
		config.Postgres.Password,
		config.Postgres.DB,
	)
	a.Database.PG, err = sql.Open("postgres", connect)
	if err != nil {
		return err
	}

	// Ping to connection
	err = a.Database.PG.Ping()
	if err != nil {
		return err
	}
	a.Config = config
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
	a.Organizer.last_mode = NORMAL
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
	a.Screen.edPct = 60

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
	a.Organizer.drawPreview()
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
	//if lsp.name != "" {
	//	shutdownLsp()
	//}
	fmt.Print("\x1b[2J\x1b[H") //clears the screen and sends cursor home
	//lsp_shutdown("all");

	if err := rawmode.Restore(a.origTermCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: disabling raw mode: %s\r\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func (a *App) MainLoop() {
	org := a.Organizer
	for a.Run {
		// Check for research notifications
		if a.HasNotifications() {
			notification := a.GetNextNotification()
			if notification != "" {
				org.ShowMessage(BL, notification)
			}
		}
		
		key, err := terminal.ReadKey()
		if err != nil {
			org.ShowMessage(BL, "Readkey problem %w", err)
		}
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
			if redraw {
				ae := a.Session.activeEditor ///// doesn't do anything
				ae.scroll()
				ae.drawText()
				ae.drawStatusBar()
			}
		} else {
			org.organizerProcessKey(k)
			if a.Session.editorMode {
				a.returnCursor()
				continue
			}
			org.scroll()
			org.refreshScreen()
			if a.Screen.divider > 10 {
				org.drawStatusBar()
			}
		}
		a.returnCursor()
	}
	a.Cleanup()
}

// addNotification adds a notification message to the queue
func (a *App) addNotification(message string) {
	a.notificationMux.Lock()
	defer a.notificationMux.Unlock()
	a.notifications = append(a.notifications, message)
}

// GetNextNotification retrieves and removes the next notification from the queue
func (a *App) GetNextNotification() string {
	a.notificationMux.Lock()
	defer a.notificationMux.Unlock()
	
	if len(a.notifications) == 0 {
		return ""
	}
	
	message := a.notifications[0]
	a.notifications = a.notifications[1:]
	return message
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
