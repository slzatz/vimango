package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/slzatz/vimango/internal/editor"
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// Keys and pastes all the way: from the key reader's channel, through the
// event loop, into the note and the database.

// The whole path, through the event loop: a key that begins a mapping, and
// then nothing. The loop's timer must end the wait by itself. "j" is mapped
// as well as "jk", so the timeout (mappings on) gives X where the <esc> that
// follows (mappings off) would have given a plain j.
func TestEventLoopTimesOutAHalfTypedMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("waits on a timer")
	}
	a, ids := editorOnlyApp(t, "alpha")
	a.keyEvents = make(chan terminal.Key, 16)
	vim.ExecuteCommand("mapclear | mapclear!")
	vim.ExecuteCommand("inoremap j X")  // not joined with "|": the space
	vim.ExecuteCommand("inoremap jk Y") // before it would be part of the X
	vim.ExecuteCommand("set timeout timeoutlen=40")
	t.Cleanup(func() {
		vim.ExecuteCommand("mapclear | mapclear!")
		vim.ExecuteCommand("set timeout timeoutlen=1000")
	})

	captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		go func() {
			for _, r := range "ij" {
				a.keyEvents <- terminal.Key{Regular: r}
			}
			time.Sleep(600 * time.Millisecond) // 'timeoutlen' passes
			for _, r := range "\x1b:x\r" {
				a.keyEvents <- terminal.Key{Regular: r}
			}
		}()
		a.handleEvents() // :x ends it: --editor has one note
	})

	if text, err := a.Store.Note(ids[0]); err != nil || text != "Xalpha" {
		t.Errorf("the note is %q (%v), want \"Xalpha\": the timeout did not fire", text, err)
	}
}

// The whole way: a paste from the key reader, through the event loop, into
// the note.
func TestEventLoopPastes(t *testing.T) {
	a, ids := editorOnlyApp(t, "- foo")
	a.keyEvents = make(chan terminal.Key, 64)
	vim.ExecuteCommand("mapclear | mapclear!")
	editor.SourceVimrc()
	t.Cleanup(func() { vim.ExecuteCommand("mapclear | mapclear!") })

	captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		a.keyEvents <- terminal.Key{Regular: 'A'}
		a.keyEvents <- terminal.Key{Special: terminal.KeyPaste, Paste: "\r- bar\r  - baz"}
		for _, r := range "\x1b:x\r" {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		a.handleEvents() // :x ends it: --editor has one note
	})

	if text, err := a.Store.Note(ids[0]); err != nil || text != "- foo\n- bar\n  - baz" {
		t.Errorf("the note is %q (%v)", text, err)
	}
}

// A mapping that types itself again never ends: the event loop is inside
// libvim for good. A Ctrl-C the key reader reads meanwhile stops it, the
// message line says so, and the note goes on as before -- here an x and :x,
// which save it.
func TestCtrlCInterruptsACommandThatNeverEnds(t *testing.T) {
	a, ids := editorOnlyApp(t, "alpha\nbravo")
	a.keyEvents = make(chan terminal.Key, 16)
	vim.ExecuteCommand("mapclear | mapclear!")
	vim.ExecuteCommand("nmap Q jkQ")
	t.Cleanup(func() { vim.ExecuteCommand("mapclear | mapclear!") })

	ctrlCKey := terminal.Key{Regular: ctrlC}
	if a.readerTakes(ctrlCKey) {
		t.Fatal("a Ctrl-C with nothing running was not passed on")
	}

	took := make(chan bool, 1)
	out := captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		a.keyEvents <- terminal.Key{Regular: 'Q'}
		go func() {
			time.Sleep(200 * time.Millisecond) // the loop is inside Q
			took <- a.readerTakes(ctrlCKey)
			for _, r := range "gg0x:x\r" {
				a.keyEvents <- terminal.Key{Regular: r}
			}
		}()
		a.handleEvents() // :x ends it: --editor has one note
	})

	if !<-took {
		t.Error("the reader passed on a Ctrl-C while a command ran")
	}
	if !strings.Contains(out, "Interrupted") {
		t.Error("the message line did not say Interrupted")
	}
	if strings.Contains(out, ":qa") {
		t.Error("the interrupt was taken as a typed Ctrl-C")
	}
	if text, err := a.Store.Note(ids[0]); err != nil || text != "lpha\nbravo" {
		t.Errorf("the note is %q (%v), want \"lpha\\nbravo\"", text, err)
	}
}

// ":s" with the c flag asks about each match in the middle of the command:
// the loop is inside the Enter that ran it, and takes the answers itself
// (waitKey). Keys typed are answers, in order; text pasted is not one; and
// when the last match is answered the loop goes on as before -- here :x,
// which saves what was accepted.
func TestEventLoopAnswersSubstituteConfirm(t *testing.T) {
	a, ids := editorOnlyApp(t, "foo bar foo\nfoo\nfoo")
	a.keyEvents = make(chan terminal.Key, 64)

	out := captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		for _, r := range ":%s/foo/X/gc\ry" {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		a.keyEvents <- terminal.Key{Special: terminal.KeyPaste, Paste: "yyyy"}
		for _, r := range "nyq:x\r" {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		a.handleEvents() // :x ends it: --editor has one note
	})

	if text, err := a.Store.Note(ids[0]); err != nil || text != "X bar foo\nX\nfoo" {
		t.Errorf("the note is %q (%v), want \"X bar foo\\nX\\nfoo\"", text, err)
	}
	if n := strings.Count(out, "replace with X (y/n/a/q/l)?"); n != 4 {
		t.Errorf("the question was written %d times, want 4", n)
	}
}

// While the question stands a Ctrl-C is not an interrupt: nothing is running
// that one could stop, the key reader passes it on, and it answers -- the
// command stops there, with what was accepted kept, and is not reported as
// interrupted.
func TestCtrlCAnswersSubstituteConfirm(t *testing.T) {
	a, ids := editorOnlyApp(t, "foo\nfoo\nfoo")
	a.keyEvents = make(chan terminal.Key, 64)
	ctrlCKey := terminal.Key{Regular: ctrlC}

	took := make(chan bool, 1)
	out := captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		for _, r := range ":%s/foo/X/c\ry" {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		go func() {
			time.Sleep(200 * time.Millisecond) // the second question stands
			taken := a.readerTakes(ctrlCKey)
			took <- taken
			if !taken {
				a.keyEvents <- ctrlCKey
			}
			for _, r := range ":x\r" {
				a.keyEvents <- terminal.Key{Regular: r}
			}
		}()
		a.handleEvents() // :x ends it: --editor has one note
	})

	if <-took {
		t.Error("the reader kept a Ctrl-C typed at the question")
	}
	if text, err := a.Store.Note(ids[0]); err != nil || text != "X\nfoo\nfoo" {
		t.Errorf("the note is %q (%v), want \"X\\nfoo\\nfoo\"", text, err)
	}
	if strings.Contains(out, "Interrupted") {
		t.Error("the command was reported as interrupted")
	}
}

// The question can stand for as long as the user likes, and what was
// accepted before it is unsaved text like any other: the recovery file is
// written a second after, and at once when a signal asks, while the loop
// waits for the answer.
func TestRecoveryFileIsKeptWhileSubstituteAsks(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the one-second write")
	}
	a, ids := editorOnlyApp(t, "foo\nfoo\nfoo")
	a.recoveryDir = t.TempDir()
	a.keyEvents = make(chan terminal.Key, 64)
	a.recoveryRequests = make(chan chan struct{})
	path := editor.RecoveryPath(a.recoveryDir, ids[0], os.Getpid())
	kept := func() string {
		data, _ := os.ReadFile(path)
		var f struct{ Text string }
		json.Unmarshal(data, &f)
		return f.Text
	}
	typed := func(s string) {
		for _, r := range s {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		for len(a.keyEvents) > 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		ended := make(chan struct{})
		go func() {
			a.handleEvents()
			close(ended)
		}()

		typed(":%s/foo/X/c\ry") // the first is changed; the second is asked about
		time.Sleep(1500 * time.Millisecond)
		if text := kept(); text != "X\nfoo\nfoo" {
			t.Errorf("a second into the question the recovery file holds %q, want \"X\\nfoo\\nfoo\"", text)
		}

		typed("y") // the second; the third is asked about
		if !a.flushRecovery(time.Second) {
			t.Error("the loop did not answer a flush while the question stood")
		}
		if text := kept(); text != "X\nX\nfoo" {
			t.Errorf("flushed during the question: the recovery file holds %q, want \"X\\nX\\nfoo\"", text)
		}

		typed("q:q!\r")
		<-ended
	})
}

// A resize while the question stands goes through the app's own redraw (what
// handleResize does once it has the terminal's size), from inside the wait:
// the question and the match are drawn again, vim is sent nothing, and the
// answers that follow are taken as before.
func TestResizeWhileSubstituteAsks(t *testing.T) {
	a, ids := editorOnlyApp(t, "foo\nfoo")
	a.keyEvents = make(chan terminal.Key, 64)
	vim.TakeRefused()

	resized := false
	out := captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		a.Session.editor.WaitKey = func() int {
			if !resized {
				resized = true
				a.Screen.screenLines, a.Screen.screenCols = 30, 60
				a.moveDividerPct(a.Screen.edPct)
			}
			return a.waitKey()
		}
		for _, r := range ":%s/foo/X/c\ryy:x\r" {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		a.handleEvents() // :x ends it: --editor has one note
	})

	if text, err := a.Store.Note(ids[0]); err != nil || text != "X\nX" {
		t.Errorf("the note is %q (%v), want \"X\\nX\"", text, err)
	}
	// asked about the first match, drawn again after the resize, and asked
	// about the second
	if n := strings.Count(out, "replace with X (y/n/a/q/l)?"); n != 3 {
		t.Errorf("the question was written %d times, want 3", n)
	}
	// the editor says so, when the command is over, if anything was
	if i := strings.Index(out, "vimango note"); i >= 0 {
		t.Errorf("the resize sent vim something while it was asking: %q", stripANSI(out[i:]))
	}
}
