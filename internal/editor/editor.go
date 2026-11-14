// Package editor is vimango's note editor: one note, a row of the database,
// edited with vim (libvim, through package vim) and drawn on a terminal with
// escape sequences.
//
// It is the same editor in the TUI's right pane and in the window a host app
// gives it (vimango --editor, which is how vimango_hybrid runs it), and it
// does not know which: it does not import the application. Whoever opens an
// editor gives it a Host -- the database, a box to show notices in, what
// follows its closing -- and a Layout, and then hands it keys:
//
//	e, err := editor.Open(host, id, "")
//	e.Layout = ...            // and again whenever the terminal changes size
//	e.Draw()
//	redraw := e.ProcessKey(k) // or e.PasteText(text)
//	e.DrawAfterKey(redraw, k)
//	row, col, show := e.Cursor()
//
// There is one vim in a process, so there is one editor at a time; InitSpell,
// SourceVimrc and RestoreRegisters set that vim up at startup, and
// SaveRegisters saves its registers at the end.
package editor

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/slzatz/vimango/internal/pane"
	"github.com/slzatz/vimango/internal/store"
	"github.com/slzatz/vimango/vim"
)

// Layout is where the editor draws and what it draws there, in the
// terminal's rows and columns (counted from 1). The editor works none of it
// out: whoever opens it sets the layout, and sets it again when the terminal
// changes size (in vimango, Screen.editorLayout and Screen.positionEditor).
// So the editor does not know which program it is in -- the TUI's right pane,
// or the whole of a window a host app gave it -- and nothing here asks.
type Layout struct {
	TopMargin   int  // the row of the first line of text
	LeftMargin  int  // the column of the border left of the text, which starts in the next
	ScreenLines int  // lines of text
	ScreenCols  int  // columns of text, line numbers included
	StatusBar   bool // the note's id and title, on the row under the text
	Corner      bool // the frame's top right corner, on the row above the text

	// Message is the editor's message line, and its command line
	// (ShowMessage). Aside is where it says what is not about the text under
	// the cursor ("Saved note 5", how far a notice has scrolled; ShowAside):
	// under the organizer in the TUI, and the message line again where there
	// is nothing beside the editor.
	Message, Aside MessageLine
}

// MessageLine is one row's worth of room for a message.
type MessageLine struct {
	Row, Col int // where it starts
	Width    int // the columns it may use
}

// lineNumberCols is the room the line numbers take at the left of the text
// (left_margin_offset, while they are on).
const lineNumberCols = 4

// sgrReset ends whatever colours and attributes are set.
const sgrReset = "\x1b[0m"

// note that there isn't a columnOffset because currently only word wrap supported

type Editor struct {
	Layout                       // where it draws: the host's to set
	Host                         // what else it is given
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
	heldKeyShown bool                // a key vim is holding is drawn at the cursor (DrawAfterKey)
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

// Host is what an editor is given by whoever opens it: everything it uses
// that is not its own. With the layout, which the host sets and sets again as
// the terminal changes, this is all an editor knows of the program it is in
// -- the TUI, or a host app's window (in vimango, App.editorHost).
type Host struct {
	Store *store.Store // the database the note is read from and written to
	// Notices is the box the editor shows :help and vim's listings in. The
	// box is the screen's; the editor is given this much of it.
	Notices NoticeBox
	// MarkdownStyle is the colours the note's markdown is drawn in (highlightChunk).
	MarkdownStyle *chroma.Style
	// OnClose is what follows the editor closing its note (closeNote): the
	// TUI goes back to the organizer, --editor ends the process (in
	// vimango, App.noteClosed). The editor itself knows of neither.
	OnClose func()
	// SyncAnswer is what :sync is answered with. Sync is never the editor's
	// to run; whose it is depends on who opened the editor (noSync).
	SyncAnswer string
}

// newEditor is an editor with no note in it: its settings as they start, and
// its ex commands.
func newEditor(host Host) *Editor {
	e := &Editor{
		Host:               host,
		mode:               NORMAL,
		highlightSyntax:    true,
		numberLines:        true,
		left_margin_offset: lineNumberCols, // room for the line numbers
	}
	e.commandRegistry = editorCommands()
	e.exCmds = e.commandRegistry.GetFunctionMap()
	return e
}

// Open makes an editor on a note: the note's text, read from the host's
// store into a vim buffer of its own, which becomes vim's current one. title
// is what the editor shows as the note's; "" is the note's title in the
// database. An error means the note could not be read, and there is no editor
// -- one made on an empty buffer would look like an empty note, and saving it
// would make it one.
//
// The editor has no layout yet and has drawn nothing: where it goes is the
// host's to say (Layout), and then to ask for (Draw).
func Open(host Host, id int, title string) (*Editor, error) {
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

// NoticeBox is what the editor needs of the box that text other than the
// note's is shown in, over the note: to put text in it, and to give it the
// keys that scroll it while it is up (HELP mode). In vimango the screen has
// the box (notice.go), and draws it where the editor's text is; what the
// editor says about the scrolling goes on its own message line.
type NoticeBox interface {
	ShowNotice(text string, hang int)
	NoticeKey(c int) (message string, handled bool)
}

// What an editor says of itself to whoever opened it: the id of its note,
// the title it shows for it, what it is doing with keys, and whether the
// buffer holds changes that are not in the database.
func (e *Editor) ID() int        { return e.id }
func (e *Editor) Title() string  { return e.title }
func (e *Editor) Mode() Mode     { return e.mode }
func (e *Editor) Modified() bool { return e.isModified() }

// ShowMessage writes a message on the editor's message line, in place of the
// last one. ShowAside writes one beside the note, for what is not about the
// text under the cursor. See Layout for where each is.
func (e *Editor) ShowMessage(format string, a ...interface{}) {
	e.writeMessage(e.Message, format, a...)
}

func (e *Editor) ShowAside(format string, a ...interface{}) {
	e.writeMessage(e.Aside, format, a...)
}

func (e *Editor) writeMessage(line MessageLine, format string, a ...interface{}) {
	// to the start of the line, and erase the whole of it
	fmt.Printf("\x1b[%d;%dH\x1b[%dX", line.Row, line.Col, line.Width)
	// by columns: cutting at a byte count split multi-byte characters, and
	// counted the bytes of an escape sequence ("-- INSERT --" is sent bold)
	str, _ := pane.ClipColumns(fmt.Sprintf(format, a...), 0, line.Width)
	fmt.Print(str)
}

// Cursor is where the terminal's cursor belongs while the editor has the
// keys: in the text, or on the command line being typed. show is false when
// no cursor should be seen (a spelling suggestion is being chosen).
func (e *Editor) Cursor() (row, col int, show bool) {
	switch e.mode {
	case SPELLING:
		return 0, 0, false
	case SEARCH, EX_COMMAND:
		// vim's place on its command line (mirrored in cmdLineCursor),
		// after the ':' or '/' the line starts with
		cur := min(e.cmdLineCursor, len(e.command_line))
		return e.Message.Row, e.Message.Col + 1 + utf8.RuneCountInString(e.command_line[:cur]), true
	}
	return e.cy + e.TopMargin, e.cx + e.LeftMargin + e.left_margin_offset + 1, true
}
