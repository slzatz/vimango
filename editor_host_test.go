package main

import (
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/localdb"
	"github.com/slzatz/vimango/vim"
)

// recordedNotices is a notice box that is not a screen: it keeps what the
// editor shows in it and the keys the editor offers it, and scrolls on Space.
type recordedNotices struct {
	shown []string
	keys  []int
}

func (n *recordedNotices) showNotice(text string, hang int) { n.shown = append(n.shown, text) }

func (n *recordedNotices) noticeKey(c int) (string, bool) {
	n.keys = append(n.keys, c)
	return "", c == ' '
}

// An editor is made from what its host gives it (editorHost) and a layout,
// and from nothing else: here there is no App, no Screen and no Session, and
// the global is nil. It opens its note, draws itself, shows :help in the box
// it was given, writes to the store it was given, reloads after :checkstale,
// answers :sync as it was told to, and says when it has closed.
func TestEditorIsMadeFromItsHost(t *testing.T) {
	prev := app
	app = nil
	t.Cleanup(func() { app = prev })
	vim.InitializeVim(0)
	vim.SendKey("<esc>")

	db := newTestStore(t)
	id, err := db.Add("a note", false, localdb.DefaultContextUUID, localdb.DefaultFolderUUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetNote(id, "alpha\nbravo"); err != nil {
		t.Fatal(err)
	}
	if _, err := openNote(editorHost{Store: db}, id+1000, ""); err == nil {
		t.Error("a note that is not there was opened")
	}

	notices := &recordedNotices{}
	closed := 0
	e, err := openNote(editorHost{
		Store:      db,
		notices:    notices,
		onClose:    func() { closed++ },
		syncAnswer: "sync is somebody else's",
	}, id, "")
	if err != nil {
		t.Fatal(err)
	}
	if e.title != "a note" || e.dbText != "alpha\nbravo" || e.isModified() {
		t.Errorf("opened %q holding %q, modified %v", e.title, e.dbText, e.isModified())
	}
	if vim.GetCurrentBuffer().GetID() != e.vbuf.GetID() {
		t.Error("vim is not on the note's buffer")
	}

	// a layout, as a host would set it: the text from row 3, a status bar
	line := messageLine{row: 14, col: 6, width: 40}
	e.editorLayout = editorLayout{top_margin: 3, left_margin: 5, screenlines: 10, screencols: 40, statusBar: true, message: line, aside: line}
	drawn := captureStdout(t, e.draw)
	if !strings.Contains(drawn, "\x1b[3;6H") || !strings.Contains(stripANSI(drawn), "alpha") || !strings.Contains(drawn, "a note") {
		t.Errorf("the editor drew %q", drawn)
	}

	// :help and a listing go to the box, and its keys are offered to it
	captureStdout(t, func() { e.exCommand("help") })
	if e.mode != HELP || len(notices.shown) != 1 || !strings.Contains(notices.shown[0], "checkstale") {
		t.Fatalf(":help: mode %v, %d notices shown", e.mode, len(notices.shown))
	}
	captureStdout(t, func() { e.editorProcessKey(' ') })
	if e.mode != HELP || len(notices.keys) != 1 || notices.keys[0] != ' ' {
		t.Errorf("Space under :help: mode %v, keys offered to the box %v", e.mode, notices.keys)
	}
	captureStdout(t, func() {
		e.editorProcessKey('\x1b')
		e.sendKeys("ma")
		e.exCommand("marks")
		e.editorProcessKey('\x1b')
	})
	if len(notices.shown) != 2 {
		t.Errorf(":marks: %d notices shown, want a second", len(notices.shown))
	}

	if msg := exMessage(t, e, "sync"); !strings.Contains(msg, "sync is somebody else's") {
		t.Errorf(":sync said %q", msg)
	}

	// :w writes to the store it was given
	captureStdout(t, func() {
		e.sendKeys("x")
		e.exCommand("w")
	})
	if text, _ := db.Note(id); text != "lpha\nbravo" || e.isModified() {
		t.Errorf("after x and :w the database holds %q (modified %v)", text, e.isModified())
	}

	// a change underneath is read from it and drawn, with the new title
	if _, err := db.DB().Exec("UPDATE task SET note='newer', title='renamed' WHERE id=?;", id); err != nil {
		t.Fatal(err)
	}
	reloaded := captureStdout(t, func() { e.exCommand("checkstale") })
	if e.line(0) != "newer" || !strings.Contains(stripANSI(reloaded), "newer") || !strings.Contains(reloaded, "renamed") {
		t.Errorf(":checkstale: the buffer is %q, drawn %q", e.line(0), reloaded)
	}

	// and closing is said, once, when the text is safe
	captureStdout(t, func() {
		e.sendKeys("x")
		e.exCommand("q")
	})
	if closed != 0 {
		t.Error(":q closed a modified note")
	}
	captureStdout(t, func() { e.exCommand("x") })
	if text, _ := db.Note(id); closed != 1 || text != "ewer" {
		t.Errorf(":x: closed %d times, the database holds %q", closed, text)
	}
}
