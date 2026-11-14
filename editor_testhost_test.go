package main

import (
	"fmt"
	"testing"

	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// testHost is who opened a test's editor. It stands where the application
// does: it has the notice box the editor shows :help and listings in, and it
// is told when the editor has closed its note. It is not a screen and has no
// organizer, so a test that passes here passes with neither.
type testHost struct {
	shown  []string // what the editor put in its notice box
	keys   []int    // the keys the editor offered the box while it was up
	closed int      // times the editor said its note was closed
}

// showNotice records the text and writes it out, as the screen's box draws
// what it is shown: a test reads what the editor showed from what was written.
func (h *testHost) showNotice(text string, hang int) {
	h.shown = append(h.shown, text)
	fmt.Print(text)
}

// noticeKey takes the keys the screen's box scrolls with, and no others.
func (h *testHost) noticeKey(c int) (string, bool) {
	h.keys = append(h.keys, c)
	switch c {
	case ' ', 'b', 'j' & 0x1f, 'k' & 0x1f, terminal.KeyHome, terminal.KeyPageDown, terminal.KeyPageUp:
		return "", true
	}
	return "", false
}

// open reports whether the editor still has its note: it has not said it closed.
func (h *testHost) open() bool { return h.closed == 0 }

// hostOf is the host a test's editor was made with.
func hostOf(e *Editor) *testHost { return e.notices.(*testHost) }

// Two layouts for tests, the two kinds vimango gives (Screen.editorLayout).
// tuiLayout is the right pane of a terminal 40 rows by 100 columns, from
// column 41: a border above the text, a status bar below it, and what is
// said beside the note under the organizer. hostAppLayout is the same room
// as a host app's window: the text from row 1, no status bar, one message
// line.
var (
	tuiLayout = editorLayout{
		top_margin: 2, left_margin: 40, screenlines: 30, screencols: 50,
		statusBar: true, corner: true,
		message: messageLine{row: 33, col: 41, width: 60},
		aside:   messageLine{row: 33, col: 1, width: 40},
	}
	hostAppLayout = editorLayout{
		top_margin: 1, left_margin: 40, screenlines: 31, screencols: 59,
		message: messageLine{row: 33, col: 41, width: 60},
		aside:   messageLine{row: 33, col: 41, width: 60},
	}
)

// newTestEditor builds the smallest Editor editorProcessKey will drive: a real
// vim buffer holding lines, both command tables, a testHost and the TUI's
// layout. The note as loaded is the saved state, as it is when one is opened.
//
// It is a bare editor, not newEditor's: no line numbers and no highlighting,
// so that a test's columns are the text's.
func newTestEditor(t *testing.T, lines ...string) *Editor {
	t.Helper()
	vim.InitializeVim(0)
	h := &testHost{}
	e := &Editor{editorHost: editorHost{
		notices:    h,
		onClose:    func() { h.closed++ },
		syncAnswer: "sync is not run from here",
	}}
	e.editorLayout = tuiLayout
	e.commandRegistry = editorCommands()
	e.exCmds = e.commandRegistry.GetFunctionMap()
	// vim is a process-wide singleton, so a previous test may have left it in
	// INSERT or VISUAL; start every editor from NORMAL
	vim.SendKey("<esc>")
	e.vbuf = vim.NewBuffer(0)
	vim.SetCurrentBuffer(e.vbuf)
	e.vbuf.SetLines(0, -1, lines)
	vim.SetCursorPosition(1, 0)
	e.ss = e.vbuf.Lines()
	e.saveTick = e.vbuf.GetLastChangedTick()
	e.mode = NORMAL
	vim.SetScreenLayout(e)
	return e
}

func (e *Editor) sendKeys(s string) {
	for _, r := range s {
		e.editorProcessKey(int(r))
	}
}

func (e *Editor) line(n int) string { return e.vbuf.Lines()[n] }

func (e *Editor) exCommand(s string) {
	e.sendKeys(":" + s)
	e.editorProcessKey('\r')
}
