package main

import (
	"fmt"

	"github.com/slzatz/vimango/vim/interfaces"
)

// note that there isn't a columnOffset because currently only word wrap supported

type Editor struct {
	cx, cy             int //screen cursor x and y position
	fc, fr             int // file cursor x and y position
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
	cmdLineCursor      int    //byte offset of the cursor within command_line (EX_COMMAND: mirrors vim's cmdline position)
	command            string // right now includes normal mode commands and command line commands
	last_command       string
	firstVisibleRow    int
	highlightSyntax    bool
	numberLines        bool
	redraw             bool
	id                 int //db id of entry
	//output             *Output
	vbuf               interfaces.VimBuffer
	ss                 []string
	searchPrefix       string
	renderedNote       string
	previewLineOffset  int
	overlay            []string // for suggest, showVimMessageLog
	highlightPositions []Position
	suggestions        []string //spelling suggestions
	bufferTick         int
	saveTick           int
	// dbText is the note exactly as this editor last read it from — or
	// wrote it to — the database. It is the baseline :checkstale compares
	// against, which is what separates "the row moved underneath me"
	// (news) from "I have unsaved edits" (not news). Set wherever a note
	// is loaded into an editor or written back.
	dbText string
	modified           bool     // tracks if the buffer has been modified
	title              string   // title of the note
	tabCompletion      struct { // for tab completion
		list  []string
		index int
	}
	normalCmds            map[string]func(*Editor, int) // map of normal commands
	exCmds                map[string]func(*Editor)      // map of ex commands
	commandRegistry       *CommandRegistry[func(*Editor)]
	normalCommandRegistry *CommandRegistry[func(*Editor, int)]
	Database              *Database // pointer to the database
	Session               *Session  // pointer to the session
	Screen                *Screen   // pointer to the screen
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
	str := fmt.Sprintf(format, a...)
	if len(str) > max_length {
		str = str[:max_length]
	}
	fmt.Print(str)
}
