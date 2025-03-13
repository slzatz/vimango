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
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
	//"github.com/alecthomas/chroma/v2"
)

var sess Session
var org = Organizer{Session: &sess}
var p *Editor
var windows []Window
var config *dbConfig
var db *sql.DB
var fts_db *sql.DB

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
	vim.Init(0)
	var err error
	config, err = FromFile("config.json")
	if err != nil {
		log.Fatal(err)
	}

	if _, err := os.Stat(config.Sqlite3.DB); err == nil {
		db, err = sql.Open("sqlite3", config.Sqlite3.DB)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec("PRAGMA foreign_keys=ON;")
	if err != nil {
		log.Fatalf("PRAGMA foreign_keys=ON: %v", err)
	}
	//PRAGMA busy_timeout=5000
	//PRAGMA journal_mode=WAL

	if _, err := os.Stat(config.Sqlite3.DB); err == nil {
		fts_db, err = sql.Open("sqlite3", config.Sqlite3.FTS_DB)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal(err)
	}
	defer fts_db.Close()


  // create markdown syntax highlighting style
  //markdown_style, _ := chroma.NewStyle("markdown", chroma.StyleEntries{
	//	chroma.Background: "bg:#ffffff",
	//})
  markdown_style, _ := selectMDStyle("gruvbox.xml")

  sess.markdown_style = markdown_style

	sess.style = [8]string{"dracula", "fruity", "gruvbox", "monokai", "native", "paraiso-dark", "rrt", "solarized-dark256"} //vim is dark but unusable
	sess.styleIndex = 2
	sess.imagePreview = false
	sess.imgSizeY = 800

	signal_chan := make(chan os.Signal, 1)
	signal.Notify(signal_chan, syscall.SIGWINCH)

	go func() {
		for {
			_ = <-signal_chan
			sess.signalHandler()
		}
	}()
	// parse config flags & parameters
	//flag.Parse()

	vim.Execute("set iskeyword+=*")
	vim.Execute("set iskeyword+=`")

	// enable raw mode
	origCfg, err := rawmode.Enable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error enabling raw mode: %v", err)
		os.Exit(1)
	}

	sess.origTermCfg = origCfg

	sess.editorMode = false

	err = sess.GetWindowSize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting window size: %v", err)
		os.Exit(1)
	}

	org.cx = 0               //cursor x position
	org.cy = 0               //cursor y position
	org.fc = 0               //file x position
	org.fr = 0               //file y position
	org.rowoff = 0           //number of rows scrolled off the screen
	org.coloff = 0           //col the user is currently scrolled to
	org.sort = "modified"    //Entry sort column
	org.show_deleted = false //not treating these separately right now
	org.show_completed = true
	org.message = "" //displayed at the bottom of screen; ex. -- INSERT --
	org.highlight[0], org.highlight[1] = -1, -1
	org.mode = NORMAL
	org.last_mode = NORMAL
	org.view = TASK
	if config.Options.Type == "folder" {
		org.taskview = BY_FOLDER
	} else {
		org.taskview = BY_CONTEXT
	}
	org.filter = config.Options.Title
	org.marked_entries = make(map[int]struct{})
	org.vbuf = vim.BufferNew(0)
	vim.BufferSetCurrent(org.vbuf)

	// ? where this should be.  Also in signal.
	sess.textLines = sess.screenLines - 2 - TOP_MARGIN // -2 for status bar and message bar
	sess.edPct = 60
	moveDividerPct(sess.edPct)                                // sets sess.divider
	sess.totaleditorcols = sess.screenCols - sess.divider - 1 // was 2
	sess.eraseScreenRedrawLines()
	org.rows = filterEntries(org.taskview, org.filter, org.show_deleted, org.sort, org.sortPriority, MAX)
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
	//vim.WindowSetHeight(sess.textLines)
	/*
		sess.showOrgMessage("rows: %d  columns: %d; win height %d win width %d",
			sess.screenLines, sess.screenCols, vim.WindowGetHeight(), vim.WindowGetWidth())
	*/
	sess.showOrgMessage("rows: %d  columns: %d", sess.screenLines, sess.screenCols)
	sess.returnCursor()
	sess.run = true

	for sess.run {

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

		// if it's been 5 secs since the last status message, reset
		//if time.Now().Sub(sess.StatusMessageTime) > time.Second*5 && sess.State == stateEditing {
		//	sess.setStatusMessage("")
		//}
	}
	sess.quitApp()
}
