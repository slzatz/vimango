package main

import (
	"fmt"

	"github.com/slzatz/vimango/internal/store"
	"github.com/slzatz/vimango/vim"
)

// note that there isn't a columnOffset because currently only word wrap supported

type Editor struct {
	cx, cy             int //screen cursor x and y position
	fc, fr             int // file cursor: fc is a byte offset into ss[fr], as libvim reports it; fr is the 0-based row
	lineOffset         int //first row based on user scroll
	screenlines        int //number of lines for this Editor
	screencols         int //number of columns for this Editor
	left_margin        int //can vary (so could TOP_MARGIN - will do that later
	left_margin_offset int // 0 if no line numbers
	top_margin         int
	highlight          [2][2]int // [line col][line col] -> note line is 1-based not zero-based
	mode               Mode
	vmode              Mode
	command_line       string //for commands on the command line; string doesn't include ':'
	exLine             string // the ex command line being carried out, range and all (runExLine)
	cmdLineCursor      int    //byte offset of the cursor within command_line (EX_COMMAND: mirrors vim's cmdline position)
	firstVisibleRow    int
	highlightSyntax    bool
	numberLines        bool
	redraw             bool
	id                 int //db id of entry
	//output             *Output
	vbuf         *vim.Buffer
	ss           []string
	searchPrefix string
	suggestions  []string            // spelling suggestions offered in SPELLING mode
	spellKeys    string              // last NORMAL keys typed, to spot zg/zw (spellListKey)
	searchDrawn  string              // vim's search state when the text was last drawn (searchState)
	heldKeyShown bool                // a key vim is holding is drawn at the cursor (drawAfterKey)
	screenWant   screenWant          // where gj/gk left the cursor, and the column wanted (screen_motion.go)
	hlCache      map[string][]string // highlighted rows by chunk text (highlightChunk)
	hlCacheBytes int
	bufferTick   int
	saveTick     int
	// dbText is the note exactly as this editor last read it from — or
	// wrote it to — the database. It is the baseline :checkstale compares
	// against, which is what separates "the row changed in the database
	// since I read it" (news) from "I have unsaved edits" (not news). Set
	// wherever a note is loaded into an editor or written back.
	dbText        string
	modified      bool     // tracks if the buffer has been modified
	title         string   // title of the note
	tabCompletion struct { // for tab completion
		list  []string
		index int
	}
	exCmds          map[string]func(*Editor) // map of ex commands
	commandRegistry *CommandRegistry[func(*Editor)]
	Store           *store.Store // the database
	// onClose is what follows the editor closing its note (closeNote), and
	// is whoever opened the editor's to say: the TUI goes back to the
	// organizer, --editor ends the process (App.noteClosed). The editor
	// itself knows of neither.
	onClose func()
	Session *Session // pointer to the session
	Screen  *Screen  // pointer to the screen
}

func (e *Editor) ShowMessage(loc Location, format string, a ...interface{}) { //Sesseion struct
	// editor-only mode: there is no organizer pane, so the BL message
	// area is divider(=1) columns wide — a BL message renders as a single
	// stray character at column 1 of the cmdline row. Route everything to
	// the editor's own (full-width) BR area instead.
	if e.Session.editorOnly {
		loc = BR
	}
	max_length := e.Screen.PositionMessage(loc)
	// by columns: cutting at a byte count split multi-byte characters, and
	// counted the bytes of an escape sequence ("-- INSERT --" is sent bold)
	str, _ := clipColumns(fmt.Sprintf(format, a...), 0, max_length)
	fmt.Print(str)
}
