package main

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
  "strings"
	"time"
  "fmt"
  "os"
	
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
	"github.com/slzatz/vimango/rawmode"
)

// App encapsulates the global application state
type App struct {
	// Core components
	Session   *Session // Session handles terminal and screen management
  Screen    *Screen // Screen handles screen management
	Organizer *Organizer // Organizer manages tasks and their display
	Editor    *Editor // Editor manages text editing
	Windows   []Window // Windows is slice of Window interfaces and manages multiple windows in the application
  Database  *Database // Database handles database connections and queries
	
	// Database connections and other config
	Config  *dbConfig
	
	// Application state
	LastSync      time.Time
	SyncInProcess bool
	Run           bool
  origTermCfg   []byte // original terminal configuration
}

// CreateApp creates and initializes the application struct
func CreateApp() *App {
  db := &Database{}
  sess := &Session{}
  screen := &Screen{}
	return &App{
		Session:   sess,
    Screen:    screen,
    Database: db,
    //Editor:    &Editor{}, // Not needed now but may want App.Editor to be a pointer to current Editor
    // maybe new Editor should have the session field and session would know the active editor window
    Organizer: &Organizer{Session: sess, Screen: screen, Database: db},
		Windows:   make([]Window, 0),
		Run:       true,
	}
}

func (a *App) NewEditor() *Editor {
	return &Editor{
		cx:                 0, //actual cursor x position (takes into account any scroll/offset)
		cy:                 0, //actual cursor y position ""
		fc:                 0, //'file' x position as defined by reading sqlite text into rows vector
		fr:                 0, //'file' y position ""
		lineOffset:         0, //the number of lines of text at the top scrolled off the screen
		mode:               NORMAL,
		command:            "",
		command_line:       "",
		firstVisibleRow:    0,
		highlightSyntax:    true, // applies to golang, c++ etc. and markdown
		numberLines:        true,
		redraw:             false,
		output:             nil,
		left_margin_offset: LEFT_MARGIN_OFFSET, // 0 if not syntax highlighting b/o synt high =>line numbers
		modified:           false,
    Screen:             a.Screen,
    Session:            a.Session,
    Database:           a.Database,
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

func (a *App) signalHandler() {
	err := a.Screen.GetWindowSize() // Should change to Screen
	if err != nil {
		//SafeExit(fmt.Errorf("couldn't get window size: %v", err))
		os.Exit(1)
	}
	a.moveDividerPct(a.Screen.edPct) // should change to Screen
}

// Most of the Sessions below (but not all) should become Screen
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
	
	markdown_style, _ := selectMDStyle("gruvbox.xml")
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
	a.Organizer.bufferTick = vim.BufferGetLastChangedTick(a.Organizer.vbuf)
	a.Organizer.drawPreview() 
	a.Organizer.refreshScreen()
	a.Organizer.drawStatusBar()

	a.Organizer.ShowMessage(BL, "rows: %d  columns: %d", a.Screen.screenLines, a.Screen.screenCols)
	a.returnCursor()
}

func (a *App) returnCursor() {
	var ab strings.Builder
	if a.Session.editorMode {
		switch p.mode { //FIXME
		case PREVIEW, SPELLING, VIEW_LOG:
			// we don't need to position cursor and don't want cursor visible
			fmt.Print(ab.String())
			return
		case EX_COMMAND, SEARCH:
			fmt.Fprintf(&ab, "\x1b[%d;%dH", a.Screen.textLines+TOP_MARGIN+2, len(p.command_line)+a.Screen.divider+2)
		default:
			fmt.Fprintf(&ab, "\x1b[%d;%dH", p.cy+p.top_margin, p.cx+p.left_margin+p.left_margin_offset+1)
		}
	} else {
		switch a.Organizer.mode {
		case FIND:
			fmt.Fprintf(&ab, "\x1b[%d;%dH\x1b[1;34m>", a.Organizer.cy+TOP_MARGIN+1, LEFT_MARGIN) //blue
			fmt.Fprintf(&ab, "\x1b[%d;%dH", a.Organizer.cy+TOP_MARGIN+1, a.Organizer.cx+LEFT_MARGIN+1)
		case COMMAND_LINE:
			fmt.Fprintf(&ab, "\x1b[%d;%dH", a.Screen.textLines+2+TOP_MARGIN, len(a.Organizer.command_line)+LEFT_MARGIN+1)

		default:
			fmt.Fprintf(&ab, "\x1b[%d;%dH\x1b[1;31m>", a.Organizer.cy+TOP_MARGIN+1, LEFT_MARGIN)
			// below restores the cursor position based on org.cx and org.cy + margin
			fmt.Fprintf(&ab, "\x1b[%d;%dH", a.Organizer.cy+TOP_MARGIN+1, a.Organizer.cx+LEFT_MARGIN+1)
		}
	}

	ab.WriteString("\x1b[0m")   //return to default fg/bg
	ab.WriteString("\x1b[?25h") //shows the cursor
	fmt.Print(ab.String())
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
	//if a.Session != nil {
	//	a.Session.quitApp()
	//}
    a.quitApp()
}

func (a *App) quitApp() {
	//if lsp.name != "" {
	//	shutdownLsp()
	//}
	fmt.Print("\x1b[2J\x1b[H") //clears the screen and sends cursor home
	//sqlite3_close(S.db); //something should probably be done here
	//PQfinish(conn);
	//lsp_shutdown("all");

	if err := rawmode.Restore(a.origTermCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: disabling raw mode: %s\r\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
// MainLoop is the main application loop
func (a *App) MainLoop() {
	
	// Set global reference for backward compatibility
	p = a.Editor
	
	// No need to sync windows as it's handled in main.go initialization
	
	//for a.Run && sess.run {
	for a.Run {
		key, err := terminal.ReadKey()
		if err != nil {
			a.Organizer.ShowMessage(BL, "Readkey problem %w", err)
		}

		var k int
		if key.Regular != 0 {
			k = int(key.Regular)
		} else {
			k = key.Special
		}

		if a.Session.editorMode {
			// Use our new context-based method
			//textChange := app.Editor.ProcessKey(app, k) // This is where the main loop will call the new method in editor_context.go
			textChange := p.editorProcessKey(k)

			if !a.Session.editorMode {
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
			if a.Screen.divider > 10 {
				a.Organizer.drawStatusBar()
			}
		}
		a.returnCursor()
	}
	
	// Clean up when the main loop exits
	a.Cleanup()
}
