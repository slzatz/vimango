package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	//"github.com/neovim/go-client/nvim"

	"github.com/slzatz/vimango/rawmode"
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

var sess Session
var org = Organizer{Session: &sess}
var p *Editor

//var editors []*Editor

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
	db, _ = sql.Open("sqlite3", config.Sqlite3.DB)
	fts_db, _ = sql.Open("sqlite3", config.Sqlite3.FTS_DB)

	sess.style = [7]string{"dracula", "fruity", "monokai", "native", "paraiso-dark", "rrt", "solarized-dark256"} //vim is dark but unusable
	sess.styleIndex = 2
	sess.imagePreview = false //image preview
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
	flag.Parse()

	// initialize neovim server
	/*
		ctx := context.Background()
		opts := []nvim.ChildProcessOption{

			// -u NONE is no vimrc and -n is no swap file
			nvim.ChildProcessArgs("-u", "NONE", "-n", "--embed", "--headless", "--noplugin"),

			//without headless nothing happens but should be OK once ui attached.
			//nvim.ChildProcessArgs("-u", "NONE", "-n", "--embed", "--noplugin"),

			nvim.ChildProcessContext(ctx),
			nvim.ChildProcessLogf(log.Printf),
		}

		os.Setenv("VIMRUNTIME", "/home/slzatz/neovim/runtime")
		opts = append(opts, nvim.ChildProcessCommand("/home/slzatz/neovim/build/bin/nvim"))

		//var err error
		v, err = nvim.NewChildProcess(opts...)
		if err != nil {
			log.Fatal(err)
		}
	*/

	// Cleanup on return.
	/*
		defer v.Close()

		wins, err := v.Windows()
		if err != nil {
			fmt.Printf("%v\n", err)
		}
		w = wins[0]
	*/

	// this allows you to change current buffer without saving
	// isModified still works when hidden is true
	/*
		err = v.SetOption("hidden", true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error setting all buffers to hidden: %v", err)
			os.Exit(1)
		}
	*/

	vim.Execute("set iskeyword+=*")
	/*
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in set iskeyword: %v", err)
			os.Exit(1)
		}
	*/
	vim.Execute("set iskeyword+=`")
	/*
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error in set iskeyword: %v", err)
				os.Exit(1)
			}
		redirectMessages(v)
		messageBuf, _ = v.CreateBuffer(true, true)
	*/

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
	org.command = ""
	org.command_line = ""
	org.repeat = 0 //number of times to repeat commands like x,s,yy ? also used for visual line mode x,y
	org.view = TASK
	org.taskview = BY_FOLDER
	org.filter = "todo"
	org.context_map = make(map[string]int)
	org.folder_map = make(map[string]int)
	org.marked_entries = make(map[int]struct{})
	org.keywordMap = make(map[string]int)
	org.vbuf = vim.BufferNew(0)    ///////////////////////////////////////////////////////
	vim.BufferSetCurrent(org.vbuf) ///////////////////////////////////////////////////

	// ? where this should be.  Also in signal.
	sess.textLines = sess.screenLines - 2 - TOP_MARGIN // -2 for status bar and message bar
	//sess.divider = sess.screencols - sess.cfg.ed_pct * sess.screencols/100
	sess.divider = sess.screenCols - (60 * sess.screenCols / 100)
	sess.totaleditorcols = sess.screenCols - sess.divider - 1 // was 2

	generateContextMap()
	generateFolderMap()
	generateKeywordMap()
	sess.eraseScreenRedrawLines()
	org.rows = filterEntries(org.taskview, org.filter, org.show_deleted, org.sort, MAX)
	if len(org.rows) == 0 {
		sess.showOrgMessage("No results were returned")
		org.mode = NO_ROWS
	}
	org.drawPreview()
	org.refreshScreen()
	org.drawStatusBar()
	sess.showOrgMessage("rows: %d  columns: %d", sess.screenLines, sess.screenCols)
	sess.returnCursor()
	sess.run = true

	err = os.RemoveAll("temp")
	if err != nil {
		sess.showOrgMessage("Error deleting temp directory: %v", err)
	}
	err = os.Mkdir("temp", 0700)
	if err != nil {
		sess.showOrgMessage("Error creating temp directory: %v", err)
	}
	vim.Execute("w temp/title")

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
