package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/slzatz/vimango/internal/pane"
	"github.com/slzatz/vimango/internal/store"
	"github.com/slzatz/vimango/vim"
)

// editorLayout is where the editor draws and what it draws there, in the
// terminal's rows and columns (counted from 1). The editor works none of it
// out: whoever opens it sets the layout, and sets it again when the terminal
// changes size (Screen.editorLayout, Screen.positionEditor). So the editor
// does not know which program it is in -- the TUI's right pane, or the whole
// of a window a host app gave it -- and nothing in its files asks.
type editorLayout struct {
	top_margin  int  // the row of the first line of text
	left_margin int  // the column of the border left of the text, which starts in the next
	screenlines int  // lines of text
	screencols  int  // columns of text, line numbers included
	statusBar   bool // the note's id and title, on the row under the text
	corner      bool // the frame's top right corner, on the row above the text

	// message is the editor's message line, and its command line (ShowMessage). aside is
	// where it says what is not about the text under the cursor ("Saved note
	// 5", how far a notice has scrolled; ShowAside): under the organizer in the TUI, and
	// the message line again where there is nothing beside the editor.
	message, aside messageLine
}

// messageLine is one row's worth of room for a message.
type messageLine struct {
	row, col int // where it starts
	width    int // the columns it may use
}

// lineNumberCols is the room the line numbers take at the left of the text
// (left_margin_offset, while they are on).
const lineNumberCols = 4

// sgrReset ends whatever colours and attributes are set.
const sgrReset = "\x1b[0m"

// note that there isn't a columnOffset because currently only word wrap supported

type Editor struct {
	editorLayout                 // where it draws: the host's to set
	editorHost                   // what else it is given
	cx, cy             int       // cursor position in the text area: column and line, from 0
	fc, fr             int       // file cursor: fc is a byte offset into ss[fr], as libvim reports it; fr is the 0-based row
	lineOffset         int       //first row based on user scroll
	left_margin_offset int       // 0 if no line numbers
	highlight          [2][2]int // [line col][line col] -> note line is 1-based not zero-based
	mode               Mode
	vmode              Mode
	command_line       string //for commands on the command line; string doesn't include ':'
	exLine             string // the ex command line being carried out, range and all (runExLine)
	cmdLineCursor      int    //byte offset of the cursor within command_line (EX_COMMAND: mirrors vim's cmdline position)
	firstVisibleRow    int
	highlightSyntax    bool
	numberLines        bool
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
	title         string   // title of the note
	tabCompletion struct { // for tab completion
		list  []string
		index int
	}
	exCmds          map[string]func(*Editor) // map of ex commands
	commandRegistry *pane.CommandRegistry[func(*Editor)]
}

// editorHost is what an editor is given by whoever opens it: everything it
// uses that is not its own. With the layout, which the host sets and sets
// again as the terminal changes, this is all an editor knows of the program
// it is in -- the TUI, or a host app's window (App.editorHost).
type editorHost struct {
	Store *store.Store // the database the note is read from and written to
	// notices is the box the editor shows :help and vim's listings in. The
	// box is the screen's; the editor is given this much of it.
	notices noticeBox
	// markdownStyle is the colours the note's markdown is drawn in (highlightChunk).
	markdownStyle *chroma.Style
	// onClose is what follows the editor closing its note (closeNote): the
	// TUI goes back to the organizer, --editor ends the process
	// (App.noteClosed). The editor itself knows of neither.
	onClose func()
	// syncAnswer is what :sync is answered with. Sync is never the editor's
	// to run; whose it is depends on who opened the editor (noSync).
	syncAnswer string
}

// newEditor is an editor with no note in it: its settings as they start, and
// its ex commands.
func newEditor(host editorHost) *Editor {
	e := &Editor{
		editorHost:         host,
		mode:               NORMAL,
		highlightSyntax:    true,
		numberLines:        true,
		left_margin_offset: lineNumberCols, // room for the line numbers
	}
	e.commandRegistry = editorCommands()
	e.exCmds = e.commandRegistry.GetFunctionMap()
	return e
}

// openNote makes an editor on a note: the note's text, read from the host's
// store into a vim buffer of its own, which becomes vim's current one. title
// is what the editor shows as the note's; "" is the note's title in the
// database. An error means the note could not be read, and there is no editor
// -- one made on an empty buffer would look like an empty note, and saving it
// would make it one.
//
// The editor has no layout yet and has drawn nothing: where it goes is the
// host's to say (Screen.positionEditor), and then to ask for (Editor.draw).
func openNote(host editorHost, id int, title string) (*Editor, error) {
	note, err := host.Store.Note(id)
	if err != nil {
		return nil, err
	}
	if title == "" {
		title, _ = host.Store.Title(id)
	}

	e := newEditor(host)
	e.id = id
	e.title = title
	e.dbText = note
	e.ss = strings.Split(note, "\n")
	e.vbuf = vim.NewBuffer(0)
	vim.SetCurrentBuffer(e.vbuf)
	e.vbuf.SetLines(0, -1, e.ss)
	e.bufferTick = e.vbuf.GetLastChangedTick()
	e.saveTick = e.bufferTick
	vim.SetScreenLayout(e) // gj, gk, H, M, L
	return e, nil
}

// noticeBox is what the editor needs of the box that text other than the
// note's is shown in, over the note: to put text in it, and to give it the
// keys that scroll it while it is up (HELP mode). The screen has the box
// (notice.go), and draws it where the editor's text is; what the editor says
// about the scrolling goes on its own message line.
type noticeBox interface {
	showNotice(text string, hang int)
	noticeKey(c int) (message string, handled bool)
}

// ShowMessage writes a message on the editor's message line, in place of the
// last one. ShowAside writes one beside the note, for what is not about the
// text under the cursor. See editorLayout for where each is.
func (e *Editor) ShowMessage(format string, a ...interface{}) {
	e.writeMessage(e.message, format, a...)
}

func (e *Editor) ShowAside(format string, a ...interface{}) {
	e.writeMessage(e.aside, format, a...)
}

func (e *Editor) writeMessage(line messageLine, format string, a ...interface{}) {
	// to the start of the line, and erase the whole of it
	fmt.Printf("\x1b[%d;%dH\x1b[%dX", line.row, line.col, line.width)
	// by columns: cutting at a byte count split multi-byte characters, and
	// counted the bytes of an escape sequence ("-- INSERT --" is sent bold)
	str, _ := pane.ClipColumns(fmt.Sprintf(format, a...), 0, line.width)
	fmt.Print(str)
}

// cursor is where the terminal's cursor belongs while the editor has the
// keys: in the text, or on the command line being typed. show is false when
// no cursor should be seen (a spelling suggestion is being chosen).
func (e *Editor) cursor() (row, col int, show bool) {
	switch e.mode {
	case SPELLING:
		return 0, 0, false
	case SEARCH, EX_COMMAND:
		// vim's place on its command line (mirrored in cmdLineCursor),
		// after the ':' or '/' the line starts with
		cur := min(e.cmdLineCursor, len(e.command_line))
		return e.message.row, e.message.col + 1 + utf8.RuneCountInString(e.command_line[:cur]), true
	}
	return e.cy + e.top_margin, e.cx + e.left_margin + e.left_margin_offset + 1, true
}
